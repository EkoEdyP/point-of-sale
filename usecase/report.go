package usecase

import (
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/model/dto/res"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
)

const maxReportDays = 366

type (
	ReportUseCase interface {
		Daily(from, to, source string) (res.DailyReportResponse, error)
	}
	reportUseCase struct {
		reports postgres.ReportRepository
	}
)

// reportAll: laporan gabungan toko dan pre-order.
const reportAll = "all"

// Daily: tanpa parameter berarti hari ini. Kalau hanya salah satu dari from/to diisi,
// hasilnya satu hari itu. Hari tanpa transaksi tetap muncul dengan nilai 0.
// source memisahkan laporan: store (penjualan di kasir), preorder (penyerahan pre-order beserta DP-nya),
// atau all/kosong (gabungan).
func (u *reportUseCase) Daily(from, to, source string) (res.DailyReportResponse, error) {
	const layout = "2006-01-02"
	from, to = strings.TrimSpace(from), strings.TrimSpace(to)

	switch source = strings.TrimSpace(source); source {
	case "":
		source = reportAll
	case reportAll, model.SourceStore, model.SourcePreorder:
		// valid
	default:
		return res.DailyReportResponse{}, badRequest("source tidak valid (store, preorder, atau all)")
	}

	switch {
	case from == "" && to == "":
		today := time.Now().Format(layout)
		from, to = today, today
	case to == "":
		to = from
	case from == "":
		from = to
	}

	// time.Parse menerima tahun 0000, tapi Postgres tidak punya tahun 0 (kueri gagal jadi 500).
	start, err := time.Parse(layout, from)
	if err != nil || start.Year() < 1 {
		return res.DailyReportResponse{}, badRequest("from tidak valid, gunakan format YYYY-MM-DD")
	}
	end, err := time.Parse(layout, to)
	if err != nil || end.Year() < 1 {
		return res.DailyReportResponse{}, badRequest("to tidak valid, gunakan format YYYY-MM-DD")
	}
	if end.Before(start) {
		return res.DailyReportResponse{}, badRequest("from tidak boleh setelah to")
	}
	if int(end.Sub(start).Hours()/24)+1 > maxReportDays {
		return res.DailyReportResponse{}, badRequest("rentang maksimal 366 hari")
	}

	sales, err := u.reports.DailySales(from, to, source)
	if err != nil {
		return res.DailyReportResponse{}, internalError(err)
	}
	byDate := make(map[string]model.DailySales, len(sales))
	for _, s := range sales {
		byDate[s.Date] = s
	}

	days := make([]res.DailyReportRow, 0, int(end.Sub(start).Hours()/24)+1)
	var total model.DailySales
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		key := d.Format(layout)
		s := byDate[key] // nilai nol kalau tidak ada transaksi
		s.Date = key
		days = append(days, res.DailyReportRow{Date: key, ReportSummary: toReportSummary(s)})
		total = addSales(total, s)
	}

	return res.DailyReportResponse{
		From:   from,
		To:     to,
		Source: source,
		Days:   days,
		Totals: toReportSummary(total),
	}, nil
}

func addSales(a, b model.DailySales) model.DailySales {
	return model.DailySales{
		TransactionCount:   a.TransactionCount + b.TransactionCount,
		ItemsSold:          a.ItemsSold + b.ItemsSold,
		Revenue:            a.Revenue + b.Revenue,
		Cash:               a.Cash + b.Cash,
		QRIS:               a.QRIS + b.QRIS,
		EWallet:            a.EWallet + b.EWallet,
		Card:               a.Card + b.Card,
		DPCash:             a.DPCash + b.DPCash,
		DPQRIS:             a.DPQRIS + b.DPQRIS,
		DPEWallet:          a.DPEWallet + b.DPEWallet,
		DPCard:             a.DPCard + b.DPCard,
		DPCancelledCash:    a.DPCancelledCash + b.DPCancelledCash,
		DPCancelledQRIS:    a.DPCancelledQRIS + b.DPCancelledQRIS,
		DPCancelledEWallet: a.DPCancelledEWallet + b.DPCancelledEWallet,
		DPCancelledCard:    a.DPCancelledCard + b.DPCancelledCard,
		VoidCount:          a.VoidCount + b.VoidCount,
		VoidTotal:          a.VoidTotal + b.VoidTotal,
	}
}

func toReportSummary(s model.DailySales) res.ReportSummary {
	var average int64
	if s.TransactionCount > 0 {
		average = s.Revenue / int64(s.TransactionCount)
	}
	return res.ReportSummary{
		TransactionCount:   s.TransactionCount,
		ItemsSold:          s.ItemsSold,
		Revenue:            s.Revenue,
		AverageTransaction: average,
		ByPaymentMethod: res.PaymentBreakdown{
			Cash:    s.Cash,
			QRIS:    s.QRIS,
			EWallet: s.EWallet,
			Card:    s.Card,
		},
		DPReceived: s.DPCash + s.DPQRIS + s.DPEWallet + s.DPCard,
		DPByPaymentMethod: res.PaymentBreakdown{
			Cash:    s.DPCash,
			QRIS:    s.DPQRIS,
			EWallet: s.DPEWallet,
			Card:    s.DPCard,
		},
		DPCancelled: s.DPCancelledCash + s.DPCancelledQRIS + s.DPCancelledEWallet + s.DPCancelledCard,
		DPCancelledByPaymentMethod: res.PaymentBreakdown{
			Cash:    s.DPCancelledCash,
			QRIS:    s.DPCancelledQRIS,
			EWallet: s.DPCancelledEWallet,
			Card:    s.DPCancelledCard,
		},
		VoidCount: s.VoidCount,
		VoidTotal: s.VoidTotal,
	}
}

// NewReportUseCase Constructor
func NewReportUseCase(reports postgres.ReportRepository) ReportUseCase {
	return &reportUseCase{
		reports: reports,
	}
}
