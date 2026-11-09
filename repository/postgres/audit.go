package postgres

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/EkoEdyP/point-of-sale/model"
)

// Audit: siapa yang melakukan perubahan, dan catatan opsional (alasan batal, gagal kirim, void). Dicatat ke
// audit_logs dalam transaksi database yang sama dengan perubahannya, jadi riwayat ada kalau dan hanya kalau
// perubahannya tersimpan. ActorId kosong untuk perubahan tanpa pengguna (skrip).
type Audit struct {
	ActorId string
	Note    string
}

// auditTarget: satu jenis data yang dicatat riwayatnya.
type auditTarget struct {
	entity     string   // audit_logs.entity
	table      string   // tabel sumber, kunci utamanya kolom id
	label      string   // kolom untuk audit_logs.entity_label
	fixedLabel string   // label kalau tabelnya tidak punya kolom nama
	columns    []string // kolom yang dibandingkan
	// Kolom rahasia hanya dicatat bahwa nilainya berubah, dengan nama lain (password_hash -> password).
	secret map[string]string
}

var (
	auditProducts = auditTarget{entity: "product", table: "products", label: "name", columns: []string{
		"kind", "name", "sku", "price", "wholesale_price", "wholesale_min_qty", "stock", "active", "deleted_at",
	}}
	auditPreorders = auditTarget{entity: "preorder", table: "preorders", label: "order_no", columns: []string{
		"status", "customer_name", "customer_phone", "address", "due_date", "due_time", "notes", "shipping_fee",
		"other_fee", "other_fee_note", "total",
	}}
	auditDeliveries = auditTarget{entity: "delivery", table: "deliveries", label: "delivery_no", columns: []string{
		"status", "status_note", "customer_name", "customer_phone", "address", "scheduled_date", "scheduled_time",
		"courier", "fee", "notes", "received_by",
	}}
	auditUsers = auditTarget{entity: "user", table: "users", label: "username",
		columns: []string{"name", "username", "role", "active"},
		secret:  map[string]string{"password_hash": "password"},
	}
	auditTransactions = auditTarget{entity: "transaction", table: "transactions", label: "receipt_no",
		columns: []string{"status"},
	}
	auditSettings = auditTarget{entity: "setting", table: "preorder_settings", fixedLabel: "Kuota pre-order harian",
		columns: []string{"daily_limit"},
	}
)

type rowSnapshot map[string]json.RawMessage

// snapshot membaca satu baris sebagai JSON per kolom. lock mengunci barisnya (FOR UPDATE) sampai transaksi selesai.
// Baris yang tidak ada menghasilkan nil tanpa error, supaya pemanggil tetap mengembalikan error "tidak ada"-nya
// sendiri.
func snapshot(tx *sql.Tx, t auditTarget, id string, lock bool) (rowSnapshot, error) {
	q := `SELECT to_jsonb(x) FROM "` + t.table + `" x WHERE x.id = $1`
	if lock {
		q += ` FOR UPDATE`
	}
	var raw []byte
	err := tx.QueryRow(q, id).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var row rowSnapshot
	if err := json.Unmarshal(raw, &row); err != nil {
		return nil, err
	}
	return row, nil
}

// diffRows: kolom yang nilainya berbeda antara before dan after, {"kolom": [lama, baru]}.
func diffRows(t auditTarget, before, after rowSnapshot) map[string][2]json.RawMessage {
	changes := map[string][2]json.RawMessage{}
	for _, col := range t.columns {
		if old, cur := before[col], after[col]; !sameJSON(old, cur) {
			changes[col] = [2]json.RawMessage{jsonOrNull(old), jsonOrNull(cur)}
		}
	}
	// Rahasia hanya dicatat saat diganti; nilai awal saat data dibuat tidak perlu dicatat.
	if before != nil {
		for col, name := range t.secret {
			if !sameJSON(before[col], after[col]) {
				changes[name] = [2]json.RawMessage{json.RawMessage("null"), json.RawMessage(`"diubah"`)}
			}
		}
	}
	return changes
}

// recordChange menulis satu baris audit_logs berisi kolom yang berbeda antara before dan after (nil = belum ada
// atau sudah tidak ada). Tidak menulis apa-apa kalau tidak ada yang berubah dan tidak ada catatan.
func recordChange(tx *sql.Tx, t auditTarget, action, id string, before, after rowSnapshot, a Audit) error {
	changes := diffRows(t, before, after)
	if len(changes) == 0 && a.Note == "" {
		return nil
	}
	label := t.fixedLabel
	if label == "" {
		label = jsonText(after[t.label])
		if label == "" {
			label = jsonText(before[t.label])
		}
	}
	return recordEntry(tx, t, action, id, label, changes, a)
}

// recordEntry menulis satu baris audit_logs dengan changes apa adanya. Dipakai langsung untuk kejadian yang tidak
// mengubah kolom baris targetnya, misalnya DP yang tersimpan di tabelnya sendiri.
func recordEntry(tx *sql.Tx, t auditTarget, action, id, label string, changes any, a Audit) error {
	payload, err := json.Marshal(changes)
	if err != nil {
		return err
	}
	_, err = tx.Exec(
		`INSERT INTO "audit_logs"(actor_id, action, entity, entity_id, entity_label, changes, note)
		 VALUES (NULLIF($1, '')::uuid, $2, $3, $4, $5, $6, NULLIF($7, ''))`,
		a.ActorId, action, t.entity, id, label, payload, a.Note,
	)
	return err
}

