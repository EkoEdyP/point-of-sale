import { useEffect, useState } from 'react'
import { ApiError } from '../api/client'
import { listDeliveries } from '../api/deliveries'
import type { Delivery, DeliveryList, DeliveryStatus } from '../api/types'
import { DeliveryDetailDialog } from '../components/DeliveryDetailDialog'
import { Pagination } from '../components/Pagination'
import { DELIVERY_STATUS_LABEL, isScheduled } from '../lib/delivery'
import { formatRupiah, formatShortDay } from '../lib/format'
import { TONE_CLASS, dueInfo } from '../lib/preorder'

const PAGE_SIZE = 20

type Tab = 'active' | 'done'

const TABS: { value: Tab; label: string }[] = [
  { value: 'active', label: 'Perlu ditangani' },
  { value: 'done', label: 'Selesai' },
]

const scheduleWhen = (d: Delivery) =>
  `${formatShortDay(d.scheduled_date)}${d.scheduled_time ? `, ${d.scheduled_time}` : ''}`

function StatusLabel({ status }: { status: DeliveryStatus }) {
  return status === 'failed' || status === 'cancelled' ? (
    <span className="px-1.5 py-0.5 text-xs font-semibold border border-merah text-merah rounded-sm">
      {DELIVERY_STATUS_LABEL[status]}
    </span>
  ) : (
    <span className="text-sm">{DELIVERY_STATUS_LABEL[status]}</span>
  )
}

