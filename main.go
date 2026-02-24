package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	"strings"
	"time"

	"wa-server-go/storage"
	"wa-server-go/whatsapp"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func authMiddleware(c *fiber.Ctx) error {
	var token string
	authHeader := c.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	} else {
		token = c.Cookies("wa_token")
	}

	if token == "" {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(401).JSON(fiber.Map{"error": "Unauthorized — please log in"})
		}
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	user := storage.FindUserByToken(token)
	if user == nil {
		if strings.HasPrefix(c.Path(), "/api/") {
			return c.Status(401).JSON(fiber.Map{"error": "Unauthorized — please log in"})
		}
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	c.Locals("userId", user.ID)
	c.Locals("userEmail", user.Email)
	return c.Next()
}

func main() {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${method} ${path} - ${status} - ${latency}\n",
	}))
	app.Use(cors.New())

	// Serve Static Files
	app.Static("/", "./public")

	// Auth Routes
	auth := app.Group("/auth")

	auth.Get("/me", authMiddleware, func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"id":    c.Locals("userId"),
			"email": c.Locals("userEmail"),
		})
	})

	auth.Post("/register", func(c *fiber.Ctx) error {
		type Req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
		}
		user, err := storage.Register(body.Email, body.Password)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		c.Cookie(&fiber.Cookie{
			Name:     "wa_token",
			Value:    user.Token,
			Path:     "/",
			HTTPOnly: true,
			SameSite: "Lax",
		})
		return c.JSON(fiber.Map{"id": user.ID, "email": user.Email, "token": user.Token})
	})

	auth.Post("/login", func(c *fiber.Ctx) error {
		type Req struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
		}
		user, err := storage.Login(body.Email, body.Password)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		c.Cookie(&fiber.Cookie{
			Name:     "wa_token",
			Value:    user.Token,
			Path:     "/",
			HTTPOnly: true,
			SameSite: "Lax",
		})
		return c.JSON(fiber.Map{"id": user.ID, "email": user.Email, "token": user.Token})
	})

	auth.Post("/logout", func(c *fiber.Ctx) error {
		c.Cookie(&fiber.Cookie{
			Name:     "wa_token",
			Value:    "",
			Path:     "/",
			HTTPOnly: true,
			Expires:  time.Now().Add(-1 * time.Hour),
		})
		return c.JSON(fiber.Map{"success": true})
	})

	auth.Post("/forgot-password", func(c *fiber.Ctx) error {
		type Req struct {
			Email string `json:"email"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
		}
		storage.ForgotPassword(body.Email)
		return c.JSON(fiber.Map{"message": "If that email exists, a password reset code has been generated."})
	})

	auth.Post("/reset-password", func(c *fiber.Ctx) error {
		type Req struct {
			Email       string `json:"email"`
			Otp         string `json:"otp"`
			NewPassword string `json:"newPassword"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid format"})
		}
		user, err := storage.ResetPassword(body.Email, body.Otp, body.NewPassword)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		c.Cookie(&fiber.Cookie{
			Name:     "wa_token",
			Value:    user.Token,
			Path:     "/",
			HTTPOnly: true,
			SameSite: "Lax",
		})
		return c.JSON(fiber.Map{"id": user.ID, "email": user.Email, "token": user.Token})
	})

	// API Routes
	api := app.Group("/api", authMiddleware)

	api.Get("/status", func(c *fiber.Ctx) error {
		userId := c.Locals("userId").(string)
		uc := whatsapp.GetUserClient(userId)
		return c.JSON(fiber.Map{
			"status":      uc.ConnectionStatus,
			"pairingCode": uc.PairingCode,
			"qr":          uc.QRCodeData,
			"info":        uc.ClientInfo,
			"error":       uc.LastError,
		})
	})

	api.Get("/stats", func(c *fiber.Ctx) error {
		userId := c.Locals("userId").(string)
		userData := storage.LoadUser(userId)
		return c.JSON(fiber.Map{
			"messagesSent":     userData.Stats.MessagesSent,
			"messagesReceived": userData.Stats.MessagesReceived,
			"groupsJoined":     userData.Stats.GroupsJoined,
			"groupsLeft":       userData.Stats.GroupsLeft,
			"webhookCount":     len(userData.Webhooks),
		})
	})

	api.Get("/messages", func(c *fiber.Ctx) error {
		userId := c.Locals("userId").(string)
		userData := storage.LoadUser(userId)

		limitStr := c.Query("limit", "50")
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			limit = 50
		}

		msgs := userData.Messages

		// Reverse and limit
		if len(msgs) > limit {
			msgs = msgs[len(msgs)-limit:]
		}

		reversed := make([]interface{}, len(msgs))
		for i, j := 0, len(msgs)-1; i < len(msgs); i, j = i+1, j-1 {
			reversed[i] = msgs[j]
		}

		return c.JSON(reversed)
	})

	api.Get("/groups", func(c *fiber.Ctx) error {
		userId := c.Locals("userId").(string)
		groups, err := whatsapp.GetGroups(userId)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(groups)
	})

	api.Post("/send-message", func(c *fiber.Ctx) error {
		type Req struct {
			Number  string `json:"number"`
			Message string `json:"message"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		if body.Number == "" || body.Message == "" {
			return c.Status(400).JSON(fiber.Map{"error": "number and message are required"})
		}

		userId := c.Locals("userId").(string)
		result, err := whatsapp.SendMessage(userId, body.Number, body.Message)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": result})
	})

	api.Post("/send-group-message", func(c *fiber.Ctx) error {
		type Req struct {
			GroupId string `json:"groupId"`
			Message string `json:"message"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		if body.GroupId == "" || body.Message == "" {
			return c.Status(400).JSON(fiber.Map{"error": "groupId and message are required"})
		}

		userId := c.Locals("userId").(string)
		result, err := whatsapp.SendGroupMessage(userId, body.GroupId, body.Message)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": result})
	})

	api.Post("/join-group", func(c *fiber.Ctx) error {
		type Req struct {
			InviteLink string `json:"inviteLink"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		userId := c.Locals("userId").(string)
		result, err := whatsapp.JoinGroup(userId, body.InviteLink)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "result": result})
	})

	api.Post("/leave-group", func(c *fiber.Ctx) error {
		type Req struct {
			GroupId string `json:"groupId"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		userId := c.Locals("userId").(string)
		result, err := whatsapp.LeaveGroup(userId, body.GroupId)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "result": result})
	})

	api.Post("/add-to-group", func(c *fiber.Ctx) error {
		type Req struct {
			GroupId      string   `json:"groupId"`
			Participants []string `json:"participants"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}
		userId := c.Locals("userId").(string)
		result, err := whatsapp.AddToGroup(userId, body.GroupId, body.Participants)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "result": result})
	})

	api.Post("/disconnect", func(c *fiber.Ctx) error {
		userId := c.Locals("userId").(string)
		err := whatsapp.Disconnect(userId)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		return c.JSON(fiber.Map{"success": true, "message": "WhatsApp disconnected"})
	})

	api.Post("/reconnect", func(c *fiber.Ctx) error {
		type Req struct {
			Method      string `json:"method"`
			PhoneNumber string `json:"phoneNumber"`
		}
		var body Req
		if err := c.BodyParser(&body); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid JSON"})
		}

		method := "qr"
		if body.Method != "" {
			method = body.Method
		}

		userId := c.Locals("userId").(string)

		// Don't await initialization in the handler
		go func() {
			err := whatsapp.Initialize(userId, method, body.PhoneNumber)
			if err != nil {
				log.Printf("Reconnect error for user %s: %v\n", userId, err)
			}
		}()

		return c.JSON(fiber.Map{"success": true, "message": "Reconnecting via " + method + "..."})
	})

	config := storage.GetGlobalConfig()
	port := config.Config.Port
	if port == 0 {
		port = 3000
	}
	
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	fmt.Printf(`========== WA Server Dashboard ==========
Bot Name: %s
Port:     %d
URL:      http://localhost:%d
=========================================
`, config.Config.BotName, port, port)

	log.Fatal(app.Listen(fmt.Sprintf(":%d", port)))
}
