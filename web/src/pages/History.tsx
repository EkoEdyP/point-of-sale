import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { listTransactions } from '../api/transactions'
import type { TransactionList } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { Pagination } from '../components/Pagination'
import { SourceBadge } from '../components/SourceBadge'
import { TransactionDetailDialog } from '../components/TransactionDetailDialog'
import { addDays, formatDateTime, formatRupiah, toDateInput } from '../lib/format'
import { PAYMENT_LABEL } from '../lib/payment'

const PAGE_SIZE = 20

const fieldClass = 'h-11 w-full sm:w-auto min-w-0 px-3 bg-white border-2 border-ink rounded-sm'

function VoidBadge() {
  return (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-merah text-merah rounded-sm">Dibatalkan</span>
  )
}

export function HistoryPage() {
  const { state } = useAuth()
  const isAdmin = state.status === 'authenticated' && state.user.role === 'admin'

  const today = toDateInput(new Date())
  const [from, setFrom] = useState(today)
  const [to, setTo] = useState(today)
  const [status, setStatus] = useState('')
  const [page, setPage] = useState(1)

  const [result, setResult] = useState<TransactionList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [openId, setOpenId] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listTransactions({ from, to, status, page, limit: PAGE_SIZE })
      .then((r) => {
        if (cancelled) return
        if (r.data.length === 0 && page > 1) setPage(page - 1)
        else setResult(r)
        setError('')
      })
      .catch((e) => {
        if (cancelled) return
        setError(e instanceof ApiError ? e.message : 'Gagal memuat riwayat.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [from, to, status, page, reloadKey])

  function setRange(start: string, end: string) {
    setFrom(start)
    setTo(end)
    setPage(1)
  }

  const yesterday = toDateInput(addDays(new Date(), -1))
  const weekStart = toDateInput(addDays(new Date(), -6))
  const ranges = [
    { label: 'Hari ini', from: today, to: today },
    { label: 'Kemarin', from: yesterday, to: yesterday },
    { label: '7 hari terakhir', short: '7 hari', from: weekStart, to: today },
  ]

  const transactions = result?.data ?? []
  const total = result?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <section>
      <h1 className="text-2xl font-bold">Riwayat</h1>
      <p className="mt-1 text-ink-soft">
        {isAdmin ? 'Semua transaksi toko.' : 'Transaksi yang kamu buat.'} Pilih transaksi untuk melihat struk,
        mencetak ulang, atau membatalkan.
      </p>

      <div className="mt-4 grid gap-3 xs:grid-cols-2 sm:flex sm:flex-wrap sm:items-end sm:gap-x-4 sm:gap-y-3">
        <div className="xs:col-span-2 grid grid-cols-3 gap-2 sm:flex">
          {ranges.map((r) => {
            const active = from === r.from && to === r.to
            return (
              <button
                key={r.label}
                type="button"
                aria-pressed={active}
                onClick={() => setRange(r.from, r.to)}
                className={`h-11 px-1 sm:px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer ${
                  active ? 'bg-ink text-paper' : 'bg-paper'
                }`}
              >
                {r.short ? (
                  <>
                    <span className="sm:hidden">{r.short}</span>
                    <span className="hidden sm:inline">{r.label}</span>
                  </>
                ) : (
                  r.label
                )}
              </button>
            )
          })}
        </div>

        {/* Dari dan Sampai pindah baris bersama; di bawah sm tetap dua sel grid. */}
        <div className="contents sm:flex sm:gap-x-4">
          <div className="min-w-0">
            <label htmlFor="h-from" className="block mb-1 text-sm font-semibold">
              Dari
            </label>
            <input
              id="h-from"
              type="date"
              value={from}
              onChange={(e) => {
                setFrom(e.target.value)
                setPage(1)
              }}
              className={fieldClass}
            />
          </div>
          <div className="min-w-0">
            <label htmlFor="h-to" className="block mb-1 text-sm font-semibold">
              Sampai
            </label>
            <input
              id="h-to"
              type="date"
              value={to}
              onChange={(e) => {
                setTo(e.target.value)
                setPage(1)
              }}
              className={fieldClass}
            />
          </div>
        </div>
        <div className="xs:col-span-2 min-w-0">
          <label htmlFor="h-status" className="block mb-1 text-sm font-semibold">
            Status
          </label>
          <select
            id="h-status"
            value={status}
            onChange={(e) => {
              setStatus(e.target.value)
              setPage(1)
            }}
            className={fieldClass}
          >
            <option value="">Semua</option>
            <option value="completed">Selesai</option>
            <option value="void">Dibatalkan</option>
          </select>
        </div>
      </div>

      {error && (
        <p role="alert" className="mt-4 font-semibold text-merah">
          {error}
        </p>
      )}

      <div
        className={`relative mt-4 lg:overflow-x-auto bg-paper border border-line ${loading && result ? 'opacity-60' : ''}`}
        aria-busy={loading}
      >
        {/* Layar lebar: tabel. Mulai lg karena dengan kolom Jenis tabelnya lebih lebar dari tablet 768 px. */}
        <table className="hidden lg:table w-full min-w-[40rem] text-left">
          <thead>
            <tr className="border-b-2 border-ink text-sm text-ink-soft">
              <th className="px-4 py-3 font-semibold">Waktu</th>
              <th className="px-4 py-3 font-semibold">No. struk</th>
              <th className="px-4 py-3 font-semibold">Jenis</th>
              {isAdmin && <th className="px-4 py-3 font-semibold">Kasir</th>}
              <th className="px-4 py-3 font-semibold">Bayar</th>
              <th className="px-4 py-3 font-semibold text-right">Total</th>
              <th className="px-4 py-3 font-semibold">Status</th>
              <th className="px-4 py-3 font-semibold">
                <span className="sr-only">Aksi</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {transactions.map((t) => {
              const isVoid = t.status === 'void'
              return (
                <tr key={t.id} className="border-b border-line hover:bg-wall">
                  <td className="px-4 py-3 font-mono text-sm whitespace-nowrap">{formatDateTime(t.created_at)}</td>
                  <td className="px-4 py-3 font-mono text-sm">{t.receipt_no}</td>
                  {/* Jenis: Toko (penjualan di kasir) atau PO (penyerahan pre-order) beserta nomornya. */}
                  <td className="px-4 py-3">
                    <SourceBadge source={t.source} />
                    {t.preorder_no && (
                      <div className="mt-1 font-mono text-sm text-ink-soft whitespace-nowrap">{t.preorder_no}</div>
                    )}
                  </td>
                  {isAdmin && <td className="px-4 py-3">{t.cashier_name}</td>}
                  <td className="px-4 py-3">{PAYMENT_LABEL[t.payment_method]}</td>
                  <td className={`px-4 py-3 font-mono text-right ${isVoid ? 'text-ink-soft line-through' : ''}`}>
                    {formatRupiah(t.total)}
                  </td>
                  <td className="px-4 py-3">{isVoid ? <VoidBadge /> : <span className="text-sm">Selesai</span>}</td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => setOpenId(t.id)}
                      aria-label={`Detail transaksi ${t.receipt_no}`}
                      className="h-11 pointer-fine:h-9 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer"
                    >
                      Detail
                    </button>
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>

        {/* HP dan tablet: satu kartu per transaksi, ketuk untuk membuka detail. */}
        <ul className="lg:hidden divide-y divide-line">
          {transactions.map((t) => {
            const isVoid = t.status === 'void'
            return (
              <li key={t.id}>
                <button type="button" onClick={() => setOpenId(t.id)} className="w-full px-4 py-3 text-left cursor-pointer hover:bg-wall">
                  <span className="flex items-start justify-between gap-3">
                    <span className="min-w-0">
                      {/* Jangan dipotong: nomor urut unik ada di akhir nomor struk. */}
                      <span className="block break-all font-mono font-semibold">{t.receipt_no}</span>
                      <span className="block font-mono text-sm text-ink-soft">{formatDateTime(t.created_at)}</span>
                    </span>
                    <span
                      className={`shrink-0 font-mono font-semibold ${isVoid ? 'text-ink-soft line-through' : ''}`}
                    >
                      {formatRupiah(t.total)}
                    </span>
                  </span>
                  <span className="mt-1 flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
                    {/* Label dan nomor PO bersama di baris pertama; cara bayar turun kalau tidak muat. */}
                    <span className="flex items-center gap-2">
                      <SourceBadge source={t.source} />
                      {t.preorder_no && <span className="font-mono text-ink-soft">{t.preorder_no}</span>}
                    </span>
                    <span className="text-ink-soft">
                      {PAYMENT_LABEL[t.payment_method]}
                      {isAdmin ? ` · ${t.cashier_name}` : ''}
                    </span>
                    {isVoid && <VoidBadge />}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>

        {!loading && transactions.length === 0 && !error && (
          <p className="px-4 py-10 text-center text-ink-soft">Tidak ada transaksi pada periode ini.</p>
        )}
        {loading && !result && <p className="px-4 py-10 text-center text-ink-soft">Memuat riwayat…</p>}
      </div>

      <Pagination page={page} pages={pages} total={total} noun="transaksi" onPage={setPage} />

      {openId && (
        <TransactionDetailDialog
          id={openId}
          onClose={() => setOpenId(null)}
          onChanged={() => setReloadKey((k) => k + 1)}
        />
      )}
    </section>
  )
}
