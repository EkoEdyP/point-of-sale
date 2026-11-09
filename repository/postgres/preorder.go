package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/lib/pq"
)

var (
	// ErrPreorderClosed: pre-order sudah selesai atau dibatalkan (atau statusnya berubah di tengah jalan).
	ErrPreorderClosed = errors.New("preorder is not open")
	// ErrPreorderOverpaid: total DP akan melebihi total pesanan.
	ErrPreorderOverpaid = errors.New("down payment exceeds total")
	// ErrPreorderChanged: DP berubah di antara pengecekan dan penyelesaian.
	ErrPreorderChanged = errors.New("preorder changed")
	// ErrReceiverRequired: pengiriman yang ikut ditandai terkirim butuh nama penerimanya.
	ErrReceiverRequired = errors.New("delivery receiver is required")
)

// QuotaExceededError: pcs pre-order pada satu hari H akan melebihi kuota harian.
type QuotaExceededError struct {
	DueDate   string // YYYY-MM-DD
	Limit     int    // kuota per hari
	Used      int    // sudah dipakai pesanan lain
	Requested int    // pcs pesanan ini
}

func (e *QuotaExceededError) Error() string {
	return fmt.Sprintf("preorder quota exceeded on %s: used %d + requested %d > limit %d",
		e.DueDate, e.Used, e.Requested, e.Limit)
}

type PreorderFilter struct {
	Statuses  []string // kosong = semua status
	DueDate   string   // YYYY-MM-DD, kosong = semua tanggal
	SortByDue bool     // true: hari H terdekat dulu; false: terbaru dulu
	Limit     int
	Offset    int
}

// PreorderChange: nilai baru data pelanggan, jadwal, catatan, dan biaya tambahan. Semua field ditulis apa adanya,
// jadi isi field yang tidak berubah dengan nilai lamanya. Total dihitung ulang dari subtotal + biaya.
type PreorderChange struct {
	CustomerName  string
	CustomerPhone *string
	Address       *string // hanya pre-order diantar
	DueDate       string  // YYYY-MM-DD
	DueTime       *string
	Notes         *string
	ShippingFee   int64
	OtherFee      int64
	OtherFeeNote  *string
	Version       int // versi baris yang menjadi dasar perubahan ini
}

type (
	// Penulisan yang menerima k *RequestKey (boleh nil) mengembalikan *DuplicateRequestError kalau kuncinya sudah
	// dipakai. Perubahan pada pre-order yang sudah ada dicatat di riwayat perubahan (audit_logs).
	PreorderRepository interface {
		// Create mengembalikan id pre-order. dp boleh nil.
		// *QuotaExceededError kalau pcs-nya tidak muat di kuota hari H.
		Create(preorder model.Preorder, dp *model.PreorderPayment, k *RequestKey) (string, error)
		FindById(id string) (model.Preorder, error)
		List(filter PreorderFilter) ([]model.Preorder, int, error)
		Summary() (model.PreorderSummary, error)
		// ProductionNeeds: kebutuhan per hari H dan produk, urut hari H lalu nama produk.
		ProductionNeeds() ([]model.ProductionNeed, error)
		AddPayment(id string, payment model.PreorderPayment, k *RequestKey) error
		// Update: *QuotaExceededError kalau hari H dipindah ke tanggal yang kuotanya tidak cukup,
		// ErrPreorderOverpaid kalau total baru lebih kecil dari DP yang sudah diterima. Pengiriman yang masih berjalan
		// ikut disamakan (pemesan, alamat, jadwal, catatan).
		Update(id string, change PreorderChange, a Audit) error
		MarkReady(id string, a Audit) error
		Cancel(id, cancelledBy, reason string) error
		// Complete membuat transaksi dari pre-order dan menandainya selesai, dalam satu DB transaction. Pengiriman
		// yang masih berjalan ikut terkirim, diterima oleh receivedBy (ErrReceiverRequired kalau nil).
		// ErrPreorderChanged kalau DP atau total pre-order (t.Total) berubah sejak dibaca.
		Complete(
			id string, expectedDownPayment int64, receivedBy *string, t model.Transaction, k *RequestKey,
		) (model.Transaction, error)
		// DailyLimit: kuota pre-order per hari (pcs, semua produk PO digabung).
		DailyLimit() (int, error)
		SetDailyLimit(limit int, updatedBy string) error
		// QuotaUsed: pcs terpakai per hari H (semua status kecuali dibatalkan). Hari tanpa pesanan tidak ada di map.
		// excludeId ("" = tidak ada) tidak ikut dihitung.
		QuotaUsed(dates []string, excludeId string) (map[string]int, error)
	}
	preorderRepository struct {
		DB *sql.DB
	}
)

