package req

// DeliveryUpdateRequest: hanya field yang dikirim yang berubah. Teks kosong menghapus isinya. Pengiriman pre-order:
// pemesan, alamat, jadwal, dan catatannya ikut diubah di pre-order-nya.
type DeliveryUpdateRequest struct {
	CustomerName  *string `json:"customer_name"`
	CustomerPhone *string `json:"customer_phone"`
	Address       *string `json:"address"`
	ScheduledDate *string `json:"scheduled_date"`
	ScheduledTime *string `json:"scheduled_time"`
	Courier       *string `json:"courier"`
	Fee           *int64  `json:"fee"`
	Notes         *string `json:"notes"`
	// Version: versi pengiriman saat formulirnya dibuka, seperti PreorderUpdateRequest.Version.
	Version *int `json:"version"`
}

type DeliveryDispatchRequest struct {
	Courier *string `json:"courier"`
}

type DeliveryReasonRequest struct {
	Reason string `json:"reason" validate:"required,max=200"`
}

type DeliveryListQuery struct {
	Status string // active (bawaan) | done | all
	Page   int
	Limit  int
}
