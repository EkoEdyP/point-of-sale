package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/lib/pq"
)

// ErrDeliveryClosed: status pengiriman tidak mengizinkan tindakan ini (atau berubah di tengah jalan).
var ErrDeliveryClosed = errors.New("delivery state does not allow this action")

type DeliveryFilter struct {
	Statuses  []string // kosong = semua status
	SortByDue bool     // true: jadwal terdekat dulu; false: yang terakhir berubah dulu
	Limit     int
	Offset    int
}

type (
	DeliveryRepository interface {
		FindById(id string) (model.Delivery, error)
		List(filter DeliveryFilter) ([]model.Delivery, int, error)
		Summary() (model.DeliverySummary, error)
		// Update menimpa data pengiriman dengan nilai yang sudah digabungkan usecase; pengiriman pre-order ikut
		// mengubah pre-order-nya (*QuotaExceededError kalau tanggal barunya tidak muat di kuota). Semua perubahan
		// status dan data dicatat di riwayat perubahan (audit_logs); untuk Fail dan Cancel, a.Note berisi alasannya.
		Update(id string, d model.Delivery, a Audit) error
		Dispatch(id string, courier *string, a Audit) error
		Fail(id, reason string, a Audit) error
		Cancel(id, reason string, a Audit) error
	}
	deliveryRepository struct {
		DB *sql.DB
	}
)

const deliverySelect = `SELECT d.id, d.delivery_no, d.preorder_id, d.transaction_id,
	COALESCE(po.order_no, t.receipt_no, ''), po.status,
	d.customer_name, d.customer_phone, d.address,
	to_char(d.scheduled_date, 'YYYY-MM-DD'), to_char(d.scheduled_time, 'HH24:MI'),
	d.courier,
	-- Ongkir pengiriman pre-order dicatat di pre-ordernya (ikut ditagihkan ke pelanggan).
	(CASE WHEN d.preorder_id IS NOT NULL THEN po.shipping_fee ELSE d.fee END)::bigint,
	d.notes, d.status, d.status_note, d.dispatched_at, d.delivered_at, d.received_by,
	(CASE WHEN po.status IN ('ordered', 'ready') THEN po.total - pay.dp ELSE 0 END)::bigint,
	-- Rincian uang untuk surat jalan: dari pre-order, atau dari struk untuk pengiriman penjualan lama.
	COALESCE(po.subtotal, t.subtotal, 0)::bigint, COALESCE(po.other_fee, t.other_fee, 0)::bigint,
	COALESCE(po.other_fee_note, t.other_fee_note), COALESCE(po.total, t.total, 0)::bigint,
	(CASE WHEN po.id IS NOT NULL THEN pay.dp ELSE COALESCE(t.down_payment, 0) END)::bigint,
	d.created_by, u.name, d.created_at, d.version
	FROM "deliveries" d
	JOIN "users" u ON u.id = d.created_by
	LEFT JOIN "preorders" po ON po.id = d.preorder_id
	LEFT JOIN LATERAL (SELECT COALESCE(sum(pp.amount), 0)::bigint AS dp FROM "preorder_payments" pp
	                   WHERE pp.preorder_id = po.id) pay ON true
	LEFT JOIN "transactions" t ON t.id = d.transaction_id`

func scanDelivery(row rowScanner) (model.Delivery, error) {
	var (
		d                                                                           model.Delivery
		preorderId, transactionId, phone, scheduledTime, courier, notes, statusNote sql.NullString
		preorderStatus, otherFeeNote, receivedBy                                    sql.NullString
		dispatchedAt, deliveredAt                                                   sql.NullTime
	)
	err := row.Scan(
		&d.Id,
		&d.DeliveryNo,
		&preorderId,
		&transactionId,
		&d.SourceNo,
		&preorderStatus,
		&d.CustomerName,
		&phone,
		&d.Address,
		&d.ScheduledDate,
		&scheduledTime,
		&courier,
		&d.Fee,
		&notes,
		&d.Status,
		&statusNote,
		&dispatchedAt,
		&deliveredAt,
		&receivedBy,
		&d.AmountDue,
		&d.Subtotal,
		&d.OtherFee,
		&otherFeeNote,
		&d.Total,
		&d.DownPayment,
		&d.CreatedBy,
		&d.CreatedByName,
		&d.CreatedAt,
		&d.Version,
	)
	if err != nil {
		return model.Delivery{}, err
	}
	d.PreorderId = nullToPtr(preorderId)
	d.PreorderStatus = nullToPtr(preorderStatus)
	d.OtherFeeNote = nullToPtr(otherFeeNote)
	d.TransactionId = nullToPtr(transactionId)
	d.CustomerPhone = nullToPtr(phone)
	d.ReceivedBy = nullToPtr(receivedBy)
	d.ScheduledTime = nullToPtr(scheduledTime)
	d.Courier = nullToPtr(courier)
	d.Notes = nullToPtr(notes)
	d.StatusNote = nullToPtr(statusNote)
	if dispatchedAt.Valid {
		v := dispatchedAt.Time
		d.DispatchedAt = &v
	}
	if deliveredAt.Valid {
		v := deliveredAt.Time
		d.DeliveredAt = &v
	}
	return d, nil
}

