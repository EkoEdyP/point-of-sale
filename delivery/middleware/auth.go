package middleware

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"

	"github.com/EkoEdyP/point-of-sale/config"
	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/EkoEdyP/point-of-sale/utility/security"
	"github.com/gofiber/fiber/v3"
)

// Kunci untuk ctx.Locals
const (
	CtxUserID = "user_id"
	CtxRole   = "role"
	// Untuk log: username pelaku (diisi Authenticate) dan username yang dicoba di halaman login.
	CtxUsername  = "username"
	CtxLoginName = "login_name"
)

// Authenticate memeriksa header "Authorization: Bearer <access token>".
//
// JWT tetap berlaku sampai habis meskipun akunnya sudah dinonaktifkan, jadi status akun
// dicek ke database di setiap request (satu query by primary key). Dengan begitu akun yang
// dinonaktifkan langsung ditolak, tidak perlu menunggu access token kedaluwarsa.
func Authenticate(cfg config.JWTConfig, users postgres.UserRepository) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		scheme, token, found := strings.Cut(ctx.Get("Authorization"), " ")
		token = strings.TrimSpace(token)
		if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
			return &exception.GetError{Code: http.StatusUnauthorized, Message: "unauthorized"}
		}

		claims, err := security.ParseAccessToken(cfg, token)
		if err != nil {
			return &exception.GetError{Code: http.StatusUnauthorized, Message: "token tidak valid atau kedaluwarsa"}
		}

		user, err := users.FindById(claims.Subject)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return &exception.GetError{Code: http.StatusUnauthorized, Message: "akun tidak ditemukan"}
			}
			return exception.Internal(err)
		}
		if !user.Active {
			return &exception.GetError{Code: http.StatusUnauthorized, Message: "akun tidak aktif"}
		}

		ctx.Locals(CtxUserID, user.Id)
		ctx.Locals(CtxRole, user.Role)
		ctx.Locals(CtxUsername, user.Username)
		return ctx.Next()
	}
}

// RequireRole harus dipasang setelah Authenticate.
func RequireRole(roles ...string) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		role, _ := ctx.Locals(CtxRole).(string)
		if !slices.Contains(roles, role) {
			return &exception.GetError{Code: http.StatusForbidden, Message: "akses ditolak"}
		}
		return ctx.Next()
	}
}
