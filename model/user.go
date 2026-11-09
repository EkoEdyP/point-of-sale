package model

import "time"

const (
	RoleAdmin   = "admin"
	RoleCashier = "cashier"
)

type User struct {
	Id           string
	Name         string
	Username     string
	PasswordHash string
	Role         string
	Active       bool
	CreatedBy    *string // nil untuk admin pertama (seed)
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// UserPatch: field yang nil tidak diubah.
type UserPatch struct {
	Name         *string
	Active       *bool
	PasswordHash *string
}
