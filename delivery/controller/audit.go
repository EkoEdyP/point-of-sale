package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

// headerIdempotencyKey: UUID dari client untuk penyimpanan yang tidak boleh tercatat dua kali (penjualan, pre-order,
// DP, penyerahan pre-order, hasil produksi). Request yang diulang dengan kunci yang sama mendapat hasil yang pertama.
const headerIdempotencyKey = "Idempotency-Key"

type AuditController struct {
	Engine       *fiber.App
	UseCase      usecase.AuditUseCase
	Authenticate fiber.Handler
}

// Route riwayat perubahan data (hanya admin)
func (c *AuditController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	audits := v1.Group("/audit-logs")

	adminOnly := middleware.RequireRole(model.RoleAdmin)
	audits.Get("/", c.Authenticate, adminOnly, c.list)
}

func (c *AuditController) list(ctx fiber.Ctx) error {
	query := req.AuditListQuery{
		Entity:   ctx.Query("entity"),
		EntityId: ctx.Query("entity_id"),
		ActorId:  ctx.Query("actor_id"),
		From:     ctx.Query("from"),
		To:       ctx.Query("to"),
		Page:     atoiDefault(ctx.Query("page"), 1),
		Limit:    atoiDefault(ctx.Query("limit"), 20),
	}
	result, err := c.UseCase.List(query)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

// NewAuditController Constructor
func NewAuditController(engine *fiber.App, useCase usecase.AuditUseCase, authenticate fiber.Handler) *AuditController {
	return &AuditController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
