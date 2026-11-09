import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import { deleteProduct, listProducts } from '../api/products'
import type { Product, ProductKind, ProductList } from '../api/types'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { Pagination } from '../components/Pagination'
import { ProductFormDialog } from '../components/ProductFormDialog'
import { useDebounced } from '../hooks/useDebounced'
import { formatNumber, formatRupiah } from '../lib/format'

const PAGE_SIZE = 20

// Tab disimpan di alamat halaman (?jenis=po), jadi tetap terbuka saat dimuat ulang.
const KIND_PARAM = 'jenis'

const KIND_TABS: { value: ProductKind; label: string }[] = [
  { value: 'store', label: 'Toko' },
  { value: 'preorder', label: 'PO' },
]

const KIND_TEXT: Record<
  ProductKind,
  { intro: string; empty: string; inactive: string; stock: string; negative: string; search: string }
> = {
  store: {
    intro: 'Barang yang dijual langsung di kasir. Stok berkurang setiap terjual.',
    empty: 'Belum ada produk toko. Tambahkan produk pertama dengan tombol Tambah produk.',
    inactive: 'Tidak tampil di kasir',
    stock: 'Stok',
    negative:
      'Stok minus artinya barang terjual lebih banyak dari yang tercatat. Hitung barang di rak, lalu isi kolom Stok lewat tombol Ubah dengan jumlah fisik saat ini.',
    search: 'Cari nama atau SKU',
  },
  preorder: {
    intro:
      'Barang yang dibuat sesuai pesanan pre-order dan tidak dijual di kasir. Stok siap bertambah saat hasil produksi dicatat, dan berkurang saat pesanan diserahkan.',
    empty: 'Belum ada produk PO. Tambahkan barang yang dibuat sesuai pesanan dengan tombol Tambah produk.',
    inactive: 'Tidak bisa dipesan',
    stock: 'Stok siap',
    negative:
      'Stok siap minus artinya pesanan sudah diserahkan sebelum hasil produksinya dicatat. Hitung barang yang sudah jadi, lalu isi kolom Stok siap lewat tombol Ubah.',
    search: 'Cari nama atau SKU',
  },
}

function InactiveBadge({ kind }: { kind: ProductKind }) {
  return (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-ink-soft text-ink-soft rounded-sm whitespace-nowrap">
      {KIND_TEXT[kind].inactive}
    </span>
  )
}

// Harga grosir, misalnya "Grosir ≥50: Rp 9.000": di tabel di bawah harga jual, di kartu HP di bawah nama/SKU.
function WholesaleNote({ product, className = '' }: { product: Product; className?: string }) {
  if (product.wholesale_price == null) return null
  return (
    <p className={`font-sans text-sm font-normal text-ink-soft ${className}`}>
      Grosir <span aria-hidden="true">≥</span>
      <span className="sr-only">mulai </span>
      {formatNumber(product.wholesale_min_qty)}
      <span className="sr-only"> pcs</span>: <span className="font-mono">{formatRupiah(product.wholesale_price)}</span>
    </p>
  )
}

