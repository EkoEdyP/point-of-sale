package middleware

import (
	"crypto/rand"
	"net"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/sirupsen/logrus"
)

// CtxRequestID: kunci ctx.Locals untuk id request, sama dengan header X-Request-ID di response.
const CtxRequestID = "request_id"

// slowRequest: request yang lebih lama dari ini dicatat sebagai warning, juga kalau berhasil.
const slowRequest = time.Second

// Logging mencatat satu baris per request: id request, metode, path, status, lama proses, alamat perangkat, dan
// pelakunya (username dan role kalau sudah login, atau username yang dicoba di halaman login). Error handler
// dipanggil di sini, jadi status yang tercatat sama dengan yang diterima client, dan penyebab error 5xx (termasuk
// stack trace panic) ada di baris yang sama.
//
// Level: 5xx error; 429, login gagal dan request lambat warning; 4xx lain (validasi, stok kurang, token kedaluwarsa)
// dan request yang mengubah data info; GET biasa debug, karena ringkasan di header dipanggil berkala oleh setiap
// tab.
func Logging(log *logrus.Logger) fiber.Handler {
	return func(ctx fiber.Ctx) error {
		start := time.Now()
		id := rand.Text()[:16]
		ctx.Locals(CtxRequestID, id)
		ctx.Set("X-Request-ID", id)

		chainErr := ctx.Next()
		if chainErr != nil {
			if err := ctx.App().ErrorHandler(ctx, chainErr); err != nil {
				_ = ctx.SendStatus(fiber.StatusInternalServerError)
			}
		}
		took := time.Since(start)
		status := ctx.Response().StatusCode()

		fields := logrus.Fields{
			"request_id": id,
			"method":     ctx.Method(),
			"path":       ctx.Path(),
			"status":     status,
			"took":       took.Round(100 * time.Microsecond).String(),
			"ip":         clientIP(ctx),
		}
		if name, ok := ctx.Locals(CtxUsername).(string); ok {
			fields["user"] = name
			fields["role"] = ctx.Locals(CtxRole)
		}
		login, isLogin := ctx.Locals(CtxLoginName).(string)
		if isLogin {
			fields["login"] = login
		}
		if chainErr != nil {
			_, message, _ := errorResponse(chainErr)
			fields["error"] = message
			if cause := errorCause(chainErr); cause != "" && status >= 500 {
				fields["cause"] = cause
			}
		}

		level := logrus.DebugLevel
		switch {
		case status >= 500:
			level = logrus.ErrorLevel
		case status == fiber.StatusTooManyRequests, isLogin && status == fiber.StatusUnauthorized, took >= slowRequest:
			level = logrus.WarnLevel
		case status >= 400, !readOnly(ctx.Method()):
			level = logrus.InfoLevel
		}
		log.WithFields(fields).Log(level, "request")
		return nil
	}
}

func readOnly(method string) bool {
	return method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions
}

// clientIP: alamat perangkat yang membuka aplikasi. Lewat proxy Vite (koneksi dari loopback) alamat itu ada di
// entri terakhir X-Forwarded-For, yang ditambahkan proxy sendiri; entri sebelumnya bisa diisi bebas oleh client.
// Header itu diabaikan kalau koneksinya tidak dari loopback.
func clientIP(ctx fiber.Ctx) string {
	peer := ctx.RequestCtx().RemoteIP()
	if peer.IsLoopback() {
		if xff := ctx.Get(fiber.HeaderXForwardedFor); xff != "" {
			last := strings.TrimSpace(xff[strings.LastIndex(xff, ",")+1:])
			if ip := net.ParseIP(last); ip != nil {
				return ip.String()
			}
		}
	}
	return peer.String()
}
