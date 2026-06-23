package main

import (
	"wa-server-go/whatsapp"

	"github.com/gofiber/fiber/v2"
)

// registerMessagingRoutes wires up all the messaging / media / presence endpoints.
func registerMessagingRoutes(api fiber.Router) {
	api.Post("/send-image", func(c *fiber.Ctx) error {
		var body struct {
			To      string `json:"to"`
			Image   string `json:"image"`
			Caption string `json:"caption"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" || body.Image == "" {
			return badRequest(c, "to and image are required")
		}
		res, err := whatsapp.SendImage(uid(c), body.To, body.Image, body.Caption)
		return jsonResult(c, res, err)
	})

	api.Post("/send-video", func(c *fiber.Ctx) error {
		var body struct {
			To          string `json:"to"`
			Video       string `json:"video"`
			Caption     string `json:"caption"`
			GifPlayback bool   `json:"gifPlayback"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" || body.Video == "" {
			return badRequest(c, "to and video are required")
		}
		res, err := whatsapp.SendVideo(uid(c), body.To, body.Video, body.Caption, body.GifPlayback)
		return jsonResult(c, res, err)
	})

	api.Post("/send-audio", func(c *fiber.Ctx) error {
		var body struct {
			To    string `json:"to"`
			Audio string `json:"audio"`
			PTT   bool   `json:"ptt"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" || body.Audio == "" {
			return badRequest(c, "to and audio are required")
		}
		res, err := whatsapp.SendAudio(uid(c), body.To, body.Audio, body.PTT)
		return jsonResult(c, res, err)
	})

	api.Post("/send-document", func(c *fiber.Ctx) error {
		var body struct {
			To       string `json:"to"`
			Document string `json:"document"`
			Filename string `json:"filename"`
			Caption  string `json:"caption"`
			Mimetype string `json:"mimetype"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" || body.Document == "" {
			return badRequest(c, "to and document are required")
		}
		res, err := whatsapp.SendDocument(uid(c), body.To, body.Document, body.Filename, body.Caption, body.Mimetype)
		return jsonResult(c, res, err)
	})

	api.Post("/send-sticker", func(c *fiber.Ctx) error {
		var body struct {
			To      string `json:"to"`
			Sticker string `json:"sticker"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" || body.Sticker == "" {
			return badRequest(c, "to and sticker are required")
		}
		res, err := whatsapp.SendSticker(uid(c), body.To, body.Sticker)
		return jsonResult(c, res, err)
	})

	api.Post("/send-location", func(c *fiber.Ctx) error {
		var body struct {
			To        string  `json:"to"`
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			Name      string  `json:"name"`
			Address   string  `json:"address"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.SendLocation(uid(c), body.To, body.Latitude, body.Longitude, body.Name, body.Address)
		return jsonResult(c, res, err)
	})

	api.Post("/send-contact", func(c *fiber.Ctx) error {
		var body struct {
			To          string `json:"to"`
			DisplayName string `json:"displayName"`
			Phone       string `json:"phone"`
			Vcard       string `json:"vcard"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.SendContact(uid(c), body.To, body.DisplayName, body.Phone, body.Vcard)
		return jsonResult(c, res, err)
	})

	api.Post("/send-poll", func(c *fiber.Ctx) error {
		var body struct {
			To              string   `json:"to"`
			Name            string   `json:"name"`
			Options         []string `json:"options"`
			SelectableCount int      `json:"selectableCount"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.SendPoll(uid(c), body.To, body.Name, body.Options, body.SelectableCount)
		return jsonResult(c, res, err)
	})

	api.Post("/reply-message", func(c *fiber.Ctx) error {
		var body struct {
			To          string `json:"to"`
			MessageId   string `json:"messageId"`
			Participant string `json:"participant"`
			Text        string `json:"text"`
			QuotedText  string `json:"quotedText"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.ReplyMessage(uid(c), body.To, body.MessageId, body.Participant, body.Text, body.QuotedText)
		return jsonResult(c, res, err)
	})

	api.Post("/send-reaction", func(c *fiber.Ctx) error {
		var body struct {
			To          string `json:"to"`
			MessageId   string `json:"messageId"`
			Participant string `json:"participant"`
			Emoji       string `json:"emoji"`
			FromMe      bool   `json:"fromMe"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.SendReaction(uid(c), body.To, body.MessageId, body.Participant, body.Emoji, body.FromMe)
		return jsonResult(c, res, err)
	})

	api.Post("/edit-message", func(c *fiber.Ctx) error {
		var body struct {
			To        string `json:"to"`
			MessageId string `json:"messageId"`
			NewText   string `json:"newText"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.EditMessage(uid(c), body.To, body.MessageId, body.NewText)
		return jsonResult(c, res, err)
	})

	api.Post("/revoke-message", func(c *fiber.Ctx) error {
		var body struct {
			To          string `json:"to"`
			MessageId   string `json:"messageId"`
			Participant string `json:"participant"`
			FromMe      bool   `json:"fromMe"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.RevokeMessage(uid(c), body.To, body.MessageId, body.Participant, body.FromMe)
		return jsonResult(c, res, err)
	})

	api.Post("/mark-read", func(c *fiber.Ctx) error {
		var body struct {
			Chat       string   `json:"chat"`
			Sender     string   `json:"sender"`
			MessageIds []string `json:"messageIds"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Chat == "" {
			return badRequest(c, "chat is required")
		}
		res, err := whatsapp.MarkRead(uid(c), body.Chat, body.Sender, body.MessageIds)
		return jsonResult(c, res, err)
	})

	api.Post("/presence", func(c *fiber.Ctx) error {
		var body struct {
			Presence string `json:"presence"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.SetPresence(uid(c), body.Presence)
		return jsonResult(c, res, err)
	})

	api.Post("/chat-presence", func(c *fiber.Ctx) error {
		var body struct {
			To    string `json:"to"`
			State string `json:"state"`
			Media string `json:"media"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.To == "" {
			return badRequest(c, "to is required")
		}
		res, err := whatsapp.SetChatPresence(uid(c), body.To, body.State, body.Media)
		return jsonResult(c, res, err)
	})

	api.Post("/subscribe-presence", func(c *fiber.Ctx) error {
		var body struct {
			Jid string `json:"jid"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.Jid == "" {
			return badRequest(c, "jid is required")
		}
		res, err := whatsapp.SubscribePresence(uid(c), body.Jid)
		return jsonResult(c, res, err)
	})

	// Download media from a previously received chat/group message.
	api.Get("/download-media", func(c *fiber.Ctx) error {
		messageId := c.Query("messageId")
		if messageId == "" {
			return badRequest(c, "messageId query param is required")
		}
		res, err := whatsapp.DownloadMessageMedia(uid(c), messageId)
		return jsonResult(c, res, err)
	})
}
