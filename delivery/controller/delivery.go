package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type DeliveryController struct {
	Engine       *fiber.App
	UseCase      usecase.DeliveryUseCase
	Authenticate fiber.Handler
}

// Route deliveries (admin dan cashier; pembatasan pembatalan ada di usecase)
func (c *DeliveryController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	deliveries := v1.Group("/deliveries")

	// /summary harus didaftarkan sebelum /:id.
	deliveries.Get("/summary", c.Authenticate, c.summary)
	deliveries.Get("/", c.Authenticate, c.list)
	deliveries.Get("/:id", c.Authenticate, c.get)
	deliveries.Patch("/:id", c.Authenticate, c.update)
	deliveries.Post("/:id/dispatch", c.Authenticate, c.dispatch)
	deliveries.Post("/:id/fail", c.Authenticate, c.fail)
	deliveries.Post("/:id/cancel", c.Authenticate, c.cancel)
}

func (c *DeliveryController) actor(ctx fiber.Ctx) (id, role string) {
	id, _ = ctx.Locals(middleware.CtxUserID).(string)
	role, _ = ctx.Locals(middleware.CtxRole).(string)
	return id, role
}

func (c *DeliveryController) summary(ctx fiber.Ctx) error {
	result, err := c.UseCase.Summary()
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *DeliveryController) list(ctx fiber.Ctx) error {
	query := req.DeliveryListQuery{
		Status: ctx.Query("status"),
		Page:   atoiDefault(ctx.Query("page"), 1),
		Limit:  atoiDefault(ctx.Query("limit"), 20),
	}

	result, err := c.UseCase.List(query)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *DeliveryController) get(ctx fiber.Ctx) error {
	delivery, err := c.UseCase.Get(ctx.Params("id"))
	if err != nil {
		return err
	}
	return ctx.JSON(delivery)
}

func (c *DeliveryController) update(ctx fiber.Ctx) error {
	var body req.DeliveryUpdateRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := c.actor(ctx)
	delivery, err := c.UseCase.Update(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(delivery)
}

func (c *DeliveryController) dispatch(ctx fiber.Ctx) error {
	var body req.DeliveryDispatchRequest
	// Body boleh kosong: nama kurir opsional.
	if len(ctx.Body()) > 0 {
		if err := parseBody(ctx, &body); err != nil {
			return err
		}
	}

	actorId, _ := c.actor(ctx)
	delivery, err := c.UseCase.Dispatch(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(delivery)
}

func (c *DeliveryController) fail(ctx fiber.Ctx) error {
	var body req.DeliveryReasonRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := c.actor(ctx)
	delivery, err := c.UseCase.Fail(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(delivery)
}

func (c *DeliveryController) cancel(ctx fiber.Ctx) error {
	var body req.DeliveryReasonRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, role := c.actor(ctx)
	delivery, err := c.UseCase.Cancel(actorId, role, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(delivery)
}

// NewDeliveryController Constructor
func NewDeliveryController(engine *fiber.App, useCase usecase.DeliveryUseCase, authenticate fiber.Handler) *DeliveryController {
	return &DeliveryController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