const insertPreorderPayment = `INSERT INTO "preorder_payments"(preorder_id, amount, method, created_by)
	VALUES ($1, $2, $3, $4)`

const preorderSelect = `SELECT p.id, p.order_no, p.customer_name, p.customer_phone,
	to_char(p.due_date, 'YYYY-MM-DD'), to_char(p.due_time, 'HH24:MI'),
	p.fulfillment, p.address, p.notes, p.status,
	p.subtotal, p.shipping_fee, p.other_fee, p.other_fee_note, p.total,
	COALESCE((SELECT sum(pp.amount) FROM "preorder_payments" pp WHERE pp.preorder_id = p.id), 0)::bigint,
	p.created_by, u.name, p.created_at, p.transaction_id, p.cancelled_at, p.cancel_reason, p.version,
	dl.delivery_no, dl.status, dl.received_by
	FROM "preorders" p JOIN "users" u ON u.id = p.created_by
	LEFT JOIN LATERAL (SELECT d.delivery_no, d.status, d.received_by FROM "deliveries" d
	                   WHERE d.preorder_id = p.id) dl ON true`

func scanPreorder(row rowScanner) (model.Preorder, error) {
	var (
		p                                                    model.Preorder
		phone, dueTime, address, notes, txId, cancelReason   sql.NullString
		otherFeeNote, deliveryNo, deliveryStatus, receivedBy sql.NullString
		cancelledAt                                          sql.NullTime
	)
	err := row.Scan(
		&p.Id,
		&p.OrderNo,
		&p.CustomerName,
		&phone,
		&p.DueDate,
		&dueTime,
		&p.Fulfillment,
		&address,
		&notes,
		&p.Status,
		&p.Subtotal,
		&p.ShippingFee,
		&p.OtherFee,
		&otherFeeNote,
		&p.Total,
		&p.DownPayment,
		&p.CreatedBy,
		&p.CreatedByName,
		&p.CreatedAt,
		&txId,
		&cancelledAt,
		&cancelReason,
		&p.Version,
		&deliveryNo,
		&deliveryStatus,
		&receivedBy,
	)
	if err != nil {
		return model.Preorder{}, err
	}
	p.DeliveryNo = nullToPtr(deliveryNo)
	p.DeliveryStatus = nullToPtr(deliveryStatus)
	p.ReceivedBy = nullToPtr(receivedBy)
	p.CustomerPhone = nullToPtr(phone)
	p.DueTime = nullToPtr(dueTime)
	p.Address = nullToPtr(address)
	p.Notes = nullToPtr(notes)
	p.OtherFeeNote = nullToPtr(otherFeeNote)
	p.TransactionId = nullToPtr(txId)
	p.CancelReason = nullToPtr(cancelReason)
	if cancelledAt.Valid {
		v := cancelledAt.Time
		p.CancelledAt = &v
	}
	return p, nil
}

