package controller

import (
	"context"
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

type HealthController struct {
	DB     *sql.DB
	Engine *fiber.App
}

// Route health
func (h *HealthController) Route() {
	api := h.Engine.Group("/api")

	v1 := api.Group("/v1")
	v1.Get("/health", h.healthHandler)
}

func (h *HealthController) healthHandler(ctx fiber.Ctx) error {
	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := h.DB.PingContext(pingCtx); err != nil {
		return ctx.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"status": "db_unavailable"})
	}
	return ctx.JSON(fiber.Map{"status": "ok"})
}

// NewHealthController Constructor
func NewHealthController(db *sql.DB, engine *fiber.App) *HealthController {
	return &HealthController{
		DB:     db,
		Engine: engine,
	}
}
