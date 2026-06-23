package main

import "github.com/gofiber/fiber/v2"

// jsonResult writes a uniform success/error envelope for whatsapp helpers that
// return (interface{}, error).
func jsonResult(c *fiber.Ctx, result interface{}, err error) error {
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"success": true, "result": result})
}

// uid returns the authenticated user id stored by authMiddleware.
func uid(c *fiber.Ctx) string {
	return c.Locals("userId").(string)
}

// badRequest is a small helper for 400 responses.
func badRequest(c *fiber.Ctx, msg string) error {
	return c.Status(400).JSON(fiber.Map{"error": msg})
}