// audited menjalankan fn di dalam transaksi database, dengan baris targetnya dikunci dan dibaca sebelum dan
// sesudahnya, lalu mencatat perbedaannya. Error dari fn dikembalikan apa adanya (dan semuanya dibatalkan).
func audited(db *sql.DB, t auditTarget, action, id string, a Audit, fn func(tx *sql.Tx) error) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := auditedTx(tx, t, action, id, a, fn); err != nil {
		return err
	}
	return tx.Commit()
}

// auditedTx seperti audited, di dalam transaksi database yang sudah berjalan.
func auditedTx(tx *sql.Tx, t auditTarget, action, id string, a Audit, fn func(tx *sql.Tx) error) error {
	before, err := snapshot(tx, t, id, true)
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	after, err := snapshot(tx, t, id, false)
	if err != nil {
		return err
	}
	return recordChange(tx, t, action, id, before, after, a)
}

// auditPreorderDeliveries menjalankan fn yang ikut mengubah pengiriman milik pre-order (data bersama, batal,
// terkirim), lalu mencatat setiap pengiriman yang benar-benar berubah dengan catatan nomor pre-order-nya.
func auditPreorderDeliveries(tx *sql.Tx, preorderId, action, actorId string, fn func() error) error {
	rows, err := tx.Query(
		`SELECT d.id, p.order_no FROM "deliveries" d JOIN "preorders" p ON p.id = d.preorder_id
		 WHERE d.preorder_id = $1 ORDER BY d.id FOR UPDATE OF d`,
		preorderId,
	)
	if err != nil {
		return err
	}
	var ids []string
	var orderNo string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id, &orderNo); err != nil {
			_ = rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	befores := make([]rowSnapshot, len(ids))
	for i, id := range ids {
		if befores[i], err = snapshot(tx, auditDeliveries, id, false); err != nil {
			return err
		}
	}
	if err := fn(); err != nil {
		return err
	}
	a := Audit{ActorId: actorId, Note: "Ikut pre-order " + orderNo}
	for i, id := range ids {
		after, err := snapshot(tx, auditDeliveries, id, false)
		if err != nil {
			return err
		}
		if len(diffRows(auditDeliveries, befores[i], after)) == 0 {
			continue
		}
		if err := recordChange(tx, auditDeliveries, action, id, befores[i], after, a); err != nil {
			return err
		}
	}
	return nil
}

func sameJSON(a, b json.RawMessage) bool {
	return bytes.Equal(bytes.TrimSpace(jsonOrNull(a)), bytes.TrimSpace(jsonOrNull(b)))
}

func jsonOrNull(v json.RawMessage) json.RawMessage {
	if len(v) == 0 {
		return json.RawMessage("null")
	}
	return v
}

// jsonText: isi string JSON, atau teks apa adanya untuk nilai lain (angka), kosong untuk null.
func jsonText(v json.RawMessage) string {
	var s string
	if err := json.Unmarshal(v, &s); err == nil {
		return s
	}
	if t := strings.TrimSpace(string(v)); t != "null" {
		return t
	}
	return ""
}

type AuditFilter struct {
	Entity   string // kosong = semua jenis
	EntityId string // kosong = semua data
	ActorId  string // kosong = semua pengguna
	From     *time.Time
	To       *time.Time // eksklusif
	Limit    int
	Offset   int
}

type (
	AuditRepository interface {
		List(filter AuditFilter) ([]model.AuditEntry, int, error)
	}
	auditRepository struct {
		DB *sql.DB
	}
)

// List mengembalikan riwayat terbaru dulu, beserta nama pelaku saat ini.
func (r *auditRepository) List(f AuditFilter) ([]model.AuditEntry, int, error) {
	var (
		where []string
		args  []any
	)
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.Entity != "" {
		add(`a.entity = ?`, f.Entity)
	}
	if f.EntityId != "" {
		add(`a.entity_id = ?`, f.EntityId)
	}
	if f.ActorId != "" {
		add(`a.actor_id = ?::uuid`, f.ActorId)
	}
	if f.From != nil {
		add(`a.created_at >= ?`, *f.From)
	}
	if f.To != nil {
		add(`a.created_at < ?`, *f.To)
	}
	cond := ""
	if len(where) > 0 {
		cond = " WHERE " + strings.Join(where, " AND ")
	}

	var total int
	if err := r.DB.QueryRow(`SELECT count(*) FROM "audit_logs" a`+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.DB.Query(
		`SELECT a.id, a.created_at, a.actor_id, u.name, a.action, a.entity, a.entity_id, a.entity_label, a.changes,
		        a.note
		 FROM "audit_logs" a LEFT JOIN "users" u ON u.id = a.actor_id`+cond+`
		 ORDER BY a.created_at DESC, a.id DESC
		 LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)),
		args...,
	)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	entries := []model.AuditEntry{}
	for rows.Next() {
		var (
			e         model.AuditEntry
			actorId   sql.NullString
			actorName sql.NullString
			note      sql.NullString
			changes   []byte
		)
		if err := rows.Scan(&e.Id, &e.CreatedAt, &actorId, &actorName, &e.Action, &e.Entity, &e.EntityId,
			&e.EntityLabel, &changes, &note); err != nil {
			return nil, 0, err
		}
		e.ActorId = nullToPtr(actorId)
		e.ActorName = nullToPtr(actorName)
		e.Note = nullToPtr(note)
		e.Changes = json.RawMessage(changes)
		entries = append(entries, e)
	}
	return entries, total, rows.Err()
}

// NewAuditRepository Constructor
func NewAuditRepository(db *sql.DB) AuditRepository {
	return &auditRepository{DB: db}
}
