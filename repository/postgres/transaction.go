package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/model"
)

// ErrTransactionNotVoidable dikembalikan Void kalau transaksi sudah dibatalkan, tidak ada, atau hasil penyerahan
// pre-order.
var ErrTransactionNotVoidable = errors.New("transaction cannot be voided")

var errProductGone = errors.New("product not found while updating stock")

type TransactionFilter struct {
	CashierId string // kosong = semua kasir
	Status    string // kosong = semua status
	From      *time.Time
	To        *time.Time // eksklusif
	Limit     int
	Offset    int
}

type (
	TransactionRepository interface {
		// Create: k (boleh nil) = kunci idempotensi; *DuplicateRequestError kalau kuncinya sudah dipakai.
		Create(transaction model.Transaction, k *RequestKey) (model.Transaction, error)
		FindById(id string) (model.Transaction, error)
		List(filter TransactionFilter) ([]model.Transaction, int, error)
		Void(id, voidedBy, reason string) error
	}
	transactionRepository struct {
		DB *sql.DB
	}
)

// Transaksi dari penyerahan pre-order dikenali lewat preorders.transaction_id (unik). vu = akun yang membatalkan.
const transactionSelect = `SELECT t.id, t.receipt_no, t.cashier_id, u.name, t.subtotal, t.discount,
	t.shipping_fee, t.other_fee, t.other_fee_note, t.total,
	t.paid, t.change_amount, t.down_payment, t.payment_method, t.status,
	t.voided_by, vu.name, t.voided_at, t.void_reason, t.created_at,
	po.id, po.order_no
	FROM "transactions" t JOIN "users" u ON u.id = t.cashier_id
	LEFT JOIN "users" vu ON vu.id = t.voided_by
	LEFT JOIN "preorders" po ON po.transaction_id = t.id`

func scanTransaction(row rowScanner) (model.Transaction, error) {
	var (
		t            model.Transaction
		otherFeeNote sql.NullString
		voidedBy     sql.NullString
		voidedByName sql.NullString
		voidedAt     sql.NullTime
		voidReason   sql.NullString
		preorderId   sql.NullString
		preorderNo   sql.NullString
	)
	err := row.Scan(
		&t.Id,
		&t.ReceiptNo,
		&t.CashierId,
		&t.CashierName,
		&t.Subtotal,
		&t.Discount,
		&t.ShippingFee,
		&t.OtherFee,
		&otherFeeNote,
		&t.Total,
		&t.Paid,
		&t.ChangeAmount,
		&t.DownPayment,
		&t.PaymentMethod,
		&t.Status,
		&voidedBy,
		&voidedByName,
		&voidedAt,
		&voidReason,
		&t.CreatedAt,
		&preorderId,
		&preorderNo,
	)
	if err != nil {
		return model.Transaction{}, err
	}
	t.OtherFeeNote = nullToPtr(otherFeeNote)
	t.VoidedBy = nullToPtr(voidedBy)
	t.VoidedByName = nullToPtr(voidedByName)
	t.VoidReason = nullToPtr(voidReason)
	t.PreorderId = nullToPtr(preorderId)
	t.PreorderNo = nullToPtr(preorderNo)
	if voidedAt.Valid {
		v := voidedAt.Time
		t.VoidedAt = &v
	}
	return t, nil
}

