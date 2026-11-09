import { useEffect, useState } from 'react'
import { DELIVERIES_CHANGED, getDeliverySummary } from '../api/deliveries'
import { PREORDERS_CHANGED } from '../api/preorders'
import type { DeliverySummary } from '../api/types'

const REFRESH_MS = 5 * 60 * 1000

// Jumlah pengiriman yang jadwalnya mendekat. Diperbarui tiap 5 menit, saat jendela kembali aktif,
// dan setiap kali pengiriman atau pre-order berubah (pre-order "diantar" membuat pengiriman otomatis).
export function useDeliverySummary(): DeliverySummary | null {
  const [summary, setSummary] = useState<DeliverySummary | null>(null)

  useEffect(() => {
    let cancelled = false
    const load = () => {
      getDeliverySummary()
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
    window.addEventListener(DELIVERIES_CHANGED, load)
    window.addEventListener(PREORDERS_CHANGED, load)
    return () => {
      cancelled = true
      clearInterval(timer)
      window.removeEventListener('focus', load)
      window.removeEventListener(DELIVERIES_CHANGED, load)
      window.removeEventListener(PREORDERS_CHANGED, load)
    }
  }, [])

  return summary
}
