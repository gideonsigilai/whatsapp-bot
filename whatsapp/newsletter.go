package whatsapp

import (
	"fmt"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
)

// newsletterToMap converts NewsletterMetadata into a JSON friendly map.
func newsletterToMap(n *types.NewsletterMetadata) map[string]interface{} {
	if n == nil {
		return nil
	}
	m := map[string]interface{}{
		"id":              n.ID.String(),
		"state":           string(n.State.Type),
		"name":            n.ThreadMeta.Name.Text,
		"description":     n.ThreadMeta.Description.Text,
		"subscriberCount": n.ThreadMeta.SubscriberCount,
		"inviteCode":      n.ThreadMeta.InviteCode,
		"verification":    string(n.ThreadMeta.VerificationState),
	}
	if n.ViewerMeta != nil {
		m["mute"] = string(n.ViewerMeta.Mute)
		m["role"] = string(n.ViewerMeta.Role)
	}
	return m
}

// GetSubscribedNewsletters lists all channels the account follows.
func GetSubscribedNewsletters(userId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	list, err := cli.GetSubscribedNewsletters(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(list))
	for _, n := range list {
		out = append(out, newsletterToMap(n))
	}
	return out, nil
}

// GetNewsletterInfoData fetches metadata about a single channel the account follows.
func GetNewsletterInfoData(userId, jid string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseNewsletterJID(jid)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.GetNewsletterInfo(ctx, target)
	if err != nil {
		return nil, err
	}
	return newsletterToMap(info), nil
}

// GetNewsletterInfoFromInvite resolves a channel invite link/key to metadata.
func GetNewsletterInfoFromInvite(userId, key string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if key == "" {
		return nil, fmt.Errorf("invite key is required")
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.GetNewsletterInfoWithInvite(ctx, key)
	if err != nil {
		return nil, err
	}
	return newsletterToMap(info), nil
}

// CreateNewsletter creates a new WhatsApp channel.
func CreateNewsletter(userId, name, description string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.CreateNewsletter(ctx, whatsmeow.CreateNewsletterParams{
		Name:        name,
		Description: description,
	})
	if err != nil {
		return nil, err
	}
	return newsletterToMap(info), nil
}

// FollowNewsletter joins a channel.
func FollowNewsletter(userId, jid string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseNewsletterJID(jid)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.FollowNewsletter(ctx, target); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "jid": target.String()}, nil
}

// UnfollowNewsletter leaves a channel.
func UnfollowNewsletter(userId, jid string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseNewsletterJID(jid)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.UnfollowNewsletter(ctx, target); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "jid": target.String()}, nil
}

// ToggleNewsletterMute mutes or unmutes a channel.
func ToggleNewsletterMute(userId, jid string, mute bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseNewsletterJID(jid)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.NewsletterToggleMute(ctx, target, mute); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "jid": target.String(), "muted": mute}, nil
}
