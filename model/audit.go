package model

import (
	"encoding/json"
	"time"
)

// AuditEntry: satu baris riwayat perubahan. Changes = {"kolom": [lama, baru]} dalam JSON.
type AuditEntry struct {
	Id          int64
	CreatedAt   time.Time
	ActorId     *string
	ActorName   *string
	Action      string
	Entity      string
	EntityId    string
	EntityLabel string
	Changes     json.RawMessage
	Note        *string
}
