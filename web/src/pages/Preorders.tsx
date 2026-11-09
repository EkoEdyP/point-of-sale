import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import { listPreorders } from '../api/preorders'
import type { Preorder, PreorderList, PreorderStatus } from '../api/types'
import { Pagination } from '../components/Pagination'
import { PreorderDetailDialog } from '../components/PreorderDetailDialog'
import { PreorderFormDialog } from '../components/PreorderFormDialog'
import { ProductionView } from '../components/ProductionView'
import { addDays, formatLongDate, formatRupiah, formatShortDay, toDateInput } from '../lib/format'
import { FULFILLMENT_LABEL, PREORDER_WARN_DAYS, STATUS_LABEL, TONE_CLASS, dueInfo } from '../lib/preorder'

const PAGE_SIZE = 20

type StatusFilter = 'open' | 'completed' | 'cancelled'

const STATUS_OPTIONS: { value: StatusFilter; label: string }[] = [
  { value: 'open', label: 'Berjalan' },
  { value: 'completed', label: 'Selesai' },
  { value: 'cancelled', label: 'Dibatalkan' },
]

const EMPTY_MESSAGE: Record<StatusFilter, string> = {
  open: 'Tidak ada pre-order yang berjalan. Buat yang baru dengan tombol Buat pre-order.',
  completed: 'Belum ada pre-order yang selesai.',
  cancelled: 'Tidak ada pre-order yang dibatalkan.',
}

const STATUS_NOUN: Record<StatusFilter, string> = {
  open: 'yang berjalan',
  completed: 'yang selesai',
  cancelled: 'yang dibatalkan',
}

const fieldClass = 'h-11 w-full sm:w-auto min-w-0 px-3 bg-white border-2 border-ink rounded-sm'

// Filter hari H disimpan di alamat halaman (?tanggal=YYYY-MM-DD), jadi tetap terpasang saat dimuat ulang.
const DATE_PARAM = 'tanggal'
const DATE_PATTERN = /^\d{4}-\d{2}-\d{2}$/

// Tampilan juga disimpan di alamat (?lihat=produksi). Filter tanggal berlaku untuk keduanya.
const VIEW_PARAM = 'lihat'
type View = 'orders' | 'production'
const VIEWS: { value: View; label: string }[] = [
  { value: 'orders', label: 'Pesanan' },
  { value: 'production', label: 'Kebutuhan produksi' },
]

const isOpen = (p: Preorder) => p.status === 'ordered' || p.status === 'ready'
const dueWhen = (p: Preorder) => `${formatShortDay(p.due_date)}${p.due_time ? `, ${p.due_time}` : ''}`

function StatusLabel({ status }: { status: PreorderStatus }) {
  return status === 'cancelled' ? (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-merah text-merah rounded-sm">
      {STATUS_LABEL[status]}
    </span>
  ) : (
    <span className="text-sm">{STATUS_LABEL[status]}</span>
  )
}

