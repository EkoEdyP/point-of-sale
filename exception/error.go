package exception

import (
	"fmt"
	"net/http"
)

// GetError dikirim ke client sebagai {"error": Message} dengan status Code. Err dan Cause hanya untuk log, tidak
// pernah dikirim ke client. Cause menyimpan error aslinya supaya bisa diperiksa dengan errors.Is/As, misalnya timeout
// database yang dijawab 503. RetryAfter (detik, 0 = tidak ada) dikirim sebagai header Retry-After. Reason (opsional)
// dikirim sebagai "code" supaya client bisa bereaksi tanpa membaca pesannya, misalnya memuat ulang harga.
type GetError struct {
	Code       int    `json:"code"`
	Message    string `json:"message"`
	Err        string `json:"err"`
	Cause      error  `json:"-"`
	RetryAfter int    `json:"-"`
	Reason     string `json:"-"`
}

func (g *GetError) Error() string {
	return fmt.Sprintf("status code : %d message: %s err: %s", g.Code, g.Message, g.Err)
}

func (g *GetError) Unwrap() error {
	return g.Cause
}

// Internal membungkus error yang tidak terduga menjadi 500. Client hanya menerima pesan umum; penyebabnya dicatat
// di log.
func Internal(err error) *GetError {
	return &GetError{Code: http.StatusInternalServerError, Message: "internal server error", Err: err.Error(), Cause: err}
}