// Create menyimpan transaksi, item, dan mengurangi stok dalam satu DB transaction.
// Stok dikurangi dengan "stock = stock - qty" (atomik), dan boleh menjadi minus.
// Item harus sudah terurut berdasarkan product_id supaya urutan lock konsisten antar transaksi.
func (r *transactionRepository) Create(t model.Transaction, k *RequestKey) (model.Transaction, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return model.Transaction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimKey(tx, k); err != nil {
		return model.Transaction{}, err
	}
	created, err := insertTransaction(tx, t)
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

// insertTransaction menyimpan transaksi dan item, lalu mengurangi stok, di dalam transaksi database
// yang diberikan. Dipakai juga oleh penyelesaian pre-order supaya semuanya berjalan atomik.
func insertTransaction(tx *sql.Tx, t model.Transaction) (model.Transaction, error) {
	err := tx.QueryRow(
		`INSERT INTO "transactions"(cashier_id, subtotal, discount, shipping_fee, other_fee, other_fee_note, total,
		   paid, change_amount, down_payment, payment_method)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, receipt_no, status, created_at`,
		t.CashierId, t.Subtotal, t.Discount, t.ShippingFee, t.OtherFee, t.OtherFeeNote, t.Total,
		t.Paid, t.ChangeAmount, t.DownPayment, t.PaymentMethod,
	).Scan(&t.Id, &t.ReceiptNo, &t.Status, &t.CreatedAt)
	if err != nil {
		return model.Transaction{}, err
	}

	for i := range t.Items {
		item := &t.Items[i]
		item.TransactionId = t.Id

		err = tx.QueryRow(
			`INSERT INTO "transaction_items"(transaction_id, product_id, product_name, unit_price, qty, discount, line_total)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)
			 RETURNING id`,
			item.TransactionId, item.ProductId, item.ProductName, item.UnitPrice, item.Qty, item.Discount, item.LineTotal,
		).Scan(&item.Id)
		if err != nil {
			return model.Transaction{}, err
		}

		result, err := tx.Exec(`UPDATE "products" SET stock = stock - $2 WHERE id = $1`, item.ProductId, item.Qty)
		if err != nil {
			return model.Transaction{}, err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return model.Transaction{}, err
		}
		if affected == 0 {
			return model.Transaction{}, errProductGone
		}
	}
	return t, nil
}

// FindById mengembalikan transaksi lengkap dengan item. sql.ErrNoRows kalau tidak ada.
func (r *transactionRepository) FindById(id string) (model.Transaction, error) {
	t, err := scanTransaction(r.DB.QueryRow(transactionSelect+` WHERE t.id = $1`, id))
	if err != nil {
		return model.Transaction{}, err
	}

	rows, err := r.DB.Query(
		`SELECT id, transaction_id, product_id, product_name, unit_price, qty, discount, line_total
		 FROM "transaction_items" WHERE transaction_id = $1 ORDER BY product_name, id`,
		id,
	)
	if err != nil {
		return model.Transaction{}, err
	}
	defer func() { _ = rows.Close() }()

	t.Items = []model.TransactionItem{}
	for rows.Next() {
		var item model.TransactionItem
		if err := rows.Scan(
			&item.Id,
			&item.TransactionId,
			&item.ProductId,
			&item.ProductName,
			&item.UnitPrice,
			&item.Qty,
			&item.Discount,
			&item.LineTotal,
		); err != nil {
			return model.Transaction{}, err
		}
		t.Items = append(t.Items, item)
	}
	if err := rows.Err(); err != nil {
		return model.Transaction{}, err
	}
	return t, nil
}

// List mengembalikan transaksi tanpa item, terbaru dulu.
func (r *transactionRepository) List(f TransactionFilter) ([]model.Transaction, int, error) {
	conds := []string{"1=1"}
	args := []any{}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if f.CashierId != "" {
		add("t.cashier_id = $%d", f.CashierId)
	}
	if f.Status != "" {
		add("t.status = $%d", f.Status)
	}
	if f.From != nil {
		add("t.created_at >= $%d", *f.From)
	}
	if f.To != nil {
		add("t.created_at < $%d", *f.To)
	}
	where := " WHERE " + strings.Join(conds, " AND ")

	var total int
	if err := r.DB.QueryRow(`SELECT count(*) FROM "transactions" t`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n := len(args)
	args = append(args, f.Limit, f.Offset)
	SQL := fmt.Sprintf(`%s%s ORDER BY t.created_at DESC, t.id DESC LIMIT $%d OFFSET $%d`, transactionSelect, where, n+1, n+2)
	rows, err := r.DB.Query(SQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	transactions := []model.Transaction{}
	for rows.Next() {
		t, err := scanTransaction(rows)
		if err != nil {
			return nil, 0, err
		}
		transactions = append(transactions, t)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return transactions, total, nil
}

// Void membatalkan transaksi dan mengembalikan stok dalam satu DB transaction, dan mencatatnya di riwayat
// perubahan dengan alasannya. Mengembalikan ErrTransactionNotVoidable kalau transaksi sudah void, tidak ada, atau
// berasal dari penyerahan pre-order (PO-nya tetap selesai, jadi transaksinya juga harus tetap).
func (r *transactionRepository) Void(id, voidedBy, reason string) error {
	return audited(r.DB, auditTransactions, "void", id, Audit{ActorId: voidedBy, Note: reason}, func(tx *sql.Tx) error {
		return voidTransaction(tx, id, voidedBy, reason)
	})
}

func voidTransaction(tx *sql.Tx, id, voidedBy, reason string) error {
	result, err := tx.Exec(
		`UPDATE "transactions" t
		 SET status = 'void', voided_by = $2, voided_at = now(), void_reason = $3
		 WHERE t.id = $1 AND t.status = 'completed'
		   AND NOT EXISTS (SELECT 1 FROM "preorders" po WHERE po.transaction_id = t.id)`,
		id, voidedBy, reason,
	)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrTransactionNotVoidable
	}

	// Kunci baris produk berurutan id seperti saat penjualan (insertTransaction). UPDATE ... FROM di bawah mengunci
	// mengikuti urutan tabel, sehingga void dan penjualan yang berjalan bersamaan bisa saling menunggu (deadlock).
	if _, err := tx.Exec(
		`SELECT p.id FROM "products" p
		 WHERE p.id IN (SELECT product_id FROM "transaction_items" WHERE transaction_id = $1)
		 ORDER BY p.id FOR UPDATE`,
		id,
	); err != nil {
		return err
	}

	_, err = tx.Exec(
		`UPDATE "products" p SET stock = p.stock + i.qty
		 FROM (SELECT product_id, SUM(qty)::int AS qty
		       FROM "transaction_items" WHERE transaction_id = $1 GROUP BY product_id) i
		 WHERE p.id = i.product_id`,
		id,
	)
	return err
}

// NewTransactionRepository Constructor
func NewTransactionRepository(db *sql.DB) TransactionRepository {
	return &transactionRepository{
		DB: db,
	}
}