export function PreordersPage() {
  const [status, setStatus] = useState<StatusFilter>('open')
  const [searchParams, setSearchParams] = useSearchParams()
  const rawDate = searchParams.get(DATE_PARAM) ?? ''
  const dueDate = DATE_PATTERN.test(rawDate) ? rawDate : ''
  const view: View = searchParams.get(VIEW_PARAM) === 'produksi' ? 'production' : 'orders'
  const [page, setPage] = useState(1)
  const [result, setResult] = useState<PreorderList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  const [creating, setCreating] = useState(false)
  const [openId, setOpenId] = useState<string | null>(null)
  // Pre-order yang baru dibuat: detailnya dibuka dengan nota siap dicetak.
  const [notaFor, setNotaFor] = useState<string | null>(null)

  useEffect(() => {
    if (view !== 'orders') return
    let cancelled = false
    setLoading(true)
    listPreorders({ status, dueDate, page, limit: PAGE_SIZE })
      .then((r) => {
        if (cancelled) return
        if (r.data.length === 0 && page > 1) setPage(page - 1)
        else setResult(r)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat pre-order.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [view, status, dueDate, page, reloadKey])

  // Ubah sebagian parameter alamat tanpa menghapus yang lain (tanggal dan tampilan).
  function updateParams(changes: Record<string, string | null>) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        for (const [key, value] of Object.entries(changes)) {
          if (value) next.set(key, value)
          else next.delete(key)
        }
        return next
      },
      { replace: true },
    )
  }

  function chooseDate(value: string) {
    updateParams({ [DATE_PARAM]: value })
    setPage(1)
  }

  function chooseView(value: View) {
    updateParams({ [VIEW_PARAM]: value === 'production' ? 'produksi' : null })
  }

  // Dari kebutuhan produksi: buka daftar pesanan untuk hari H itu.
  function showOrdersFor(date: string) {
    updateParams({ [VIEW_PARAM]: null, [DATE_PARAM]: date })
    setStatus('open')
    setPage(1)
  }

  const now = new Date()
  const dateChoices = [
    { label: 'Semua tanggal', short: 'Semua', value: '' },
    { label: 'Hari ini', value: toDateInput(now) },
    { label: 'Besok', value: toDateInput(addDays(now, 1)) },
    { label: 'Lusa', value: toDateInput(addDays(now, 2)) },
  ]

  const reload = () => setReloadKey((k) => k + 1)
  const preorders = result?.data ?? []
  const total = result?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <section>
      <div className="flex flex-wrap items-center gap-3">
        <h1 className="text-2xl font-bold mr-auto">Pre-order</h1>
        <button
          type="button"
          onClick={() => setCreating(true)}
          className="h-11 px-5 bg-kunyit border-2 border-ink rounded-sm font-bold cursor-pointer"
        >
          Buat pre-order
        </button>
      </div>
      <p className="mt-1 text-ink-soft">
        Pesanan untuk hari tertentu. Yang paling dekat hari H ada di atas, dan yang mendekati jatuh tempo diberi tanda.
      </p>

      <div role="group" aria-label="Tampilan" className="mt-4 grid grid-cols-2 gap-2 sm:flex">
        {VIEWS.map((v) => (
          <button
            key={v.value}
            type="button"
            aria-pressed={view === v.value}
            onClick={() => chooseView(v.value)}
            className={`min-h-11 px-2 sm:px-4 leading-tight border-2 border-ink rounded-sm font-semibold cursor-pointer ${
              view === v.value ? 'bg-ink text-paper' : 'bg-paper'
            }`}
          >
            {v.label}
          </button>
        ))}
      </div>

      <div className="mt-4 grid gap-3 xs:grid-cols-2 sm:flex sm:flex-wrap sm:items-end sm:gap-x-4 sm:gap-y-3">
        <div role="group" aria-label="Filter tanggal" className="xs:col-span-2 grid grid-cols-4 gap-2 sm:flex sm:flex-wrap">
          {dateChoices.map((c) => {
            const active = dueDate === c.value
            return (
              <button
                key={c.label}
                type="button"
                aria-pressed={active}
                onClick={() => chooseDate(c.value)}
                className={`h-11 px-1 sm:px-3 text-sm sm:text-base border-2 border-ink rounded-sm font-semibold cursor-pointer ${
                  active ? 'bg-ink text-paper' : 'bg-paper'
                }`}
              >
                {c.short ? (
                  <>
                    <span className="sm:hidden">{c.short}</span>
                    <span className="hidden sm:inline">{c.label}</span>
                  </>
                ) : (
                  c.label
                )}
              </button>
            )
          })}
        </div>
        <div className="min-w-0">
          <label htmlFor="po-due-date" className="block mb-1 text-sm font-semibold">
            Tanggal
          </label>
          <input
            id="po-due-date"
            type="date"
            value={dueDate}
            onChange={(e) => chooseDate(e.target.value)}
            className={fieldClass}
          />
        </div>
        {view === 'orders' && (
          <div className="min-w-0 sm:ml-auto">
            <label htmlFor="po-status" className="block mb-1 text-sm font-semibold">
              Status
            </label>
            <select
              id="po-status"
              value={status}
              onChange={(e) => {
                setStatus(e.target.value as StatusFilter)
                setPage(1)
              }}
              className={fieldClass}
            >
              {STATUS_OPTIONS.map((s) => (
                <option key={s.value} value={s.value}>
                  {s.label}
                </option>
              ))}
            </select>
          </div>
        )}
      </div>

      {view === 'production' ? (
        <ProductionView dueDate={dueDate} reloadKey={reloadKey} onShowOrders={showOrdersFor} />
      ) : (
        <>
          {error && (
            <p role="alert" className="mt-4 font-semibold text-merah">
              {error}
            </p>
          )}

          <div
            className={`relative mt-4 overflow-x-auto bg-paper border border-line ${loading && result ? 'opacity-60' : ''}`}
            aria-busy={loading}
          >
            {/* Layar lebar: tabel. */}
            <table className="hidden lg:table w-full min-w-[46rem] text-left">
              <thead>
                <tr className="border-b-2 border-ink text-sm text-ink-soft">
                  <th className="px-4 py-3 font-semibold">Hari H</th>
                  <th className="px-4 py-3 font-semibold">Pelanggan</th>
                  <th className="px-4 py-3 font-semibold">Cara serah</th>
                  <th className="px-4 py-3 font-semibold text-right">Total</th>
                  <th className="px-4 py-3 font-semibold text-right">Sisa</th>
                  <th className="px-4 py-3 font-semibold">Status</th>
                  <th className="px-4 py-3 font-semibold">
                    <span className="sr-only">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {preorders.map((p) => {
                  const open = isOpen(p)
                  const due = dueInfo(p.due_date, PREORDER_WARN_DAYS)
                  const urgent = open && (due.tone === 'late' || due.tone === 'today')
                  return (
                    <tr key={p.id} className={`border-b border-line hover:bg-wall ${urgent ? 'bg-merah/10' : ''}`}>
                      <td className="px-4 py-3 whitespace-nowrap">
                        {open && due.tone !== 'normal' ? (
                          <>
                            <span className={`px-2 py-0.5 rounded-sm text-sm font-semibold ${TONE_CLASS[due.tone]}`}>
                              {due.label}
                            </span>
                            <div className="mt-1 text-sm text-ink-soft">{dueWhen(p)}</div>
                          </>
                        ) : (
                          <>
                            <div className="font-semibold">{formatShortDay(p.due_date)}</div>
                            <div className="text-sm text-ink-soft">{p.due_time ?? formatLongDate(p.due_date)}</div>
                          </>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <div className="font-semibold">{p.customer_name}</div>
                        <div className="font-mono text-sm text-ink-soft">{p.order_no}</div>
                      </td>
                      <td className="px-4 py-3">{FULFILLMENT_LABEL[p.fulfillment]}</td>
                      <td className="px-4 py-3 font-mono text-right">{formatRupiah(p.total)}</td>
                      <td className="px-4 py-3 font-mono text-right">
                        {open ? (
                          formatRupiah(p.remaining)
                        ) : p.status === 'cancelled' && p.down_payment > 0 ? (
                          <span className="text-sm font-semibold text-merah">DP {formatRupiah(p.down_payment)}</span>
                        ) : (
                          '-'
                        )}
                      </td>
                      <td className="px-4 py-3">
                        <StatusLabel status={p.status} />
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          type="button"
                          onClick={() => setOpenId(p.id)}
                          aria-label={`Detail pre-order ${p.order_no}`}
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

            {/* HP dan tablet: satu kartu per pre-order, ketuk untuk membuka detail. */}
            <ul className="lg:hidden divide-y divide-line">
              {preorders.map((p) => {
                const open = isOpen(p)
                const due = dueInfo(p.due_date, PREORDER_WARN_DAYS)
                const urgent = open && (due.tone === 'late' || due.tone === 'today')
                return (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => setOpenId(p.id)}
                      className={`w-full px-4 py-3 text-left cursor-pointer hover:bg-wall ${urgent ? 'bg-merah/10' : ''}`}
                    >
                      <span className="flex items-start justify-between gap-3">
                        <span className="min-w-0">
                          {/* Nama tidak dipotong; status turun ke baris berikutnya kalau tidak muat. */}
                          <span className="flex flex-wrap items-center gap-x-2">
                            <span className="min-w-0 font-semibold break-words">{p.customer_name}</span>
                            <span className="text-ink-soft">
                              <StatusLabel status={p.status} />
                            </span>
                          </span>
                          <span className="block font-mono text-sm text-ink-soft">{p.order_no}</span>
                        </span>
                        <span className="shrink-0 text-right">
                          <span className="block font-mono font-semibold">{formatRupiah(p.total)}</span>
                          {open ? (
                            <span className="block font-mono text-sm text-ink-soft">Sisa {formatRupiah(p.remaining)}</span>
                          ) : (
                            p.status === 'cancelled' &&
                            p.down_payment > 0 && (
                              <span className="block text-sm font-semibold text-merah">DP {formatRupiah(p.down_payment)}</span>
                            )
                          )}
                        </span>
                      </span>
                      <span className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                        {open && due.tone !== 'normal' && (
                          <span className={`px-2 py-0.5 rounded-sm font-semibold ${TONE_CLASS[due.tone]}`}>{due.label}</span>
                        )}
                        <span className="text-ink-soft">
                          {dueWhen(p)} · {FULFILLMENT_LABEL[p.fulfillment]}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>

            {!loading && preorders.length === 0 && !error && (
              <p className="px-4 py-10 text-center text-ink-soft">
                {dueDate
                  ? `Tidak ada pre-order ${STATUS_NOUN[status]} dengan hari H ${formatLongDate(dueDate)}.`
                  : EMPTY_MESSAGE[status]}
              </p>
            )}
            {loading && !result && <p className="px-4 py-10 text-center text-ink-soft">Memuat pre-order…</p>}
          </div>

          <Pagination page={page} pages={pages} total={total} noun="pre-order" onPage={setPage} />
        </>
      )}

      {creating && (
        <PreorderFormDialog
          onClose={() => setCreating(false)}
          onCreated={(created) => {
            setCreating(false)
            setOpenId(created.id)
            setNotaFor(created.id)
            reload()
          }}
        />
      )}

      {openId && (
        <PreorderDetailDialog
          id={openId}
          openNota={openId === notaFor}
          onClose={() => {
            setOpenId(null)
            setNotaFor(null)
          }}
          onChanged={reload}
        />
      )}
    </section>
  )
}
