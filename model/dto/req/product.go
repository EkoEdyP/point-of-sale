package req

// ProductRequest dipakai untuk create (POST) dan update (PUT).
type ProductRequest struct {
	// Kind hanya dipakai saat create (kosong berarti produk toko). Jenis produk tidak bisa diubah.
	Kind       string  `json:"kind" validate:"omitempty,oneof=store preorder"`
	Sku        *string `json:"sku" validate:"omitempty,max=50"`
	Name       string  `json:"name" validate:"required,max=200"`
	CategoryId *string `json:"category_id" validate:"omitempty,uuid"`
	Price      *int64  `json:"price" validate:"required,gte=0"`
	// Harga grosir (opsional): null berarti tanpa harga grosir. Diperiksa di usecase: lebih dari 0 dan lebih murah
	// dari harga jual. WholesaleMinQty: berlaku mulai jumlah ini (2-10000 pcs); null berarti 50.
	WholesalePrice  *int64 `json:"wholesale_price"`
	WholesaleMinQty *int   `json:"wholesale_min_qty"`
	// Stock: stok awal saat create. Saat update stok tidak ditimpa: berubah sebesar Stock - StockBefore (angka yang
	// tampil saat form dibuka) dari stok sekarang, supaya penjualan selama form terbuka tidak hilang. Tanpa
	// StockBefore stok tidak diubah.
	Stock       int  `json:"stock" validate:"gte=-1000000,lte=1000000"`
	StockBefore *int `json:"stock_before" validate:"omitempty,gte=-1000000,lte=1000000"`
	// Active: kosong berarti aktif saat create, dan tidak diubah saat update.
	Active *bool `json:"active"`
}

type ProductListQuery struct {
	Kind          string // store (bawaan) atau preorder
	Search        string
	Ids           []string // hanya produk ini (maksimal 100), untuk memuat ulang harga di keranjang
	Page          int
	Limit         int
	NegativeStock bool // hanya produk dengan stok di bawah 0
	OnlyActive    bool // hanya produk aktif, juga untuk admin (layar Kasir dan form pre-order)
}

// ProductionRequest mencatat hasil produksi produk PO: stok siap bertambah sebanyak Qty.
type ProductionRequest struct {
	Qty        int    `json:"qty" validate:"gte=1,lte=100000"`
	RequestKey string `json:"-"`
}
