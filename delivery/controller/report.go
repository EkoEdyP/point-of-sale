package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type ReportController struct {
	Engine       *fiber.App
	UseCase      usecase.ReportUseCase
	Authenticate fiber.Handler
}

// Route reports (hanya admin)
func (c *ReportController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	reports := v1.Group("/reports")

	authenticate := c.Authenticate
	adminOnly := middleware.RequireRole(model.RoleAdmin)

	reports.Get("/daily", authenticate, adminOnly, c.daily)
}

func (c *ReportController) daily(ctx fiber.Ctx) error {
	result, err := c.UseCase.Daily(ctx.Query("from"), ctx.Query("to"), ctx.Query("source"))
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

// NewReportController Constructor
func NewReportController(engine *fiber.App, useCase usecase.ReportUseCase, authenticate fiber.Handler) *ReportController {
	return &ReportController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