func (r *preorderRepository) Create(p model.Preorder, dp *model.PreorderPayment, k *RequestKey) (string, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimKey(tx, k); err != nil {
		return "", err
	}
	qty := 0
	for _, item := range p.Items {
		qty += item.Qty
	}
	if err := checkQuota(tx, p.DueDate, "", qty); err != nil {
		return "", err
	}

	var id, orderNo string
	err = tx.QueryRow(
		`INSERT INTO "preorders"(customer_name, customer_phone, due_date, due_time, fulfillment, address, notes,
		   subtotal, shipping_fee, other_fee, other_fee_note, total, created_by)
		 VALUES ($1, $2, $3::date, $4::time, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		 RETURNING id, order_no`,
		p.CustomerName, p.CustomerPhone, p.DueDate, p.DueTime, p.Fulfillment, p.Address, p.Notes,
		p.Subtotal, p.ShippingFee, p.OtherFee, p.OtherFeeNote, p.Total, p.CreatedBy,
	).Scan(&id, &orderNo)
	if err != nil {
		return "", err
	}

	for _, item := range p.Items {
		if _, err := tx.Exec(
			`INSERT INTO "preorder_items"(preorder_id, product_id, product_name, unit_price, qty, line_total)
			 VALUES ($1, $2, $3, $4, $5, $6)`,
			id, item.ProductId, item.ProductName, item.UnitPrice, item.Qty, item.LineTotal,
		); err != nil {
			return "", err
		}
	}

	if dp != nil {
		if _, err := tx.Exec(insertPreorderPayment, id, dp.Amount, dp.Method, dp.CreatedBy); err != nil {
			return "", err
		}
		if err := recordPayment(tx, id, orderNo, *dp, 0, p.Total); err != nil {
			return "", err
		}
	}

	if p.Fulfillment == model.FulfillmentDelivery {
		if _, err := tx.Exec(
			`INSERT INTO "deliveries"(preorder_id, customer_name, customer_phone, address, scheduled_date,
			   scheduled_time, notes, created_by)
			 VALUES ($1, $2, $3, $4, $5::date, $6::time, $7, $8)`,
			id, p.CustomerName, p.CustomerPhone, p.Address, p.DueDate, p.DueTime, p.Notes, p.CreatedBy,
		); err != nil {
			return "", err
		}
	}

	if err := finishKey(tx, k, id); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return id, nil
}

// FindById mengembalikan pre-order lengkap dengan item dan DP. sql.ErrNoRows kalau tidak ada.
func (r *preorderRepository) FindById(id string) (model.Preorder, error) {
	p, err := scanPreorder(r.DB.QueryRow(preorderSelect+` WHERE p.id = $1`, id))
	if err != nil {
		return model.Preorder{}, err
	}

	itemRows, err := r.DB.Query(
		`SELECT id, preorder_id, product_id, product_name, unit_price, qty, line_total
		 FROM "preorder_items" WHERE preorder_id = $1 ORDER BY product_name, id`,
		id,
	)
	if err != nil {
		return model.Preorder{}, err
	}
	defer func() { _ = itemRows.Close() }()

	p.Items = []model.PreorderItem{}
	for itemRows.Next() {
		var item model.PreorderItem
		if err := itemRows.Scan(
			&item.Id,
			&item.PreorderId,
			&item.ProductId,
			&item.ProductName,
			&item.UnitPrice,
			&item.Qty,
			&item.LineTotal,
		); err != nil {
			return model.Preorder{}, err
		}
		p.Items = append(p.Items, item)
	}
	if err := itemRows.Err(); err != nil {
		return model.Preorder{}, err
	}

	paymentRows, err := r.DB.Query(
		`SELECT pp.id, pp.preorder_id, pp.amount, pp.method, pp.created_by, u.name, pp.created_at
		 FROM "preorder_payments" pp JOIN "users" u ON u.id = pp.created_by
		 WHERE pp.preorder_id = $1 ORDER BY pp.created_at, pp.id`,
		id,
	)
	if err != nil {
		return model.Preorder{}, err
	}
	defer func() { _ = paymentRows.Close() }()

	p.Payments = []model.PreorderPayment{}
	for paymentRows.Next() {
		var pay model.PreorderPayment
		if err := paymentRows.Scan(
			&pay.Id,
			&pay.PreorderId,
			&pay.Amount,
			&pay.Method,
			&pay.CreatedBy,
			&pay.CreatedByName,
			&pay.CreatedAt,
		); err != nil {
			return model.Preorder{}, err
		}
		p.Payments = append(p.Payments, pay)
	}
	if err := paymentRows.Err(); err != nil {
		return model.Preorder{}, err
	}
	return p, nil
}

