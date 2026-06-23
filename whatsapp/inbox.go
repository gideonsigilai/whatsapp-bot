package whatsapp

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"wa-server-go/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// ── Inbox / conversation view ──
//
// The inbox is derived from the per-user message log: messages are grouped by
// their conversation JID ("chat"), the most recent message of each conversation
// is surfaced, and the list is sorted newest-first — much like the chat list in
// WhatsApp or the inbox of an email client. Optionally each conversation can be
// enriched with the contact/group profile picture, name and "about" text.

func msgStr(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func msgBool(m map[string]interface{}, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}

func jidUserPart(jid string) string {
	return strings.SplitN(jid, "@", 2)[0]
}

// chatKey returns the conversation key for a stored message, with a fallback for
// older records that predate the "chat" field.
func chatKey(m map[string]interface{}) string {
	if c := msgStr(m, "chat"); c != "" {
		return c
	}
	if msgStr(m, "type") == "sent" {
		return msgStr(m, "to")
	}
	return msgStr(m, "from")
}

type conversation struct {
	chat    string
	isGroup bool
	name    string
	last    map[string]interface{}
	lastTS  time.Time
	count   int
}

// GetInbox returns the list of conversations ordered by most recent activity.
// When enrich is true (and a client is connected) each conversation is augmented
// with a profile picture, resolved name and "about"/topic, best-effort.
func GetInbox(userId string, enrich bool, limit int) (interface{}, error) {
	data := storage.LoadUser(userId)

	convs := map[string]*conversation{}
	for _, raw := range data.Messages {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		key := chatKey(m)
		if key == "" {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, msgStr(m, "timestamp"))
		c := convs[key]
		if c == nil {
			c = &conversation{
				chat:    key,
				isGroup: msgBool(m, "isGroup") || strings.HasSuffix(key, "@"+types.GroupServer),
			}
			convs[key] = c
		}
		c.count++
		if c.last == nil || !ts.Before(c.lastTS) {
			c.last = m
			c.lastTS = ts
		}
		// Prefer a received message's push name as the contact display name;
		// sent messages only carry the raw number. Group names are resolved on
		// enrichment, so fall back to the group id here.
		if !c.isGroup && msgStr(m, "type") == "received" {
			if cn := msgStr(m, "contactName"); cn != "" {
				c.name = cn
			}
		}
	}

	list := make([]*conversation, 0, len(convs))
	for _, c := range convs {
		if c.name == "" {
			c.name = jidUserPart(c.chat)
		}
		list = append(list, c)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].lastTS.After(list[j].lastTS) })
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}

	out := make([]map[string]interface{}, len(list))
	for i, c := range list {
		out[i] = map[string]interface{}{
			"chat":         c.chat,
			"isGroup":      c.isGroup,
			"name":         c.name,
			"messageCount": c.count,
			"lastMessage": map[string]interface{}{
				"id":        msgStr(c.last, "id"),
				"body":      msgStr(c.last, "body"),
				"timestamp": msgStr(c.last, "timestamp"),
				"type":      msgStr(c.last, "type"),
				"fromMe":    msgStr(c.last, "type") == "sent",
				"hasMedia":  msgBool(c.last, "hasMedia"),
				"mediaType": msgStr(c.last, "mediaType"),
			},
		}
	}

	if enrich {
		enrichConversations(userId, out)
	}
	return out, nil
}

