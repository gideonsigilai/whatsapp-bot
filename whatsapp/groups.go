package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"wa-server-go/storage"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// groupInfoToMap converts a whatsmeow GroupInfo into a JSON friendly map.
func groupInfoToMap(g *types.GroupInfo) map[string]interface{} {
	if g == nil {
		return nil
	}
	participants := make([]map[string]interface{}, 0, len(g.Participants))
	for _, p := range g.Participants {
		participants = append(participants, map[string]interface{}{
			"jid":          jidString(p.JID),
			"phoneNumber":  jidString(p.PhoneNumber),
			"lid":          jidString(p.LID),
			"isAdmin":      p.IsAdmin,
			"isSuperAdmin": p.IsSuperAdmin,
			"displayName":  p.DisplayName,
		})
	}
	return map[string]interface{}{
		"id":                g.JID.User,
		"jid":               g.JID.String(),
		"name":              g.Name,
		"topic":             g.Topic,
		"owner":             jidString(g.OwnerJID),
		"created":           g.GroupCreated.UTC().Format(time.RFC3339),
		"participantCount":  len(g.Participants),
		"participants":      participants,
		"isAnnounce":        g.IsAnnounce,
		"isLocked":          g.IsLocked,
		"isEphemeral":       g.IsEphemeral,
		"disappearingTimer": g.DisappearingTimer,
		"isCommunity":       g.IsParent,
		"linkedParent":      jidString(g.LinkedParentJID),
		"memberAddMode":     string(g.MemberAddMode),
		"joinApproval":      g.IsJoinApprovalRequired,
	}
}

// GetGroupInfo fetches full metadata for a single group.
func GetGroupInfo(userId, groupId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.GetGroupInfo(ctx, jid)
	if err != nil {
		return nil, err
	}
	return groupInfoToMap(info), nil
}

// CreateGroup creates a new group with the given name and participant phone numbers.
func CreateGroup(userId, name string, participants []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("group name is required")
	}
	jids, err := parseJIDList(participants)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.CreateGroup(ctx, whatsmeow.ReqCreateGroup{
		Name:         name,
		Participants: jids,
	})
	if err != nil {
		return nil, err
	}
	storage.IncrementStatUser(userId, "groupsJoined")
	return groupInfoToMap(info), nil
}

// UpdateParticipants adds, removes, promotes or demotes group members.
func UpdateParticipants(userId, groupId, action string, participants []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	jids, err := parseJIDList(participants)
	if err != nil {
		return nil, err
	}

	var change whatsmeow.ParticipantChange
	switch strings.ToLower(action) {
	case "add":
		change = whatsmeow.ParticipantChangeAdd
	case "remove":
		change = whatsmeow.ParticipantChangeRemove
	case "promote":
		change = whatsmeow.ParticipantChangePromote
	case "demote":
		change = whatsmeow.ParticipantChangeDemote
	default:
		return nil, fmt.Errorf("action must be one of add, remove, promote, demote")
	}

	ctx, cancel := opCtx()
	defer cancel()
	result, err := cli.UpdateGroupParticipants(ctx, jid, jids, change)
	if err != nil {
		return nil, err
	}

	out := make([]map[string]interface{}, 0, len(result))
	for _, p := range result {
		out = append(out, map[string]interface{}{
			"jid":     jidString(p.JID),
			"error":   p.Error,
			"isAdmin": p.IsAdmin,
		})
	}
	return map[string]interface{}{"success": true, "action": action, "participants": out}, nil
}

// SetGroupName updates a group's subject/name.
func SetGroupName(userId, groupId, name string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetGroupName(ctx, jid, name); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "name": name}, nil
}

// SetGroupTopic updates a group's topic/description.
func SetGroupTopic(userId, groupId, topic string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetGroupTopic(ctx, jid, "", "", topic); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "topic": topic}, nil
}

// SetGroupPhoto sets (or, with an empty source, removes) a group's picture. Returns the new picture id.
func SetGroupPhoto(userId, groupId, source string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	var avatar []byte
	if strings.TrimSpace(source) != "" {
		avatar, _, err = resolveMedia(source)
		if err != nil {
			return nil, err
		}
	}
	ctx, cancel := opCtx()
	defer cancel()
	pictureID, err := cli.SetGroupPhoto(ctx, jid, avatar)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "pictureId": pictureID}, nil
}