export function DeliveriesPage() {
  const [tab, setTab] = useState<Tab>('active')
  const [page, setPage] = useState(1)
  const [result, setResult] = useState<DeliveryList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)
  const [openId, setOpenId] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listDeliveries({ status: tab, page, limit: PAGE_SIZE })
      .then((r) => {
        if (cancelled) return
        if (r.data.length === 0 && page > 1) setPage(page - 1)
        else setResult(r)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat pengiriman.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [tab, page, reloadKey])

  const reload = () => setReloadKey((k) => k + 1)
  const deliveries = result?.data ?? []
  const total = result?.total ?? 0
  const pages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  return (
    <section>
      <h1 className="text-2xl font-bold">Pengiriman</h1>
      <p className="mt-1 text-ink-soft">
        Antaran ke pelanggan. Pre-order dengan cara serah Diantar masuk ke sini otomatis.
      </p>

      <div className="mt-4 grid grid-cols-2 gap-2 sm:flex">
        {TABS.map((t) => (
          <button
            key={t.value}
            type="button"
            aria-pressed={tab === t.value}
            onClick={() => {
              setTab(t.value)
              setPage(1)
            }}
            className={`min-h-11 px-2 sm:px-4 leading-tight border-2 border-ink rounded-sm font-semibold cursor-pointer ${
              tab === t.value ? 'bg-ink text-paper' : 'bg-paper'
            }`}
          >
            {t.label}
          </button>
        ))}
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
        {/* Layar lebar: tabel. */}
        <table className="hidden lg:table w-full min-w-[48rem] text-left">
          <thead>
            <tr className="border-b-2 border-ink text-sm text-ink-soft">
              <th className="px-4 py-3 font-semibold">Jadwal</th>
              <th className="px-4 py-3 font-semibold">Pemesan</th>
              <th className="px-4 py-3 font-semibold">Alamat</th>
              <th className="px-4 py-3 font-semibold">Kurir</th>
              <th className="px-4 py-3 font-semibold text-right">Ongkir</th>
              <th className="px-4 py-3 font-semibold">Status</th>
              <th className="px-4 py-3 font-semibold">
                <span className="sr-only">Aksi</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {deliveries.map((d) => {
              const scheduled = isScheduled(d.status)
              const due = dueInfo(d.scheduled_date)
              const urgent = scheduled && (due.tone === 'late' || due.tone === 'today')
              const attention = urgent || d.status === 'failed'
              return (
                <tr key={d.id} className={`border-b border-line hover:bg-wall ${attention ? 'bg-merah/10' : ''}`}>
                  <td className="px-4 py-3 whitespace-nowrap">
                    {scheduled && due.tone !== 'normal' ? (
                      <>
                        <span className={`px-2 py-0.5 rounded-sm text-sm font-semibold ${TONE_CLASS[due.tone]}`}>
                          {due.label}
                        </span>
                        <div className="mt-1 text-sm text-ink-soft">{scheduleWhen(d)}</div>
                      </>
                    ) : (
                      <>
                        <div className="font-semibold">{formatShortDay(d.scheduled_date)}</div>
                        {d.scheduled_time && <div className="text-sm text-ink-soft">{d.scheduled_time}</div>}
                      </>
                    )}
                  </td>
                  <td className="px-4 py-3">
                    <div className="font-semibold">{d.customer_name}</div>
                    <div className="font-mono text-sm text-ink-soft">{d.delivery_no}</div>
                    {d.received_by && <div className="text-sm text-ink-soft">Diterima {d.received_by}</div>}
                  </td>
                  <td className="px-4 py-3 max-w-64">
                    <div className="line-clamp-2">{d.address}</div>
                  </td>
                  <td className="px-4 py-3">{d.courier ?? <span className="text-ink-soft">-</span>}</td>
                  <td className="px-4 py-3 font-mono text-right">{d.fee > 0 ? formatRupiah(d.fee) : '-'}</td>
                  <td className="px-4 py-3">
                    <StatusLabel status={d.status} />
                  </td>
                  <td className="px-4 py-3 text-right">
                    <button
                      type="button"
                      onClick={() => setOpenId(d.id)}
                      aria-label={`Detail pengiriman ${d.delivery_no}`}
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

        {/* HP dan tablet: satu kartu per pengiriman, ketuk untuk membuka detail. */}
        <ul className="lg:hidden divide-y divide-line">
          {deliveries.map((d) => {
            const scheduled = isScheduled(d.status)
            const due = dueInfo(d.scheduled_date)
            const urgent = scheduled && (due.tone === 'late' || due.tone === 'today')
            const attention = urgent || d.status === 'failed'
            return (
              <li key={d.id}>
                <button
                  type="button"
                  onClick={() => setOpenId(d.id)}
                  className={`w-full px-4 py-3 text-left cursor-pointer hover:bg-wall ${attention ? 'bg-merah/10' : ''}`}
                >
                  <span className="flex items-start justify-between gap-3">
                    <span className="min-w-0">
                      {/* Nama tidak dipotong; status turun ke baris berikutnya kalau tidak muat. */}
                      <span className="flex flex-wrap items-center gap-x-2">
                        <span className="min-w-0 font-semibold break-words">{d.customer_name}</span>
                        <span className="text-ink-soft">
                          <StatusLabel status={d.status} />
                        </span>
                      </span>
                      <span className="block font-mono text-sm text-ink-soft">{d.delivery_no}</span>
                      {d.received_by && (
                        <span className="block text-sm text-ink-soft break-words">Diterima {d.received_by}</span>
                      )}
                    </span>
                    {d.fee > 0 && (
                      <span className="shrink-0 text-right">
                        <span className="block font-mono font-semibold">{formatRupiah(d.fee)}</span>
                        <span className="block text-sm text-ink-soft">ongkir</span>
                      </span>
                    )}
                  </span>
                  <span className="mt-2 flex flex-wrap items-center gap-x-2 gap-y-1 text-sm">
                    {scheduled && due.tone !== 'normal' && (
                      <span className={`px-2 py-0.5 rounded-sm font-semibold ${TONE_CLASS[due.tone]}`}>{due.label}</span>
                    )}
                    <span className="text-ink-soft">
                      {scheduleWhen(d)}
                      {d.courier ? ` · ${d.courier}` : ''}
                    </span>
                  </span>
                  <span className="mt-1 block text-sm line-clamp-2">{d.address}</span>
                </button>
              </li>
            )
          })}
        </ul>

        {!loading && deliveries.length === 0 && !error && (
          <p className="px-4 py-10 text-center text-ink-soft">
            {tab === 'active' ? 'Tidak ada pengiriman yang perlu ditangani.' : 'Belum ada pengiriman yang selesai.'}
          </p>
        )}
        {loading && !result && <p className="px-4 py-10 text-center text-ink-soft">Memuat pengiriman…</p>}
      </div>

      <Pagination page={page} pages={pages} total={total} noun="pengiriman" onPage={setPage} />

      {openId && <DeliveryDetailDialog id={openId} onClose={() => setOpenId(null)} onChanged={reload} />}
    </section>
  )
}