// enrichConversations fetches profile picture / name / about for each conversation
// concurrently (bounded, best-effort). It silently no-ops if no client is connected.
func enrichConversations(userId string, convs []map[string]interface{}) {
	cli, err := requireClient(userId)
	if err != nil {
		return
	}

	sem := make(chan struct{}, 6)
	var wg sync.WaitGroup
	for _, c := range convs {
		wg.Add(1)
		go func(c map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			jid, err := parseRecipient(msgStr(c, "chat"))
			if err != nil {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()

			if pic, err := cli.GetProfilePictureInfo(ctx, jid, &whatsmeow.GetProfilePictureParams{Preview: true}); err == nil && pic != nil {
				c["profilePicture"] = pic.URL
			}

			if isGroup, _ := c["isGroup"].(bool); isGroup {
				if info, err := cli.GetGroupInfo(ctx, jid); err == nil && info != nil {
					if info.Name != "" {
						c["name"] = info.Name
					}
					c["about"] = info.Topic
					c["participantCount"] = len(info.Participants)
				}
			} else {
				if infos, err := cli.GetUserInfo(ctx, []types.JID{jid}); err == nil {
					for _, ui := range infos {
						c["about"] = ui.Status
						break
					}
				}
				if contact, err := cli.Store.Contacts.GetContact(ctx, jid); err == nil && contact.Found {
					if name := contact.FullName; name != "" {
						c["name"] = name
					} else if contact.PushName != "" {
						c["name"] = contact.PushName
					}
				}
			}
		}(c)
	}
	wg.Wait()
}

// GetConversation returns the stored messages for a single conversation, oldest
// first, limited to the most recent `limit` messages.
func GetConversation(userId, chat string, limit int) (interface{}, error) {
	if chat == "" {
		return nil, fmt.Errorf("chat is required")
	}
	target := chat
	if jid, err := parseRecipient(chat); err == nil {
		target = jid.String()
	}

	data := storage.LoadUser(userId)
	msgs := make([]map[string]interface{}, 0)
	for _, raw := range data.Messages {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		key := chatKey(m)
		if key == target || key == chat || msgStr(m, "from") == target || msgStr(m, "to") == target {
			msgs = append(msgs, m)
		}
	}
	sort.Slice(msgs, func(i, j int) bool {
		ti, _ := time.Parse(time.RFC3339, msgStr(msgs[i], "timestamp"))
		tj, _ := time.Parse(time.RFC3339, msgStr(msgs[j], "timestamp"))
		return ti.Before(tj)
	})
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

// ConversationActions returns the actions available for a conversation, so a UI
// can render the right per-contact / per-group action menu.
func ConversationActions(chat string) (interface{}, error) {
	jid, err := parseRecipient(chat)
	if err != nil {
		return nil, err
	}
	if jid.Server == types.GroupServer {
		return map[string]interface{}{
			"chat": jid.String(),
			"type": "group",
			"actions": []map[string]interface{}{
				{"id": "exit", "label": "Exit group", "params": []string{}},
				{"id": "set_name", "label": "Change name", "params": []string{"name"}},
				{"id": "set_description", "label": "Change description", "params": []string{"description"}},
				{"id": "set_photo", "label": "Change group picture", "params": []string{"image"}},
				{"id": "announce", "label": "Admins-only messages", "params": []string{"announce"}},
				{"id": "lock", "label": "Admins-only info editing", "params": []string{"locked"}},
				{"id": "invite_link", "label": "Get invite link", "params": []string{"reset"}},
				{"id": "add", "label": "Add members", "params": []string{"participants"}},
				{"id": "remove", "label": "Remove members", "params": []string{"participants"}},
				{"id": "promote", "label": "Promote to admin", "params": []string{"participants"}},
				{"id": "demote", "label": "Demote admin", "params": []string{"participants"}},
			},
		}, nil
	}
	return map[string]interface{}{
		"chat": jid.String(),
		"type": "contact",
		"actions": []map[string]interface{}{
			{"id": "block", "label": "Block contact", "params": []string{}},
			{"id": "unblock", "label": "Unblock contact", "params": []string{}},
			{"id": "subscribe_presence", "label": "Subscribe to presence", "params": []string{}},
		},
	}, nil
}

// RunConversationAction executes an action against a contact or group.
func RunConversationAction(userId, chat, action string, params map[string]interface{}) (interface{}, error) {
	jid, err := parseRecipient(chat)
	if err != nil {
		return nil, err
	}
	isGroup := jid.Server == types.GroupServer
	gid := jid.User

	getStr := func(keys ...string) string {
		for _, k := range keys {
			if v, ok := params[k].(string); ok && v != "" {
				return v
			}
		}
		return ""
	}
	getBool := func(k string) bool { v, _ := params[k].(bool); return v }
	getList := func(k string) []string {
		out := []string{}
		if arr, ok := params[k].([]interface{}); ok {
			for _, e := range arr {
				if s, ok := e.(string); ok {
					out = append(out, s)
				}
			}
		}
		return out
	}

	switch strings.ToLower(action) {
	case "block":
		return SetBlocked(userId, chat, true)
	case "unblock":
		return SetBlocked(userId, chat, false)
	case "subscribe_presence":
		return SubscribePresence(userId, chat)
	}

	if !isGroup {
		return nil, fmt.Errorf("action %q is only valid for groups", action)
	}

	switch strings.ToLower(action) {
	case "exit", "leave":
		return LeaveGroup(userId, gid)
	case "set_name":
		return SetGroupName(userId, gid, getStr("name"))
	case "set_description", "set_topic":
		return SetGroupTopic(userId, gid, getStr("description", "topic"))
	case "set_photo", "set_picture":
		return SetGroupPhoto(userId, gid, getStr("image", "photo"))
	case "announce":
		return SetGroupAnnounce(userId, gid, getBool("announce"))
	case "lock":
		return SetGroupLocked(userId, gid, getBool("locked"))
	case "invite_link":
		return GetGroupInviteLink(userId, gid, getBool("reset"))
	case "add", "remove", "promote", "demote":
		return UpdateParticipants(userId, gid, strings.ToLower(action), getList("participants"))
	default:
		return nil, fmt.Errorf("unknown action %q", action)
	}
}
