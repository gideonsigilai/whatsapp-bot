package main

import (
	"wa-server-go/whatsapp"

	"github.com/gofiber/fiber/v2"
)

// registerGroupRoutes wires up the full group / community management surface.
func registerGroupRoutes(api fiber.Router) {
	api.Get("/group-info", func(c *fiber.Ctx) error {
		groupId := c.Query("groupId")
		if groupId == "" {
			return badRequest(c, "groupId query param is required")
		}
		res, err := whatsapp.GetGroupInfo(uid(c), groupId)
		return jsonResult(c, res, err)
	})

	api.Post("/create-group", func(c *fiber.Ctx) error {
		var body struct {
			Name         string   `json:"name"`
			Participants []string `json:"participants"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.CreateGroup(uid(c), body.Name, body.Participants)
		return jsonResult(c, res, err)
	})

	api.Post("/group/participants", func(c *fiber.Ctx) error {
		var body struct {
			GroupId      string   `json:"groupId"`
			Action       string   `json:"action"`
			Participants []string `json:"participants"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.UpdateParticipants(uid(c), body.GroupId, body.Action, body.Participants)
		return jsonResult(c, res, err)
	})

	api.Post("/group/name", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Name    string `json:"name"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupName(uid(c), body.GroupId, body.Name)
		return jsonResult(c, res, err)
	})

	api.Post("/group/topic", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Topic   string `json:"topic"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupTopic(uid(c), body.GroupId, body.Topic)
		return jsonResult(c, res, err)
	})

	api.Post("/group/photo", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Image   string `json:"image"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupPhoto(uid(c), body.GroupId, body.Image)
		return jsonResult(c, res, err)
	})

	api.Post("/group/announce", func(c *fiber.Ctx) error {
		var body struct {
			GroupId  string `json:"groupId"`
			Announce bool   `json:"announce"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupAnnounce(uid(c), body.GroupId, body.Announce)
		return jsonResult(c, res, err)
	})

	api.Post("/group/locked", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Locked  bool   `json:"locked"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupLocked(uid(c), body.GroupId, body.Locked)
		return jsonResult(c, res, err)
	})

	api.Get("/group/invite-link", func(c *fiber.Ctx) error {
		groupId := c.Query("groupId")
		if groupId == "" {
			return badRequest(c, "groupId query param is required")
		}
		reset := c.Query("reset") == "true"
		res, err := whatsapp.GetGroupInviteLink(uid(c), groupId, reset)
		return jsonResult(c, res, err)
	})

	api.Get("/group/info-from-link", func(c *fiber.Ctx) error {
		link := c.Query("link")
		if link == "" {
			return badRequest(c, "link query param is required")
		}
		res, err := whatsapp.GetGroupInfoFromLink(uid(c), link)
		return jsonResult(c, res, err)
	})

	api.Get("/group/join-requests", func(c *fiber.Ctx) error {
		groupId := c.Query("groupId")
		if groupId == "" {
			return badRequest(c, "groupId query param is required")
		}
		res, err := whatsapp.GetGroupRequestParticipants(uid(c), groupId)
		return jsonResult(c, res, err)
	})

	api.Post("/group/join-requests", func(c *fiber.Ctx) error {
		var body struct {
			GroupId      string   `json:"groupId"`
			Action       string   `json:"action"`
			Participants []string `json:"participants"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.UpdateGroupRequestParticipants(uid(c), body.GroupId, body.Action, body.Participants)
		return jsonResult(c, res, err)
	})

	api.Post("/group/member-add-mode", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Mode    string `json:"mode"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupMemberAddMode(uid(c), body.GroupId, body.Mode)
		return jsonResult(c, res, err)
	})

	api.Post("/group/disappearing", func(c *fiber.Ctx) error {
		var body struct {
			GroupId string `json:"groupId"`
			Seconds uint32 `json:"seconds"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		if body.GroupId == "" {
			return badRequest(c, "groupId is required")
		}
		res, err := whatsapp.SetGroupDisappearingTimer(uid(c), body.GroupId, body.Seconds)
		return jsonResult(c, res, err)
	})

	api.Get("/group/subgroups", func(c *fiber.Ctx) error {
		communityId := c.Query("communityId")
		if communityId == "" {
			return badRequest(c, "communityId query param is required")
		}
		res, err := whatsapp.GetSubGroups(uid(c), communityId)
		return jsonResult(c, res, err)
	})

	api.Post("/group/link", func(c *fiber.Ctx) error {
		var body struct {
			Parent string `json:"parent"`
			Child  string `json:"child"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.LinkGroup(uid(c), body.Parent, body.Child)
		return jsonResult(c, res, err)
	})

	api.Post("/group/unlink", func(c *fiber.Ctx) error {
		var body struct {
			Parent string `json:"parent"`
			Child  string `json:"child"`
		}
		if err := c.BodyParser(&body); err != nil {
			return badRequest(c, "Invalid JSON")
		}
		res, err := whatsapp.UnlinkGroup(uid(c), body.Parent, body.Child)
		return jsonResult(c, res, err)
	})
}
