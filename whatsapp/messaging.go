package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"wa-server-go/storage"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"google.golang.org/protobuf/proto"
)

// dispatchMessage sends a user-visible message, records it in the user's log,
// bumps the sent counter and broadcasts it over the websocket hub.
func dispatchMessage(userId string, to types.JID, msg *waProto.Message, preview string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	resp, err := cli.SendMessage(ctx, to, msg)
	if err != nil {
		return nil, err
	}

	isGroup := to.Server == types.GroupServer
	messageData := map[string]interface{}{
		"id":          string(resp.ID),
		"chat":        to.String(),
		"from":        "me",
		"to":          to.String(),
		"body":        preview,
		"timestamp":   resp.Timestamp.UTC().Format(time.RFC3339),
		"type":        "sent",
		"contactName": to.User,
		"isGroup":     isGroup,
		"groupName":   nil,
	}

	storage.PushToUserMessage(userId, messageData)
	storage.IncrementStatUser(userId, "messagesSent")
	Broadcast(userId, "message", messageData)

	return messageData, nil
}

// sendControl sends a non-logged protocol/control message (reaction, edit, revoke, ...).
func sendControl(userId string, to types.JID, msg *waProto.Message) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()

	resp, err := cli.SendMessage(ctx, to, msg)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"id":        string(resp.ID),
		"to":        to.String(),
		"timestamp": resp.Timestamp.UTC().Format(time.RFC3339),
	}, nil
}

// ── Media messages ──

// SendImage uploads and sends an image. source may be a URL, data URI or base64 string.
func SendImage(userId, to, source, caption string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	data, mime, err := resolveMedia(source)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mime, "image/") {
		mime = "image/jpeg"
	}

	ctx, cancel := opCtx()
	defer cancel()
	up, err := cli.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	msg := &waProto.Message{ImageMessage: &waProto.ImageMessage{
		Caption:       proto.String(caption),
		Mimetype:      proto.String(mime),
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}
	return dispatchMessage(userId, jid, msg, previewCaption("📷 Image", caption))
}

// SendVideo uploads and sends a video (set gifPlayback for animated/GIF-style playback).
func SendVideo(userId, to, source, caption string, gifPlayback bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	data, mime, err := resolveMedia(source)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mime, "video/") {
		mime = "video/mp4"
	}

	ctx, cancel := opCtx()
	defer cancel()
	up, err := cli.Upload(ctx, data, whatsmeow.MediaVideo)
	if err != nil {
		return nil, err
	}

	msg := &waProto.Message{VideoMessage: &waProto.VideoMessage{
		Caption:       proto.String(caption),
		GifPlayback:   proto.Bool(gifPlayback),
		Mimetype:      proto.String(mime),
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}
	return dispatchMessage(userId, jid, msg, previewCaption("🎬 Video", caption))
}

