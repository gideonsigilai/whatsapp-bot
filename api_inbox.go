package main

import (
	"encoding/json"
	"strconv"

	"wa-server-go/whatsapp"

	"github.com/gofiber/fiber/v2"
)

// registerInboxRoutes wires up the inbox / conversation / per-conversation action endpoints.
func registerInboxRoutes(api fiber.Router) {
	// Inbox: conversation list (newest first) with each conversation's last message.
	// ?enrich=true adds profile picture, resolved name and about/topic (needs a connection).
	api.Get("/inbox", func(c *fiber.Ctx) error {
		enrich := c.Query("enrich") == "true"
		limit, _ := strconv.Atoi(c.Query("limit", "0"))
		res, err := whatsapp.GetInbox(uid(c), enrich, limit)
		return jsonResult(c, res, err)
	})

	// Messages of a single conversation (oldest first).
	api.Get("/conversation", func(c *fiber.Ctx) error {
		chat := c.Query("chat")
		if chat == "" {
			return badRequest(c, "chat query param is required")
		}
		limit, _ := strconv.Atoi(c.Query("limit", "100"))
		res, err := whatsapp.GetConversation(uid(c), chat, limit)
		return jsonResult(c, res, err)
	})

	// Descriptor of the actions available for a conversation (contact vs group).
	api.Get("/conversation/actions", func(c *fiber.Ctx) error {
		chat := c.Query("chat")
		if chat == "" {
			return badRequest(c, "chat query param is required")
		}
		res, err := whatsapp.ConversationActions(chat)
		return jsonResult(c, res, err)
	})

	// WhatsApp Status (Stories): captured status@broadcast posts grouped by poster.
	api.Get("/status-updates", func(c *fiber.Ctx) error {
		res, err := whatsapp.GetStatusUpdates(uid(c))
		return jsonResult(c, res, err)
	})

	// Execute an action against a contact or group.
	// Body: { "chat": "<jid>", "action": "<id>", ...action params }
	api.Post("/conversation/action", func(c *fiber.Ctx) error {
		var body map[string]interface{}
		if err := json.Unmarshal(c.Body(), &body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		chat, _ := body["chat"].(string)
		action, _ := body["action"].(string)
		if chat == "" || action == "" {
			return badRequest(c, "chat and action are required")
		}
		res, err := whatsapp.RunConversationAction(uid(c), chat, action, body)
		return jsonResult(c, res, err)
	})
}
