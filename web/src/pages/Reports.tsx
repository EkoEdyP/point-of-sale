import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { ApiError } from '../api/client'
import { getDailyReport } from '../api/reports'
import type { DailyReport, PaymentMethod, ReportSource } from '../api/types'
import {
  addDays,
  formatLongDate,
  formatNumber,
  formatRupiah,
  formatShortDay,
  toDateInput,
} from '../lib/format'
import { PAYMENT_METHODS } from '../lib/payment'

const fieldClass = 'h-11 w-full sm:w-auto min-w-0 px-3 bg-white border-2 border-ink rounded-sm'

// Laporan toko dan PO dipisah. Tab disimpan di alamat (?jenis=po / ?jenis=semua), bawaan Toko.
const SOURCE_PARAM = 'jenis'
const SOURCE_TABS: { value: ReportSource; label: string; param: string | null }[] = [
  { value: 'store', label: 'Toko', param: null },
  { value: 'preorder', label: 'PO', param: 'po' },
  { value: 'all', label: 'Semua', param: 'semua' },
]

const DRAWER_NOTE =
  'Untuk mencocokkan uang di laci, buka tab Semua: laci berisi uang penjualan toko dan pre-order sekaligus.'

const SOURCE_TEXT: Record<
  ReportSource,
  {
    intro: string
    revenue: string
    count: string
    countUnit: string
    average: string
    empty: string
    sales: string
    note: string
  }
> = {
  store: {
    intro: 'Penjualan di kasir per hari. Transaksi yang dibatalkan tidak dihitung ke omzet.',
    revenue: 'Omzet toko',
    count: 'Transaksi',
    countUnit: 'transaksi',
    average: 'Rata-rata per transaksi',
    empty: 'Belum ada penjualan toko pada periode ini.',
    sales: 'Penjualan',
    note: `Hanya uang dari penjualan di kasir. ${DRAWER_NOTE}`,
  },
  preorder: {
    intro:
      'Pre-order yang diserahkan per hari. Omzet PO dihitung saat pesanan diserahkan, sedangkan DP masuk ke uang masuk pada hari DP diterima.',
    revenue: 'Omzet PO',
    count: 'Transaksi',
    countUnit: 'transaksi',
    average: 'Rata-rata per transaksi',
    empty: 'Belum ada pre-order yang diserahkan pada periode ini.',
    sales: 'Pelunasan',
    note: `Pelunasan adalah sisa tagihan yang dibayar saat pesanan diserahkan. DP dihitung pada hari diterima, termasuk DP pesanan yang belum diserahkan, dan baru masuk omzet PO saat pesanan diserahkan. ${DRAWER_NOTE}`,
  },
  all: {
    intro: 'Penjualan toko dan pre-order digabung. Transaksi yang dibatalkan tidak dihitung ke omzet.',
    revenue: 'Omzet toko dan PO',
    count: 'Transaksi',
    countUnit: 'transaksi',
    average: 'Rata-rata per transaksi',
    empty: 'Belum ada penjualan pada periode ini.',
    sales: 'Penjualan',
    note: 'Total masuk tunai adalah uang yang seharusnya ada di laci dari periode ini (di luar modal awal laci), dengan anggapan DP dari pre-order yang dibatalkan sudah dikembalikan. Kalau belum, laci masih berisi tambahan sebesar DP tunai yang dibatalkan di atas. DP dihitung sebagai omzet saat pre-order diserahkan, bukan saat diterima.',
  },
}

function periodLabel(from: string, to: string): string {
  return from === to ? formatLongDate(from) : `${formatLongDate(from)} sampai ${formatLongDate(to)}`
}

type RangeKey = 'today' | 'yesterday' | 'week' | 'month'

