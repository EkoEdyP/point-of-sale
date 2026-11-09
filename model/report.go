package model

// DailySales adalah ringkasan penjualan satu hari. Transaksi void tidak masuk omzet.
type DailySales struct {
	Date               string // YYYY-MM-DD
	TransactionCount   int
	ItemsSold          int64
	Revenue            int64
	Cash               int64
	QRIS               int64
	EWallet            int64
	Card               int64
	DPCash             int64 // DP pre-order yang diterima hari itu
	DPQRIS             int64
	DPEWallet          int64
	DPCard             int64
	DPCancelledCash    int64 // DP dari pre-order yang dibatalkan (harus dikembalikan, bukan pemasukan)
	DPCancelledQRIS    int64
	DPCancelledEWallet int64
	DPCancelledCard    int64
	VoidCount          int
	VoidTotal          int64
}