// List mengembalikan pre-order tanpa item dan DP rinci.
func (r *preorderRepository) List(f PreorderFilter) ([]model.Preorder, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	if len(f.Statuses) > 0 {
		args = append(args, pq.Array(f.Statuses))
		conds = append(conds, fmt.Sprintf("p.status = ANY($%d)", len(args)))
	}
	if f.DueDate != "" {
		args = append(args, f.DueDate)
		conds = append(conds, fmt.Sprintf("p.due_date = $%d::date", len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.DB.QueryRow(`SELECT count(*) FROM "preorders" p`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := ` ORDER BY p.created_at DESC, p.id DESC`
	if f.SortByDue {
		order = ` ORDER BY p.due_date ASC, p.due_time ASC NULLS LAST, p.created_at ASC`
	}

	n := len(args)
	args = append(args, f.Limit, f.Offset)
	SQL := fmt.Sprintf(`%s%s%s LIMIT $%d OFFSET $%d`, preorderSelect, where, order, n+1, n+2)
	rows, err := r.DB.Query(SQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	preorders := []model.Preorder{}
	for rows.Next() {
		p, err := scanPreorder(rows)
		if err != nil {
			return nil, 0, err
		}
		preorders = append(preorders, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return preorders, total, nil
}

// Summary menghitung pre-order yang belum selesai: terlambat, hari ini, besok, dan lusa (H-2).
// Hari mengikuti zona waktu sesi database (APP_TIME_ZONE).
func (r *preorderRepository) Summary() (model.PreorderSummary, error) {
	var s model.PreorderSummary
	err := r.DB.QueryRow(`
		SELECT
		  count(*) FILTER (WHERE due_date < current_date),
		  count(*) FILTER (WHERE due_date = current_date),
		  count(*) FILTER (WHERE due_date = current_date + 1),
		  count(*) FILTER (WHERE due_date = current_date + 2)
		FROM "preorders" WHERE status IN ('ordered', 'ready')`,
	).Scan(&s.Overdue, &s.Today, &s.Tomorrow, &s.DayAfterTomorrow)
	return s, err
}

func (r *preorderRepository) ProductionNeeds() ([]model.ProductionNeed, error) {
	rows, err := r.DB.Query(`
		SELECT to_char(p.due_date, 'YYYY-MM-DD'), i.product_id, pr.name, pr.stock,
		       sum(i.qty)::int, count(DISTINCT p.id)::int
		FROM "preorders" p
		JOIN "preorder_items" i ON i.preorder_id = p.id
		JOIN "products" pr ON pr.id = i.product_id
		WHERE p.status IN ('ordered', 'ready')
		GROUP BY p.due_date, i.product_id, pr.name, pr.stock
		ORDER BY p.due_date, pr.name, i.product_id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	needs := []model.ProductionNeed{}
	for rows.Next() {
		var n model.ProductionNeed
		if err := rows.Scan(&n.DueDate, &n.ProductId, &n.ProductName, &n.Stock, &n.Needed, &n.Orders); err != nil {
			return nil, err
		}
		needs = append(needs, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return needs, nil
}

// AddPayment mencatat DP baru beserta riwayatnya. Mengembalikan sql.ErrNoRows, ErrPreorderClosed, atau
// ErrPreorderOverpaid.
func (r *preorderRepository) AddPayment(id string, pay model.PreorderPayment, k *RequestKey) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimKey(tx, k); err != nil {
		return err
	}
	var (
		status, orderNo string
		total           int64
	)
	err = tx.QueryRow(`SELECT status, total, order_no FROM "preorders" WHERE id = $1 FOR UPDATE`, id).
		Scan(&status, &total, &orderNo)
	if err != nil {
		return err
	}
	if status != model.PreorderOrdered && status != model.PreorderReady {
		return ErrPreorderClosed
	}

	var paid int64
	if err := tx.QueryRow(
		`SELECT COALESCE(sum(amount), 0)::bigint FROM "preorder_payments" WHERE preorder_id = $1`, id,
	).Scan(&paid); err != nil {
		return err
	}
	// Dibandingkan dengan sisa tagihan, bukan paid+amount, supaya nominal raksasa tidak bisa overflow lalu lolos.
	if pay.Amount > total-paid {
		return ErrPreorderOverpaid
	}

	if _, err := tx.Exec(insertPreorderPayment, id, pay.Amount, pay.Method, pay.CreatedBy); err != nil {
		return err
	}
	if err := recordPayment(tx, id, orderNo, pay, paid, total); err != nil {
		return err
	}
	if err := finishKey(tx, k, id); err != nil {
		return err
	}
	return tx.Commit()
}

// recordPayment mencatat DP di riwayat perubahan pre-order, dalam transaksi database yang sama dengan DP-nya:
// jumlah, metode, dan sisa tagihan sebelum dan sesudahnya. paidBefore adalah jumlah DP sebelum DP ini.
func recordPayment(tx *sql.Tx, id, orderNo string, pay model.PreorderPayment, paidBefore, total int64) error {
	remaining := total - paidBefore
	changes := map[string][2]any{
		"amount":    {nil, pay.Amount},
		"method":    {nil, pay.Method},
		"remaining": {remaining, remaining - pay.Amount},
	}
	return recordEntry(tx, auditPreorders, "payment", id, orderNo, changes, Audit{ActorId: pay.CreatedBy})
}

// Update menimpa data pelanggan, jadwal, catatan, dan biaya tambahan. Pengiriman yang masih berjalan ikut disamakan
// (syncDeliveriesSQL).
// Mengembalikan ErrPreorderClosed kalau pre-order sudah tidak berjalan, atau ErrStale kalau barisnya sudah diubah
// sejak versi c.Version dibaca.
func (r *preorderRepository) Update(id string, c PreorderChange, a Audit) error {
	return audited(r.DB, auditPreorders, "update", id, a, func(tx *sql.Tx) error {
		return updatePreorder(tx, id, c, a)
	})
}

func updatePreorder(tx *sql.Tx, id string, c PreorderChange, a Audit) error {
	var (
		current  string
		subtotal int64
		version  int
	)
	err := tx.QueryRow(
		`SELECT to_char(due_date, 'YYYY-MM-DD'), subtotal, version FROM "preorders"
		 WHERE id = $1 AND status IN ('ordered', 'ready') FOR UPDATE`,
		id,
	).Scan(&current, &subtotal, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrPreorderClosed
	}
	if err != nil {
		return err
	}
	if version != c.Version {
		return ErrStale
	}
	if err := checkMoveQuota(tx, id, current, c.DueDate); err != nil {
		return err
	}

	// Biaya boleh diturunkan selama total tidak jadi lebih kecil dari DP yang sudah diterima.
	var paid int64
	if err := tx.QueryRow(
		`SELECT COALESCE(sum(amount), 0)::bigint FROM "preorder_payments" WHERE preorder_id = $1`, id,
	).Scan(&paid); err != nil {
		return err
	}
	if paid > subtotal+c.ShippingFee+c.OtherFee {
		return ErrPreorderOverpaid
	}

	result, err := tx.Exec(
		`UPDATE "preorders" SET customer_name = $2, customer_phone = $3, address = $4,
		   due_date = $5::date, due_time = $6::time, notes = $7,
		   shipping_fee = $8, other_fee = $9, other_fee_note = $10, total = subtotal + $8 + $9
		 WHERE id = $1 AND status IN ('ordered', 'ready')`,
		id, c.CustomerName, c.CustomerPhone, c.Address, c.DueDate, c.DueTime, c.Notes,
		c.ShippingFee, c.OtherFee, c.OtherFeeNote,
	)
	if err != nil {
		return err
	}
	if err := requireAffected(result); err != nil {
		return err
	}

	return auditPreorderDeliveries(tx, id, "update", a.ActorId, func() error {
		_, err := tx.Exec(syncDeliveriesSQL, id)
		return err
	})
}

// syncDeliveriesSQL menyamakan pengiriman pre-order $1 yang masih berjalan dengan pre-order-nya: pemesan, alamat,
// jadwal, dan catatan. Pengiriman yang sudah sama tidak ditulis ulang.
const syncDeliveriesSQL = `UPDATE "deliveries" d
	SET customer_name = p.customer_name, customer_phone = p.customer_phone, address = p.address,
	    scheduled_date = p.due_date, scheduled_time = p.due_time, notes = p.notes
	FROM "preorders" p
	WHERE p.id = $1 AND d.preorder_id = p.id AND d.status IN ('pending', 'on_the_way', 'failed')
	  AND (d.customer_name, d.customer_phone, d.address, d.scheduled_date, d.scheduled_time, d.notes)
	      IS DISTINCT FROM (p.customer_name, p.customer_phone, p.address, p.due_date, p.due_time, p.notes)`

// checkMoveQuota: pre-order yang hari H-nya dipindah dari current ke dueDate harus muat seluruhnya di kuota tanggal
// yang baru. Tanpa pindah tanggal tidak dicek, supaya pesanan di hari yang kuotanya sudah lewat tetap bisa diubah
// data lainnya.
func checkMoveQuota(tx *sql.Tx, id, current, dueDate string) error {
	if dueDate == current {
		return nil
	}
	var qty int
	err := tx.QueryRow(`SELECT COALESCE(sum(qty), 0)::int FROM "preorder_items" WHERE preorder_id = $1`, id).Scan(&qty)
	if err != nil {
		return err
	}
	return checkQuota(tx, dueDate, id, qty)
}

func (r *preorderRepository) MarkReady(id string, a Audit) error {
	return audited(r.DB, auditPreorders, "ready", id, a, func(tx *sql.Tx) error {
		result, err := tx.Exec(`UPDATE "preorders" SET status = 'ready' WHERE id = $1 AND status = 'ordered'`, id)
		if err != nil {
			return err
		}
		return requireAffected(result)
	})
}

// Cancel membatalkan pre-order dan pengirimannya, dan mencatatnya di riwayat perubahan dengan alasannya.
func (r *preorderRepository) Cancel(id, cancelledBy, reason string) error {
	a := Audit{ActorId: cancelledBy, Note: reason}
	return audited(r.DB, auditPreorders, "cancel", id, a, func(tx *sql.Tx) error {
		result, err := tx.Exec(
			`UPDATE "preorders"
			 SET status = 'cancelled', cancelled_by = $2, cancelled_at = now(), cancel_reason = $3
			 WHERE id = $1 AND status IN ('ordered', 'ready')`,
			id, cancelledBy, reason,
		)
		if err != nil {
			return err
		}
		if err := requireAffected(result); err != nil {
			return err
		}

		return auditPreorderDeliveries(tx, id, "cancel", cancelledBy, func() error {
			_, err := tx.Exec(
				`UPDATE "deliveries" SET status = 'cancelled', status_note = 'Pre-order dibatalkan'
				 WHERE preorder_id = $1 AND status IN ('pending', 'on_the_way', 'failed')`,
				id,
			)
			return err
		})
	})
}

func requireAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrPreorderClosed
	}
	return nil
}

// Complete mengunci pre-order, memastikan DP dan total tidak berubah, membuat transaksi (termasuk mengurangi stok),
// lalu menandai pre-order selesai. Semuanya dalam satu DB transaction.
func (r *preorderRepository) Complete(
	id string, expectedDownPayment int64, receivedBy *string, t model.Transaction, k *RequestKey,
) (model.Transaction, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return model.Transaction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimKey(tx, k); err != nil {
		return model.Transaction{}, err
	}
	var created model.Transaction
	err = auditedTx(tx, auditPreorders, "complete", id, Audit{ActorId: t.CashierId}, func(tx *sql.Tx) error {
		created, err = completePreorder(tx, id, expectedDownPayment, receivedBy, t)
		return err
	})
	if err != nil {
		return model.Transaction{}, err
	}
	if err := finishKey(tx, k, created.Id); err != nil {
		return model.Transaction{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Transaction{}, err
	}
	return created, nil
}

func completePreorder(
	tx *sql.Tx, id string, expectedDownPayment int64, receivedBy *string, t model.Transaction,
) (model.Transaction, error) {
	var (
		status string
		total  int64
	)
	err := tx.QueryRow(`SELECT status, total FROM "preorders" WHERE id = $1 FOR UPDATE`, id).Scan(&status, &total)
	if err != nil {
		return model.Transaction{}, err
	}
	if status != model.PreorderOrdered && status != model.PreorderReady {
		return model.Transaction{}, ErrPreorderClosed
	}
	// Ongkir atau biaya lain diubah kasir lain sejak pre-order dibaca.
	if total != t.Total {
		return model.Transaction{}, ErrPreorderChanged
	}

	var downPayment int64
	if err := tx.QueryRow(
		`SELECT COALESCE(sum(amount), 0)::bigint FROM "preorder_payments" WHERE preorder_id = $1`, id,
	).Scan(&downPayment); err != nil {
		return model.Transaction{}, err
	}
	if downPayment != expectedDownPayment {
		return model.Transaction{}, ErrPreorderChanged
	}
	// Pengiriman yang masih berjalan ikut terkirim di bawah, dan butuh nama penerimanya. Dicek sebelum struknya
	// dibuat, supaya nomor struk tidak terlewat.
	if receivedBy == nil {
		var active bool
		if err := tx.QueryRow(
			`SELECT EXISTS (SELECT 1 FROM "deliveries"
			 WHERE preorder_id = $1 AND status IN ('pending', 'on_the_way', 'failed'))`, id,
		).Scan(&active); err != nil {
			return model.Transaction{}, err
		}
		if active {
			return model.Transaction{}, ErrReceiverRequired
		}
	}

	created, err := insertTransaction(tx, t)
	if err != nil {
		return model.Transaction{}, err
	}

	if _, err := tx.Exec(
		`UPDATE "preorders" SET status = 'completed', transaction_id = $2 WHERE id = $1`,
		id, created.Id,
	); err != nil {
		return model.Transaction{}, err
	}

	// Pre-order diantar: pengirimannya ikut terkirim dalam transaksi yang sama, jadi tidak perlu ditandai lagi di
	// menu Pengiriman. Pengiriman yang gagal juga ditutup, karena barangnya sudah sampai ke pelanggan.
	err = auditPreorderDeliveries(tx, id, "deliver", t.CashierId, func() error {
		_, err := tx.Exec(
			`UPDATE "deliveries" SET status = 'delivered', delivered_at = now(), status_note = NULL, received_by = $2
			 WHERE preorder_id = $1 AND status IN ('pending', 'on_the_way', 'failed')`,
			id, receivedBy,
		)
		return err
	})
	if err != nil {
		return model.Transaction{}, err
	}
	return created, nil
}

// quotaLockKey: kunci advisory pertama untuk kuota pre-order; kunci kedua = nomor hari H.
const quotaLockKey = 7401

// quotaUsedSQL: pcs terpakai per hari H dari semua pesanan kecuali yang dibatalkan.
// $2 = id pre-order yang tidak ikut dihitung ("" = tidak ada).
const quotaUsedSQL = `SELECT to_char(p.due_date, 'YYYY-MM-DD'), sum(pi.qty)::int
	FROM "preorders" p JOIN "preorder_items" pi ON pi.preorder_id = p.id
	WHERE p.due_date = ANY($1::date[]) AND p.status <> 'cancelled' AND p.id::text <> $2
	GROUP BY p.due_date`

type sqlQuerier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

func quotaUsed(q sqlQuerier, dates []string, excludeId string) (map[string]int, error) {
	used := make(map[string]int, len(dates))
	if len(dates) == 0 {
		return used, nil
	}
	rows, err := q.Query(quotaUsedSQL, pq.Array(dates), excludeId)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var date string
		var qty int
		if err := rows.Scan(&date, &qty); err != nil {
			return nil, err
		}
		used[date] = qty
	}
	return used, rows.Err()
}

// checkQuota memastikan pesanan sebanyak requested pcs masih muat di kuota hari H.
// Hari H dikunci dulu (advisory lock sampai transaksi selesai), jadi dua kasir yang menyimpan
// bersamaan untuk tanggal yang sama antre dan tidak sama-sama lolos.
func checkQuota(tx *sql.Tx, dueDate, excludeId string, requested int) error {
	lock := `SELECT pg_advisory_xact_lock($1, $2::date - DATE '2000-01-01')`
	if _, err := tx.Exec(lock, quotaLockKey, dueDate); err != nil {
		return err
	}
	var limit int
	if err := tx.QueryRow(`SELECT daily_limit FROM "preorder_settings" WHERE id = 1`).Scan(&limit); err != nil {
		return err
	}
	used, err := quotaUsed(tx, []string{dueDate}, excludeId)
	if err != nil {
		return err
	}
	if used[dueDate]+requested > limit {
		return &QuotaExceededError{DueDate: dueDate, Limit: limit, Used: used[dueDate], Requested: requested}
	}
	return nil
}

func (r *preorderRepository) DailyLimit() (int, error) {
	var limit int
	err := r.DB.QueryRow(`SELECT daily_limit FROM "preorder_settings" WHERE id = 1`).Scan(&limit)
	return limit, err
}

func (r *preorderRepository) SetDailyLimit(limit int, updatedBy string) error {
	return audited(r.DB, auditSettings, "update", "1", Audit{ActorId: updatedBy}, func(tx *sql.Tx) error {
		result, err := tx.Exec(
			`UPDATE "preorder_settings" SET daily_limit = $1, updated_by = NULLIF($2, '')::uuid, updated_at = now()
			 WHERE id = 1`,
			limit, updatedBy,
		)
		if err != nil {
			return err
		}
		n, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if n != 1 {
			return errors.New("preorder_settings row is missing")
		}
		return nil
	})
}

func (r *preorderRepository) QuotaUsed(dates []string, excludeId string) (map[string]int, error) {
	return quotaUsed(r.DB, dates, excludeId)
}

// NewPreorderRepository Constructor
func NewPreorderRepository(db *sql.DB) PreorderRepository {
	return &preorderRepository{
		DB: db,
	}
}
