package res

import "time"

type ProductResponse struct {
	Id         string  `json:"id"`
	Kind       string  `json:"kind"` // store atau preorder
	Sku        *string `json:"sku"`
	Name       string  `json:"name"`
	CategoryId *string `json:"category_id"`
	Price      int64   `json:"price"`
	// Harga grosir: null kalau tidak ada. Berlaku kalau jumlah produk ini dalam satu transaksi atau pre-order
	// mencapai wholesale_min_qty (aturan sama dengan model.Product.UnitPrice).
	WholesalePrice  *int64    `json:"wholesale_price"`
	WholesaleMinQty int       `json:"wholesale_min_qty"`
	Stock           int       `json:"stock"`
	Active          bool      `json:"active"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProductListResponse struct {
	Data  []ProductResponse `json:"data"`
	Page  int               `json:"page"`
	Limit int               `json:"limit"`
	Total int               `json:"total"`
}
