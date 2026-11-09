import { useEffect, useRef, useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { createPreorder } from '../api/preorders'
import { listProducts, productsByIds } from '../api/products'
import type { Fulfillment, PaymentMethod, Preorder, PreorderInput, Product } from '../api/types'
import { useDebounced } from '../hooks/useDebounced'
import { usePreorderQuota } from '../hooks/usePreorderQuota'
import { addDays, formatNumber, formatRupiah, sentence, toDateInput } from '../lib/format'
import { FULFILLMENT_LABEL, MAX_PREORDER_FEE } from '../lib/preorder'
import { PAYMENT_METHODS } from '../lib/payment'
import { withRequestKey } from '../lib/requestKey'
import { priceRule, unitPrice } from '../lib/wholesale'
import { Modal } from './Modal'
import { QuotaRemaining, quotaProblem } from './PreorderQuota'
import { UnitPriceText } from './UnitPriceText'

interface Line {
  product: Product
  qty: number
}

// Jumlah satu baris: harga grosir otomatis kalau qty mencapai batas produk itu (server menghitung ulang).
const lineTotal = (l: Line) => unitPrice(priceRule(l.product), l.qty) * l.qty

const inputClass = 'w-full h-11 px-3 bg-white border-2 border-ink rounded-sm'
const labelClass = 'block mb-1 text-sm font-semibold'
const digitsOnly = (value: string) => value.replace(/\D/g, '').slice(0, 12)

const PREORDER_FORM_KEY = 'preorder-form'

export function PreorderFormDialog({
  onClose,
  onCreated,
}: {
  onClose: () => void
  onCreated: (preorder: Preorder) => void
}) {
  const now = new Date()
  const [name, setName] = useState('')
  const [phone, setPhone] = useState('')
  const [dueDate, setDueDate] = useState(toDateInput(addDays(now, 1)))
  const [dueTime, setDueTime] = useState('')
  const [fulfillment, setFulfillment] = useState<Fulfillment>('pickup')
  const [address, setAddress] = useState('')
  const [notes, setNotes] = useState('')
  const [lines, setLines] = useState<Line[]>([])
  const [search, setSearch] = useState('')
  const debouncedSearch = useDebounced(search.trim(), 250)
  const [results, setResults] = useState<Product[]>([])
  // Kata kunci yang hasilnya sudah datang, supaya "tidak ditemukan" tidak muncul saat masih memuat.
  const [searchedFor, setSearchedFor] = useState('')
  // Pencarian langsung dari Enter sedang berjalan: Enter berikutnya diabaikan supaya produk tidak masuk dua kali.
  const lookingUp = useRef(false)
  const [dpAmount, setDpAmount] = useState('')
  const [dpMethod, setDpMethod] = useState<PaymentMethod>('cash')
  // Ongkir (hanya kalau diantar) dan biaya lain, mis. kresek. Ikut total yang dibayar pelanggan.
  const [shippingFee, setShippingFee] = useState('')
  const [otherFee, setOtherFee] = useState('')
  const [otherFeeNote, setOtherFeeNote] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  // Pernah ditolak karena kuota: pesannya ikut tampil di dekat tombol, dan hilang sendiri setelah diperbaiki.
  const [quotaBlocked, setQuotaBlocked] = useState(false)

  // Pre-order hanya memesan produk PO (dibuat sesuai pesanan), bukan produk toko.
  useEffect(() => {
    if (!debouncedSearch) {
      setResults([])
      setSearchedFor('')
      return
    }
    let cancelled = false
    listProducts({ kind: 'preorder', search: debouncedSearch, page: 1, limit: 6, activeOnly: true })
      .then((r) => {
        if (!cancelled) setResults(r.data.filter((p) => p.active))
      })
      .catch(() => {
        if (!cancelled) setResults([])
      })
      .finally(() => {
        if (!cancelled) setSearchedFor(debouncedSearch)
      })
    return () => {
      cancelled = true
    }
  }, [debouncedSearch])

  const subtotal = lines.reduce((sum, l) => sum + lineTotal(l), 0)
  const ongkir = fulfillment === 'delivery' ? Number(shippingFee || '0') : 0
  const lain = Number(otherFee || '0')
  const total = subtotal + ongkir + lain
  const dp = Number(dpAmount || '0')
  // Kuota dihitung dalam pcs, semua produk PO digabung.
  const pieces = lines.reduce((sum, l) => sum + l.qty, 0)
  const quota = usePreorderQuota(dueDate)
  const overQuota = quotaProblem(quota, pieces)
  const piecesNote =
    overQuota ||
    (quota
      ? `Jumlah ${formatNumber(pieces)} pcs. Sisa kuota tanggal ini setelah pesanan ini: ` +
        `${formatNumber(quota.remaining - pieces)} pcs.`
      : `Jumlah ${formatNumber(pieces)} pcs.`)

  function addProduct(product: Product) {
    setLines((prev) =>
      prev.some((l) => l.product.id === product.id)
        ? prev.map((l) => (l.product.id === product.id ? { product, qty: Math.min(l.qty + 1, 9999) } : l))
        : [...prev, { product, qty: 1 }],
    )
    setSearch('')
    setResults([])
  }

  // Enter di kolom cari: produk pertama untuk teks yang sedang diketik. Kalau hasilnya belum datang (mengetik cepat
  // lalu Enter), produknya dicari langsung, supaya yang masuk bukan hasil dari kata kunci sebelumnya.
  async function addFirstMatch() {
    const query = search.trim()
    if (!query || lookingUp.current) return
    if (query === searchedFor) {
      if (results[0]) addProduct(results[0])
      return
    }
    lookingUp.current = true
    try {
      const found = await listProducts({ kind: 'preorder', search: query, page: 1, limit: 1, activeOnly: true })
      const product = found.data.find((p) => p.active)
      if (product) addProduct(product)
    } catch {
      // pencarian biasa untuk teks yang sama menyusul dan menampilkan hasil atau pesannya
    } finally {
      lookingUp.current = false
    }
  }

  function setQty(id: string, qty: number) {
    setLines((prev) => prev.map((l) => (l.product.id === id ? { ...l, qty: Math.min(Math.max(qty, 1), 9999) } : l)))
  }

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')

    if (!name.trim()) return setError('Nama pelanggan wajib diisi.')
    if (!dueDate) return setError('Tanggal hari H wajib diisi.')
    if (fulfillment === 'delivery' && !address.trim()) return setError('Alamat pengantaran wajib diisi.')
    if (lines.length === 0) return setError('Tambahkan minimal satu barang.')
    if (overQuota) return setQuotaBlocked(true)
    if (ongkir > MAX_PREORDER_FEE || lain > MAX_PREORDER_FEE) {
      return setError(`Ongkir dan biaya lain maksimal ${formatRupiah(MAX_PREORDER_FEE)}.`)
    }
    if (dp > total) return setError('DP tidak boleh melebihi total pesanan.')

    const input: PreorderInput = {
      customer_name: name.trim(),
      customer_phone: phone.trim() || null,
      due_date: dueDate,
      due_time: dueTime || null,
      fulfillment,
      address: fulfillment === 'delivery' ? address.trim() || null : null,
      notes: notes.trim() || null,
      items: lines.map((l) => ({ product_id: l.product.id, qty: l.qty })),
      shipping_fee: ongkir,
      other_fee: lain,
      other_fee_note: lain > 0 ? otherFeeNote.trim() || null : null,
      down_payment: dp,
      down_payment_method: dp > 0 ? dpMethod : '',
      expected_total: total,
    }

    setBusy(true)
    try {
      // Satu kunci sampai pre-order ini tersimpan, juga kalau form ditutup lalu dibuka lagi setelah jaringan putus.
      onCreated(await withRequestKey(PREORDER_FORM_KEY, (key) => createPreorder(input, key)))
    } catch (err) {
      if (err instanceof ApiError && err.code === 'price_changed') setError(await refreshPrices(err.message))
      else setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  // Harga produk diubah selama form terbuka: ambil yang terbaru supaya total di layar sama dengan yang disimpan.
  async function refreshPrices(serverMessage: string): Promise<string> {
    try {
      const latest = new Map(
        (await productsByIds('preorder', lines.map((l) => l.product.id))).map((p) => [p.id, p]),
      )
      setLines((prev) => prev.map((l) => ({ ...l, product: latest.get(l.product.id) ?? l.product })))
      return `${sentence(serverMessage)}. Harga barang sudah diperbarui; periksa totalnya lalu simpan lagi.`
    } catch {
      return `${sentence(serverMessage)}. Tutup lalu buka lagi form ini untuk memuat harga terbaru.`
    }
  }

  return (
    <Modal title="Buat pre-order" onClose={onClose} size="wide" busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-5">
        <div className="grid sm:grid-cols-2 gap-3">
          <div>
            <label htmlFor="po-name" className={labelClass}>
              Nama pelanggan
            </label>
            <input
              id="po-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
              maxLength={100}
              className={inputClass}
            />
          </div>
          <div>
            <label htmlFor="po-phone" className={labelClass}>
              No. HP (opsional)
            </label>
            <input
              id="po-phone"
              type="tel"
              inputMode="tel"
              value={phone}
              onChange={(e) => setPhone(e.target.value)}
              maxLength={30}
              className={`${inputClass} font-mono`}
            />
          </div>
          <div>
            <label htmlFor="po-date" className={labelClass}>
              Tanggal hari H
            </label>
            <input
              id="po-date"
              type="date"
              min={toDateInput(now)}
              value={dueDate}
              onChange={(e) => setDueDate(e.target.value)}
              aria-describedby="po-date-quota"
              className={inputClass}
            />
            <QuotaRemaining quota={quota} id="po-date-quota" />
          </div>
          <div>
            <label htmlFor="po-time" className={labelClass}>
              Jam (opsional)
            </label>
            <input
              id="po-time"
              type="time"
              value={dueTime}
              onChange={(e) => setDueTime(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>

        <div>
          <p className={labelClass}>Cara serah</p>
          <div role="radiogroup" aria-label="Cara serah" className="flex gap-2">
            {(['pickup', 'delivery'] as const).map((f) => (
              <button
                key={f}
                type="button"
                role="radio"
                aria-checked={fulfillment === f}
                onClick={() => setFulfillment(f)}
                className={`h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer ${
                  fulfillment === f ? 'bg-ink text-paper' : 'bg-paper'
                }`}
              >
                {FULFILLMENT_LABEL[f]}
              </button>
            ))}
          </div>
          {fulfillment === 'delivery' && (
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <div className="sm:col-span-2">
                <label htmlFor="po-address" className={labelClass}>
                  Alamat pengantaran
                </label>
                <textarea
                  id="po-address"
                  value={address}
                  onChange={(e) => setAddress(e.target.value)}
                  maxLength={300}
                  rows={2}
                  className="w-full px-3 py-2 bg-white border-2 border-ink rounded-sm"
                />
              </div>
              <div>
                <label htmlFor="po-shipping" className={labelClass}>
                  Ongkir (opsional, Rp)
                </label>
                <input
                  id="po-shipping"
                  inputMode="numeric"
                  autoComplete="off"
                  value={shippingFee ? formatNumber(Number(shippingFee)) : ''}
                  onChange={(e) => setShippingFee(digitsOnly(e.target.value))}
                  className={`${inputClass} font-mono`}
                />
              </div>
            </div>
          )}
        </div>

        <div>
          <label htmlFor="po-search" className={labelClass}>
            Barang yang dipesan
          </label>
          <input
            id="po-search"
            type="search"
            autoComplete="off"
            placeholder="Cari produk PO lalu pilih"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            // Enter di kolom ini menambah produk, bukan menyimpan form.
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                void addFirstMatch()
              }
            }}
            className={inputClass}
          />
          {debouncedSearch &&
            search.trim() === debouncedSearch &&
            searchedFor === debouncedSearch &&
            results.length === 0 && (
              <p className="mt-1 text-sm text-ink-soft">
                Tidak ada produk PO untuk "{debouncedSearch}". Produk PO ditambahkan admin di menu Produk, tab PO.
              </p>
            )}
          {results.length > 0 && (
            <ul className="mt-1 border-2 border-ink bg-white divide-y divide-line">
              {results.map((p) => (
                <li key={p.id}>
                  <button
                    type="button"
                    onClick={() => addProduct(p)}
                    className="w-full px-3 py-2 flex items-baseline justify-between gap-3 text-left hover:bg-wall cursor-pointer"
                  >
                    <span className="font-semibold">{p.name}</span>
                    <span className="font-mono text-sm">{formatRupiah(p.price)}</span>
                  </button>
                </li>
              ))}
            </ul>
          )}

          {lines.length > 0 && (
            <ul className="mt-3 divide-y divide-dashed divide-line border-y border-line">
              {lines.map((l) => (
                // HP: nama + × di atas, jumlah + total di bawah (seperti keranjang Kasir). Mulai sm: satu baris.
                <li key={l.product.id} className="py-2 flex flex-wrap items-center gap-x-3 gap-y-1">
                  <div className="min-w-0 flex-1">
                    <p className="font-semibold leading-tight">{l.product.name}</p>
                    <UnitPriceText rule={priceRule(l.product)} qty={l.qty} align="start" />
                  </div>
                  <button
                    type="button"
                    aria-label={`Hapus ${l.product.name}`}
                    onClick={() => setLines((prev) => prev.filter((x) => x.product.id !== l.product.id))}
                    className="size-8 self-start sm:self-center sm:order-last grid place-items-center text-xl leading-none text-ink-soft hover:text-merah cursor-pointer"
                  >
                    ×
                  </button>
                  <div className="w-full sm:w-auto flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
                    <div className="flex items-center">
                      <button
                        type="button"
                        aria-label={`Kurangi ${l.product.name}`}
                        disabled={l.qty <= 1}
                        onClick={() => setQty(l.product.id, l.qty - 1)}
                        className="size-11 pointer-fine:size-9 grid place-items-center border-2 border-ink bg-white rounded-l-sm text-lg font-bold cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                      >
                        −
                      </button>
                      <input
                        aria-label={`Jumlah ${l.product.name}`}
                        inputMode="numeric"
                        value={l.qty}
                        onFocus={(e) => e.target.select()}
                        onChange={(e) => {
                          const n = Number(e.target.value.replace(/\D/g, ''))
                          if (n > 0) setQty(l.product.id, n)
                        }}
                        className="w-12 h-11 pointer-fine:h-9 text-center font-mono border-y-2 border-ink bg-white"
                      />
                      <button
                        type="button"
                        aria-label={`Tambah ${l.product.name}`}
                        onClick={() => setQty(l.product.id, l.qty + 1)}
                        className="size-11 pointer-fine:size-9 grid place-items-center border-2 border-ink bg-white rounded-r-sm text-lg font-bold cursor-pointer"
                      >
                        +
                      </button>
                    </div>
                    <span className="ml-auto sm:w-28 text-right font-mono font-semibold">
                      {formatRupiah(lineTotal(l))}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}

          <p className="mt-3 flex items-baseline justify-between gap-3">
            <span className="font-semibold">Subtotal barang</span>
            <span className="font-mono text-lg font-semibold">{formatRupiah(subtotal)}</span>
          </p>
          <p className="text-sm text-ink-soft">Harga dikunci sekarang, jadi tidak berubah walau harga produk naik.</p>
          <p
            aria-live="polite"
            className={`mt-2 empty:hidden text-sm ${overQuota ? 'font-semibold text-merah' : 'text-ink-soft'}`}
          >
            {pieces > 0 ? piecesNote : ''}
          </p>
        </div>

        <div className="grid sm:grid-cols-2 gap-3">
          <div>
            <label htmlFor="po-other-fee" className={labelClass}>
              Biaya lain (opsional, Rp)
            </label>
            <input
              id="po-other-fee"
              inputMode="numeric"
              autoComplete="off"
              value={otherFee ? formatNumber(Number(otherFee)) : ''}
              onChange={(e) => setOtherFee(digitsOnly(e.target.value))}
              className={`${inputClass} font-mono`}
            />
          </div>
          <div>
            <label htmlFor="po-other-note" className={labelClass}>
              Keterangan biaya lain
            </label>
            <input
              id="po-other-note"
              placeholder="mis. kresek"
              value={otherFeeNote}
              onChange={(e) => setOtherFeeNote(e.target.value)}
              disabled={lain === 0}
              maxLength={40}
              className={`${inputClass} disabled:opacity-50`}
            />
          </div>
        </div>

        <div>
          <p className="flex items-baseline justify-between gap-3">
            <span className="font-semibold">Total pesanan</span>
            <span className="font-mono text-2xl font-semibold">{formatRupiah(total)}</span>
          </p>
          {ongkir + lain > 0 && (
            <p className="text-sm text-ink-soft">
              Barang {formatRupiah(subtotal)}
              {lain > 0 && ` + biaya lain ${formatRupiah(lain)}`}
              {ongkir > 0 && ` + ongkir ${formatRupiah(ongkir)}`}
            </p>
          )}
        </div>

        <div className="grid sm:grid-cols-2 gap-3">
          <div>
            <label htmlFor="po-dp" className={labelClass}>
              DP diterima (opsional, Rp)
            </label>
            <input
              id="po-dp"
              inputMode="numeric"
              autoComplete="off"
              value={dpAmount ? formatNumber(Number(dpAmount)) : ''}
              onChange={(e) => setDpAmount(digitsOnly(e.target.value))}
              className={`${inputClass} font-mono`}
            />
          </div>
          <div>
            <label htmlFor="po-dp-method" className={labelClass}>
              Metode DP
            </label>
            <select
              id="po-dp-method"
              value={dpMethod}
              onChange={(e) => setDpMethod(e.target.value as PaymentMethod)}
              disabled={dp === 0}
              className={`${inputClass} disabled:opacity-50`}
            >
              {PAYMENT_METHODS.map((m) => (
                <option key={m.value} value={m.value}>
                  {m.label}
                </option>
              ))}
            </select>
          </div>
        </div>
        {dp > 0 && dp <= total && (
          <p className="font-mono">Sisa yang dibayar saat serah: {formatRupiah(total - dp)}</p>
        )}

        <div>
          <label htmlFor="po-notes" className={labelClass}>
            Catatan (opsional)
          </label>
          <textarea
            id="po-notes"
            value={notes}
            onChange={(e) => setNotes(e.target.value)}
            maxLength={500}
            rows={2}
            className="w-full px-3 py-2 bg-white border-2 border-ink rounded-sm"
          />
        </div>

        {(error || (quotaBlocked && overQuota)) && (
          <p role="alert" className="font-semibold text-merah">
            {error || overQuota}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
          >
            Batal
          </button>
          <button
            type="submit"
            disabled={busy}
            className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
          >
            Simpan pre-order
          </button>
        </div>
      </form>
    </Modal>
  )
}
