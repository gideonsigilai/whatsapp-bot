package whatsapp

import (
	"fmt"
	"strings"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

func verifiedNameStr(vn *types.VerifiedName) string {
	if vn == nil || vn.Details == nil {
		return ""
	}
	return vn.Details.GetVerifiedName()
}

// CheckOnWhatsApp checks whether the given phone numbers are registered on WhatsApp.
func CheckOnWhatsApp(userId string, numbers []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	if len(numbers) == 0 {
		return nil, fmt.Errorf("at least one phone number is required")
	}
	phones := make([]string, len(numbers))
	for i, n := range numbers {
		n = strings.TrimSpace(n)
		if !strings.HasPrefix(n, "+") {
			n = "+" + normalizeNumber(n)
		}
		phones[i] = n
	}
	ctx, cancel := opCtx()
	defer cancel()
	resp, err := cli.IsOnWhatsApp(ctx, phones)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(resp))
	for _, r := range resp {
		out = append(out, map[string]interface{}{
			"query":        r.Query,
			"jid":          jidString(r.JID),
			"isOnWhatsApp": r.IsIn,
			"verifiedName": verifiedNameStr(r.VerifiedName),
		})
	}
	return out, nil
}

// GetUsersInfo fetches status/picture/device info for one or more users.
func GetUsersInfo(userId string, jids []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	targets, err := parseJIDList(jids)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	info, err := cli.GetUserInfo(ctx, targets)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(info))
	for jid, ui := range info {
		devices := make([]string, 0, len(ui.Devices))
		for _, d := range ui.Devices {
			devices = append(devices, d.String())
		}
		out = append(out, map[string]interface{}{
			"jid":          jid.String(),
			"status":       ui.Status,
			"pictureId":    ui.PictureID,
			"verifiedName": verifiedNameStr(ui.VerifiedName),
			"devices":      devices,
		})
	}
	return out, nil
}

// GetProfilePicture returns the URL + metadata of a user's or group's profile picture.
func GetProfilePicture(userId, jid string, preview bool) (interface{}, error) {
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
	pic, err := cli.GetProfilePictureInfo(ctx, target, &whatsmeow.GetProfilePictureParams{Preview: preview})
	if err != nil {
		return nil, err
	}
	if pic == nil {
		return map[string]interface{}{"jid": target.String(), "url": nil}, nil
	}
	return map[string]interface{}{
		"jid":        target.String(),
		"url":        pic.URL,
		"id":         pic.ID,
		"type":       pic.Type,
		"directPath": pic.DirectPath,
	}, nil
}

// GetBusinessProfileData fetches the business profile details of a WhatsApp business account.
func GetBusinessProfileData(userId, jid string) (interface{}, error) {
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
	bp, err := cli.GetBusinessProfile(ctx, target)
	if err != nil {
		return nil, err
	}
	if bp == nil {
		return nil, nil
	}
	categories := make([]map[string]string, 0, len(bp.Categories))
	for _, c := range bp.Categories {
		categories = append(categories, map[string]string{"id": c.ID, "name": c.Name})
	}
	return map[string]interface{}{
		"jid":        bp.JID.String(),
		"email":      bp.Email,
		"address":    bp.Address,
		"categories": categories,
		"timezone":   bp.BusinessHoursTimeZone,
	}, nil
}

// GetUserDevicesData lists the device JIDs for one or more users.
func GetUserDevicesData(userId string, jids []string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	targets, err := parseJIDList(jids)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	devices, err := cli.GetUserDevices(ctx, targets)
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(devices))
	for _, d := range devices {
		out = append(out, d.String())
	}
	return out, nil
}

// GetAllContacts returns the local contact cache.
func GetAllContacts(userId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	contacts, err := cli.Store.Contacts.GetAllContacts(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]map[string]interface{}, 0, len(contacts))
	for jid, c := range contacts {
		out = append(out, map[string]interface{}{
			"jid":          jid.String(),
			"firstName":    c.FirstName,
			"fullName":     c.FullName,
			"pushName":     c.PushName,
			"businessName": c.BusinessName,
		})
	}
	return out, nil
}

// GetBlocklistData returns the list of blocked users.
func GetBlocklistData(userId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	bl, err := cli.GetBlocklist(ctx)
	if err != nil {
		return nil, err
	}
	jids := make([]string, 0)
	if bl != nil {
		for _, j := range bl.JIDs {
			jids = append(jids, j.String())
		}
	}
	return map[string]interface{}{"blocked": jids}, nil
}

// SetBlocked blocks or unblocks a user.
func SetBlocked(userId, jid string, block bool) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	target, err := parseRecipient(jid)
	if err != nil {
		return nil, err
	}
	action := events.BlocklistChangeActionBlock
	if !block {
		action = events.BlocklistChangeActionUnblock
	}
	ctx, cancel := opCtx()
	defer cancel()
	if _, err := cli.UpdateBlocklist(ctx, target, action); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "jid": target.String(), "blocked": block}, nil
}

// GetPrivacy returns the account's privacy settings.
func GetPrivacy(userId string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	s := cli.GetPrivacySettings(ctx)
	return map[string]interface{}{
		"groupAdd":     string(s.GroupAdd),
		"lastSeen":     string(s.LastSeen),
		"status":       string(s.Status),
		"profile":      string(s.Profile),
		"readReceipts": string(s.ReadReceipts),
		"online":       string(s.Online),
		"callAdd":      string(s.CallAdd),
	}, nil
}

// SetStatus updates the account's "about" / status text.
func SetStatus(userId, status string) (interface{}, error) {
	cli, err := requireClient(userId)
	if err != nil {
		return nil, err
	}
	ctx, cancel := opCtx()
	defer cancel()
	if err := cli.SetStatusMessage(ctx, status); err != nil {
		return nil, err
	}
	return map[string]interface{}{"success": true, "status": status}, nil
}
