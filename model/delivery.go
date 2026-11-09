package model

import "time"

const (
	DeliveryPending   = "pending"
	DeliveryOnTheWay  = "on_the_way"
	DeliveryDelivered = "delivered"
	DeliveryFailed    = "failed"
	DeliveryCancelled = "cancelled"
)

type Delivery struct {
	Id            string
	DeliveryNo    string
	PreorderId    *string
	TransactionId *string
	SourceNo      string // nomor pre-order atau nomor struk
	// Pemesan, alamat, jadwal, dan catatan pengiriman pre-order selalu sama dengan pre-order-nya.
	CustomerName  string
	CustomerPhone *string
	Address       string
	ScheduledDate string  // YYYY-MM-DD
	ScheduledTime *string // HH:MM
	Courier       *string
	Fee           int64
	Notes         *string
	Status        string
	StatusNote    *string
	DispatchedAt  *time.Time
	DeliveredAt   *time.Time
	ReceivedBy    *string // nama orang yang menerima barangnya, diisi saat ditandai terkirim
	AmountDue     int64   // sisa tagihan pre-order yang belum dilunasi (0 untuk penjualan biasa)
	// Rincian uang sumbernya untuk surat jalan (pre-order; struk untuk data lama): Total = Subtotal + ongkir (Fee)
	// + biaya lain, DownPayment = DP yang sudah diterima. Bayar ke kurir = AmountDue.
	Subtotal     int64
	OtherFee     int64
	OtherFeeNote *string
	Total        int64
	DownPayment  int64
	// PreorderStatus: status pre-order sumbernya (nil untuk penjualan biasa). Selama pre-order masih berjalan,
	// pengiriman ini ditandai terkirim dengan menyelesaikan pre-order-nya, supaya keduanya selesai bersama.
	PreorderStatus *string
	CreatedBy      string
	CreatedByName  string
	CreatedAt      time.Time
	Version        int // naik setiap kali barisnya berubah
	Items          []DeliveryItem
}

type DeliveryItem struct {
	ProductName string
	Qty         int
	UnitPrice   int64 // harga yang dikunci di pre-order (atau struk)
	LineTotal   int64
}

// DeliverySummary: pengiriman yang belum selesai menurut kedekatan jadwal.
type DeliverySummary struct {
	Overdue  int
	Today    int
	Tomorrow int
}
