package usecase

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
)

// Jenis request yang memakai kunci idempotensi (idempotency_keys.action).
const (
	actionSale             = "transaction.create"
	actionPreorderCreate   = "preorder.create"
	actionPreorderPayment  = "preorder.payment"
	actionPreorderComplete = "preorder.complete"
	actionProduction       = "product.production"
)

// reasonDuplicateRequest: kode error untuk kunci idempotensi yang dipakai lagi dengan isi yang berbeda. Client
// membuang kuncinya, supaya percobaan berikutnya dianggap request baru.
const reasonDuplicateRequest = "duplicate_request"

// requestKeyRetention: kunci idempotensi hanya dibutuhkan selama request yang sama mungkin dikirim ulang. Client
// menyimpan kuncinya sampai request berhasil atau tab-nya ditutup, jadi kunci yang lebih tua dari ini aman dihapus.
const requestKeyRetention = 30 * 24 * time.Hour

type (
	IdempotencyUseCase interface {
		// DeleteOldKeys menghapus kunci idempotensi yang lebih tua dari 30 hari, dijalankan server berkala.
		// Mengembalikan jumlah yang dihapus.
		DeleteOldKeys() (int64, error)
	}
	idempotencyUseCase struct {
		keys postgres.IdempotencyRepository
	}
)

func (u *idempotencyUseCase) DeleteOldKeys() (int64, error) {
	n, err := u.keys.DeleteBefore(time.Now().Add(-requestKeyRetention))
	if err != nil {
		return 0, internalError(err)
	}
	return n, nil
}

// requestKey menyiapkan kunci idempotensi dari header Idempotency-Key. Tanpa header (klien lama, skrip) hasilnya
// nil dan request diproses seperti biasa. payload berisi bagian request yang menentukan "request yang sama".
func requestKey(raw, userId, action string, payload any) (*postgres.RequestKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if !validUUID(raw) {
		return nil, badRequest("Idempotency-Key tidak valid (harus UUID)")
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, internalError(err)
	}
	sum := sha256.Sum256(body)
	return &postgres.RequestKey{
		Key:    strings.ToLower(raw),
		UserId: userId,
		Action: action,
		Hash:   hex.EncodeToString(sum[:]),
	}, nil
}

// earlierRequest memeriksa, sebelum request diproses, apakah kuncinya sudah dipakai request yang tersimpan.
// Diperiksa di awal (tidak hanya di transaksi database) supaya pengulangan request yang sudah tersimpan tetap
// mendapat hasil yang sama walaupun harga, stok, atau status sudah berubah sejak itu.
func earlierRequest(
	keys postgres.IdempotencyRepository, k *postgres.RequestKey,
) (*postgres.DuplicateRequestError, error) {
	if k == nil {
		return nil, nil
	}
	dup, err := keys.Find(*k)
	if err != nil {
		return nil, internalError(err)
	}
	return dup, nil
}

func duplicateRequest(message string) error {
	return &exception.GetError{Code: http.StatusConflict, Message: message, Reason: reasonDuplicateRequest}
}

// NewIdempotencyUseCase Constructor
func NewIdempotencyUseCase(keys postgres.IdempotencyRepository) IdempotencyUseCase {
	return &idempotencyUseCase{keys: keys}
}
