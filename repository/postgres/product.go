package postgres

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/lib/pq"
)

type ProductFilter struct {
	Kind          string // wajib: store atau preorder
	Search        string
	Ids           []string // kosong = semua; dipakai untuk memuat ulang harga barang di keranjang
	OnlyActive    bool
	NegativeStock bool
	Limit         int
	Offset        int
}

type (
	// Perubahan produk (tambah, ubah, hapus, produksi) dicatat di riwayat perubahan (audit_logs) dengan pelakunya.
	ProductRepository interface {
		Save(product model.Product, a Audit) (model.Product, error)
		Update(id string, product model.Product, stockChange int, active *bool, a Audit) (model.Product, error)
		FindByIds(ids []string) ([]model.Product, error)
		List(filter ProductFilter) ([]model.Product, int, error)
		SoftDelete(id string, a Audit) error
		// AddPreorderStock: k (boleh nil) = kunci idempotensi; *DuplicateRequestError kalau kuncinya sudah dipakai.
		AddPreorderStock(id string, qty int, a Audit, k *RequestKey) (model.Product, error)
	}
	productRepository struct {
		DB *sql.DB
	}
)

const productColumns = `id, kind, sku, name, category_id, price, wholesale_price, wholesale_min_qty, stock, active,
	created_at, updated_at`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanProduct(row rowScanner) (model.Product, error) {
	var (
		p              model.Product
		sku            sql.NullString
		categoryId     sql.NullString
		wholesalePrice sql.NullInt64
	)
	err := row.Scan(
		&p.Id,
		&p.Kind,
		&sku,
		&p.Name,
		&categoryId,
		&p.Price,
		&wholesalePrice,
		&p.WholesaleMinQty,
		&p.Stock,
		&p.Active,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return model.Product{}, err
	}
	p.Sku = nullToPtr(sku)
	p.CategoryId = nullToPtr(categoryId)
	if wholesalePrice.Valid {
		v := wholesalePrice.Int64
		p.WholesalePrice = &v
	}
	return p, nil
}

func nullToPtr(n sql.NullString) *string {
	if !n.Valid {
		return nil
	}
	v := n.String
	return &v
}

func (r *productRepository) Save(p model.Product, a Audit) (model.Product, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return model.Product{}, err
	}
	defer func() { _ = tx.Rollback() }()

	SQL := `INSERT INTO "products"(id, kind, sku, name, category_id, price, wholesale_price, wholesale_min_qty,
	            stock, active)
	        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	        RETURNING ` + productColumns
	saved, err := scanProduct(tx.QueryRow(SQL,
		p.Id, p.Kind, p.Sku, p.Name, p.CategoryId, p.Price, p.WholesalePrice, p.WholesaleMinQty, p.Stock, p.Active,
	))
	if err != nil {
		return model.Product{}, err
	}
	after, err := snapshot(tx, auditProducts, saved.Id, false)
	if err != nil {
		return model.Product{}, err
	}
	if err := recordChange(tx, auditProducts, "create", saved.Id, nil, after, a); err != nil {
		return model.Product{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Product{}, err
	}
	return saved, nil
}

// Update mengembalikan sql.ErrNoRows kalau produk tidak ada atau sudah dihapus.
// active nil berarti nilai active tidak diubah. Jenis produk (kind) tidak pernah diubah.
// Stok tidak ditimpa dengan product.Stock: ditambah stockChange (boleh minus, 0 = tetap), supaya penjualan yang
// terjadi selama form produk terbuka tidak hilang.
func (r *productRepository) Update(
	id string, p model.Product, stockChange int, active *bool, a Audit,
) (model.Product, error) {
	SQL := `UPDATE "products"
	        SET sku = $2, name = $3, category_id = $4, price = $5, wholesale_price = $6, wholesale_min_qty = $7,
	            stock = stock + $8, active = COALESCE($9, active)
	        WHERE id = $1 AND deleted_at IS NULL
	        RETURNING ` + productColumns
	var saved model.Product
	err := audited(r.DB, auditProducts, "update", id, a, func(tx *sql.Tx) error {
		var err error
		saved, err = scanProduct(tx.QueryRow(SQL,
			id, p.Sku, p.Name, p.CategoryId, p.Price, p.WholesalePrice, p.WholesaleMinQty, stockChange, active,
		))
		return err
	})
	return saved, err
}

