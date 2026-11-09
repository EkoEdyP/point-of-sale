import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { createProduct, updateProduct } from '../api/products'
import type { Product, ProductKind } from '../api/types'
import { MAX_QTY } from '../hooks/useCart'
import { formatNumber } from '../lib/format'
import { DEFAULT_WHOLESALE_MIN_QTY } from '../lib/wholesale'
import { AuditHistoryDialog } from './AuditHistoryDialog'
import { Modal } from './Modal'

const digitsOnly = (value: string) => value.replace(/\D/g, '')

const inputClass = 'w-full h-11 px-3 bg-white border-2 border-ink rounded-sm text-base'
const labelClass = 'block mb-1 text-sm font-semibold'

// product null berarti produk baru berjenis kind. Produk yang sudah ada memakai jenisnya sendiri.
export function ProductFormDialog({
  product,
  kind,
  onClose,
  onSaved,
}: {
  product: Product | null
  kind: ProductKind
  onClose: () => void
  onSaved: () => void
}) {
  const productKind = product?.kind ?? kind
  // Produk PO dibuat sesuai pesanan; stoknya berarti stok siap.
  const isPreorder = productKind === 'preorder'
  const [name, setName] = useState(product?.name ?? '')
  const [sku, setSku] = useState(product?.sku ?? '')
  const [price, setPrice] = useState(product ? String(product.price) : '')
  const [wholesale, setWholesale] = useState(product?.wholesale_price != null ? String(product.wholesale_price) : '')
  const [minQty, setMinQty] = useState(String(product?.wholesale_min_qty ?? DEFAULT_WHOLESALE_MIN_QTY))
  const [stock, setStock] = useState(product ? String(product.stock) : '0')
  const [active, setActive] = useState(product?.active ?? true)
  const [showingHistory, setShowingHistory] = useState(false)
  const [error, setError] = useState('')
  const [saving, setSaving] = useState(false)

  const minQtyNumber = Number(minQty)
  const minQtyValid = minQty !== '' && minQtyNumber >= 2 && minQtyNumber <= MAX_QTY

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')

    if (name.trim() === '') return setError('Nama produk wajib diisi.')
    if (price === '') return setError('Harga jual wajib diisi.')
    if (wholesale !== '') {
      if (Number(wholesale) === 0) {
        return setError('Harga grosir tidak boleh 0. Kosongkan kalau tidak ada harga grosir.')
      }
      if (Number(wholesale) >= Number(price)) return setError('Harga grosir harus lebih murah dari harga jual.')
      if (!minQtyValid) return setError(`Harga grosir berlaku mulai 2 sampai ${formatNumber(MAX_QTY)} pcs.`)
    }

    const input = {
      kind: productKind,
      name: name.trim(),
      sku: sku.trim() || null,
      category_id: product?.category_id ?? null, // kategori belum diatur dari layar ini, jangan dihapus
      price: Number(price),
      wholesale_price: wholesale === '' ? null : Number(wholesale),
      // Tanpa harga grosir, batas yang tidak valid tidak menghalangi simpan: server memakai 50.
      wholesale_min_qty: minQtyValid ? minQtyNumber : null,
      stock: Number(stock || '0'),
      stock_before: product?.stock ?? null,
      active,
    }

    setSaving(true)
    try {
      if (product) await updateProduct(product.id, input)
      else await createProduct(input)
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setSaving(false)
    }
  }

  const noun = isPreorder ? 'produk PO' : 'produk toko'

  return (
    <>
      <Modal title={product ? `Ubah ${noun}` : `Tambah ${noun}`} onClose={onClose} busy={saving}>
        <form onSubmit={(e) => void handleSubmit(e)} className="mt-4 space-y-4">
          <div>
            <label htmlFor="p-name" className={labelClass}>
              Nama produk
            </label>
            <input
              id="p-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
              maxLength={200}
              className={inputClass}
            />
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label htmlFor="p-sku" className={labelClass}>
                SKU
              </label>
              <input
                id="p-sku"
                value={sku}
                onChange={(e) => setSku(e.target.value)}
                maxLength={50}
                className={`${inputClass} font-mono`}
              />
            </div>
            <div>
              <label htmlFor="p-price" className={labelClass}>
                Harga jual (Rp)
              </label>
              <input
                id="p-price"
                inputMode="numeric"
                value={price}
                onChange={(e) => setPrice(digitsOnly(e.target.value))}
                className={`${inputClass} font-mono`}
              />
            </div>
          </div>

          <div>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label htmlFor="p-wholesale" className={labelClass}>
                  Harga grosir (Rp)
                </label>
                <input
                  id="p-wholesale"
                  inputMode="numeric"
                  value={wholesale}
                  onChange={(e) => setWholesale(digitsOnly(e.target.value))}
                  aria-describedby="p-wholesale-hint"
                  className={`${inputClass} font-mono`}
                />
              </div>
              <div>
                <label htmlFor="p-min-qty" className={labelClass}>
                  Mulai dari (pcs)
                </label>
                <input
                  id="p-min-qty"
                  inputMode="numeric"
                  value={minQty}
                  onChange={(e) => setMinQty(digitsOnly(e.target.value))}
                  // Dikosongkan lalu ditinggal: kembali ke 50, supaya angka yang dipakai selalu terlihat.
                  onBlur={() => {
                    if (minQty === '') setMinQty(String(DEFAULT_WHOLESALE_MIN_QTY))
                  }}
                  aria-describedby="p-wholesale-hint"
                  className={`${inputClass} font-mono`}
                />
              </div>
            </div>
            <p id="p-wholesale-hint" className="mt-1 text-sm text-ink-soft">
              {wholesale === ''
                ? 'Kosongkan harga grosir kalau produk ini tidak punya harga grosir.'
                : minQtyValid
                  ? `Dipakai otomatis di ${isPreorder ? 'pre-order' : 'kasir'} kalau beli ` +
                    `${formatNumber(minQtyNumber)} pcs atau lebih.`
                  : `Isi mulai dari berapa pcs, 2 sampai ${formatNumber(MAX_QTY)}.`}
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-2 sm:items-end">
            <div>
              <label htmlFor="p-stock" className={labelClass}>
                {isPreorder ? 'Stok siap' : 'Stok'}
              </label>
              <input
                id="p-stock"
                inputMode="numeric"
                value={stock}
                onChange={(e) => setStock(digitsOnly(e.target.value))}
                aria-describedby={isPreorder ? 'p-stock-hint' : undefined}
                className={`${inputClass} font-mono`}
              />
            </div>
            <label className="h-11 flex items-center gap-2 font-semibold cursor-pointer">
              <input
                type="checkbox"
                checked={active}
                onChange={(e) => setActive(e.target.checked)}
                className="size-5 accent-ink"
              />
              {isPreorder ? 'Bisa dipesan' : 'Tampil di kasir'}
            </label>
          </div>
          {isPreorder && (
            <p id="p-stock-hint" className="-mt-2 text-sm text-ink-soft">
              Stok siap: barang yang sudah diproduksi dan belum diserahkan. Bertambah lewat Catat produksi di halaman
              Pre-order, berkurang saat pesanan diserahkan.
            </p>
          )}

          {error && (
            <p role="alert" className="font-semibold text-merah">
              {error}
            </p>
          )}

          <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            {product && (
              <button
                type="button"
                onClick={() => setShowingHistory(true)}
                className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer sm:mr-auto"
              >
                Riwayat
              </button>
            )}
            <button
              type="button"
              onClick={onClose}
              className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
            >
              Batal
            </button>
            <button
              type="submit"
              disabled={saving}
              className="h-11 px-5 bg-kunyit border-2 border-ink text-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
            >
              {product ? 'Simpan perubahan' : 'Tambah produk'}
            </button>
          </div>
        </form>
      </Modal>
      {product && showingHistory && (
        <AuditHistoryDialog
          entity="product"
          entityId={product.id}
          title={`Riwayat ${product.name}`}
          created={{ at: product.created_at, by: null, label: 'Produk dibuat' }}
          onClose={() => setShowingHistory(false)}
        />
      )}
    </>
  )
}