// FindById mengembalikan pengiriman beserta barangnya. sql.ErrNoRows kalau tidak ada.
func (r *deliveryRepository) FindById(id string) (model.Delivery, error) {
	d, err := scanDelivery(r.DB.QueryRow(deliverySelect+` WHERE d.id = $1`, id))
	if err != nil {
		return model.Delivery{}, err
	}

	var itemsSQL string
	var sourceId string
	switch {
	case d.PreorderId != nil:
		itemsSQL = `SELECT product_name, qty, unit_price, line_total FROM "preorder_items"
		            WHERE preorder_id = $1 ORDER BY product_name`
		sourceId = *d.PreorderId
	case d.TransactionId != nil:
		itemsSQL = `SELECT product_name, qty, unit_price, line_total FROM "transaction_items"
		            WHERE transaction_id = $1 ORDER BY product_name`
		sourceId = *d.TransactionId
	default:
		return d, nil
	}

	rows, err := r.DB.Query(itemsSQL, sourceId)
	if err != nil {
		return model.Delivery{}, err
	}
	defer func() { _ = rows.Close() }()

	d.Items = []model.DeliveryItem{}
	for rows.Next() {
		var item model.DeliveryItem
		if err := rows.Scan(&item.ProductName, &item.Qty, &item.UnitPrice, &item.LineTotal); err != nil {
			return model.Delivery{}, err
		}
		d.Items = append(d.Items, item)
	}
	if err := rows.Err(); err != nil {
		return model.Delivery{}, err
	}
	return d, nil
}

