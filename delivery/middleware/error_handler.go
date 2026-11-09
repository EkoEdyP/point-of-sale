package middleware

import (
	"errors"
	"strconv"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
	"github.com/gofiber/fiber/v3"
)

// NewErrorHandler mengubah semua error menjadi JSON {"error": "...", "code": "..."} ("code" hanya kalau ada). Detail
// error 5xx tidak dikirim ke client; penyebabnya dicatat oleh middleware Logging di baris log request yang sama.
func NewErrorHandler() fiber.ErrorHandler {
	return func(ctx fiber.Ctx, err error) error {
		code, message, retryAfter := errorResponse(err)
		if retryAfter > 0 {
			ctx.Set(fiber.HeaderRetryAfter, strconv.Itoa(retryAfter))
		}
		body := fiber.Map{"error": message}
		var ge *exception.GetError
		if errors.As(err, &ge) && ge.Reason != "" && code == ge.Code {
			body["code"] = ge.Reason
		}
		return ctx.Status(code).JSON(body)
	}
}

// busyMessage: database sedang menolak sementara (timeout, antrean kunci, koneksi penuh). Client cukup mencoba lagi.
const busyMessage = "server sedang sibuk, coba lagi sebentar lagi"

// errorResponse menentukan status, pesan dan Retry-After yang diterima client untuk sebuah error.
func errorResponse(err error) (code int, message string, retryAfter int) {
	var ge *exception.GetError
	var fe *fiber.Error
	switch {
	case errors.As(err, &ge):
		if ge.Code >= 500 && postgres.IsBusy(err) {
			return fiber.StatusServiceUnavailable, busyMessage, 2
		}
		return ge.Code, ge.Message, ge.RetryAfter
	case errors.As(err, &fe):
		return fe.Code, fe.Message, 0
	case postgres.IsBusy(err):
		return fiber.StatusServiceUnavailable, busyMessage, 2
	default:
		return fiber.StatusInternalServerError, "internal server error", 0
	}
}

// errorCause: penyebab internal sebuah error untuk log (kosong untuk error biasa seperti validasi).
func errorCause(err error) string {
	var ge *exception.GetError
	var fe *fiber.Error
	switch {
	case errors.As(err, &ge):
		if ge.Err != "" {
			return ge.Err
		}
		if ge.Cause != nil {
			return ge.Cause.Error()
		}
		return ""
	case errors.As(err, &fe):
		return ""
	default:
		return err.Error()
	}
}