export function ProductsPage() {
  const [searchParams, setSearchParams] = useSearchParams()
  const kind: ProductKind = searchParams.get(KIND_PARAM) === 'po' ? 'preorder' : 'store'
  const text = KIND_TEXT[kind]
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounced(search.trim(), 300)
  const [page, setPage] = useState(1)
  const [negativeOnly, setNegativeOnly] = useState(false)
  const [negativeCount, setNegativeCount] = useState(0)
  const [result, setResult] = useState<ProductList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const [editing, setEditing] = useState<Product | 'new' | null>(null)
  const [deleting, setDeleting] = useState<Product | null>(null)

  // Ganti tab (juga lewat tombol kembali): mulai dari halaman 1 tanpa filter stok minus,
  // dan jangan tampilkan daftar jenis sebelumnya selama memuat.
  const [shownKind, setShownKind] = useState(kind)
  if (shownKind !== kind) {
    setShownKind(kind)
    setPage(1)
    setNegativeOnly(false)
    setNegativeCount(0)
    setResult(null)
  }

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listProducts({ kind, search: debouncedSearch, page, limit: PAGE_SIZE, negativeStock: negativeOnly })
      .then((r) => {
        if (cancelled) return
        // Halaman terakhir baru saja kosong (misalnya setelah menghapus): mundur satu halaman.
        if (r.data.length === 0 && page > 1) setPage(page - 1)
        else setResult(r)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat produk.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [kind, debouncedSearch, page, negativeOnly, reloadKey])

  // Jumlah produk bersetok minus, ditampilkan di tombol filter supaya langsung terlihat.
  useEffect(() => {
    let cancelled = false
    listProducts({ kind, negativeStock: true, page: 1, limit: 1 })
      .then((r) => {
        if (!cancelled) setNegativeCount(r.total)
      })
      .catch(() => {
        // hitungan ini tidak kritis, abaikan kalau gagal
      })
    return () => {
      cancelled = true
    }
  }, [kind, reloadKey])

  const reload = () => setReloadKey((k) => k + 1)
  const chooseKind = (value: ProductKind) => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === 'preorder') next.set(KIND_PARAM, 'po')
        else next.delete(KIND_PARAM)
        return next
      },
      { replace: true },
    )
  }
  const products = result?.data ?? []
  const total = result?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  let emptyMessage = text.empty
  if (negativeOnly) {
    emptyMessage = debouncedSearch
      ? `Tidak ada produk bersetok minus untuk "${debouncedSearch}".`
      : 'Tidak ada produk dengan stok minus.'
  } else if (debouncedSearch) {
    emptyMessage = `Tidak ada produk untuk "${debouncedSearch}".`
  }

  const editButton = (p: Product) => (
    <button
      type="button"
      onClick={() => setEditing(p)}
      aria-label={`Ubah ${p.name}`}
      className="h-11 md:pointer-fine:h-9 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer"
    >
      Ubah
    </button>
  )
  const deleteButton = (p: Product) => (
    <button
      type="button"
      onClick={() => setDeleting(p)}
      aria-label={`Hapus ${p.name}`}
      className="h-11 md:pointer-fine:h-9 px-3 border-2 border-merah text-merah rounded-sm font-semibold cursor-pointer"
    >
      Hapus
    </button>
  )

  return (
    <section>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold mr-auto">Produk</h1>
        <button
          type="button"
          onClick={() => setEditing('new')}
          className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer whitespace-nowrap"
        >
          Tambah produk
        </button>
      </div>

      {/* Produk toko dan produk PO adalah dua daftar terpisah. */}
      <div role="group" aria-label="Jenis produk" className="mt-4 grid grid-cols-2 gap-2 sm:flex">
        {KIND_TABS.map((t) => (
          <button
            key={t.value}
            type="button"
            aria-pressed={kind === t.value}
            onClick={() => chooseKind(t.value)}
            className={`min-h-11 px-2 sm:px-6 leading-tight border-2 border-ink rounded-sm font-semibold cursor-pointer ${
              kind === t.value ? 'bg-ink text-paper' : 'bg-paper'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <p className="mt-2 max-w-prose text-ink-soft">{text.intro}</p>

      <div className="mt-4 flex flex-wrap items-center gap-3">
        <input
          type="search"
          aria-label="Cari produk"
          placeholder={text.search}
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            setPage(1)
          }}
          autoFocus
          className="h-11 w-full sm:w-72 px-3 bg-white border-2 border-ink rounded-sm"
        />
        <button
          type="button"
          aria-pressed={negativeOnly}
          onClick={() => {
            setNegativeOnly((v) => !v)
            setPage(1)
          }}
          className={`flex-1 sm:flex-none h-11 px-4 border-2 rounded-sm font-semibold cursor-pointer whitespace-nowrap ${
            negativeOnly
              ? 'bg-ink border-ink text-paper'
              : negativeCount > 0
                ? 'bg-paper border-merah text-merah'
                : 'bg-paper border-ink'
          }`}
        >
          Stok minus{negativeCount > 0 ? ` (${negativeCount})` : ''}
        </button>
      </div>

      {negativeOnly && <p className="mt-4 max-w-prose text-ink-soft">{text.negative}</p>}

      {error && (
        <p role="alert" className="mt-4 font-semibold text-merah">
          {error}
        </p>
      )}

      <div
        className={`relative mt-4 overflow-x-auto bg-paper border border-line ${loading && result ? 'opacity-60' : ''}`}
        aria-busy={loading}
      >
        {/* Tablet dan layar lebar: tabel. */}
        <table className="hidden md:table w-full min-w-[44rem] text-left">
          <thead>
            <tr className="border-b-2 border-ink text-sm text-ink-soft">
              <th className="px-4 py-3 font-semibold">Produk</th>
              <th className="px-4 py-3 font-semibold text-right">Harga jual</th>
              <th className="px-4 py-3 font-semibold text-right">{text.stock}</th>
              <th className="px-4 py-3 font-semibold">
                <span className="sr-only">Aksi</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {products.map((p) => (
              <tr key={p.id} className="border-b border-line hover:bg-wall">
                <td className="px-4 py-3">
                  <span className={`font-semibold ${p.active ? '' : 'text-ink-soft'}`}>{p.name}</span>
                  {!p.active && (
                    <span className="ml-2">
                      <InactiveBadge kind={kind} />
                    </span>
                  )}
                  {p.sku && <div className="font-mono text-sm text-ink-soft">{p.sku}</div>}
                </td>
                <td className="px-4 py-3 font-mono text-right">
                  {formatRupiah(p.price)}
                  <WholesaleNote product={p} className="whitespace-nowrap" />
                </td>
                <td
                  className={`px-4 py-3 font-mono text-right ${p.stock < 0 ? 'text-merah font-semibold' : ''}`}
                  title={p.stock < 0 ? 'Stok minus: koreksi stoknya' : undefined}
                >
                  {formatNumber(p.stock)}
                </td>
                <td className="px-4 py-3">
                  <div className="flex justify-end gap-2">
                    {editButton(p)}
                    {deleteButton(p)}
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>

        {/* HP: satu kartu per produk. */}
        <ul className="md:hidden divide-y divide-line">
          {products.map((p) => (
            <li key={p.id} className="px-4 py-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className={`font-semibold break-words ${p.active ? '' : 'text-ink-soft'}`}>{p.name}</p>
                  {p.sku && <p className="font-mono text-sm text-ink-soft break-all">{p.sku}</p>}
                  <WholesaleNote product={p} />
                  {!p.active && (
                    <p className="mt-1">
                      <InactiveBadge kind={kind} />
                    </p>
                  )}
                </div>
                <div className="shrink-0 text-right">
                  <p className="font-mono font-semibold">{formatRupiah(p.price)}</p>
                </div>
              </div>
              <div className="mt-2 flex items-center justify-between gap-3">
                <p className={`text-sm ${p.stock < 0 ? 'text-merah font-semibold' : 'text-ink-soft'}`}>
                  {text.stock} <span className="font-mono">{formatNumber(p.stock)}</span>
                  {p.stock < 0 && ' (minus)'}
                </p>
                <div className="flex gap-2">
                  {editButton(p)}
                  {deleteButton(p)}
                </div>
              </div>
            </li>
          ))}
        </ul>

        {!loading && products.length === 0 && !error && (
          <p className="px-4 py-10 text-center text-ink-soft">{emptyMessage}</p>
        )}
        {loading && !result && <p className="px-4 py-10 text-center text-ink-soft">Memuat produk…</p>}
      </div>

      <Pagination
        page={page}
        pages={pages}
        total={total}
        noun={kind === 'store' ? 'produk toko' : 'produk PO'}
        onPage={setPage}
      />

      {editing && (
        <ProductFormDialog
          product={editing === 'new' ? null : editing}
          kind={kind}
          onClose={() => setEditing(null)}
          onSaved={() => {
            setEditing(null)
            reload()
          }}
        />
      )}

      {deleting && (
        <ConfirmDialog
          title="Hapus produk?"
          confirmLabel="Hapus produk"
          onClose={() => setDeleting(null)}
          onConfirm={async () => {
            await deleteProduct(deleting.id)
            setDeleting(null)
            reload()
          }}
        >
          {deleting.kind === 'preorder' ? (
            <p>
              <span className="font-semibold text-ink">{deleting.name}</span> tidak bisa dipesan lagi. Pre-order yang masih
              berjalan dengan produk ini juga tidak akan bisa diserahkan, jadi selesaikan dulu. Riwayat lama tetap utuh.
            </p>
          ) : (
            <p>
              <span className="font-semibold text-ink">{deleting.name}</span> tidak akan muncul lagi di kasir. Riwayat
              transaksi lama tetap utuh.
            </p>
          )}
        </ConfirmDialog>
      )}
    </section>
  )
}