// List mengembalikan pengiriman tanpa daftar barang.
func (r *deliveryRepository) List(f DeliveryFilter) ([]model.Delivery, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	if len(f.Statuses) > 0 {
		args = append(args, pq.Array(f.Statuses))
		conds = append(conds, fmt.Sprintf("d.status = ANY($%d)", len(args)))
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.DB.QueryRow(`SELECT count(*) FROM "deliveries" d`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	order := ` ORDER BY d.updated_at DESC, d.id DESC`
	if f.SortByDue {
		order = ` ORDER BY d.scheduled_date ASC, d.scheduled_time ASC NULLS LAST, d.created_at ASC`
	}

	n := len(args)
	args = append(args, f.Limit, f.Offset)
	SQL := fmt.Sprintf(`%s%s%s LIMIT $%d OFFSET $%d`, deliverySelect, where, order, n+1, n+2)
	rows, err := r.DB.Query(SQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	deliveries := []model.Delivery{}
	for rows.Next() {
		d, err := scanDelivery(rows)
		if err != nil {
			return nil, 0, err
		}
		deliveries = append(deliveries, d)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return deliveries, total, nil
}

// Summary menghitung pengiriman yang belum selesai: terlambat, hari ini, dan besok.
func (r *deliveryRepository) Summary() (model.DeliverySummary, error) {
	var s model.DeliverySummary
	err := r.DB.QueryRow(`
		SELECT
		  count(*) FILTER (WHERE scheduled_date < current_date),
		  count(*) FILTER (WHERE scheduled_date = current_date),
		  count(*) FILTER (WHERE scheduled_date = current_date + 1)
		FROM "deliveries" WHERE status IN ('pending', 'on_the_way')`,
	).Scan(&s.Overdue, &s.Today, &s.Tomorrow)
	return s, err
}

// Update hanya berhasil kalau barisnya masih di versi d.Version (versi yang dibaca usecase sebelum menggabungkan
// perubahannya). Kalau tidak, ErrDeliveryClosed (status tidak bisa diubah lagi) atau ErrStale (sudah diubah).
// Pengiriman pre-order berbagi pemesan, alamat, jadwal, dan catatan dengan pre-order-nya, jadi pre-order-nya ikut
// diubah di transaksi database yang sama dan dicatat di riwayatnya dengan nomor pengiriman ini.
func (r *deliveryRepository) Update(id string, d model.Delivery, a Audit) error {
	tx, err := r.DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var preorderBefore rowSnapshot
	if d.PreorderId != nil {
		if preorderBefore, err = lockPreorderOf(tx, *d.PreorderId, d.ScheduledDate); err != nil {
			return err
		}
	}
	err = auditedTx(tx, auditDeliveries, "update", id, a, func(tx *sql.Tx) error {
		if err := updateDeliveryRow(tx, id, d); err != nil || d.PreorderId == nil {
			return err
		}
		_, err := tx.Exec(
			`UPDATE "preorders" SET customer_name = $2, customer_phone = $3, address = $4,
			   due_date = $5::date, due_time = $6::time, notes = $7
			 WHERE id = $1 AND (customer_name, customer_phone, address, due_date, due_time, notes)
			   IS DISTINCT FROM ($2::text, $3::text, $4::text, $5::date, $6::time, $7::text)`,
			*d.PreorderId, d.CustomerName, d.CustomerPhone, d.Address, d.ScheduledDate, d.ScheduledTime, d.Notes,
		)
		return err
	})
	if err != nil {
		return err
	}
	if d.PreorderId != nil {
		after, err := snapshot(tx, auditPreorders, *d.PreorderId, false)
		if err != nil {
			return err
		}
		if len(diffRows(auditPreorders, preorderBefore, after)) > 0 {
			note := Audit{ActorId: a.ActorId, Note: "Ikut pengiriman " + d.DeliveryNo}
			if err := recordChange(tx, auditPreorders, "update", *d.PreorderId, preorderBefore, after, note); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// lockPreorderOf mengunci pre-order milik pengiriman yang akan diubah dan membaca isinya untuk riwayat. Pre-order
// dikunci sebelum pengirimannya, urutan yang sama dengan perubahan dari pre-order, supaya dua perubahan bersamaan
// saling menunggu, bukan saling mengunci. Tanggal kirim baru memindah hari H, jadi harus muat di kuota tanggal itu
// (*QuotaExceededError).
func lockPreorderOf(tx *sql.Tx, preorderId, scheduledDate string) (rowSnapshot, error) {
	before, err := snapshot(tx, auditPreorders, preorderId, true)
	if err != nil {
		return nil, err
	}
	var (
		dueDate string
		open    bool
	)
	err = tx.QueryRow(
		`SELECT to_char(due_date, 'YYYY-MM-DD'), status IN ('ordered', 'ready') FROM "preorders" WHERE id = $1`,
		preorderId,
	).Scan(&dueDate, &open)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return nil, ErrDeliveryClosed
	case err != nil:
		return nil, err
	case !open:
		return nil, ErrDeliveryClosed
	}
	return before, checkMoveQuota(tx, preorderId, dueDate, scheduledDate)
}

// updateDeliveryRow menulis data pengiriman hanya kalau barisnya masih di versi d.Version. Ongkir pengiriman
// pre-order tersimpan di pre-ordernya (d.Fee hanya tampilannya), jadi kolom fee hanya ditulis untuk pengiriman
// penjualan lama.
func updateDeliveryRow(tx *sql.Tx, id string, d model.Delivery) error {
	result, err := tx.Exec(
		`UPDATE "deliveries"
		 SET customer_name = $2, customer_phone = $3, address = $4,
		     scheduled_date = $5::date, scheduled_time = $6::time, courier = $7,
		     fee = CASE WHEN preorder_id IS NULL THEN $8::bigint ELSE fee END,
		     notes = $9, status = $10, status_note = $11
		 WHERE id = $1 AND status IN ('pending', 'on_the_way', 'failed') AND version = $12`,
		id, d.CustomerName, d.CustomerPhone, d.Address, d.ScheduledDate, d.ScheduledTime,
		d.Courier, d.Fee, d.Notes, d.Status, d.StatusNote, d.Version,
	)
	if err != nil {
		return err
	}
	if err := requireDeliveryAffected(result); !errors.Is(err, ErrDeliveryClosed) {
		return err
	}
	// Barisnya sudah dikunci auditedTx, jadi alasannya bisa dibaca dengan pasti.
	var open bool
	err = tx.QueryRow(
		`SELECT status IN ('pending', 'on_the_way', 'failed') FROM "deliveries" WHERE id = $1`, id,
	).Scan(&open)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ErrDeliveryClosed
	case err != nil:
		return err
	case open:
		return ErrStale
	}
	return ErrDeliveryClosed
}

func (r *deliveryRepository) Dispatch(id string, courier *string, a Audit) error {
	return r.change(id, "dispatch", a,
		`UPDATE "deliveries" SET status = 'on_the_way', dispatched_at = now(), courier = COALESCE($2, courier)
		 WHERE id = $1 AND status = 'pending'`,
		id, courier,
	)
}

func (r *deliveryRepository) Fail(id, reason string, a Audit) error {
	return r.change(id, "fail", a,
		`UPDATE "deliveries" SET status = 'failed', status_note = $2
		 WHERE id = $1 AND status IN ('pending', 'on_the_way')`,
		id, reason,
	)
}

func (r *deliveryRepository) Cancel(id, reason string, a Audit) error {
	return r.change(id, "cancel", a,
		`UPDATE "deliveries" SET status = 'cancelled', status_note = $2
		 WHERE id = $1 AND status IN ('pending', 'on_the_way', 'failed')`,
		id, reason,
	)
}

// change menjalankan satu UPDATE pengiriman dan mencatatnya di riwayat perubahan. ErrDeliveryClosed kalau
// statusnya tidak lagi mengizinkan perubahan itu.
func (r *deliveryRepository) change(id, action string, a Audit, query string, args ...any) error {
	return audited(r.DB, auditDeliveries, action, id, a, func(tx *sql.Tx) error {
		result, err := tx.Exec(query, args...)
		if err != nil {
			return err
		}
		return requireDeliveryAffected(result)
	})
}

func requireDeliveryAffected(result sql.Result) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrDeliveryClosed
	}
	return nil
}

// NewDeliveryRepository Constructor
func NewDeliveryRepository(db *sql.DB) DeliveryRepository {
	return &deliveryRepository{
		DB: db,
	}
}