export function ReportsPage() {
  const now = new Date()
  const today = toDateInput(now)
  const yesterday = toDateInput(addDays(now, -1))
  const weekStart = toDateInput(addDays(now, -6))
  const monthStart = toDateInput(new Date(now.getFullYear(), now.getMonth(), 1))

  const [searchParams, setSearchParams] = useSearchParams()
  const source: ReportSource =
    SOURCE_TABS.find((t) => t.param !== null && t.param === searchParams.get(SOURCE_PARAM))?.value ?? 'store'
  const [from, setFrom] = useState(today)
  const [to, setTo] = useState(today)
  // Pilihan periode yang terakhir diklik. Dicatat sendiri, karena tanggal saja tidak cukup: pada tanggal 1,
  // Hari ini dan Bulan ini sama-sama 1 s.d. 1, dan dulu kedua tombolnya menyala bersamaan.
  // Kalau tanggal diubah manual, tidak ada pilihan yang menyala (null).
  const [preset, setPreset] = useState<RangeKey | null>('today')
  const [report, setReport] = useState<DailyReport | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [reloadKey, setReloadKey] = useState(0)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    getDailyReport(from, to, source)
      .then((r) => {
        if (cancelled) return
        setReport(r)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat laporan.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [from, to, source, reloadKey])

  function chooseSource(param: string | null) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (param) next.set(SOURCE_PARAM, param)
        else next.delete(SOURCE_PARAM)
        return next
      },
      { replace: true },
    )
  }

  const ranges: { key: RangeKey; label: string; name: string; from: string; to: string }[] = [
    { key: 'today', label: 'Hari ini', name: 'hari ini', from: today, to: today },
    { key: 'yesterday', label: 'Kemarin', name: 'kemarin', from: yesterday, to: yesterday },
    { key: 'week', label: '7 hari terakhir', name: '7 hari terakhir', from: weekStart, to: today },
    { key: 'month', label: 'Bulan ini', name: 'bulan ini', from: monthStart, to: today },
  ]

  const totals = report?.totals
  // Label mengikuti laporan yang sedang tampil (bukan tab yang baru diklik), supaya selama memuat
  // angka lama tidak tampil dengan judul jenis yang baru.
  const text = SOURCE_TEXT[report?.source ?? source]
  // Nama periode (hari ini, bulan ini, ...) hanya kalau tanggal laporan yang tampil cocok dengan pilihan itu.
  const shownRange = report
    ? ranges.find((r) => r.key === preset && r.from === report.from && r.to === report.to)
    : undefined
  const withDP = report?.source !== 'store' // laporan toko tidak punya DP
  const maxRevenue = report ? Math.max(...report.days.map((d) => d.revenue), 0) : 0
  // Panjang batang omzet per hari, relatif terhadap hari dengan omzet terbesar.
  const barWidth = (revenue: number) =>
    maxRevenue > 0 ? Math.max((revenue / maxRevenue) * 100, revenue > 0 ? 2 : 0) : 0

  return (
    <section>
      <h1 className="text-2xl font-bold">Laporan</h1>

      <div role="group" aria-label="Jenis laporan" className="mt-4 grid grid-cols-3 gap-2 sm:flex">
        {SOURCE_TABS.map((t) => (
          <button
            key={t.value}
            type="button"
            aria-pressed={source === t.value}
            onClick={() => chooseSource(t.param)}
            className={`min-h-11 px-2 sm:px-6 leading-tight border-2 border-ink rounded-sm font-semibold cursor-pointer ${
              source === t.value ? 'bg-ink text-paper' : 'bg-paper'
            }`}
          >
            {t.label}
          </button>
        ))}
      </div>
      <p className="mt-2 max-w-prose text-ink-soft">{SOURCE_TEXT[source].intro}</p>

      <div className="mt-4 grid gap-3 xs:grid-cols-2 sm:flex sm:flex-wrap sm:items-end sm:gap-x-4 sm:gap-y-3">
        <div className="xs:col-span-2 grid grid-cols-2 gap-2 sm:flex sm:flex-wrap">
          {ranges.map((r) => {
            const active = preset === r.key && from === r.from && to === r.to
            return (
              <button
                key={r.key}
                type="button"
                aria-pressed={active}
                onClick={() => {
                  setPreset(r.key)
                  setFrom(r.from)
                  setTo(r.to)
                }}
                className={`h-11 px-2 sm:px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer ${
                  active ? 'bg-ink text-paper' : 'bg-paper'
                }`}
              >
                {r.label}
              </button>
            )
          })}
        </div>
        <div className="min-w-0">
          <label htmlFor="r-from" className="block mb-1 text-sm font-semibold">
            Dari
          </label>
          <input
            id="r-from"
            type="date"
            value={from}
            onChange={(e) => {
              setPreset(null)
              setFrom(e.target.value)
            }}
            className={fieldClass}
          />
        </div>
        <div className="min-w-0">
          <label htmlFor="r-to" className="block mb-1 text-sm font-semibold">
            Sampai
          </label>
          <input
            id="r-to"
            type="date"
            value={to}
            onChange={(e) => {
              setPreset(null)
              setTo(e.target.value)
            }}
            className={fieldClass}
          />
        </div>
        <button
          type="button"
          onClick={() => setReloadKey((k) => k + 1)}
          className="xs:col-span-2 h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer"
        >
          Muat ulang
        </button>
      </div>

      {error && (
        <p role="alert" className="mt-4 font-semibold text-merah">
          {error}
        </p>
      )}

      {!report && loading && <p className="mt-8 text-ink-soft">Memuat laporan…</p>}

      {report && totals && (
        <div className={loading ? 'opacity-60' : ''} aria-busy={loading}>
          <div className="mt-6 bg-paper border border-line p-4 sm:p-5">
            <p className="text-sm font-semibold text-ink-soft">
              {text.revenue}
              {shownRange ? ` ${shownRange.name}` : ''}, {periodLabel(report.from, report.to)}
            </p>
            <p className="mt-1 font-mono text-3xl xs:text-4xl sm:text-5xl font-semibold break-words">
              {formatRupiah(totals.revenue)}
            </p>
            {shownRange?.key === 'month' && report.from === report.to && (
              <p className="mt-1 text-sm text-ink-soft">
                Hari ini tanggal 1, jadi angka bulan ini masih sama dengan hari ini.
              </p>
            )}

            <dl className="mt-5 pt-4 border-t-2 border-dashed border-ink grid xs:grid-cols-2 sm:grid-cols-3 gap-x-4 sm:gap-x-6 gap-y-4">
              <div className="min-w-0">
                <dt className="text-sm text-ink-soft">{text.count}</dt>
                <dd className="font-mono text-lg sm:text-xl font-semibold">{formatNumber(totals.transaction_count)}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-sm text-ink-soft">Barang terjual</dt>
                <dd className="font-mono text-lg sm:text-xl font-semibold">{formatNumber(totals.items_sold)}</dd>
              </div>
              <div className="min-w-0">
                <dt className="text-sm text-ink-soft">{text.average}</dt>
                <dd className="font-mono text-lg sm:text-xl font-semibold break-words">
                  {formatRupiah(totals.average_transaction)}
                </dd>
              </div>
            </dl>

            {totals.transaction_count === 0 && <p className="mt-4 text-ink-soft">{text.empty}</p>}
          </div>

          <div className="mt-4 grid gap-4 lg:grid-cols-2">
            <div className="min-w-0 bg-paper border border-line p-4 sm:p-5">
              <h2 className="text-lg font-bold">Uang masuk per metode bayar</h2>

              {/* HP: satu baris per metode, karena empat kolom angka tidak muat. */}
              <ul className="sm:hidden mt-3 border-t-2 border-ink divide-y divide-line">
                {PAYMENT_METHODS.map((m) => {
                  const sales = totals.by_payment_method[m.value as PaymentMethod]
                  const dp = totals.dp_by_payment_method[m.value as PaymentMethod]
                  return (
                    <li key={m.value} className="py-2">
                      <div className="flex items-baseline justify-between gap-3">
                        <span className="font-semibold">{m.label}</span>
                        <span className="font-mono font-semibold">{formatRupiah(sales + dp)}</span>
                      </div>
                      {withDP && (
                        <div className="flex flex-wrap gap-x-3 font-mono text-sm text-ink-soft">
                          <span>
                            {text.sales} {formatRupiah(sales)}
                          </span>
                          <span>DP {formatRupiah(dp)}</span>
                        </div>
                      )}
                    </li>
                  )
                })}
              </ul>

              <table className="hidden sm:table mt-3 w-full text-left">
                <thead>
                  <tr className="border-b-2 border-ink text-sm text-ink-soft">
                    <th className="py-2 font-semibold">Metode</th>
                    <th className="py-2 font-semibold text-right">{text.sales}</th>
                    {withDP && (
                      <>
                        <th className="py-2 font-semibold text-right">DP</th>
                        <th className="py-2 font-semibold text-right">Total masuk</th>
                      </>
                    )}
                  </tr>
                </thead>
                <tbody>
                  {PAYMENT_METHODS.map((m) => {
                    const sales = totals.by_payment_method[m.value as PaymentMethod]
                    const dp = totals.dp_by_payment_method[m.value as PaymentMethod]
                    return (
                      <tr key={m.value} className="border-b border-line">
                        <td className="py-2 font-semibold">{m.label}</td>
                        <td className={`py-2 font-mono text-right ${withDP ? '' : 'font-semibold'}`}>
                          {formatRupiah(sales)}
                        </td>
                        {withDP && (
                          <>
                            <td className="py-2 font-mono text-right">{formatRupiah(dp)}</td>
                            <td className="py-2 font-mono text-right font-semibold">{formatRupiah(sales + dp)}</td>
                          </>
                        )}
                      </tr>
                    )
                  })}
                </tbody>
              </table>
              {totals.dp_cancelled > 0 && (
                <div className="mt-4 p-3 border-2 border-merah">
                  <p className="font-semibold text-merah">
                    DP pre-order dibatalkan: {formatRupiah(totals.dp_cancelled)}
                  </p>
                  <p className="mt-1 font-mono text-sm">
                    {PAYMENT_METHODS.filter((m) => totals.dp_cancelled_by_payment_method[m.value as PaymentMethod] > 0)
                      .map((m) => `${m.label} ${formatRupiah(totals.dp_cancelled_by_payment_method[m.value as PaymentMethod])}`)
                      .join(', ')}
                  </p>
                  <p className="mt-1 text-sm text-ink-soft">
                    Uang ini harus dikembalikan ke pelanggan, jadi tidak dihitung di tabel di atas.
                  </p>
                </div>
              )}
              <p className="mt-4 text-sm text-ink-soft">{text.note}</p>
            </div>

            <div className="min-w-0 bg-paper border border-line p-4 sm:p-5">
              <h2 className="text-lg font-bold">Pembatalan</h2>
              {totals.void_count === 0 ? (
                <p className="mt-3 text-ink-soft">Tidak ada transaksi yang dibatalkan.</p>
              ) : (
                <>
                  <p className="mt-3 font-mono text-xl font-semibold text-merah">
                    {formatNumber(totals.void_count)} transaksi, {formatRupiah(totals.void_total)}
                  </p>
                  <p className="mt-1 text-sm text-ink-soft">
                    Tidak dihitung ke omzet. Pembatalan dikelompokkan menurut tanggal transaksi dibuat, bukan tanggal
                    dibatalkan. Lihat alasannya di menu Riwayat.
                  </p>
                </>
              )}
            </div>
          </div>

          {report.days.length > 1 && (
            <div className="mt-4 overflow-x-auto bg-paper border border-line">
              {/* HP: satu kartu per hari, batang omzet di bawah tanggal. */}
              <ul className="md:hidden divide-y divide-line">
                {report.days.map((d) => (
                  <li key={d.date} className="px-4 py-3">
                    <div className="flex items-baseline justify-between gap-3">
                      <span className="font-semibold">{formatShortDay(d.date)}</span>
                      <span className="font-mono font-semibold">{formatRupiah(d.revenue)}</span>
                    </div>
                    <div className="mt-1.5 h-2 bg-wall" aria-hidden="true">
                      <div className="h-2 bg-kunyit border-r-2 border-ink" style={{ width: `${barWidth(d.revenue)}%` }} />
                    </div>
                    <div className="mt-1.5 flex flex-wrap gap-x-3 text-sm text-ink-soft">
                      <span>
                        {formatNumber(d.transaction_count)} {text.countUnit}
                      </span>
                      <span>{formatNumber(d.items_sold)} barang</span>
                    </div>
                  </li>
                ))}
              </ul>

              <table className="hidden md:table w-full min-w-[40rem] text-left">
                <thead>
                  <tr className="border-b-2 border-ink text-sm text-ink-soft">
                    <th className="px-4 py-3 font-semibold">Tanggal</th>
                    <th className="px-4 py-3 font-semibold text-right">{text.count}</th>
                    <th className="px-4 py-3 font-semibold text-right">Barang</th>
                    <th className="px-4 py-3 font-semibold text-right">Omzet</th>
                    <th className="px-4 py-3 font-semibold w-1/3">
                      <span className="sr-only">Perbandingan omzet</span>
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {report.days.map((d) => (
                    <tr key={d.date} className="border-b border-line">
                      <td className="px-4 py-3 whitespace-nowrap">{formatShortDay(d.date)}</td>
                      <td className="px-4 py-3 font-mono text-right">{formatNumber(d.transaction_count)}</td>
                      <td className="px-4 py-3 font-mono text-right">{formatNumber(d.items_sold)}</td>
                      <td className="px-4 py-3 font-mono text-right">{formatRupiah(d.revenue)}</td>
                      <td className="px-4 py-3" aria-hidden="true">
                        <div className="h-3 bg-wall">
                          <div className="h-3 bg-kunyit border-r-2 border-ink" style={{ width: `${barWidth(d.revenue)}%` }} />
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </section>
  )
}
