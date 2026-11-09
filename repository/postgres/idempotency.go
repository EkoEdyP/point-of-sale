package postgres

import (
	"database/sql"
	"errors"
	"time"
)

// RequestKey: kunci idempotensi dari header Idempotency-Key. Kuncinya dicatat dalam transaksi database yang sama
// dengan datanya, jadi request yang dikirim ulang (misalnya karena jawabannya hilang di jaringan setelah data
// tersimpan) tidak menyimpan data untuk kedua kalinya.
type RequestKey struct {
	Key    string // UUID dari client
	UserId string
	Action string // jenis request, misalnya "transaction.create"
	Hash   string // sidik isi request yang sudah dirapikan
}

// DuplicateRequestError: kuncinya sudah dipakai request lain yang tersimpan. TargetId = data yang dibuat request
// itu. Same: pengguna, jenis, dan isi request-nya sama, jadi hasil yang tersimpan boleh dikembalikan lagi; kalau
// tidak sama, request ini berbeda dari yang tersimpan dan harus ditolak.
type DuplicateRequestError struct {
	Same     bool
	TargetId string
}

func (e *DuplicateRequestError) Error() string {
	if e.Same {
		return "request already processed: " + e.TargetId
	}
	return "idempotency key reused for a different request: " + e.TargetId
}

type (
	IdempotencyRepository interface {
		// Find mengembalikan request tersimpan yang memakai kunci k.Key, atau nil kalau kuncinya belum dipakai.
		Find(k RequestKey) (*DuplicateRequestError, error)
		// DeleteBefore menghapus kunci yang dicatat sebelum cutoff dan mengembalikan jumlahnya.
		DeleteBefore(cutoff time.Time) (int64, error)
	}
	idempotencyRepository struct {
		DB *sql.DB
	}
)

func (r *idempotencyRepository) Find(k RequestKey) (*DuplicateRequestError, error) {
	return findKey(r.DB, k)
}

func (r *idempotencyRepository) DeleteBefore(cutoff time.Time) (int64, error) {
	result, err := r.DB.Exec(`DELETE FROM "idempotency_keys" WHERE created_at < $1`, cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

type rowQuerier interface {
	QueryRow(query string, args ...any) *sql.Row
}

func findKey(q rowQuerier, k RequestKey) (*DuplicateRequestError, error) {
	var (
		userId, action, hash string
		target               sql.NullString
	)
	err := q.QueryRow(
		`SELECT user_id, action, request_hash, target_id FROM "idempotency_keys" WHERE request_key = $1`, k.Key,
	).Scan(&userId, &action, &hash, &target)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	same := userId == k.UserId && action == k.Action && hash == k.Hash
	return &DuplicateRequestError{Same: same, TargetId: target.String}, nil
}

// claimKey mencatat kunci di awal transaksi database (k nil = tanpa kunci). Request dengan kunci yang sama yang
// berjalan bersamaan menunggu di sini sampai transaksi pertama selesai, lalu mendapat *DuplicateRequestError; kalau
// transaksi pertama batal, request itu lanjut seperti biasa.
func claimKey(tx *sql.Tx, k *RequestKey) error {
	if k == nil {
		return nil
	}
	result, err := tx.Exec(
		`INSERT INTO "idempotency_keys"(request_key, user_id, action, request_hash) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (request_key) DO NOTHING`,
		k.Key, k.UserId, k.Action, k.Hash,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 1 {
		return nil
	}
	dup, err := findKey(tx, *k)
	if err != nil {
		return err
	}
	if dup == nil {
		return errors.New("idempotency key missing after conflict")
	}
	return dup
}

// finishKey menyimpan id data yang dibuat request pada kuncinya, untuk dikembalikan lagi kalau request diulang.
func finishKey(tx *sql.Tx, k *RequestKey, targetId string) error {
	if k == nil {
		return nil
	}
	_, err := tx.Exec(`UPDATE "idempotency_keys" SET target_id = $2 WHERE request_key = $1`, k.Key, targetId)
	return err
}

// NewIdempotencyRepository Constructor
func NewIdempotencyRepository(db *sql.DB) IdempotencyRepository {
	return &idempotencyRepository{DB: db}
}
