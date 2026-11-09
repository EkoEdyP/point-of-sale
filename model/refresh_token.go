package model

import "time"

type RefreshToken struct {
	Id         string
	UserId     string
	TokenHash  string
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	ReplacedBy *string
	CreatedAt  time.Time
}
