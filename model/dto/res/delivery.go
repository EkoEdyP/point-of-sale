package res

import "time"

type DeliveryItemResponse struct {
	ProductName string `json:"product_name"`
	Qty         int    `json:"qty"`
	UnitPrice   int64  `json:"unit_price"`
	LineTotal   int64  `json:"line_total"`
}

type DeliveryResponse struct {
	Id         string `json:"id"`
	DeliveryNo string `json:"delivery_no"`
	SourceType string `json:"source_type"` // preorder | transaction
	SourceId   string `json:"source_id"`
	SourceNo   string `json:"source_no"`
	// Pemesan, alamat, jadwal, dan catatan pengiriman pre-order selalu sama dengan pre-order-nya.
	CustomerName  string     `json:"customer_name"`
	CustomerPhone *string    `json:"customer_phone"`
	Address       string     `json:"address"`
	ScheduledDate string     `json:"scheduled_date"`
	ScheduledTime *string    `json:"scheduled_time"`
	Courier       *string    `json:"courier"`
	Fee           int64      `json:"fee"`
	Notes         *string    `json:"notes"`
	Status        string     `json:"status"`
	StatusNote    *string    `json:"status_note"`
	DispatchedAt  *time.Time `json:"dispatched_at"`
	DeliveredAt   *time.Time `json:"delivered_at"`
	ReceivedBy    *string    `json:"received_by"` // nama penerima, diisi saat ditandai terkirim
	AmountDue     int64      `json:"amount_due"`
	// Rincian uang untuk surat jalan: total = subtotal + ongkir (fee) + biaya lain; bayar ke kurir = amount_due.
	Subtotal     int64   `json:"subtotal"`
	OtherFee     int64   `json:"other_fee"`
	OtherFeeNote *string `json:"other_fee_note"`
	Total        int64   `json:"total"`
	DownPayment  int64   `json:"down_payment"`
	// Status pre-order sumber (null untuk struk). Masih ordered/ready: tandai terkirim lewat
	// POST /preorders/:id/complete, yang sekaligus mencatat sisa pembayaran dan menandai pengiriman ini terkirim.
	PreorderStatus *string                `json:"preorder_status"`
	CreatedById    string                 `json:"created_by_id"`
	CreatedByName  string                 `json:"created_by_name"`
	CreatedAt      time.Time              `json:"created_at"`
	Items          []DeliveryItemResponse `json:"items,omitempty"` // tidak ada di daftar
	// Version naik setiap kali pengiriman berubah; dikirim kembali oleh formulir ubah (lihat PreorderResponse).
	Version int `json:"version"`
}

type DeliveryListResponse struct {
	Data  []DeliveryResponse `json:"data"`
	Page  int                `json:"page"`
	Limit int                `json:"limit"`
	Total int                `json:"total"`
}

type DeliverySummaryResponse struct {
	Overdue  int `json:"overdue"`
	Today    int `json:"today"`
	Tomorrow int `json:"tomorrow"`
}
