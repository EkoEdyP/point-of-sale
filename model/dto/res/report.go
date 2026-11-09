package res

type PaymentBreakdown struct {
	Cash    int64 `json:"cash"`
	QRIS    int64 `json:"qris"`
	EWallet int64 `json:"ewallet"`
	Card    int64 `json:"card"`
}

type ReportSummary struct {
	TransactionCount           int              `json:"transaction_count"`
	ItemsSold                  int64            `json:"items_sold"`
	Revenue                    int64            `json:"revenue"`
	AverageTransaction         int64            `json:"average_transaction"`
	ByPaymentMethod            PaymentBreakdown `json:"by_payment_method"`
	DPReceived                 int64            `json:"dp_received"`
	DPByPaymentMethod          PaymentBreakdown `json:"dp_by_payment_method"`
	DPCancelled                int64            `json:"dp_cancelled"`
	DPCancelledByPaymentMethod PaymentBreakdown `json:"dp_cancelled_by_payment_method"`
	VoidCount                  int              `json:"void_count"`
	VoidTotal                  int64            `json:"void_total"`
}

type DailyReportRow struct {
	Date string `json:"date"`
	ReportSummary
}

type DailyReportResponse struct {
	From   string           `json:"from"`
	To     string           `json:"to"`
	Source string           `json:"source"` // store, preorder, atau all
	Days   []DailyReportRow `json:"days"`
	Totals ReportSummary    `json:"totals"`
}
