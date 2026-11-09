package req

// AuditListQuery: filter riwayat perubahan. Semua field opsional.
type AuditListQuery struct {
	Entity   string // product | preorder | delivery | user | transaction | setting
	EntityId string // satu data saja, misalnya satu produk
	ActorId  string // perubahan oleh satu pengguna
	From     string // YYYY-MM-DD, inklusif
	To       string // YYYY-MM-DD, inklusif
	Page     int
	Limit    int
}
