package req

type PreorderItemRequest struct {
	ProductId string `json:"product_id" validate:"required,uuid"`
	Qty       int    `json:"qty" validate:"gt=0,lte=10000"`
}

// PreorderRequest: harga tidak dikirim client, server memakai harga produk saat pre-order dibuat.
// Ongkir hanya untuk pesanan diantar. Biaya lain (mis. kresek) boleh diberi keterangan.
type PreorderRequest struct {
	CustomerName      string                `json:"customer_name" validate:"required,max=100"`
	CustomerPhone     *string               `json:"customer_phone"`
	DueDate           string                `json:"due_date" validate:"required"` // YYYY-MM-DD
	DueTime           *string               `json:"due_time"`                     // HH:MM
	Fulfillment       string                `json:"fulfillment" validate:"required,oneof=pickup delivery"`
	Address           *string               `json:"address"`
	Notes             *string               `json:"notes"`
	Items             []PreorderItemRequest `json:"items" validate:"required,min=1,max=200,dive"`
	ShippingFee       int64                 `json:"shipping_fee" validate:"gte=0,lte=100000000"`
	OtherFee          int64                 `json:"other_fee" validate:"gte=0,lte=100000000"`
	OtherFeeNote      *string               `json:"other_fee_note"`
	DownPayment       int64                 `json:"down_payment" validate:"gte=0"`
	DownPaymentMethod string                `json:"down_payment_method" validate:"omitempty,oneof=cash qris ewallet card"`
	// ExpectedTotal (opsional): total yang tampil di form. Kalau berbeda dengan total dari harga terbaru, pre-order
	// ditolak 409 supaya harga yang baru terlihat dulu.
	ExpectedTotal *int64 `json:"expected_total"`
	// RequestKey diisi controller dari header Idempotency-Key, bukan dari body.
	RequestKey string `json:"-"`
}

type PreorderPaymentRequest struct {
	Amount     int64  `json:"amount" validate:"gt=0,lte=999999999999"` // 12 digit, sama dengan kolom DP di layar
	Method     string `json:"method" validate:"required,oneof=cash qris ewallet card"`
	RequestKey string `json:"-"`
}

// PreorderUpdateRequest: hanya field yang dikirim yang berubah. customer_phone, due_time, notes, atau
// other_fee_note berisi "" berarti dikosongkan. Mengubah ongkir atau biaya lain menghitung ulang total dan sisa
// tagihan. Pengiriman yang masih berjalan ikut berubah (pemesan, alamat, jadwal, catatan).
type PreorderUpdateRequest struct {
	CustomerName  *string `json:"customer_name"`
	CustomerPhone *string `json:"customer_phone"`
	Address       *string `json:"address"` // hanya pre-order diantar
	DueDate       *string `json:"due_date"`
	DueTime       *string `json:"due_time"`
	Notes         *string `json:"notes"`
	ShippingFee   *int64  `json:"shipping_fee"`
	OtherFee      *int64  `json:"other_fee"`
	OtherFeeNote  *string `json:"other_fee_note"`
	// Version: versi pre-order saat formulirnya dibuka. Kalau pre-order sudah diubah sejak itu, perubahan ditolak
	// (409 stale). Tidak dikirim = tidak dicek (skrip, client lama).
	Version *int `json:"version"`
}

type PreorderCompleteRequest struct {
	PaymentMethod string `json:"payment_method" validate:"omitempty,oneof=cash qris ewallet card"`
	Paid          int64  `json:"paid" validate:"gte=0"`
	// ReceivedBy: nama orang yang menerima barangnya. Wajib selama pengiriman pre-order ini masih berjalan, karena
	// pengiriman itu ikut ditandai terkirim.
	ReceivedBy string `json:"received_by"`
	RequestKey string `json:"-"`
}

type PreorderCancelRequest struct {
	Reason string `json:"reason" validate:"required,max=200"`
}

// PreorderDailyLimitRequest: kuota pre-order per hari, total pcs semua produk PO.
type PreorderDailyLimitRequest struct {
	DailyLimit int `json:"daily_limit" validate:"gte=1,lte=1000000"`
}

type PreorderListQuery struct {
	Status  string // open (bawaan) | completed | cancelled | all
	DueDate string // YYYY-MM-DD, kosong = semua tanggal
	Page    int
	Limit   int
}
