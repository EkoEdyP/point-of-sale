package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type TransactionController struct {
	Engine       *fiber.App
	UseCase      usecase.TransactionUseCase
	Authenticate fiber.Handler
}

// Route transactions (admin dan cashier; pembatasan per kasir ada di usecase)
func (c *TransactionController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	transactions := v1.Group("/transactions")

	authenticate := c.Authenticate

	transactions.Post("/", authenticate, c.create)
	transactions.Get("/", authenticate, c.list)
	transactions.Get("/:id", authenticate, c.get)
	transactions.Post("/:id/void", authenticate, c.void)
}

func (c *TransactionController) actor(ctx fiber.Ctx) (id, role string) {
	id, _ = ctx.Locals(middleware.CtxUserID).(string)
	role, _ = ctx.Locals(middleware.CtxRole).(string)
	return id, role
}

func (c *TransactionController) create(ctx fiber.Ctx) error {
	var body req.TransactionRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	body.RequestKey = ctx.Get(headerIdempotencyKey)
	actorId, _ := c.actor(ctx)
	transaction, err := c.UseCase.Create(actorId, body)
	if err != nil {
		return err
	}
	return ctx.Status(fiber.StatusCreated).JSON(transaction)
}

func (c *TransactionController) list(ctx fiber.Ctx) error {
	actorId, role := c.actor(ctx)
	query := req.TransactionListQuery{
		From:   ctx.Query("from"),
		To:     ctx.Query("to"),
		Status: ctx.Query("status"),
		Page:   atoiDefault(ctx.Query("page"), 1),
		Limit:  atoiDefault(ctx.Query("limit"), 20),
	}

	result, err := c.UseCase.List(actorId, role, query)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *TransactionController) get(ctx fiber.Ctx) error {
	actorId, role := c.actor(ctx)

	transaction, err := c.UseCase.Get(actorId, role, ctx.Params("id"))
	if err != nil {
		return err
	}
	return ctx.JSON(transaction)
}

func (c *TransactionController) void(ctx fiber.Ctx) error {
	var body req.VoidRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, role := c.actor(ctx)
	transaction, err := c.UseCase.Void(actorId, role, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(transaction)
}

// NewTransactionController Constructor
func NewTransactionController(engine *fiber.App, useCase usecase.TransactionUseCase, authenticate fiber.Handler) *TransactionController {
	return &TransactionController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
