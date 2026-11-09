package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type PreorderController struct {
	Engine       *fiber.App
	UseCase      usecase.PreorderUseCase
	Authenticate fiber.Handler
}

// Route preorders (admin dan cashier; pembatasan pembatalan ada di usecase, kuota hanya diubah admin)
func (c *PreorderController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	preorders := v1.Group("/preorders")
	adminOnly := middleware.RequireRole(model.RoleAdmin)

	// /summary, /production, dan /quota harus didaftarkan sebelum /:id.
	preorders.Get("/summary", c.Authenticate, c.summary)
	preorders.Get("/production", c.Authenticate, c.production)
	preorders.Get("/quota", c.Authenticate, c.quota)
	preorders.Put("/quota", c.Authenticate, adminOnly, c.setQuota)
	preorders.Post("/", c.Authenticate, c.create)
	preorders.Get("/", c.Authenticate, c.list)
	preorders.Get("/:id", c.Authenticate, c.get)
	preorders.Patch("/:id", c.Authenticate, c.update)
	preorders.Post("/:id/payments", c.Authenticate, c.addPayment)
	preorders.Post("/:id/ready", c.Authenticate, c.markReady)
	preorders.Post("/:id/complete", c.Authenticate, c.complete)
	preorders.Post("/:id/cancel", c.Authenticate, c.cancel)
}

func (c *PreorderController) actor(ctx fiber.Ctx) (id, role string) {
	id, _ = ctx.Locals(middleware.CtxUserID).(string)
	role, _ = ctx.Locals(middleware.CtxRole).(string)
	return id, role
}

func (c *PreorderController) summary(ctx fiber.Ctx) error {
	result, err := c.UseCase.Summary()
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *PreorderController) production(ctx fiber.Ctx) error {
	result, err := c.UseCase.Production(ctx.Query("due_date"))
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

// quota: ?due_date=YYYY-MM-DD wajib; ?exclude=<id pre-order> opsional (saat memindah jadwal).
func (c *PreorderController) quota(ctx fiber.Ctx) error {
	result, err := c.UseCase.Quota(ctx.Query("due_date"), ctx.Query("exclude"))
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *PreorderController) setQuota(ctx fiber.Ctx) error {
	var body req.PreorderDailyLimitRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := c.actor(ctx)
	result, err := c.UseCase.SetDailyLimit(actorId, body)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *PreorderController) create(ctx fiber.Ctx) error {
	var body req.PreorderRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	body.RequestKey = ctx.Get(headerIdempotencyKey)
	actorId, _ := c.actor(ctx)
	preorder, err := c.UseCase.Create(actorId, body)
	if err != nil {
		return err
	}
	return ctx.Status(fiber.StatusCreated).JSON(preorder)
}

func (c *PreorderController) list(ctx fiber.Ctx) error {
	query := req.PreorderListQuery{
		Status:  ctx.Query("status"),
		DueDate: ctx.Query("due_date"),
		Page:    atoiDefault(ctx.Query("page"), 1),
		Limit:   atoiDefault(ctx.Query("limit"), 20),
	}

	result, err := c.UseCase.List(query)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *PreorderController) get(ctx fiber.Ctx) error {
	preorder, err := c.UseCase.Get(ctx.Params("id"))
	if err != nil {
		return err
	}
	return ctx.JSON(preorder)
}

func (c *PreorderController) update(ctx fiber.Ctx) error {
	var body req.PreorderUpdateRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := c.actor(ctx)
	preorder, err := c.UseCase.Update(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(preorder)
}

func (c *PreorderController) addPayment(ctx fiber.Ctx) error {
	var body req.PreorderPaymentRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	body.RequestKey = ctx.Get(headerIdempotencyKey)
	actorId, _ := c.actor(ctx)
	preorder, err := c.UseCase.AddPayment(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(preorder)
}

func (c *PreorderController) markReady(ctx fiber.Ctx) error {
	actorId, _ := c.actor(ctx)
	preorder, err := c.UseCase.MarkReady(actorId, ctx.Params("id"))
	if err != nil {
		return err
	}
	return ctx.JSON(preorder)
}

// complete mengembalikan transaksi yang terbentuk (untuk struk).
func (c *PreorderController) complete(ctx fiber.Ctx) error {
	var body req.PreorderCompleteRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	body.RequestKey = ctx.Get(headerIdempotencyKey)
	actorId, _ := c.actor(ctx)
	transaction, err := c.UseCase.Complete(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(transaction)
}

func (c *PreorderController) cancel(ctx fiber.Ctx) error {
	var body req.PreorderCancelRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, role := c.actor(ctx)
	preorder, err := c.UseCase.Cancel(actorId, role, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(preorder)
}

// NewPreorderController Constructor
func NewPreorderController(engine *fiber.App, useCase usecase.PreorderUseCase, authenticate fiber.Handler) *PreorderController {
	return &PreorderController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
