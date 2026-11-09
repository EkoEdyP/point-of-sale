package res

import "time"

type TransactionItemResponse struct {
	Id          string `json:"id"`
	ProductId   string `json:"product_id"`
	ProductName string `json:"product_name"`
	UnitPrice   int64  `json:"unit_price"`
	Qty         int    `json:"qty"`
	Discount    int64  `json:"discount"`
	LineTotal   int64  `json:"line_total"`
}

type TransactionResponse struct {
	Id            string                    `json:"id"`
	ReceiptNo     string                    `json:"receipt_no"`
	CashierId     string                    `json:"cashier_id"`
	CashierName   string                    `json:"cashier_name"`
	Subtotal      int64                     `json:"subtotal"`
	Discount      int64                     `json:"discount"`
	ShippingFee   int64                     `json:"shipping_fee"`   // ongkir pre-order
	OtherFee      int64                     `json:"other_fee"`      // biaya lain pre-order
	OtherFeeNote  *string                   `json:"other_fee_note"` // keterangan biaya lain
	Total         int64                     `json:"total"`          // subtotal - diskon + ongkir + biaya lain
	Paid          int64                     `json:"paid"`
	Change        int64                     `json:"change"`
	DownPayment   int64                     `json:"down_payment"`
	PaymentMethod string                    `json:"payment_method"`
	Status        string                    `json:"status"`
	VoidedBy      *string                   `json:"voided_by"`
	VoidedByName  *string                   `json:"voided_by_name"` // akun yang membatalkan, dari sesi login-nya
	VoidedAt      *time.Time                `json:"voided_at"`
	VoidReason    *string                   `json:"void_reason"`
	CreatedAt     time.Time                 `json:"created_at"`
	Source        string                    `json:"source"`      // store (kasir) atau preorder
	PreorderId    *string                   `json:"preorder_id"` // terisi kalau source = preorder
	PreorderNo    *string                   `json:"preorder_no"`
	Items         []TransactionItemResponse `json:"items,omitempty"` // tidak ada di daftar
}

type TransactionListResponse struct {
	Data  []TransactionResponse `json:"data"`
	Page  int                   `json:"page"`
	Limit int                   `json:"limit"`
	Total int                   `json:"total"`
}
