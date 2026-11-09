import { useEffect, useMemo, useReducer } from 'react'
import type { Product } from '../api/types'
import { CART_STORAGE_KEY } from '../lib/cartStorage'
import { DEFAULT_WHOLESALE_MIN_QTY, priceRule, unitPrice, type PriceRule } from '../lib/wholesale'

// Satu baris keranjang. price/wholesalePrice/wholesaleMinQty adalah aturan harga produk (PriceRule): harga
// satuannya mengikuti qty (lib/wholesale.ts), jadi grosir otomatis berlaku atau lepas saat qty diubah.
export interface CartLine extends PriceRule {
  productId: string
  name: string
  stock: number
  qty: number
}

// Server membatasi 10000 per produk.
export const MAX_QTY = 9999

type Action =
  | { type: 'add'; product: Product }
  | { type: 'setQty'; productId: string; qty: number }
  | { type: 'remove'; productId: string }
  | { type: 'refresh'; products: Product[] }
  | { type: 'clear' }

const clamp = (qty: number) => Math.min(Math.max(Math.floor(qty), 1), MAX_QTY)

// Data produk terbaru untuk baris keranjang (nama, stok, harga, harga grosir).
function snapshot(product: Product) {
  return { productId: product.id, name: product.name, stock: product.stock, ...priceRule(product) }
}

export function lineUnitPrice(line: CartLine) {
  return unitPrice(line, line.qty)
}

function reducer(state: CartLine[], action: Action): CartLine[] {
  switch (action.type) {
    case 'add': {
      const { product } = action
      // Produk yang sudah ada: qty +1 dan harga ikut diperbarui, kalau-kalau admin baru saja mengubahnya.
      if (state.some((l) => l.productId === product.id)) {
        return state.map((l) => (l.productId === product.id ? { ...snapshot(product), qty: clamp(l.qty + 1) } : l))
      }
      return [...state, { ...snapshot(product), qty: 1 }]
    }
    case 'setQty':
      return state.map((l) => (l.productId === action.productId ? { ...l, qty: clamp(action.qty) } : l))
    case 'remove':
      return state.filter((l) => l.productId !== action.productId)
    case 'refresh': {
      // Harga, nama, dan stok terbaru; qty tetap. Produk yang tidak ikut terbaca (nonaktif atau dihapus) dibiarkan,
      // server menolaknya dengan nama produknya saat dibayar.
      const latest = new Map(action.products.map((p) => [p.id, p]))
      return state.map((l) => {
        const product = latest.get(l.productId)
        return product ? { ...snapshot(product), qty: l.qty } : l
      })
    }
    case 'clear':
      return []
  }
}

function isLine(value: unknown): value is CartLine {
  if (typeof value !== 'object' || value === null) return false
  const l = value as Record<string, unknown>
  return (
    typeof l.productId === 'string' &&
    typeof l.name === 'string' &&
    typeof l.price === 'number' &&
    typeof l.stock === 'number' &&
    typeof l.qty === 'number'
  )
}

// Keranjang yang disimpan sebelum ada harga grosir tidak punya field grosir: anggap tanpa harga grosir.
function withRule(l: CartLine): CartLine {
  return {
    ...l,
    wholesalePrice: typeof l.wholesalePrice === 'number' ? l.wholesalePrice : null,
    wholesaleMinQty: typeof l.wholesaleMinQty === 'number' ? l.wholesaleMinQty : DEFAULT_WHOLESALE_MIN_QTY,
  }
}

function load(): CartLine[] {
  try {
    const raw = sessionStorage.getItem(CART_STORAGE_KEY)
    if (!raw) return []
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed) ? parsed.filter(isLine).map(withRule) : []
  } catch {
    return []
  }
}

export function useCart() {
  const [lines, dispatch] = useReducer(reducer, undefined, load)

  useEffect(() => {
    try {
      sessionStorage.setItem(CART_STORAGE_KEY, JSON.stringify(lines))
    } catch {
      // penyimpanan penuh atau diblokir: keranjang tetap jalan tanpa penyimpanan
    }
  }, [lines])

  const total = useMemo(() => lines.reduce((sum, l) => sum + lineUnitPrice(l) * l.qty, 0), [lines])
  const itemCount = useMemo(() => lines.reduce((sum, l) => sum + l.qty, 0), [lines])

  return {
    lines,
    total,
    itemCount,
    add: (product: Product) => dispatch({ type: 'add', product }),
    setQty: (productId: string, qty: number) => dispatch({ type: 'setQty', productId, qty }),
    remove: (productId: string) => dispatch({ type: 'remove', productId }),
    refresh: (products: Product[]) => dispatch({ type: 'refresh', products }),
    clear: () => dispatch({ type: 'clear' }),
  }
}
