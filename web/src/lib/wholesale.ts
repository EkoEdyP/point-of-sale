import type { Product } from '../api/types'

// Harga grosir berlaku mulai jumlah ini kalau batasnya tidak diisi (sama dengan model.DefaultWholesaleMinQty).
export const DEFAULT_WHOLESALE_MIN_QTY = 50

// Aturan harga satu produk: harga jual biasa, plus harga grosir (null = tidak ada) yang berlaku mulai
// wholesaleMinQty pcs per transaksi atau per pre-order.
export interface PriceRule {
  price: number
  wholesalePrice: number | null
  wholesaleMinQty: number
}

export function priceRule(p: Product): PriceRule {
  return {
    price: p.price,
    wholesalePrice: p.wholesale_price ?? null,
    wholesaleMinQty: p.wholesale_min_qty ?? DEFAULT_WHOLESALE_MIN_QTY,
  }
}

// Harga satuan untuk qty pcs: harga grosir kalau ada dan qty sudah mencapai batasnya, selain itu harga jual.
// Aturannya sama dengan model.Product.UnitPrice di server. Server yang menentukan harga di struk dan nota,
// jadi angka di layar hanya perkiraan kalau harga produk berubah sementara keranjang masih terbuka.
export function unitPrice(rule: PriceRule, qty: number): number {
  return rule.wholesalePrice !== null && qty >= rule.wholesaleMinQty ? rule.wholesalePrice : rule.price
}
