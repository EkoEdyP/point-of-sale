import type { PreorderQuota } from '../api/types'
import { formatNumber, formatShortDay } from '../lib/format'

// Kuota pre-order di form: total pcs semua produk PO per hari H. Ini supaya kasir tahu lebih dulu;
// server tetap menolak pesanan yang tidak muat.

// Pesan kalau pesanan sebanyak qty pcs tidak muat di kuota hari H, atau '' kalau muat (atau kuota belum dimuat).
export function quotaProblem(quota: PreorderQuota | null, qty: number): string {
  if (!quota || qty <= quota.remaining) return ''
  const day = formatShortDay(quota.due_date)
  if (quota.remaining === 0) return `Kuota pre-order ${day} sudah penuh. Pilih tanggal lain.`
  return (
    `Pesanan ini ${formatNumber(qty)} pcs, melebihi sisa kuota ${day} (${formatNumber(quota.remaining)} pcs). ` +
    'Kurangi jumlah atau pilih tanggal lain.'
  )
}

// Sisa kuota tanggal yang dipilih, di bawah kolom tanggal.
export function QuotaRemaining({ quota, id }: { quota: PreorderQuota | null; id?: string }) {
  if (!quota) return null
  if (quota.remaining === 0) {
    return (
      <p id={id} className="mt-1 text-sm font-semibold text-merah">
        Kuota penuh (kuota {formatNumber(quota.daily_limit)} pcs, sudah dipesan {formatNumber(quota.used)} pcs).
      </p>
    )
  }
  return (
    <p id={id} className="mt-1 text-sm text-ink-soft">
      Sisa kuota: <span className="font-mono font-semibold text-ink">{formatNumber(quota.remaining)}</span> dari{' '}
      {formatNumber(quota.daily_limit)} pcs.
    </p>
  )
}
