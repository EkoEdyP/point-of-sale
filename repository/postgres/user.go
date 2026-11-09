package postgres

import (
	"database/sql"

	"github.com/EkoEdyP/point-of-sale/model"
)

type (
	// Save dan Update dicatat di riwayat perubahan (audit_logs). Pelaku Save adalah user.CreatedBy; password hanya
	// dicatat bahwa diganti, tanpa isinya.
	UserRepository interface {
		Save(user model.User) error
		FindByUsername(username string) (model.User, error)
		FindById(id string) (model.User, error)
		List() ([]model.User, error)
		Update(id string, patch model.UserPatch, a Audit) (model.User, error)
	}
	userRepository struct {
		DB *sql.DB
	}
)

const userColumns = `id, name, username, password_hash, role, active, created_by, created_at, updated_at`

func (r *userRepository) Save(user model.User) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	SQL := `INSERT INTO "users"(id, name, username, password_hash, role, created_by) VALUES ($1, $2, $3, $4, $5, $6)`
	if _, err := tx.Exec(SQL,
		user.Id,
		user.Name,
		user.Username,
		user.PasswordHash,
		user.Role,
		user.CreatedBy,
	); err != nil {
		return err
	}
	after, err := snapshot(tx, auditUsers, user.Id, false)
	if err != nil {
		return err
	}
	var a Audit
	if user.CreatedBy != nil {
		a.ActorId = *user.CreatedBy
	}
	if err := recordChange(tx, auditUsers, "create", user.Id, nil, after, a); err != nil {
		return err
	}
	return tx.Commit()
}

// FindByUsername mencari tanpa membedakan huruf besar/kecil. Mengembalikan sql.ErrNoRows kalau tidak ada.
func (r *userRepository) FindByUsername(username string) (model.User, error) {
	SQL := `SELECT ` + userColumns + ` FROM "users" WHERE lower(username) = lower($1)`
	return scanUser(r.DB.QueryRow(SQL, username))
}

// FindById mengembalikan sql.ErrNoRows kalau tidak ada.
func (r *userRepository) FindById(id string) (model.User, error) {
	SQL := `SELECT ` + userColumns + ` FROM "users" WHERE id = $1`
	return scanUser(r.DB.QueryRow(SQL, id))
}

// List mengembalikan semua akun, admin dulu lalu kasir, diurutkan menurut username.
func (r *userRepository) List() ([]model.User, error) {
	SQL := `SELECT ` + userColumns + ` FROM "users" ORDER BY role ASC, lower(username) ASC`
	rows, err := r.DB.Query(SQL)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	users := []model.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return users, nil
}

// Update mengubah hanya field yang tidak nil. Mengembalikan sql.ErrNoRows kalau user tidak ada.
func (r *userRepository) Update(id string, patch model.UserPatch, a Audit) (model.User, error) {
	SQL := `UPDATE "users"
	        SET name = COALESCE($2, name),
	            active = COALESCE($3, active),
	            password_hash = COALESCE($4, password_hash)
	        WHERE id = $1
	        RETURNING ` + userColumns
	var updated model.User
	err := audited(r.DB, auditUsers, "update", id, a, func(tx *sql.Tx) error {
		var err error
		updated, err = scanUser(tx.QueryRow(SQL, id, patch.Name, patch.Active, patch.PasswordHash))
		return err
	})
	return updated, err
}

func scanUser(row rowScanner) (model.User, error) {
	var (
		user      model.User
		createdBy sql.NullString
	)
	err := row.Scan(
		&user.Id,
		&user.Name,
		&user.Username,
		&user.PasswordHash,
		&user.Role,
		&user.Active,
		&createdBy,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		return model.User{}, err
	}
	user.CreatedBy = nullToPtr(createdBy)
	return user, nil
}

// NewUserRepository Constructor
func NewUserRepository(db *sql.DB) UserRepository {
	return &userRepository{
		DB: db,
	}
}
