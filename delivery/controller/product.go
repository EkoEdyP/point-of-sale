package controller

import (
	"strconv"
	"strings"

	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type ProductController struct {
	Engine       *fiber.App
	UseCase      usecase.ProductUseCase
	Authenticate fiber.Handler
}

// Route products
func (c *ProductController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	products := v1.Group("/products")

	authenticate := c.Authenticate
	adminOnly := middleware.RequireRole(model.RoleAdmin)

	// admin dan cashier
	products.Get("/", authenticate, c.list)

	// hanya admin
	products.Post("/", authenticate, adminOnly, c.create)
	products.Put("/:id", authenticate, adminOnly, c.update)
	products.Delete("/:id", authenticate, adminOnly, c.delete)
	products.Post("/:id/production", authenticate, adminOnly, c.addProduction)
}

func (c *ProductController) list(ctx fiber.Ctx) error {
	role, _ := ctx.Locals(middleware.CtxRole).(string)
	query := req.ProductListQuery{
		Kind:          ctx.Query("kind"),
		Search:        ctx.Query("search"),
		Page:          atoiDefault(ctx.Query("page"), 1),
		Limit:         atoiDefault(ctx.Query("limit"), 20),
		NegativeStock: ctx.Query("stock") == "negative",
		OnlyActive:    ctx.Query("active") == "true",
	}
	if ids := strings.TrimSpace(ctx.Query("ids")); ids != "" {
		query.Ids = strings.Split(ids, ",")
	}

	result, err := c.UseCase.List(role, query)
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *ProductController) create(ctx fiber.Ctx) error {
	var body req.ProductRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	product, err := c.UseCase.Create(actorId, body)
	if err != nil {
		return err
	}
	return ctx.Status(fiber.StatusCreated).JSON(product)
}

func (c *ProductController) update(ctx fiber.Ctx) error {
	var body req.ProductRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	product, err := c.UseCase.Update(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(product)
}

func (c *ProductController) delete(ctx fiber.Ctx) error {
	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	if err := c.UseCase.Delete(actorId, ctx.Params("id")); err != nil {
		return err
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

// addProduction: stok siap produk PO bertambah sebanyak qty.
func (c *ProductController) addProduction(ctx fiber.Ctx) error {
	var body req.ProductionRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	body.RequestKey = ctx.Get(headerIdempotencyKey)
	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	product, err := c.UseCase.AddProduction(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(product)
}

func atoiDefault(s string, def int) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return n
}

// NewProductController Constructor
func NewProductController(engine *fiber.App, useCase usecase.ProductUseCase, authenticate fiber.Handler) *ProductController {
	return &ProductController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
