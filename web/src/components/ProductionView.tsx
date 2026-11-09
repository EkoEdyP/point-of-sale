import { useEffect, useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { getPreorderProduction, setPreorderDailyLimit } from '../api/preorders'
import { addProduction } from '../api/products'
import type { PreorderProduction, ProductionItem, ProductionProduct } from '../api/types'
import { useAuth } from '../auth/AuthContext'
import { QUOTA_SETTING_ID } from '../lib/audit'
import { formatLongDate, formatNumber, formatWeekdayDate } from '../lib/format'
import { PREORDER_WARN_DAYS, TONE_CLASS, dueInfo } from '../lib/preorder'
import { withRequestKey } from '../lib/requestKey'
import { AuditHistoryDialog } from './AuditHistoryDialog'
import { Modal } from './Modal'

const MAX_PRODUCTION = 100_000
const MAX_DAILY_LIMIT = 1_000_000

// Kebutuhan produksi: per hari H, berapa yang dibutuhkan pre-order berjalan, berapa yang sudah siap
// (dari stok siap produk PO), dan berapa yang masih kurang. Stok siap dipakai untuk hari H paling awal dulu.
export function ProductionView({
  dueDate,
  reloadKey,
  onShowOrders,
}: {
  dueDate: string // kosong = semua hari H
  reloadKey: number
  onShowOrders: (dueDate: string) => void
}) {
  const { state } = useAuth()
  const isAdmin = state.status === 'authenticated' && state.user.role === 'admin'
  const [data, setData] = useState<PreorderProduction | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [savedKey, setSavedKey] = useState(0)
  const [recording, setRecording] = useState<ProductionProduct | null>(null)
  const [editingQuota, setEditingQuota] = useState(false)

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    getPreorderProduction(dueDate)
      .then((r) => {
        if (cancelled) return
        setData(r)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat kebutuhan produksi.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [dueDate, reloadKey, savedKey])

  const days = data?.days ?? []
  const products = new Map((data?.products ?? []).map((p) => [p.product_id, p]))
  const limit = data?.daily_limit ?? 0

  return (
    <div>
      <p className="mt-4 max-w-prose text-ink-soft">
        Dihitung dari pre-order yang belum diserahkan. Stok siap dipakai untuk hari H paling awal lebih dulu.
        {isAdmin ? ' Setiap ada barang yang selesai dibuat, catat lewat tombol Catat produksi.' : ''}
      </p>

      {data && (
        <div className="mt-4 px-4 py-3 bg-paper border border-line flex flex-wrap items-center gap-x-5 gap-y-2">
          <div>
            <p className="text-sm text-ink-soft">Kuota pre-order per hari</p>
            <p className="font-mono text-xl font-semibold">{formatNumber(limit)} pcs</p>
          </div>
          <p className="flex-1 min-w-52 max-w-prose text-sm text-ink-soft">
            Total pcs semua produk PO untuk satu hari H, termasuk pesanan yang sudah diserahkan. Pesanan baru yang
            melebihi kuota ditolak.
          </p>
          {isAdmin && (
            <button
              type="button"
              onClick={() => setEditingQuota(true)}
              className="shrink-0 h-11 pointer-fine:h-9 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer whitespace-nowrap"
            >
              Ubah kuota
            </button>
          )}
        </div>
      )}

      {error && (
        <p role="alert" className="mt-4 font-semibold text-merah">
          {error}
        </p>
      )}

      <div className={`mt-4 space-y-4 ${loading && data ? 'opacity-60' : ''}`} aria-busy={loading}>
        {days.map((day) => {
          const due = dueInfo(day.due_date, PREORDER_WARN_DAYS)
          const headingId = `produksi-${day.due_date}`
          const quotaFull = day.quota_used >= limit
          return (
            <section key={day.due_date} aria-labelledby={headingId} className="bg-paper border border-line">
              <div className="px-4 py-3 flex flex-wrap items-center gap-x-3 gap-y-2 border-b-2 border-ink">
                <h2 id={headingId} className="text-lg font-bold">
                  {formatWeekdayDate(day.due_date)}
                </h2>
                {due.tone !== 'normal' && (
                  <span className={`px-2 py-0.5 rounded-sm text-sm font-semibold ${TONE_CLASS[due.tone]}`}>
                    {due.label}
                  </span>
                )}
                <button
                  type="button"
                  onClick={() => onShowOrders(day.due_date)}
                  className="ml-auto py-2 -my-2 text-sm font-semibold underline underline-offset-2 cursor-pointer"
                >
                  Lihat pesanan
                </button>
                {/* Kuota hari ini: pesanan yang sudah diserahkan ikut dihitung, jadi bisa lebih dari total "Butuh". */}
                {limit > 0 && due.tone !== 'late' && (
                  <p className={`w-full text-sm ${quotaFull ? 'font-semibold text-merah' : 'text-ink-soft'}`}>
                    Kuota terisi <span className="font-mono">{formatNumber(day.quota_used)}</span> dari{' '}
                    {formatNumber(limit)} pcs
                    {quotaFull ? ' (penuh)' : `, sisa ${formatNumber(limit - day.quota_used)} pcs`}
                  </p>
                )}
              </div>
              <ul className="divide-y divide-line">
                {day.items.map((item) => {
                  const product = products.get(item.product_id)
                  return (
                    <ProductionRow
                      key={item.product_id}
                      item={item}
                      onRecord={isAdmin && product ? () => setRecording(product) : undefined}
                    />
                  )
                })}
              </ul>
            </section>
          )
        })}

        {!loading && days.length === 0 && !error && (
          <p className="px-4 py-10 bg-paper border border-line text-center text-ink-soft">
            {dueDate
              ? `Tidak ada pre-order berjalan dengan hari H ${formatLongDate(dueDate)}, jadi tidak ada yang perlu diproduksi.`
              : 'Belum ada pre-order berjalan, jadi belum ada yang perlu diproduksi.'}
          </p>
        )}
        {loading && !data && (
          <p className="px-4 py-10 bg-paper border border-line text-center text-ink-soft">Memuat kebutuhan produksi…</p>
        )}
      </div>

      {recording && (
        <ProductionDialog
          product={recording}
          onClose={() => setRecording(null)}
          onSaved={() => {
            setRecording(null)
            setSavedKey((k) => k + 1)
          }}
        />
      )}
      {editingQuota && data && (
        <DailyLimitDialog
          current={data.daily_limit}
          onClose={() => setEditingQuota(false)}
          onSaved={() => {
            setEditingQuota(false)
            setSavedKey((k) => k + 1)
          }}
        />
      )}
    </div>
  )
}

function ProductionRow({ item, onRecord }: { item: ProductionItem; onRecord?: () => void }) {
  const percent = item.needed > 0 ? Math.round((item.ready / item.needed) * 100) : 0
  const done = item.short === 0
  return (
    <li className="px-4 py-3">
      {/* Tombol selalu di kanan atas; nama yang panjang turun baris di kolomnya sendiri. */}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="font-semibold break-words">{item.product_name}</p>
          <p className="text-sm text-ink-soft">dari {formatNumber(item.orders)} pesanan</p>
        </div>
        {onRecord && (
          <button
            type="button"
            onClick={onRecord}
            aria-label={`Catat produksi ${item.product_name}`}
            className="shrink-0 h-11 pointer-fine:h-9 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer whitespace-nowrap"
          >
            Catat produksi
          </button>
        )}
      </div>
      <dl className="mt-2 grid grid-cols-3 gap-2 sm:max-w-md">
        <div>
          <dt className="text-sm text-ink-soft">Butuh</dt>
          <dd className="font-mono text-lg font-semibold">{formatNumber(item.needed)}</dd>
        </div>
        <div>
          <dt className="text-sm text-ink-soft">Sudah siap</dt>
          <dd className="font-mono text-lg font-semibold">{formatNumber(item.ready)}</dd>
        </div>
        <div>
          <dt className="text-sm text-ink-soft">Kurang</dt>
          <dd className={`font-mono text-lg font-semibold ${done ? '' : 'text-merah'}`}>{formatNumber(item.short)}</dd>
        </div>
      </dl>
      {/* Hanya hiasan: angkanya sudah ada di atas. */}
      <div className="mt-2 h-2 sm:max-w-md bg-wall rounded-sm overflow-hidden" aria-hidden="true">
        <div className={`h-full ${done ? 'bg-hijau' : 'bg-ink'}`} style={{ width: `${percent}%` }} />
      </div>
    </li>
  )
}

const digitsOnly = (value: string) => value.replace(/\D/g, '').slice(0, 6)

// Catat produksi: stok siap produk bertambah, lalu dipakai pesanan dengan hari H paling awal.
function ProductionDialog({
  product,
  onClose,
  onSaved,
}: {
  product: ProductionProduct
  onClose: () => void
  onSaved: () => void
}) {
  const [qty, setQty] = useState('')
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const n = Number(qty || '0')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (n < 1) return setError('Isi jumlah yang baru selesai dibuat.')
    if (n > MAX_PRODUCTION) return setError(`Maksimal ${formatNumber(MAX_PRODUCTION)} per catatan.`)

    setBusy(true)
    try {
      const id = product.product_id
      await withRequestKey(`production:${id}`, (key) => addProduction(id, n, key))
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <Modal title="Catat produksi" onClose={onClose} busy={busy}>
      <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
        <div>
          <p className="text-lg font-semibold break-words">{product.product_name}</p>
          <dl className="mt-2 grid grid-cols-[1fr_auto] gap-x-3 gap-y-1">
            <dt className="text-ink-soft">Stok siap sekarang</dt>
            <dd className="font-mono text-right">{formatNumber(product.stock)}</dd>
            <dt className="text-ink-soft">Butuh untuk semua pesanan</dt>
            <dd className="font-mono text-right">{formatNumber(product.needed)}</dd>
            <dt className="text-ink-soft">Masih kurang</dt>
            <dd className={`font-mono text-right font-semibold ${product.short > 0 ? 'text-merah' : ''}`}>
              {formatNumber(product.short)}
            </dd>
          </dl>
        </div>

        <div>
          <label htmlFor="produksi-qty" className="block mb-1 text-sm font-semibold">
            Jumlah yang baru selesai dibuat
          </label>
          <div className="flex flex-wrap gap-2">
            <input
              id="produksi-qty"
              inputMode="numeric"
              autoComplete="off"
              value={qty ? formatNumber(n) : ''}
              onChange={(e) => setQty(digitsOnly(e.target.value))}
              aria-describedby="produksi-hasil"
              className="flex-1 min-w-32 h-11 px-3 bg-white border-2 border-ink rounded-sm font-mono text-base"
            />
            {product.short > 0 && (
              <button
                type="button"
                onClick={() => setQty(String(Math.min(product.short, MAX_PRODUCTION)))}
                className="h-11 px-3 border-2 border-ink rounded-sm font-semibold cursor-pointer whitespace-nowrap"
              >
                Isi kekurangan ({formatNumber(product.short)})
              </button>
            )}
          </div>
          <p id="produksi-hasil" className="mt-1 text-sm text-ink-soft" aria-live="polite">
            {n > 0 ? (
              <>
                Stok siap menjadi <span className="font-mono font-semibold text-ink">{formatNumber(product.stock + n)}</span>
                .
              </>
            ) : (
              'Stok siap bertambah sebanyak jumlah ini.'
            )}
          </p>
        </div>

        {error && (
          <p role="alert" className="font-semibold text-merah">
            {error}
          </p>
        )}

        <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
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
            className="h-11 px-5 bg-kunyit border-2 border-ink text-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
          >
            Simpan
          </button>
        </div>
      </form>
    </Modal>
  )
}

const limitDigits = (value: string) => value.replace(/\D/g, '').slice(0, 7)

// Ubah kuota pre-order per hari (hanya admin). Pesanan yang sudah ada tidak berubah.
function DailyLimitDialog({
  current,
  onClose,
  onSaved,
}: {
  current: number
  onClose: () => void
  onSaved: () => void
}) {
  const [value, setValue] = useState(String(current))
  const [error, setError] = useState('')
  const [busy, setBusy] = useState(false)
  const [showingHistory, setShowingHistory] = useState(false)
  const n = Number(value || '0')

  async function handleSubmit(e: FormEvent) {
    e.preventDefault()
    setError('')
    if (n < 1 || n > MAX_DAILY_LIMIT) return setError(`Isi kuota 1 sampai ${formatNumber(MAX_DAILY_LIMIT)} pcs.`)

    setBusy(true)
    try {
      await setPreorderDailyLimit(n)
      onSaved()
    } catch (err) {
      setError(err instanceof ApiError ? err.message : 'Terjadi kesalahan. Coba lagi.')
      setBusy(false)
    }
  }

  return (
    <>
      <Modal title="Ubah kuota pre-order" onClose={onClose} busy={busy}>
        <form onSubmit={(e) => void handleSubmit(e)} noValidate className="mt-4 space-y-4">
          <div>
            <label htmlFor="kuota-harian" className="block mb-1 text-sm font-semibold">
              Kuota per hari (pcs)
            </label>
            <input
              id="kuota-harian"
              inputMode="numeric"
              autoComplete="off"
              value={value ? formatNumber(n) : ''}
              onChange={(e) => setValue(limitDigits(e.target.value))}
              aria-describedby="kuota-harian-info"
              className="w-full sm:w-48 h-11 px-3 bg-white border-2 border-ink rounded-sm font-mono text-base"
            />
            <p id="kuota-harian-info" className="mt-1 text-sm text-ink-soft">
              Total pcs semua produk PO untuk satu hari H. Pesanan yang sudah ada tidak berubah; kuota baru berlaku
              untuk pesanan baru dan pemindahan jadwal.
            </p>
          </div>

          {error && (
            <p role="alert" className="font-semibold text-merah">
              {error}
            </p>
          )}

          <div className="flex flex-col-reverse gap-2 pt-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              onClick={() => setShowingHistory(true)}
              className="h-11 px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer sm:mr-auto"
            >
              Riwayat
            </button>
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
              className="h-11 px-5 bg-kunyit border-2 border-ink text-ink rounded-sm font-bold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
            >
              Simpan
            </button>
          </div>
        </form>
      </Modal>
      {showingHistory && (
        <AuditHistoryDialog
          entity="setting"
          entityId={QUOTA_SETTING_ID}
          title="Riwayat kuota pre-order"
          onClose={() => setShowingHistory(false)}
        />
      )}
    </>
  )
}
