package main

import (
	"strings"

	"wa-server-go/whatsapp"

	"github.com/gofiber/fiber/v2"
)

// splitCSV splits a comma separated query value into a trimmed, non-empty slice.
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// registerContactRoutes wires up contact, profile, presence and privacy endpoints.
func registerContactRoutes(api fiber.Router) {
	api.Post("/check-number", func(c *fiber.Ctx) error {
		var body struct {
			Numbers []string `json:"numbers"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if len(body.Numbers) == 0 {
			return badRequest(c, "numbers array is required")
		}
		res, err := whatsapp.CheckOnWhatsApp(uid(c), body.Numbers)
		return jsonResult(c, res, err)
	})

	api.Get("/user-info", func(c *fiber.Ctx) error {
		jids := splitCSV(c.Query("jids"))
		if len(jids) == 0 {
			return badRequest(c, "jids query param is required (comma separated)")
		}
		res, err := whatsapp.GetUsersInfo(uid(c), jids)
		return jsonResult(c, res, err)
	})

	api.Get("/profile-picture", func(c *fiber.Ctx) error {
		jid := c.Query("jid")
		if jid == "" {
			return badRequest(c, "jid query param is required")
		}
		preview := c.Query("preview") == "true"
		res, err := whatsapp.GetProfilePicture(uid(c), jid, preview)
		return jsonResult(c, res, err)
	})

	api.Get("/business-profile", func(c *fiber.Ctx) error {
		jid := c.Query("jid")
		if jid == "" {
			return badRequest(c, "jid query param is required")
		}
		res, err := whatsapp.GetBusinessProfileData(uid(c), jid)
		return jsonResult(c, res, err)
	})

	api.Get("/user-devices", func(c *fiber.Ctx) error {
		jids := splitCSV(c.Query("jids"))
		if len(jids) == 0 {
			return badRequest(c, "jids query param is required (comma separated)")
		}
		res, err := whatsapp.GetUserDevicesData(uid(c), jids)
		return jsonResult(c, res, err)
	})

	api.Get("/contacts", func(c *fiber.Ctx) error {
		res, err := whatsapp.GetAllContacts(uid(c))
		return jsonResult(c, res, err)
	})

	api.Get("/blocklist", func(c *fiber.Ctx) error {
		res, err := whatsapp.GetBlocklistData(uid(c))
		return jsonResult(c, res, err)
	})

	api.Post("/block", func(c *fiber.Ctx) error {
		var body struct {
			Jid string `json:"jid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.SetBlocked(uid(c), body.Jid, true)
		return jsonResult(c, res, err)
	})

	api.Post("/unblock", func(c *fiber.Ctx) error {
		var body struct {
			Jid string `json:"jid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.SetBlocked(uid(c), body.Jid, false)
		return jsonResult(c, res, err)
	})

	api.Get("/privacy-settings", func(c *fiber.Ctx) error {
		res, err := whatsapp.GetPrivacy(uid(c))
		return jsonResult(c, res, err)
	})

	api.Post("/set-status", func(c *fiber.Ctx) error {
		var body struct {
			Status string `json:"status"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.SetStatus(uid(c), body.Status)
		return jsonResult(c, res, err)
	})
}

// registerNewsletterRoutes wires up WhatsApp channel (newsletter) endpoints.
func registerNewsletterRoutes(api fiber.Router) {
	api.Get("/newsletters", func(c *fiber.Ctx) error {
		res, err := whatsapp.GetSubscribedNewsletters(uid(c))
		return jsonResult(c, res, err)
	})

	api.Get("/newsletter/info", func(c *fiber.Ctx) error {
		jid := c.Query("jid")
		if jid == "" {
			return badRequest(c, "jid query param is required")
		}
		res, err := whatsapp.GetNewsletterInfoData(uid(c), jid)
		return jsonResult(c, res, err)
	})

	api.Get("/newsletter/info-from-invite", func(c *fiber.Ctx) error {
		key := c.Query("key")
		if key == "" {
			return badRequest(c, "key query param is required")
		}
		res, err := whatsapp.GetNewsletterInfoFromInvite(uid(c), key)
		return jsonResult(c, res, err)
	})

	api.Post("/newsletter/create", func(c *fiber.Ctx) error {
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.CreateNewsletter(uid(c), body.Name, body.Description)
		return jsonResult(c, res, err)
	})

	api.Post("/newsletter/follow", func(c *fiber.Ctx) error {
		var body struct {
			Jid string `json:"jid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.FollowNewsletter(uid(c), body.Jid)
		return jsonResult(c, res, err)
	})

	api.Post("/newsletter/unfollow", func(c *fiber.Ctx) error {
		var body struct {
			Jid string `json:"jid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.UnfollowNewsletter(uid(c), body.Jid)
		return jsonResult(c, res, err)
	})

	api.Post("/newsletter/mute", func(c *fiber.Ctx) error {
		var body struct {
			Jid  string `json:"jid"`
			Mute bool   `json:"mute"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.ToggleNewsletterMute(uid(c), body.Jid, body.Mute)
		return jsonResult(c, res, err)
	})
}
