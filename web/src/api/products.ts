import { idempotencyHeader } from '../lib/requestKey'
import { api } from './client'
import type { Product, ProductInput, ProductKind, ProductList } from './types'

// kind wajib: daftar produk toko dan produk PO terpisah. activeOnly: hanya produk aktif, juga untuk admin (layar
// Kasir dan form pre-order), supaya limit tidak terpakai produk nonaktif. ids: hanya produk ini (maksimal 100),
// untuk memuat ulang harga barang yang sudah ada di keranjang atau form.
export function listProducts(params: {
  kind: ProductKind
  search?: string
  ids?: string[]
  page?: number
  limit?: number
  negativeStock?: boolean
  activeOnly?: boolean
}) {
  const query = new URLSearchParams()
  query.set('kind', params.kind)
  if (params.search) query.set('search', params.search)
  if (params.ids?.length) query.set('ids', params.ids.join(','))
  if (params.negativeStock) query.set('stock', 'negative')
  if (params.activeOnly) query.set('active', 'true')
  query.set('page', String(params.page ?? 1))
  query.set('limit', String(params.limit ?? 20))
  return api<ProductList>(`/api/v1/products?${query}`)
}

export function createProduct(input: ProductInput) {
  return api<Product>('/api/v1/products', { method: 'POST', body: JSON.stringify(input) })
}

export function updateProduct(id: string, input: ProductInput) {
  return api<Product>(`/api/v1/products/${id}`, { method: 'PUT', body: JSON.stringify(input) })
}

export function deleteProduct(id: string) {
  return api<void>(`/api/v1/products/${id}`, { method: 'DELETE' })
}

// Hasil produksi produk PO: stok siap bertambah sebanyak qty. Hanya admin.
export function addProduction(id: string, qty: number, key?: string) {
  return api<Product>(`/api/v1/products/${id}/production`, {
    method: 'POST',
    body: JSON.stringify({ qty }),
    headers: idempotencyHeader(key),
  })
}

// Data terbaru produk-produk ini (aktif saja), dalam kelompok 100 sesuai batas server.
export async function productsByIds(kind: ProductKind, ids: string[]): Promise<Product[]> {
  const found: Product[] = []
  for (let i = 0; i < ids.length; i += 100) {
    const chunk = ids.slice(i, i + 100)
    const page = await listProducts({ kind, ids: chunk, limit: 100, activeOnly: true })
    found.push(...page.data)
  }
  return found
}
