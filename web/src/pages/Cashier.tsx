import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { listProducts, productsByIds } from '../api/products'
import { createTransaction } from '../api/transactions'
import type { PaymentMethod, Product, ProductList, Transaction } from '../api/types'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { PaymentDialog } from '../components/PaymentDialog'
import { ReceiptDialog } from '../components/ReceiptDialog'
import { UnitPriceText } from '../components/UnitPriceText'
import { lineUnitPrice, MAX_QTY, useCart, type CartLine } from '../hooks/useCart'
import { useDebounced } from '../hooks/useDebounced'
import { formatRupiah, sentence } from '../lib/format'
import { withRequestKey } from '../lib/requestKey'

const GRID_SIZE = 24

interface Notice {
  kind: 'ok' | 'warn' | 'error'
  text: string
}

const SALE_KEY = 'sale'

const NOTICE_CLASS: Record<Notice['kind'], string> = {
  ok: 'text-hijau',
  warn: 'text-ink bg-kunyit/30 px-2',
  error: 'text-merah',
}

export function CashierPage() {
  const cart = useCart()
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounced(search.trim(), 250)
  const [grid, setGrid] = useState<ProductList | null>(null)
  const [gridError, setGridError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [notice, setNotice] = useState<Notice | null>(null)

  const [paying, setPaying] = useState(false)
  const [receipt, setReceipt] = useState<Transaction | null>(null)
  const [confirmClear, setConfirmClear] = useState(false)
  const dialogOpen = paying || receipt !== null || confirmClear

  const searchRef = useRef<HTMLInputElement>(null)

  // Penjualan yang sama memakai kunci idempotensi yang sama sampai berhasil, juga setelah dialog ditutup atau
  // halaman dimuat ulang, supaya bayar ulang setelah jaringan putus tidak tercatat dua kali.
  async function pay(method: PaymentMethod, paid: number) {
    let transaction: Transaction
    try {
      transaction = await withRequestKey(SALE_KEY, (key) =>
        createTransaction(
          {
            items: cart.lines.map((l) => ({ product_id: l.productId, qty: l.qty })),
            payment_method: method,
            paid,
            expected_total: cart.total,
          },
          key,
        ),
      )
    } catch (err) {
      if (err instanceof ApiError && err.code === 'price_changed') {
        throw new ApiError(err.status, await refreshPrices(err.message), err.code)
      }
      throw err
    }
    setPaying(false)
    setReceipt(transaction)
    cart.clear()
    setReloadKey((k) => k + 1) // stok di daftar produk berubah
  }

  // Harga di keranjang sudah lama (diubah admin selama keranjang terbuka): ambil yang terbaru, lalu kasir memeriksa
  // total baru sebelum membayar lagi.
  async function refreshPrices(serverMessage: string): Promise<string> {
    try {
      cart.refresh(await productsByIds('store', cart.lines.map((l) => l.productId)))
      setReloadKey((k) => k + 1)
      return `${sentence(serverMessage)}. Keranjang sudah memakai harga terbaru; periksa totalnya lalu bayar lagi.`
    } catch {
      return `${sentence(serverMessage)}. Muat ulang halaman untuk melihat harga terbaru.`
    }
  }

  // Di HP (juga HP landscape yang pendek) keranjang ada di bawah daftar produk. Selama bagian
  // total + Bayar di keranjang belum terlihat, bar ringkas di bawah layar menggantikannya.
  const cartFooterRef = useRef<HTMLDivElement>(null)
  const [cartFooterVisible, setCartFooterVisible] = useState(false)
  useEffect(() => {
    const el = cartFooterRef.current
    if (!el) return
    const observer = new IntersectionObserver(([entry]) => setCartFooterVisible(entry.isIntersecting))
    observer.observe(el)
    return () => observer.disconnect()
  }, [])

  // Daftar produk untuk layar sentuh. Kasir hanya menjual produk toko.
  useEffect(() => {
    let cancelled = false
    listProducts({ kind: 'store', search: debouncedSearch, page: 1, limit: GRID_SIZE, activeOnly: true })
      .then((r) => {
        if (cancelled) return
        setGrid(r)
        setGridError('')
      })
      .catch((e) => {
        if (!cancelled) setGridError(e instanceof ApiError ? e.message : 'Gagal memuat produk.')
      })
    return () => {
      cancelled = true
    }
  }, [debouncedSearch, reloadKey])

  // Pesan status hilang sendiri.
  useEffect(() => {
    if (!notice) return
    const timer = setTimeout(() => setNotice(null), 3000)
    return () => clearTimeout(timer)
  }, [notice])

  // Kolom cari selalu siap: setelah dialog tertutup, fokus kembali ke sana.
  useEffect(() => {
    if (!dialogOpen) searchRef.current?.focus()
  }, [dialogOpen])

  // Mengetik di mana saja langsung masuk ke kolom cari: kalau fokus sedang di tombol atau di luar kolom isian,
  // karakter pertama mengarahkan fokus ke kolom cari supaya huruf yang diketik tidak hilang.
  useEffect(() => {
    if (dialogOpen) return
    function onKeyDown(e: KeyboardEvent) {
      if (e.ctrlKey || e.metaKey || e.altKey || e.key.length !== 1 || e.key === ' ') return
      const el = document.activeElement
      const typing =
        el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement || (el as HTMLElement | null)?.isContentEditable
      if (!typing) searchRef.current?.focus()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [dialogOpen])

  function addProduct(product: Product) {
    cart.add(product)
    if (product.stock <= 0) {
      setNotice({ kind: 'warn', text: `${product.name}: stok tercatat ${product.stock}. Tetap bisa dijual.` })
    } else {
      setNotice({ kind: 'ok', text: `${product.name} ditambahkan` })
    }
  }

  // Enter di kolom cari: produk pertama dari hasil pencarian (nama atau SKU) langsung masuk keranjang.
  async function handleSearchEnter(e: FormEvent) {
    e.preventDefault()
    const query = search.trim()
    if (!query) return
    setSearch('')

    try {
      const found = await listProducts({ kind: 'store', search: query, page: 1, limit: 1, activeOnly: true })
      const product = found.data[0]
      if (product) addProduct(product)
      else setNotice({ kind: 'error', text: `"${query}" tidak ditemukan.` })
    } catch (err) {
      setNotice({ kind: 'error', text: err instanceof ApiError ? err.message : 'Gagal mencari produk.' })
    }
  }

  const products = grid?.data ?? []
  // Jumlah tiap produk di keranjang, untuk angka di pojok kartu produk.
  const qtyInCart = new Map(cart.lines.map((l) => [l.productId, l.qty]))

  return (
    <div className="grid gap-4 kasir-lebar:grid-cols-[minmax(0,1fr)_20rem] kasir-lebar:lg:grid-cols-[minmax(0,1fr)_24rem] items-start pb-24 kasir-lebar:pb-0">
      <section aria-label="Pilih produk" className="@container min-w-0">
        <form onSubmit={(e) => void handleSearchEnter(e)}>
          <label htmlFor="cari-produk" className="sr-only">
            Cari produk
          </label>
          <input
            id="cari-produk"
            ref={searchRef}
            type="search"
            enterKeyHint="search"
            autoComplete="off"
            autoFocus
            placeholder="Cari produk"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full h-14 px-4 text-lg bg-white border-2 border-ink rounded-sm"
          />
        </form>

        <p role="status" className="mt-2 min-h-7 font-semibold">
          {notice && <span className={NOTICE_CLASS[notice.kind]}>{notice.text}</span>}
        </p>

        {gridError && (
          <p role="alert" className="font-semibold text-merah">
            {gridError}
          </p>
        )}

        {/* Kartu produk: bayangan tegas seperti tombol, harga di label kuning seperti label harga di rak, stok
            sebagai lencana, dan angka di pojok = jumlah barang itu di keranjang. */}
        <ul className="grid grid-cols-2 @lg:grid-cols-3 @2xl:grid-cols-4 gap-2">
          {products.map((p) => {
            const inCart = qtyInCart.get(p.id) ?? 0
            return (
              <li key={p.id}>
                <button
                  type="button"
                  onClick={() => addProduct(p)}
                  className="relative w-full h-full min-h-25 p-2.5 text-left bg-white border-2 border-ink rounded-md shadow-[3px_3px_0_var(--color-ink)] flex flex-col justify-between gap-1.5 cursor-pointer transition-[translate,box-shadow] duration-75 hover:bg-kunyit/10 active:translate-x-0.5 active:translate-y-0.5 active:shadow-[1px_1px_0_var(--color-ink)]"
                >
                  <span className="text-[15px] font-bold leading-tight line-clamp-2">{p.name}</span>
                  <span className="flex flex-wrap items-center justify-between gap-x-[3px] gap-y-1">
                    <span className="px-[7px] py-px border-2 border-ink rounded-full bg-kunyit font-mono text-[12.5px] font-bold whitespace-nowrap">
                      {formatRupiah(p.price)}
                    </span>
                    {p.stock <= 0 ? (
                      <span className="px-[5px] py-px rounded-full bg-merah text-white text-[11px] font-bold whitespace-nowrap">
                        Stok {p.stock}
                      </span>
                    ) : p.stock <= 5 ? (
                      <span className="px-[5px] py-px rounded-full bg-[#fdeeb8] text-[#6b4d00] text-[11px] font-bold whitespace-nowrap">
                        Sisa {p.stock}
                      </span>
                    ) : null}
                  </span>
                  {inCart > 0 && (
                    <>
                      <span
                        aria-hidden="true"
                        className="absolute -top-2 -right-2 min-w-6 h-6 px-1.5 border-2 border-paper rounded-full bg-ink text-white font-mono text-xs font-bold leading-5 text-center"
                      >
                        {inCart}
                      </span>
                      <span className="sr-only">, {inCart} di keranjang</span>
                    </>
                  )}
                </button>
              </li>
            )
          })}
        </ul>

        {grid && products.length === 0 && !gridError && (
          <p className="py-10 text-center text-ink-soft">
            {debouncedSearch
              ? `Tidak ada produk untuk "${debouncedSearch}".`
              : 'Belum ada produk aktif. Admin bisa menambahkannya di menu Produk.'}
          </p>
        )}
        {grid && grid.total > GRID_SIZE && (
          <p className="mt-3 text-sm text-ink-soft">
            Menampilkan {GRID_SIZE} dari {grid.total} produk. Ketik untuk mempersempit.
          </p>
        )}
      </section>

      <aside
        id="keranjang"
        aria-label="Keranjang"
        className="struk scroll-mt-4 bg-paper px-4 sm:px-5 pt-5 pb-10 flex flex-col kasir-lebar:sticky kasir-lebar:top-4 kasir-lebar:max-h-[calc(100dvh-7rem)]"
      >
        <div className="flex items-baseline justify-between">
          <h2 className="text-xl font-bold">Keranjang</h2>
          {cart.lines.length > 0 && (
            <button
              type="button"
              onClick={() => setConfirmClear(true)}
              className="py-2 -my-2 text-sm font-semibold underline underline-offset-2 text-ink-soft cursor-pointer"
            >
              Kosongkan
            </button>
          )}
        </div>

        <div className="mt-3 flex-1 min-h-0 overflow-y-auto overflow-x-hidden">
          {cart.lines.length === 0 ? (
            <p className="py-8 text-center text-ink-soft">Keranjang kosong. Cari atau pilih produk.</p>
          ) : (
            <ul className="divide-y divide-dashed divide-line">
              {cart.lines.map((line) => (
                <CartRow
                  key={line.productId}
                  line={line}
                  onQty={(qty) => cart.setQty(line.productId, qty)}
                  onRemove={() => cart.remove(line.productId)}
                />
              ))}
            </ul>
          )}
        </div>

        <div ref={cartFooterRef} className="mt-3 pt-3 border-t-2 border-dashed border-ink">
          <div className="flex flex-wrap items-baseline justify-between gap-x-3">
            <span className="font-semibold">Total ({cart.itemCount} barang)</span>
            <span className="font-mono text-3xl font-semibold">{formatRupiah(cart.total)}</span>
          </div>
          <button
            type="button"
            disabled={cart.lines.length === 0}
            onClick={() => setPaying(true)}
            className="mt-3 w-full h-14 bg-kunyit border-2 border-ink rounded-sm text-lg font-bold cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Bayar
          </button>
        </div>
      </aside>

      {!cartFooterVisible && (
        <div className="kasir-lebar:hidden fixed inset-x-0 bottom-0 z-20 bg-paper border-t-2 border-ink px-4 pt-3 pb-[max(0.75rem,env(safe-area-inset-bottom))] flex items-center gap-3">
          <button
            type="button"
            onClick={() => document.getElementById('keranjang')?.scrollIntoView({ behavior: 'smooth', block: 'start' })}
            className="flex-1 min-w-0 text-left cursor-pointer"
          >
            <span className="block text-sm font-semibold">
              {cart.lines.length === 0 ? 'Keranjang kosong' : `Keranjang · ${cart.itemCount} barang`}{' '}
              <span className="text-ink-soft underline underline-offset-2">Lihat</span>
            </span>
            <span className="block truncate font-mono text-xl font-semibold">{formatRupiah(cart.total)}</span>
          </button>
          <button
            type="button"
            disabled={cart.lines.length === 0}
            onClick={() => setPaying(true)}
            className="shrink-0 h-12 px-6 bg-kunyit border-2 border-ink rounded-sm text-lg font-bold cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
          >
            Bayar
          </button>
        </div>
      )}

      {paying && <PaymentDialog total={cart.total} onClose={() => setPaying(false)} onPay={pay} />}

      {receipt && <ReceiptDialog transaction={receipt} onClose={() => setReceipt(null)} />}

      {confirmClear && (
        <ConfirmDialog
          title="Kosongkan keranjang?"
          confirmLabel="Kosongkan"
          onClose={() => setConfirmClear(false)}
          onConfirm={async () => {
            cart.clear()
            setConfirmClear(false)
          }}
        >
          <p>Semua barang di keranjang akan dihapus.</p>
        </ConfirmDialog>
      )}
    </div>
  )
}

function CartRow({
  line,
  onQty,
  onRemove,
}: {
  line: CartLine
  onQty: (qty: number) => void
  onRemove: () => void
}) {
  const stepClass = 'size-11 pointer-fine:size-9 grid place-items-center border-2 border-ink bg-white text-lg font-bold cursor-pointer'

  return (
    <li className="py-3">
      <div className="flex items-start justify-between gap-3">
        <p className="font-semibold leading-tight">{line.name}</p>
        <button
          type="button"
          aria-label={`Hapus ${line.name} dari keranjang`}
          onClick={onRemove}
          className="size-8 -mr-1 -mt-1 grid place-items-center text-xl leading-none text-ink-soft hover:text-merah cursor-pointer"
        >
          ×
        </button>
      </div>

      <div className="mt-2 flex items-center justify-between gap-3">
        <div className="flex items-center">
          <button
            type="button"
            aria-label={`Kurangi ${line.name}`}
            disabled={line.qty <= 1}
            onClick={() => onQty(line.qty - 1)}
            className={`${stepClass} rounded-l-sm disabled:opacity-40 disabled:cursor-not-allowed`}
          >
            −
          </button>
          <input
            aria-label={`Jumlah ${line.name}`}
            inputMode="numeric"
            value={line.qty}
            onFocus={(e) => e.target.select()}
            onChange={(e) => {
              const n = Number(e.target.value.replace(/\D/g, ''))
              if (n > 0) onQty(Math.min(n, MAX_QTY))
            }}
            className="w-12 h-11 pointer-fine:h-9 text-center font-mono border-y-2 border-ink bg-white"
          />
          <button
            type="button"
            aria-label={`Tambah ${line.name}`}
            onClick={() => onQty(line.qty + 1)}
            className={`${stepClass} rounded-r-sm`}
          >
            +
          </button>
        </div>

        <div className="min-w-0 text-right">
          <div className="font-mono font-semibold">{formatRupiah(lineUnitPrice(line) * line.qty)}</div>
          <UnitPriceText rule={line} qty={line.qty} align="end" />
        </div>
      </div>

      {line.qty > line.stock && (
        <p className="mt-1 text-xs font-semibold text-merah">Stok tercatat {line.stock}. Tetap bisa dijual.</p>
      )}
    </li>
  )
}
