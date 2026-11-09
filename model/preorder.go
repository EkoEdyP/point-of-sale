package model

import "time"

const (
	PreorderOrdered   = "ordered"
	PreorderReady     = "ready"
	PreorderCompleted = "completed"
	PreorderCancelled = "cancelled"

	FulfillmentPickup   = "pickup"
	FulfillmentDelivery = "delivery"
)

type Preorder struct {
	Id            string
	OrderNo       string
	CustomerName  string
	CustomerPhone *string
	DueDate       string  // YYYY-MM-DD
	DueTime       *string // HH:MM
	Fulfillment   string
	Address       *string
	Notes         *string
	Status        string
	Subtotal      int64 // harga barang
	ShippingFee   int64 // ongkir, hanya untuk pesanan diantar
	OtherFee      int64 // biaya lain, mis. kresek
	OtherFeeNote  *string
	Total         int64 // yang harus dibayar pelanggan: subtotal + ongkir + biaya lain
	DownPayment   int64 // jumlah semua DP (dihitung dari preorder_payments)
	CreatedBy     string
	CreatedByName string
	CreatedAt     time.Time
	TransactionId *string
	CancelledAt   *time.Time
	CancelReason  *string
	Version       int // naik setiap kali barisnya berubah
	// Pengiriman pre-order diantar (nil kalau diambil): nomor, status, dan nama penerimanya setelah terkirim.
	DeliveryNo     *string
	DeliveryStatus *string
	ReceivedBy     *string
	Items          []PreorderItem
	Payments       []PreorderPayment
}

type PreorderItem struct {
	Id          string
	PreorderId  string
	ProductId   string
	ProductName string
	UnitPrice   int64
	Qty         int
	LineTotal   int64
}

type PreorderPayment struct {
	Id            string
	PreorderId    string
	Amount        int64
	Method        string
	CreatedBy     string
	CreatedByName string
	CreatedAt     time.Time
}

// PreorderSummary: jumlah pre-order yang belum selesai menurut kedekatan hari H.
// Peringatan pre-order dimulai H-2 (lusa), lebih awal dari pengiriman (H-1).
type PreorderSummary struct {
	Overdue          int
	Today            int
	Tomorrow         int
	DayAfterTomorrow int // H-2
}

// ProductionNeed: jumlah satu produk yang dibutuhkan pre-order berjalan (dipesan atau siap)
// pada satu hari H. Stock adalah stok siap produk itu saat ini, sama untuk semua hari H.
type ProductionNeed struct {
	DueDate     string // YYYY-MM-DD
	ProductId   string
	ProductName string
	Stock       int
	Needed      int
	Orders      int // jumlah pre-order yang memuat produk ini
}
