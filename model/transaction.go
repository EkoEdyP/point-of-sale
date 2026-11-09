package model

import "time"

const (
	StatusCompleted = "completed"
	StatusVoid      = "void"

	PaymentCash    = "cash"
	PaymentQRIS    = "qris"
	PaymentEWallet = "ewallet"
	PaymentCard    = "card"

	// Sumber transaksi: penjualan di kasir, atau penyerahan pre-order.
	SourceStore    = "store"
	SourcePreorder = "preorder"
)

type Transaction struct {
	Id            string
	ReceiptNo     string
	CashierId     string
	CashierName   string
	Subtotal      int64
	Discount      int64
	ShippingFee   int64   // ongkir pre-order (penjualan toko selalu 0)
	OtherFee      int64   // biaya lain pre-order, mis. kresek
	OtherFeeNote  *string // keterangan biaya lain
	Total         int64   // subtotal - diskon + ongkir + biaya lain
	Paid          int64
	ChangeAmount  int64
	DownPayment   int64 // bagian total yang sudah dibayar lewat DP pre-order
	PaymentMethod string
	Status        string
	VoidedBy      *string
	VoidedByName  *string // nama akun yang membatalkan, saat dibaca
	VoidedAt      *time.Time
	VoidReason    *string
	CreatedAt     time.Time
	PreorderId    *string // terisi kalau transaksi ini dari penyerahan pre-order
	PreorderNo    *string
	Items         []TransactionItem
}

type TransactionItem struct {
	Id            string
	TransactionId string
	ProductId     string
	ProductName   string // snapshot saat transaksi
	UnitPrice     int64  // snapshot saat transaksi
	Qty           int
	Discount      int64
	LineTotal     int64
}
