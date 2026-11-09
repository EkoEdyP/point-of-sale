import { useEffect, useState } from 'react'
import { getPreorderQuota } from '../api/preorders'
import type { PreorderQuota } from '../api/types'

const FULL_DATE = /^\d{4}-\d{2}-\d{2}$/

// Kuota hari H untuk form pre-order. null selama memuat, kalau tanggal belum lengkap, atau kalau gagal
// dimuat (server tetap menolak pesanan yang melebihi kuota). excludeId: pre-order yang sedang dipindah jadwalnya.
export function usePreorderQuota(dueDate: string, excludeId?: string): PreorderQuota | null {
  const [quota, setQuota] = useState<PreorderQuota | null>(null)

  useEffect(() => {
    if (!FULL_DATE.test(dueDate)) return
    let cancelled = false
    getPreorderQuota(dueDate, excludeId)
      .then((q) => {
        if (!cancelled) setQuota(q)
      })
      .catch(() => {
        if (!cancelled) setQuota(null)
      })
    return () => {
      cancelled = true
    }
  }, [dueDate, excludeId])

  // Kuota tanggal sebelumnya tidak ditampilkan selama kuota tanggal yang baru masih dimuat.
  return quota && quota.due_date === dueDate ? quota : null
}