// SendAudio uploads and sends an audio clip. Set ptt=true for a push-to-talk voice note.
func SendAudio(userId, to, source string, ptt bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	data, mime, err := resolveMedia(source)
	if err != nil {
		return nil, err
	}
	if !strings.HasPrefix(mime, "audio/") {
		if ptt {
			mime = "audio/ogg; codecs=opus"
		} else {
			mime = "audio/mpeg"
		}
	}

	ctx, cancel := opCtx()
	defer cancel()
	up, err := cli.Upload(ctx, data, whatsmeow.MediaAudio)
	if err != nil {
		return nil, err
	}

	msg := &waProto.Message{AudioMessage: &waProto.AudioMessage{
		PTT:           proto.Bool(ptt),
		Mimetype:      proto.String(mime),
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}
	label := "🎵 Audio"
	if ptt {
		label = "🎤 Voice note"
	}
	return dispatchMessage(userId, jid, msg, label)
}

// SendDocument uploads and sends an arbitrary file as a document.
func SendDocument(userId, to, source, filename, caption, mimetype string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	data, detected, err := resolveMedia(source)
	if err != nil {
		return nil, err
	}
	mime := mimetype
	if mime == "" {
		mime = detected
	}
	if mime == "" {
		mime = "application/octet-stream"
	}
	if filename == "" {
		filename = "file"
	}

	ctx, cancel := opCtx()
	defer cancel()
	up, err := cli.Upload(ctx, data, whatsmeow.MediaDocument)
	if err != nil {
		return nil, err
	}

	msg := &waProto.Message{DocumentMessage: &waProto.DocumentMessage{
		Title:         proto.String(filename),
		FileName:      proto.String(filename),
		Caption:       proto.String(caption),
		Mimetype:      proto.String(mime),
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}
	return dispatchMessage(userId, jid, msg, previewCaption("📄 "+filename, caption))
}

// SendSticker uploads and sends a sticker (ideally a WebP image).
func SendSticker(userId, to, source string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	data, mime, err := resolveMedia(source)
	if err != nil {
		return nil, err
	}
	if mime == "" {
		mime = "image/webp"
	}

	ctx, cancel := opCtx()
	defer cancel()
	up, err := cli.Upload(ctx, data, whatsmeow.MediaImage)
	if err != nil {
		return nil, err
	}

	msg := &waProto.Message{StickerMessage: &waProto.StickerMessage{
		Mimetype:      proto.String(mime),
		URL:           proto.String(up.URL),
		DirectPath:    proto.String(up.DirectPath),
		MediaKey:      up.MediaKey,
		FileEncSHA256: up.FileEncSHA256,
		FileSHA256:    up.FileSHA256,
		FileLength:    proto.Uint64(up.FileLength),
	}}
	return dispatchMessage(userId, jid, msg, "🩹 Sticker")
}

// ── Rich content messages ──

// SendLocation sends a static location pin.
func SendLocation(userId, to string, latitude, longitude float64, name, address string) (interface{}, error) {
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	msg := &waProto.Message{LocationMessage: &waProto.LocationMessage{
		DegreesLatitude:  proto.Float64(latitude),
		DegreesLongitude: proto.Float64(longitude),
		Name:             proto.String(name),
		Address:          proto.String(address),
	}}
	return dispatchMessage(userId, jid, msg, "📍 Location")
}

// SendContact sends a contact card. Either a full vCard or a name + phone pair may be supplied.
func SendContact(userId, to, displayName, phone, vcard string) (interface{}, error) {
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if vcard == "" {
		if displayName == "" || phone == "" {
			return nil, fmt.Errorf("either vcard, or both displayName and phone, are required")
		}
		vcard = fmt.Sprintf("BEGIN:VCARD\nVERSION:3.0\nFN:%s\nTEL;type=CELL;type=VOICE;waid=%s:+%s\nEND:VCARD",
			displayName, normalizeNumber(phone), normalizeNumber(phone))
	}
	if displayName == "" {
		displayName = "Contact"
	}
	msg := &waProto.Message{ContactMessage: &waProto.ContactMessage{
		DisplayName: proto.String(displayName),
		Vcard:       proto.String(vcard),
	}}
	return dispatchMessage(userId, jid, msg, "👤 "+displayName)
}

// SendPoll creates a poll. selectableCount is how many options a voter may pick (defaults to 1).
func SendPoll(userId, to, name string, options []string, selectableCount int) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if name == "" || len(options) < 2 {
		return nil, fmt.Errorf("a poll requires a name and at least two options")
	}
	if selectableCount <= 0 {
		selectableCount = 1
	}
	msg := cli.BuildPollCreation(name, options, selectableCount)
	return dispatchMessage(userId, jid, msg, "📊 Poll: "+name)
}

// ── Replies, reactions, edits, deletions ──

// ReplyMessage sends a text reply quoting a previous message.
func ReplyMessage(userId, to, messageId, participant, text, quotedText string) (interface{}, error) {
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if messageId == "" || text == "" {
		return nil, fmt.Errorf("messageId and text are required")
	}

	sender := jid
	if participant != "" {
		if sender, err = parseRecipient(participant); err != nil {
			return nil, err
		}
	}

	ctxInfo := &waProto.ContextInfo{
		StanzaID:    proto.String(messageId),
		Participant: proto.String(sender.String()),
	}
	if quotedText != "" {
		ctxInfo.QuotedMessage = &waProto.Message{Conversation: proto.String(quotedText)}
	}

	msg := &waProto.Message{ExtendedTextMessage: &waProto.ExtendedTextMessage{
		Text:        proto.String(text),
		ContextInfo: ctxInfo,
	}}
	return dispatchMessage(userId, jid, msg, text)
}

// SendReaction reacts to a message with an emoji. Pass an empty emoji to remove a reaction.
func SendReaction(userId, to, messageId, participant, emoji string, fromMe bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	chat, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if messageId == "" {
		return nil, fmt.Errorf("messageId is required")
	}

	sender, err := reactionSender(cli, chat, participant, fromMe)
	if err != nil {
		return nil, err
	}

	msg := cli.BuildReaction(chat, sender, types.MessageID(messageId), emoji)
	return sendControl(userId, chat, msg)
}

// EditMessage replaces the text of a previously sent message.
func EditMessage(userId, to, messageId, newText string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	chat, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if messageId == "" || newText == "" {
		return nil, fmt.Errorf("messageId and newText are required")
	}
	newContent := &waProto.Message{Conversation: proto.String(newText)}
	msg := cli.BuildEdit(chat, types.MessageID(messageId), newContent)
	return sendControl(userId, chat, msg)
}

// RevokeMessage deletes a message for everyone in the chat.
func RevokeMessage(userId, to, messageId, participant string, fromMe bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	chat, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	if messageId == "" {
		return nil, fmt.Errorf("messageId is required")
	}
	sender, err := reactionSender(cli, chat, participant, fromMe)
	if err != nil {
		return nil, err
	}
	msg := cli.BuildRevoke(chat, sender, types.MessageID(messageId))
	return sendControl(userId, chat, msg)
}

// reactionSender resolves the JID that originally sent the target message.
func reactionSender(cli *whatsmeow.Client, chat types.JID, participant string, fromMe bool) (types.JID, error) {
	if fromMe {
		return ownJID(cli), nil
	}
	if participant != "" {
		return parseRecipient(participant)
	}
	// In a 1:1 chat the sender is the chat itself; in a group the caller should
	// supply the participant. Fall back to the chat JID.
	return chat, nil
}

// ── Receipts & presence ──

// MarkRead marks one or more messages in a chat as read (blue ticks).
func MarkRead(userId, chat, sender string, messageIds []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	chatJID, err := parseRecipient(chat)
	if err != nil {
		return nil, err
	}
	senderJID := chatJID
	if sender != "" {
		if senderJID, err = parseRecipient(sender); err != nil {
			return nil, err
		}
	}
	if len(messageIds) == 0 {
		return nil, fmt.Errorf("at least one messageId is required")
	}
	ids := make([]types.MessageID, len(messageIds))
	for i, id := range messageIds {
		ids[i] = types.MessageID(id)
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.MarkRead(ctx, ids, time.Now(), chatJID, senderJID); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "marked": len(ids)}, nil
}

// SetPresence sets the account-wide presence: "available" (online) or "unavailable" (offline).
func SetPresence(userId, presence string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	state := types.PresenceAvailable
	if strings.EqualFold(presence, "unavailable") || strings.EqualFold(presence, "offline") {
		state = types.PresenceUnavailable
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SendPresence(ctx, state); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "presence": string(state)}, nil
}

// SetChatPresence sets a typing/recording indicator in a specific chat.
// state is "composing" or "paused"; media is "" (text) or "audio" (recording).
func SetChatPresence(userId, to, state, media string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseRecipient(to)
	if err != nil {
		return nil, err
	}
	chatState := types.ChatPresenceComposing
	if strings.EqualFold(state, "paused") || strings.EqualFold(state, "stop") {
		chatState = types.ChatPresencePaused
	}
	chatMedia := types.ChatPresenceMediaText
	if strings.EqualFold(media, "audio") || strings.EqualFold(media, "recording") {
		chatMedia = types.ChatPresenceMediaAudio
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SendChatPresence(ctx, jid, chatState, chatMedia); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "state": string(chatState)}, nil
}

// SubscribePresence subscribes to online/offline + last-seen updates for a user.
func SubscribePresence(userId, jid string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseRecipient(jid)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SubscribePresence(ctx, target); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "jid": target.String()}, nil
}

func previewCaption(label, caption string) string {
	if caption == "" {
		return label
	}
	return label + ": " + caption
}
