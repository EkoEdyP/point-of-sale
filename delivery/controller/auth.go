package controller

import (
	"encoding/json"
	"errors"
	"time"

	"github.com/EkoEdyP/point-of-sale/delivery/middleware"
	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/req"
	"github.com/EkoEdyP/point-of-sale/usecase"
	"github.com/gofiber/fiber/v3"
)

const (
	refreshCookieName = "refresh_token"
	refreshCookiePath = "/api/v1/auth" // cookie hanya dikirim ke endpoint auth
)

type AuthController struct {
	Engine       *fiber.App
	UseCase      usecase.AuthUseCase
	Authenticate fiber.Handler
}

// Route auth
func (c *AuthController) Route() {
	api := c.Engine.Group("/api")
	v1 := api.Group("/v1")
	auth := v1.Group("/auth")

	auth.Post("/login", c.login)
	auth.Post("/refresh", c.refresh)
	auth.Post("/logout", c.logout)
	auth.Post("/register",
		c.Authenticate,
		middleware.RequireRole(model.RoleAdmin),
		c.register,
	)
}

func (c *AuthController) login(ctx fiber.Ctx) error {
	var body req.LoginRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}
	ctx.Locals(middleware.CtxLoginName, body.Username)

	result, err := c.UseCase.Login(body)
	if err != nil {
		return err
	}

	c.setRefreshCookie(ctx, result.RefreshToken, result.RefreshExpiresAt)
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(result.Response)
}

func (c *AuthController) register(ctx fiber.Ctx) error {
	var body req.RegisterRequest
	if err := parseBody(ctx, &body); err != nil {
		return err
	}

	actorId, _ := ctx.Locals(middleware.CtxUserID).(string)
	user, err := c.UseCase.Register(actorId, body)
	if err != nil {
		return err
	}
	return ctx.Status(fiber.StatusCreated).JSON(user)
}

func (c *AuthController) refresh(ctx fiber.Ctx) error {
	result, err := c.UseCase.Refresh(ctx.Cookies(refreshCookieName))
	if err != nil {
		// Cookie hanya dihapus kalau sesinya memang tidak berlaku (401). Saat token baru saja diputar tab lain (409)
		// browser sudah menyimpan cookie barunya, dan saat server error (5xx) sesinya masih sah.
		var ge *exception.GetError
		if errors.As(err, &ge) && ge.Code == fiber.StatusUnauthorized {
			c.clearRefreshCookie(ctx)
		}
		return err
	}

	c.setRefreshCookie(ctx, result.RefreshToken, result.RefreshExpiresAt)
	ctx.Set("Cache-Control", "no-store")
	return ctx.JSON(result.Response)
}

// Cookie selalu dihapus, juga kalau pencabutan di database gagal: browser ini tetap keluar, errornya tercatat di log.
func (c *AuthController) logout(ctx fiber.Ctx) error {
	token := ctx.Cookies(refreshCookieName)
	c.clearRefreshCookie(ctx)
	if err := c.UseCase.Logout(token); err != nil {
		return err
	}
	return ctx.SendStatus(fiber.StatusNoContent)
}

// Secure=false karena server di LAN toko memakai HTTP. Kalau nanti memakai HTTPS, ubah ke true.
func (c *AuthController) setRefreshCookie(ctx fiber.Ctx, token string, expires time.Time) {
	ctx.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		Path:     refreshCookiePath,
		Expires:  expires,
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Strict",
	})
}

func (c *AuthController) clearRefreshCookie(ctx fiber.Ctx) {
	ctx.Cookie(&fiber.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		Path:     refreshCookiePath,
		Expires:  time.Unix(0, 0),
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Strict",
	})
}

func parseBody(ctx fiber.Ctx, out any) error {
	if err := json.Unmarshal(ctx.Body(), out); err != nil {
		return &exception.GetError{Code: fiber.StatusBadRequest, Message: "request body tidak valid"}
	}
	return nil
}

// NewAuthController Constructor
func NewAuthController(engine *fiber.App, useCase usecase.AuthUseCase, authenticate fiber.Handler) *AuthController {
	return &AuthController{
		Engine:       engine,
		UseCase:      useCase,
		Authenticate: authenticate,
	}
}