// FindByIds mengembalikan produk yang belum dihapus (aktif maupun nonaktif).
// Id yang tidak ditemukan tidak muncul di hasil.
func (r *productRepository) FindByIds(ids []string) ([]model.Product, error) {
	SQL := `SELECT ` + productColumns + ` FROM "products" WHERE id = ANY($1::uuid[]) AND deleted_at IS NULL`
	rows, err := r.DB.Query(SQL, pq.Array(ids))
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	products := []model.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return products, nil
}

func (r *productRepository) List(f ProductFilter) ([]model.Product, int, error) {
	where := `WHERE deleted_at IS NULL AND kind = $1`
	args := []any{f.Kind}
	if f.OnlyActive {
		where += ` AND active = true`
	}
	if f.NegativeStock {
		where += ` AND stock < 0`
	}
	if f.Search != "" {
		args = append(args, "%"+escapeLike(f.Search)+"%")
		where += fmt.Sprintf(` AND (name ILIKE $%[1]d OR sku ILIKE $%[1]d)`, len(args))
	}
	if len(f.Ids) > 0 {
		args = append(args, pq.Array(f.Ids))
		where += fmt.Sprintf(` AND id = ANY($%d::uuid[])`, len(args))
	}

	var total int
	if err := r.DB.QueryRow(`SELECT count(*) FROM "products" `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	n := len(args)
	args = append(args, f.Limit, f.Offset)
	SQL := fmt.Sprintf(
		`SELECT %s FROM "products" %s ORDER BY name ASC, id ASC LIMIT $%d OFFSET $%d`,
		productColumns, where, n+1, n+2,
	)
	rows, err := r.DB.Query(SQL, args...)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = rows.Close() }()

	products := []model.Product{}
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, 0, err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return products, total, nil
}

// SoftDelete mengembalikan sql.ErrNoRows kalau produk tidak ada atau sudah dihapus.
func (r *productRepository) SoftDelete(id string, a Audit) error {
	return audited(r.DB, auditProducts, "delete", id, a, func(tx *sql.Tx) error {
		result, err := tx.Exec(
			`UPDATE "products" SET deleted_at = now(), active = false WHERE id = $1 AND deleted_at IS NULL`,
			id,
		)
		if err != nil {
			return err
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return sql.ErrNoRows
		}
		return nil
	})
}

// AddPreorderStock menambah stok siap produk PO secara atomik (stock = stock + qty),
// supaya tidak menimpa pengurangan dari pre-order yang diserahkan pada saat yang sama.
// Mengembalikan sql.ErrNoRows kalau produk tidak ada, sudah dihapus, atau bukan produk PO.
func (r *productRepository) AddPreorderStock(id string, qty int, a Audit, k *RequestKey) (model.Product, error) {
	tx, err := r.DB.Begin()
	if err != nil {
		return model.Product{}, err
	}
	defer func() { _ = tx.Rollback() }()

	if err := claimKey(tx, k); err != nil {
		return model.Product{}, err
	}
	var saved model.Product
	err = auditedTx(tx, auditProducts, "production", id, a, func(tx *sql.Tx) error {
		saved, err = scanProduct(tx.QueryRow(
			`UPDATE "products" SET stock = stock + $2
			 WHERE id = $1 AND kind = 'preorder' AND deleted_at IS NULL
			 RETURNING `+productColumns,
			id, qty,
		))
		return err
	})
	if err != nil {
		return model.Product{}, err
	}
	if err := finishKey(tx, k, id); err != nil {
		return model.Product{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Product{}, err
	}
	return saved, nil
}

// escapeLike menetralkan karakter khusus LIKE agar pencarian "50%" tidak dianggap wildcard.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// NewProductRepository Constructor
func NewProductRepository(db *sql.DB) ProductRepository {
	return &productRepository{
		DB: db,
	}
}