// SetGroupAnnounce toggles announce mode (only admins can send messages).
func SetGroupAnnounce(userId, groupId string, announce bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetGroupAnnounce(ctx, jid, announce); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "announce": announce}, nil
}

// SetGroupLocked toggles locked mode (only admins can edit group info).
func SetGroupLocked(userId, groupId string, locked bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetGroupLocked(ctx, jid, locked); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "locked": locked}, nil
}

// GetGroupInviteLink returns the group's invite link, optionally resetting (revoking) the old one.
func GetGroupInviteLink(userId, groupId string, reset bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	link, err := cli.GetGroupInviteLink(ctx, jid, reset)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "link": link, "reset": reset}, nil
}

// GetGroupInfoFromLink resolves an invite link to group metadata without joining.
func GetGroupInfoFromLink(userId, link string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if link == "" {
		return nil, fmt.Errorf("link is required")
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.GetGroupInfoFromLink(ctx, link)
	if err != nil {
		return nil, err
	}
	return groupInfoToMap(info), nil
}

// GetGroupRequestParticipants lists pending join requests for a group.
func GetGroupRequestParticipants(userId, groupId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	reqs, err := cli.GetGroupRequestParticipants(ctx, jid)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(reqs))
	for _, r := range reqs {
		out = append(out, map[string]interface{}{
			"jid":         jidString(r.JID),
			"requestedAt": r.RequestedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

// UpdateGroupRequestParticipants approves or rejects pending join requests.
func UpdateGroupRequestParticipants(userId, groupId, action string, participants []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	jids, err := parseJIDList(participants)
	if err != nil {
		return nil, err
	}
	var change whatsmeow.ParticipantRequestChange
	switch strings.ToLower(action) {
	case "approve":
		change = whatsmeow.ParticipantChangeApprove
	case "reject":
		change = whatsmeow.ParticipantChangeReject
	default:
		return nil, fmt.Errorf("action must be approve or reject")
	}
	ctx, cancel := opCtx()
	defer cancel()
	result, err := cli.UpdateGroupRequestParticipants(ctx, jid, jids, change)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(result))
	for _, p := range result {
		out = append(out, map[string]interface{}{"jid": jidString(p.JID), "error": p.Error})
	}
	return map[string]interface{}{"success": true, "action": action, "participants": out}, nil
}

// SetGroupMemberAddMode controls who may add members ("admin_add" or "all_member_add").
func SetGroupMemberAddMode(userId, groupId, mode string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	var addMode types.GroupMemberAddMode
	switch strings.ToLower(mode) {
	case "admin_add", "admin":
		addMode = types.GroupMemberAddModeAdmin
	case "all_member_add", "all", "all_member":
		addMode = types.GroupMemberAddModeAllMember
	default:
		return nil, fmt.Errorf("mode must be admin_add or all_member_add")
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetGroupMemberAddMode(ctx, jid, addMode); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "mode": string(addMode)}, nil
}

// SetGroupDisappearingTimer sets the disappearing message timer (seconds; 0 disables).
func SetGroupDisappearingTimer(userId, groupId string, seconds uint32) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(groupId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetDisappearingTimer(ctx, jid, time.Duration(seconds)*time.Second, time.Now()); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "seconds": seconds}, nil
}

// GetSubGroups lists the subgroups of a community.
func GetSubGroups(userId, communityId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	jid, err := parseGroupJID(communityId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	subs, err := cli.GetSubGroups(ctx, jid)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(subs))
	for _, s := range subs {
		out = append(out, map[string]interface{}{
			"jid":          jidString(s.JID),
			"name":         s.Name,
			"isDefaultSub": s.IsDefaultSubGroup,
		})
	}
	return out, nil
}

// LinkGroup links a child group into a parent community.
func LinkGroup(userId, parentId, childId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	parent, err := parseGroupJID(parentId)
	if err != nil {
		return nil, err
	}
	child, err := parseGroupJID(childId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.LinkGroup(ctx, parent, child); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}

// UnlinkGroup removes a child group from a parent community.
func UnlinkGroup(userId, parentId, childId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	parent, err := parseGroupJID(parentId)
	if err != nil {
		return nil, err
	}
	child, err := parseGroupJID(childId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.UnlinkGroup(ctx, parent, child); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true}, nil
}
