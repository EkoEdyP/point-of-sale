package res

import "time"

type PreorderItemResponse struct {
	Id          string `json:"id"`
	ProductId   string `json:"product_id"`
	ProductName string `json:"product_name"`
	UnitPrice   int64  `json:"unit_price"`
	Qty         int    `json:"qty"`
	LineTotal   int64  `json:"line_total"`
}

type PreorderPaymentResponse struct {
	Id            string    `json:"id"`
	Amount        int64     `json:"amount"`
	Method        string    `json:"method"`
	CreatedByName string    `json:"created_by_name"`
	CreatedAt     time.Time `json:"created_at"`
}

type PreorderResponse struct {
	Id            string     `json:"id"`
	OrderNo       string     `json:"order_no"`
	CustomerName  string     `json:"customer_name"`
	CustomerPhone *string    `json:"customer_phone"`
	DueDate       string     `json:"due_date"`
	DueTime       *string    `json:"due_time"`
	Fulfillment   string     `json:"fulfillment"`
	Address       *string    `json:"address"`
	Notes         *string    `json:"notes"`
	Status        string     `json:"status"`
	Subtotal      int64      `json:"subtotal"`       // harga barang
	ShippingFee   int64      `json:"shipping_fee"`   // ongkir
	OtherFee      int64      `json:"other_fee"`      // biaya lain, mis. kresek
	OtherFeeNote  *string    `json:"other_fee_note"` // keterangan biaya lain
	Total         int64      `json:"total"`          // subtotal + ongkir + biaya lain
	DownPayment   int64      `json:"down_payment"`
	Remaining     int64      `json:"remaining"`
	CreatedById   string     `json:"created_by_id"`
	CreatedByName string     `json:"created_by_name"`
	CreatedAt     time.Time  `json:"created_at"`
	TransactionId *string    `json:"transaction_id"`
	CancelledAt   *time.Time `json:"cancelled_at"`
	CancelReason  *string    `json:"cancel_reason"`
	// Pengiriman pre-order diantar (null kalau diambil). Selama statusnya pending, on_the_way, atau failed,
	// menyelesaikan pre-order ini ikut menandainya terkirim, dan nama penerimanya wajib dikirim (received_by).
	DeliveryNo     *string                   `json:"delivery_no"`
	DeliveryStatus *string                   `json:"delivery_status"`
	ReceivedBy     *string                   `json:"received_by"`
	Items          []PreorderItemResponse    `json:"items,omitempty"`    // tidak ada di daftar
	Payments       []PreorderPaymentResponse `json:"payments,omitempty"` // tidak ada di daftar
	// Version naik setiap kali pre-order berubah. Formulir ubah mengirimnya kembali, supaya perubahan dari data lama
	// ditolak.
	Version int `json:"version"`
}

type PreorderListResponse struct {
	Data  []PreorderResponse `json:"data"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
	Total int                `json:"total"`
}

type PreorderSummaryResponse struct {
	Overdue          int `json:"overdue"`
	Today            int `json:"today"`
	Tomorrow         int `json:"tomorrow"`
	DayAfterTomorrow int `json:"day_after_tomorrow"` // H-2
}

// PreorderProductionResponse: kebutuhan produksi dari pre-order yang belum diserahkan.
// Stok siap dipakai untuk hari H paling awal lebih dulu.
type PreorderProductionResponse struct {
	Days []ProductionDayResponse `json:"days"`
	// Products: total per produk untuk semua hari H, tidak ikut filter tanggal.
	Products []ProductionProductResponse `json:"products"`
	// DailyLimit: kuota pre-order per hari (pcs, semua produk PO digabung).
	DailyLimit int `json:"daily_limit"`
}

type ProductionDayResponse struct {
	DueDate string                   `json:"due_date"`
	Items   []ProductionItemResponse `json:"items"`
	// QuotaUsed: pcs yang sudah memakai kuota hari H ini, termasuk pesanan yang sudah diserahkan.
	QuotaUsed int `json:"quota_used"`
}

type ProductionItemResponse struct {
	ProductId   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Needed      int    `json:"needed"` // butuh
	Ready       int    `json:"ready"`  // sudah siap (bagian stok siap untuk hari H ini)
	Short       int    `json:"short"`  // kurang = butuh - siap
	Orders      int    `json:"orders"` // dari berapa pre-order
}

type ProductionProductResponse struct {
	ProductId   string `json:"product_id"`
	ProductName string `json:"product_name"`
	Stock       int    `json:"stock"` // stok siap saat ini
	Needed      int    `json:"needed"`
	Short       int    `json:"short"`
}

// PreorderQuotaResponse: kuota pre-order satu hari H, total pcs semua produk PO.
type PreorderQuotaResponse struct {
	DueDate    string `json:"due_date"`
	DailyLimit int    `json:"daily_limit"`
	Used       int    `json:"used"`      // semua pesanan kecuali yang dibatalkan
	Remaining  int    `json:"remaining"` // tidak pernah negatif, walau kuota diturunkan di bawah yang terpakai
}

type PreorderDailyLimitResponse struct {
	DailyLimit int `json:"daily_limit"`
}
