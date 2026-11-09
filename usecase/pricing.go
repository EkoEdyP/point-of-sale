package usecase

import (
	"net/http"
	"sort"
	"strings"

	"github.com/EkoEdyP/point-of-sale/exception"
	"github.com/EkoEdyP/point-of-sale/model"
	"github.com/EkoEdyP/point-of-sale/repository/postgres"
)

// itemQty: satu produk dan jumlahnya dalam request penjualan atau pre-order.
type itemQty struct {
	ProductId string `json:"product_id"`
	Qty       int    `json:"qty"`
}

// pricedItem: satu produk dengan jumlah gabungan dan harga dari database.
type pricedItem struct {
	product   model.Product
	qty       int
	unitPrice int64
	lineTotal int64
}

// mergeItems menggabungkan produk yang muncul lebih dari sekali lalu mengurutkannya per id, supaya urutan update
// stok sama di semua transaksi database dan dua request berisi barang yang sama menghasilkan isi yang sama.
func mergeItems(items []itemQty) ([]itemQty, error) {
	qtyByProduct := make(map[string]int, len(items))
	for _, item := range items {
		qtyByProduct[strings.ToLower(item.ProductId)] += item.Qty
	}
	merged := make([]itemQty, 0, len(qtyByProduct))
	for id, qty := range qtyByProduct {
		if qty > maxQtyPerProduct {
			return nil, unprocessable("jumlah per produk maksimal 10000")
		}
		merged = append(merged, itemQty{ProductId: id, Qty: qty})
	}
	sort.Slice(merged, func(i, j int) bool { return merged[i].ProductId < merged[j].ProductId })
	return merged, nil
}

// priceItems menghitung harga barang (hasil mergeItems) dari database, bukan dari client: harga grosir kalau
// jumlahnya mencapai batas produk itu. Hanya produk aktif dengan jenis kind yang boleh: produk toko untuk kasir,
// produk PO untuk pre-order.
func priceItems(products postgres.ProductRepository, items []itemQty, kind string) ([]pricedItem, int64, error) {
	ids := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ProductId
	}
	found, err := products.FindByIds(ids)
	if err != nil {
		return nil, 0, internalError(err)
	}
	byId := make(map[string]model.Product, len(found))
	for _, p := range found {
		byId[p.Id] = p
	}

	priced := make([]pricedItem, 0, len(items))
	var subtotal int64
	for _, item := range items {
		p, ok := byId[item.ProductId]
		if !ok {
			return nil, 0, unprocessable("produk tidak ditemukan: " + item.ProductId)
		}
		if !p.Active {
			return nil, 0, unprocessable("produk nonaktif: " + p.Name)
		}
		if p.Kind != kind {
			if kind == model.ProductStore {
				return nil, 0, unprocessable("produk PO tidak dijual di kasir: " + p.Name)
			}
			return nil, 0, unprocessable("produk toko tidak bisa dipesan lewat pre-order: " + p.Name)
		}
		unitPrice := p.UnitPrice(item.Qty)
		line := unitPrice * int64(item.Qty)
		priced = append(priced, pricedItem{product: p, qty: item.Qty, unitPrice: unitPrice, lineTotal: line})
		subtotal += line
	}
	return priced, subtotal, nil
}

// reasonPriceChanged: kode error saat total yang dilihat pengguna berbeda dengan total dari harga terbaru, misalnya
// karena harga produk diubah selama keranjang atau form terbuka. Client memuat ulang harganya.
const reasonPriceChanged = "price_changed"

// checkExpectedTotal menolak request kalau total di layar client (expected, nil = tidak dikirim) berbeda dengan
// total yang dihitung server, supaya pelanggan tidak membayar jumlah yang berbeda dari yang tercatat.
func checkExpectedTotal(expected *int64, total int64) error {
	if expected == nil || *expected == total {
		return nil
	}
	return &exception.GetError{
		Code: http.StatusConflict,
		Message: "harga berubah: total sekarang Rp " + groupThousands(int(total)) +
			", bukan Rp " + groupThousands(int(*expected)),
		Reason: reasonPriceChanged,
	}
}
