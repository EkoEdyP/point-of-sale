package controller

import (
	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

type UserController struct {
	Engine       *fiber.App
	UseCase      usecase.UserUseCase
	Authenticate fiber.Handler
}

// Route users (hanya admin)
func (c *UserController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	users := v1.Group("/users")

	authenticate := c.Authenticate
	adminOnly := middleware.RequireRole(model.RoleAdmin)

	users.Get("/", authenticate, adminOnly, c.list)
	users.Patch("/:id", authenticate, adminOnly, c.update)
}

func (c *UserController) list(ctx fiber.Ctx) error {
	result, err := c.UseCase.List()
	if err != nil {
		return err
	}
	return ctx.JSON(result)
}

func (c *UserController) update(ctx fiber.Ctx) error {
	var body req.UserUpdateRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	user, err := c.UseCase.Update(actorId, ctx.Params("id"), body)
	if err != nil {
		return err
	}
	return ctx.JSON(user)
}

// NewUserController Constructor
func NewUserController(engine *fiber.App, useCase usecase.UserUseCase, authenticate fiber.Handler) *UserController {
	return &UserController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
