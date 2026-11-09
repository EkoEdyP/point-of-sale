import { useEffect, useState } from 'react'
import { PREORDERS_CHANGED, getPreorderSummary } from '../api/preorders'
import type { PreorderSummary } from '../api/types'

const REFRESH_MS = 5 * 60 * 1000

// Jumlah pre-order yang mendekati hari H. Diperbarui tiap 5 menit, saat jendela kembali aktif,
// dan setiap kali ada perubahan pre-order.
export function usePreorderSummary(): PreorderSummary | null {
  const [summary, setSummary] = useState<PreorderSummary | null>(null)

  useEffect(() => {
    let cancelled = false
    const load = () => {
      getPreorderSummary()
        .then((s) => {
          if (!cancelled) setSummary(s)
        })
        .catch(() => {
          // peringatan ini tidak kritis, abaikan kalau gagal
        })
    }

    load()
    const timer = setInterval(load, REFRESH_MS)
    window.addEventListener('focus', load)
    window.addEventListener(PREORDERS_CHANGED, load)
    return () => {
      cancelled = true
      clearInterval(timer)
      window.removeEventListener('focus', load)
      window.removeEventListener(PREORDERS_CHANGED, load)
    }
  }, [])

  return summary
}
