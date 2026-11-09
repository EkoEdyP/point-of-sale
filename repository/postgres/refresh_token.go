package postgres

import (
	"database/sql"
	"errors"

	"github.com/EkoEdyP/point-of-sale/model"
)

// ErrTokenAlreadyRevoked dikembalikan Rotate kalau token lama sudah dicabut
// (dipakai dua kali), yang dianggap tanda token dicuri.
var ErrTokenAlreadyRevoked = errors.New("refresh token already revoked")

type (
	RefreshTokenRepository interface {
		Save(token model.RefreshToken) error
		FindByHash(hash string) (model.RefreshToken, error)
		Rotate(oldId string, newToken model.RefreshToken) error
		RevokeByHash(hash string) error
		RevokeAllByUser(userId string) error
		// DeleteExpired menghapus token yang sudah kedaluwarsa dan mengembalikan jumlahnya.
		DeleteExpired() (int64, error)
	}
	refreshTokenRepository struct {
		DB *sql.DB
	}
)

const insertRefreshToken = `INSERT INTO "refresh_tokens"(id, user_id, token_hash, expires_at) VALUES ($1, $2, $3, $4)`

func (r *refreshTokenRepository) Save(token model.RefreshToken) error {
	_, err := r.DB.Exec(insertRefreshToken, token.Id, token.UserId, token.TokenHash, token.ExpiresAt)
	return err
}

// FindByHash mengembalikan sql.ErrNoRows kalau tidak ada.
func (r *refreshTokenRepository) FindByHash(hash string) (model.RefreshToken, error) {
	SQL := `SELECT id, user_id, token_hash, expires_at, revoked_at, replaced_by, created_at
	        FROM "refresh_tokens" WHERE token_hash = $1`

	var (
		token      model.RefreshToken
		revokedAt  sql.NullTime
		replacedBy sql.NullString
	)
	err := r.DB.QueryRow(SQL, hash).Scan(
		&token.Id,
		&token.UserId,
		&token.TokenHash,
		&token.ExpiresAt,
		&revokedAt,
		&replacedBy,
		&token.CreatedAt,
	)
	if err != nil {
		return model.RefreshToken{}, err
	}
	if revokedAt.Valid {
		v := revokedAt.Time
		token.RevokedAt = &v
	}
	if replacedBy.Valid {
		v := replacedBy.String
		token.ReplacedBy = &v
	}
	return token, nil
}

// Rotate menyimpan token baru dan mencabut token lama dalam satu transaksi.
func (r *refreshTokenRepository) Rotate(oldId string, newToken model.RefreshToken) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(insertRefreshToken, newToken.Id, newToken.UserId, newToken.TokenHash, newToken.ExpiresAt); err != nil {
		return err
	}

	result, err := tx.Exec(
		`UPDATE "refresh_tokens" SET revoked_at = now(), replaced_by = $1 WHERE id = $2 AND revoked_at IS NULL`,
		newToken.Id, oldId,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTokenAlreadyRevoked
	}
	return tx.Commit()
}

func (r *refreshTokenRepository) RevokeByHash(hash string) error {
	_, err := r.DB.Exec(`UPDATE "refresh_tokens" SET revoked_at = now() WHERE token_hash = $1 AND revoked_at IS NULL`, hash)
	return err
}

func (r *refreshTokenRepository) RevokeAllByUser(userId string) error {
	_, err := r.DB.Exec(`UPDATE "refresh_tokens" SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userId)
	return err
}

// Token yang kedaluwarsa tidak bisa dipakai lagi, dicabut atau tidak, jadi tidak perlu disimpan. Token lama yang
// menunjuknya lewat replaced_by kehilangan tautannya (ON DELETE SET NULL).
func (r *refreshTokenRepository) DeleteExpired() (int64, error) {
	result, err := r.DB.Exec(`DELETE FROM "refresh_tokens" WHERE expires_at < now()`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// NewRefreshTokenRepository Constructor
func NewRefreshTokenRepository(db *sql.DB) RefreshTokenRepository {
	return &refreshTokenRepository{
		DB: db,
	}
}
