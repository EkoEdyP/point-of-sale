import { useEffect, useState, type ReactNode } from 'react'
import { listAuditLogs } from '../api/audit'
import { ApiError } from '../api/client'
import type { AuditEntity, AuditEntry } from '../api/types'
import { AUDIT_START } from '../lib/audit'
import { formatDayTime, formatLongDate, toDateInput } from '../lib/format'
import { AuditEntryView } from './AuditEntryView'
import { Modal } from './Modal'

const PAGE_SIZE = 30

// Riwayat perubahan satu data, terbaru di atas. created: kapan dan oleh siapa datanya dibuat, untuk data yang
// pembuatannya tidak tercatat di riwayat (pre-order, pengiriman) atau dibuat sebelum riwayat mulai dicatat.
export function AuditHistoryDialog({
  entity,
  entityId,
  title,
  created,
  onClose,
}: {
  entity: AuditEntity
  entityId: string
  title: string
  created?: { at: string; by: string | null; label: string }
  onClose: () => void
}) {
  const [entries, setEntries] = useState<AuditEntry[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(1)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  useEffect(() => {
    let cancelled = false
    setLoading(true)
    listAuditLogs({ entity, entityId, page, limit: PAGE_SIZE })
      .then((r) => {
        if (cancelled) return
        // Perubahan baru yang masuk di antara dua halaman menggeser isinya; yang sudah tampil tidak diulang.
        setEntries((prev) => {
          if (page === 1) return r.data
          const shown = new Set(prev.map((e) => e.id))
          return [...prev, ...r.data.filter((e) => !shown.has(e.id))]
        })
        setTotal(r.total)
        setError('')
      })
      .catch((e) => {
        if (!cancelled) setError(e instanceof ApiError ? e.message : 'Gagal memuat riwayat perubahan.')
      })
      .finally(() => {
        if (!cancelled) setLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [entity, entityId, page])

  const allLoaded = !loading && !error && entries.length >= total
  const hasCreate = entries.some((e) => e.action === 'create')
  const showCreated = allLoaded && !hasCreate && created !== undefined
  // Data yang sudah ada sebelum riwayat dicatat: perubahan lamanya tidak ada di daftar.
  const createdDay = created ? toDateInput(new Date(created.at)) : null
  const olderMissing = allLoaded && !hasCreate && (createdDay === null || createdDay < AUDIT_START)
  // Pembatalan lama diisi belakangan (lihat AUDIT_START), jadi yang tidak tercatat hanya perubahan lainnya.
  const hasOlder = entries.some((e) => toDateInput(new Date(e.created_at)) < AUDIT_START)

  return (
    <Modal title={title} onClose={onClose}>
      {error && (
        <p role="alert" className="mt-3 font-semibold text-merah">
          {error}
        </p>
      )}
      {loading && entries.length === 0 && <p className="mt-3 text-ink-soft">Memuat riwayat…</p>}

      {(entries.length > 0 || showCreated) && (
        <ol className="mt-4">
          {entries.map((e) => (
            <TimelineItem key={e.id} when={e.created_at} by={e.actor_name ?? 'Sistem'}>
              <AuditEntryView entry={e} />
            </TimelineItem>
          ))}
          {showCreated && created && (
            <TimelineItem when={created.at} by={created.by}>
              <p className="font-semibold">{created.label}</p>
            </TimelineItem>
          )}
        </ol>
      )}

      {allLoaded && entries.length === 0 && !showCreated && (
        <p className="mt-3 text-ink-soft">Belum ada perubahan yang tercatat.</p>
      )}
      {entries.length < total && (
        <button
          type="button"
          disabled={loading}
          onClick={() => setPage((p) => p + 1)}
          className="mt-2 h-11 w-full px-4 border-2 border-ink rounded-sm font-semibold cursor-pointer disabled:opacity-60 disabled:cursor-wait"
        >
          {loading ? 'Memuat…' : 'Muat perubahan sebelumnya'}
        </button>
      )}
      {olderMissing && (
        <p className="mt-3 text-sm text-ink-soft">
          Riwayat dicatat mulai {formatLongDate(AUDIT_START)}. Perubahan {hasOlder ? 'lain ' : ''}sebelum itu
          tidak tercatat.
        </p>
      )}

      <div className="mt-6 flex justify-end">
        <button
          type="button"
          onClick={onClose}
          className="h-11 w-full sm:w-auto px-5 border-2 border-ink rounded-sm font-semibold cursor-pointer"
        >
          Tutup
        </button>
      </div>
    </Modal>
  )
}

// Satu titik di garis waktu: kapan dan oleh siapa, lalu isinya.
function TimelineItem({ when, by, children }: { when: string; by: string | null; children: ReactNode }) {
  return (
    <li className="relative ml-1.5 pl-5 pb-4 border-l-2 border-line last:border-transparent last:pb-0">
      <span aria-hidden="true" className="absolute -left-[7px] top-1 size-3 rounded-full bg-ink ring-2 ring-paper" />
      <p className="text-sm text-ink-soft">
        {formatDayTime(when)}
        {by ? ` · ${by}` : ''}
      </p>
      <div className="mt-0.5">{children}</div>
    </li>
  )
}
