package postgres

import (
	"context"
	"database/sql"
	"sort"

	"github.com/EkoEdyP/point-of-sale/model"
)

type (
	ReportRepository interface {
		// DailySales mengembalikan ringkasan per hari (from dan to inklusif, format YYYY-MM-DD).
		// Hari tanpa transaksi tidak muncul di hasil. source: "store", "preorder", atau "all".
		DailySales(from, to, source string) ([]model.DailySales, error)
	}
	reportRepository struct {
		DB *sql.DB
	}
)

// transactionSourceFilter membatasi transaksi t menurut asalnya: penyerahan pre-order dikenali dari
// preorders.transaction_id (indeks unik), sisanya penjualan toko. Isinya konstanta, bukan input pengguna.
func transactionSourceFilter(source string) string {
	switch source {
	case model.SourceStore:
		return ` AND NOT EXISTS (SELECT 1 FROM "preorders" po WHERE po.transaction_id = t.id)`
	case model.SourcePreorder:
		return ` AND EXISTS (SELECT 1 FROM "preorders" po WHERE po.transaction_id = t.id)`
	}
	return ""
}

// Batas hari mengikuti zona waktu sesi database (APP_TIME_ZONE), karena timestamptz::date memakai zona sesi.
// Ketiga query membaca snapshot yang sama (REPEATABLE READ), supaya penjualan atau DP yang masuk di tengah
// perhitungan tidak membuat jumlah transaksi, barang terjual, dan DP saling tidak cocok.
func (r *reportRepository) DailySales(from, to, source string) ([]model.DailySales, error) {
	tx, err := r.DB.BeginTx(context.Background(), &sql.TxOptions{Isolation: sql.LevelRepeatableRead, ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()

	byDay := map[string]*model.DailySales{}
	get := func(day string) *model.DailySales {
		if s, ok := byDay[day]; ok {
			return s
		}
		s := &model.DailySales{Date: day}
		byDay[day] = s
		return s
	}

	// 1. Dari tabel transactions: jumlah, omzet, rincian metode bayar, dan void.
	rows, err := tx.Query(`
		SELECT to_char(t.created_at::date, 'YYYY-MM-DD'),
		  count(*) FILTER (WHERE t.status = 'completed'),
		  COALESCE(sum(t.total) FILTER (WHERE t.status = 'completed'), 0)::bigint,
		  COALESCE(sum(t.total - t.down_payment) FILTER (WHERE t.status = 'completed' AND t.payment_method = 'cash'), 0)::bigint,
		  COALESCE(sum(t.total - t.down_payment) FILTER (WHERE t.status = 'completed' AND t.payment_method = 'qris'), 0)::bigint,
		  COALESCE(sum(t.total - t.down_payment) FILTER (WHERE t.status = 'completed' AND t.payment_method = 'ewallet'), 0)::bigint,
		  COALESCE(sum(t.total - t.down_payment) FILTER (WHERE t.status = 'completed' AND t.payment_method = 'card'), 0)::bigint,
		  count(*) FILTER (WHERE t.status = 'void'),
		  COALESCE(sum(t.total) FILTER (WHERE t.status = 'void'), 0)::bigint
		FROM "transactions" t
		WHERE t.created_at >= $1::date AND t.created_at < ($2::date + 1)`+transactionSourceFilter(source)+`
		GROUP BY t.created_at::date`, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var day string
		var s model.DailySales
		if err := rows.Scan(
			&day,
			&s.TransactionCount,
			&s.Revenue,
			&s.Cash,
			&s.QRIS,
			&s.EWallet,
			&s.Card,
			&s.VoidCount,
			&s.VoidTotal,
		); err != nil {
			return nil, err
		}
		s.Date = day
		*get(day) = s
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 2. Dari transaction_items: barang terjual (transaksi selesai saja).
	itemRows, err := tx.Query(`
		SELECT to_char(t.created_at::date, 'YYYY-MM-DD'), COALESCE(sum(i.qty), 0)::bigint
		FROM "transaction_items" i
		JOIN "transactions" t ON t.id = i.transaction_id
		WHERE t.status = 'completed'
		  AND t.created_at >= $1::date AND t.created_at < ($2::date + 1)`+transactionSourceFilter(source)+`
		GROUP BY t.created_at::date`, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = itemRows.Close() }()

	for itemRows.Next() {
		var day string
		var itemsSold int64
		if err := itemRows.Scan(&day, &itemsSold); err != nil {
			return nil, err
		}
		get(day).ItemsSold = itemsSold
	}
	if err := itemRows.Err(); err != nil {
		return nil, err
	}

	// 3. DP pre-order yang diterima pada hari itu (menurut tanggal pencatatan DP), per metode bayar.
	// DP dari pre-order yang dibatalkan dipisahkan: uangnya harus dikembalikan, jadi bukan pemasukan.
	// Laporan toko tidak memuat DP sama sekali.
	if source == model.SourceStore {
		return sortedSales(byDay), nil
	}
	dpRows, err := tx.Query(`
		SELECT to_char(pp.created_at::date, 'YYYY-MM-DD'),
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status <> 'cancelled' AND pp.method = 'cash'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status <> 'cancelled' AND pp.method = 'qris'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status <> 'cancelled' AND pp.method = 'ewallet'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status <> 'cancelled' AND pp.method = 'card'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status = 'cancelled' AND pp.method = 'cash'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status = 'cancelled' AND pp.method = 'qris'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status = 'cancelled' AND pp.method = 'ewallet'), 0)::bigint,
		  COALESCE(sum(pp.amount) FILTER (WHERE po.status = 'cancelled' AND pp.method = 'card'), 0)::bigint
		FROM "preorder_payments" pp
		JOIN "preorders" po ON po.id = pp.preorder_id
		WHERE pp.created_at >= $1::date AND pp.created_at < ($2::date + 1)
		GROUP BY pp.created_at::date`, from, to)
	if err != nil {
		return nil, err
	}
	defer func() { _ = dpRows.Close() }()

	for dpRows.Next() {
		var day string
		var s model.DailySales
		if err := dpRows.Scan(
			&day,
			&s.DPCash, &s.DPQRIS, &s.DPEWallet, &s.DPCard,
			&s.DPCancelledCash, &s.DPCancelledQRIS, &s.DPCancelledEWallet, &s.DPCancelledCard,
		); err != nil {
			return nil, err
		}
		target := get(day)
		target.DPCash, target.DPQRIS, target.DPEWallet, target.DPCard = s.DPCash, s.DPQRIS, s.DPEWallet, s.DPCard
		target.DPCancelledCash, target.DPCancelledQRIS = s.DPCancelledCash, s.DPCancelledQRIS
		target.DPCancelledEWallet, target.DPCancelledCard = s.DPCancelledEWallet, s.DPCancelledCard
	}
	if err := dpRows.Err(); err != nil {
		return nil, err
	}
	return sortedSales(byDay), nil
}

func sortedSales(byDay map[string]*model.DailySales) []model.DailySales {
	result := make([]model.DailySales, 0, len(byDay))
	for _, s := range byDay {
		result = append(result, *s)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Date < result[j].Date })
	return result
}

// NewReportRepository Constructor
func NewReportRepository(db *sql.DB) ReportRepository {
	return &reportRepository{
		DB: db,
	}
}
