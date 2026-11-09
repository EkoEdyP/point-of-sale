package req

type TransactionItemRequest struct {
	ProductId string `json:"product_id" validate:"required,uuid"`
	Qty       int    `json:"qty" validate:"gt=0,lte=10000"`
}

// TransactionRequest: harga tidak dikirim client, server memakai harga dari database.
type TransactionRequest struct {
	Items         []TransactionItemRequest `json:"items" validate:"required,min=1,max=200,dive"`
	PaymentMethod string                   `json:"payment_method" validate:"required,oneof=cash qris ewallet card"`
	// Paid dipakai untuk cash. Untuk metode lain diabaikan (dianggap sama dengan total).
	Paid int64 `json:"paid" validate:"gte=0"`
	// ExpectedTotal (opsional): total yang tampil di layar kasir. Kalau berbeda dengan total dari harga terbaru,
	// penjualan ditolak 409 supaya kasir melihat harga yang baru dulu.
	ExpectedTotal *int64 `json:"expected_total"`
	// RequestKey diisi controller dari header Idempotency-Key, bukan dari body.
	RequestKey string `json:"-"`
}

type VoidRequest struct {
	Reason string `json:"reason" validate:"required,max=200"`
}

type TransactionListQuery struct {
	From   string // YYYY-MM-DD, inklusif
	To     string // YYYY-MM-DD, inklusif
	Status string // completed | void | kosong (semua)
	Page   int
	Limit  int
}
