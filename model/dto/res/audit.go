package res

import (
	"encoding/json"
	"time"
)

// AuditEntryResponse: satu perubahan. Changes = {"kolom": [lama, baru]}; kolom password hanya berisi "diubah".
// EntityLabel = nama atau nomor data saat perubahan terjadi. ActorName kosong untuk perubahan tanpa pengguna.
type AuditEntryResponse struct {
	Id          int64           `json:"id"`
	CreatedAt   time.Time       `json:"created_at"`
	ActorId     *string         `json:"actor_id"`
	ActorName   *string         `json:"actor_name"`
	Action      string          `json:"action"`
	Entity      string          `json:"entity"`
	EntityId    string          `json:"entity_id"`
	EntityLabel string          `json:"entity_label"`
	Changes     json.RawMessage `json:"changes"`
	Note        *string         `json:"note"`
}

type AuditListResponse struct {
	Data  []AuditEntryResponse `json:"data"`
	Page  int                  `json:"page"`
	Limit int                  `json:"limit"`
	Total int                  `json:"total"`
}
