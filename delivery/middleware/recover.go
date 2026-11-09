package middleware

import (
	"fmt"
	"runtime/debug"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/gofiber/fiber/v3"
)

// Recover mengubah panic di handler menjadi error 500 biasa, supaya satu bug tidak mematikan server untuk semua
// kasir. Middleware Logging mencatat stack trace-nya; client hanya menerima "internal server error".
func Recover() fiber.Handler {
	return func(ctx fiber.Ctx) (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = &exception.GetError{
					Code:    fiber.StatusInternalServerError,
					Message: "internal server error",
					Err:     fmt.Sprintf("panic %s %s: %v\n%s", ctx.Method(), ctx.Path(), r, debug.Stack()),
				}
			}
		}()
		return ctx.Next()
	}
}
