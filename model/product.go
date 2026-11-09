package model

import "time"

// Jenis produk. Daftar produk toko dan produk PO terpisah: kasir hanya menjual produk toko,
// pre-order hanya memesan produk PO.
const (
	ProductStore    = "store"    // dijual langsung di kasir
	ProductPreorder = "preorder" // dibuat sesuai pesanan pre-order
)

// DefaultWholesaleMinQty: harga grosir berlaku mulai jumlah ini kalau batasnya tidak diisi.
const DefaultWholesaleMinQty = 50

type Product struct {
	Id         string
	Kind       string
	Sku        *string
	Name       string
	CategoryId *string
	Price      int64 // Rupiah
	// Harga grosir (opsional, Rupiah): dipakai kalau jumlah produk ini dalam satu transaksi atau pre-order
	// mencapai WholesaleMinQty. nil = tanpa harga grosir.
	WholesalePrice  *int64
	WholesaleMinQty int
	Stock           int // toko: stok di rak; PO: stok siap (sudah diproduksi, belum diserahkan)
	Active          bool
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// UnitPrice adalah harga satuan untuk qty pcs produk ini dalam satu transaksi atau satu pre-order:
// harga grosir kalau ada dan qty sudah mencapai batasnya, selain itu harga jual biasa.
func (p Product) UnitPrice(qty int) int64 {
	if p.WholesalePrice != nil && qty >= p.WholesaleMinQty {
		return *p.WholesalePrice
	}
	return p.Price
}
