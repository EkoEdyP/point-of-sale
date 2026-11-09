import type { Fulfillment, PreorderStatus } from '../api/types'
import { formatShortDay, parseDateInput } from './format'

export const STATUS_LABEL: Record<PreorderStatus, string> = {
  ordered: 'Dipesan',
  ready: 'Siap',
  completed: 'Selesai',
  cancelled: 'Dibatalkan',
}

export const FULFILLMENT_LABEL: Record<Fulfillment, string> = {
  pickup: 'Ambil di toko',
  delivery: 'Diantar',
}

// Batas ongkir dan biaya lain pre-order (Rp), sama dengan server.
export const MAX_PREORDER_FEE = 100_000_000

// Selisih hari dari hari ini: negatif berarti terlambat, 0 hari ini, 1 besok.
export function daysUntil(dueDate: string): number {
  const now = new Date()
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  return Math.round((parseDateInput(dueDate).getTime() - today.getTime()) / 86_400_000)
}

// Warna: late/today merah, soon (besok) kuning, upcoming (lusa dan seterusnya) hijau.
export type DueTone = 'late' | 'today' | 'soon' | 'upcoming' | 'normal'

// Peringatan pre-order dimulai H-2 (lusa) supaya ada waktu menyiapkan pesanan.
// Pengiriman tetap memakai bawaan dueInfo, yaitu H-1.
export const PREORDER_WARN_DAYS = 2

// warnDays: mulai berapa hari sebelum hari H label peringatan muncul.
export function dueInfo(dueDate: string, warnDays = 1): { label: string; tone: DueTone } {
  const days = daysUntil(dueDate)
  if (days < 0) return { label: `Terlambat ${-days} hari`, tone: 'late' }
  if (days === 0) return { label: 'Hari ini', tone: 'today' }
  if (days > warnDays) return { label: formatShortDay(dueDate), tone: 'normal' }
  if (days === 1) return { label: 'Besok', tone: 'soon' }
  if (days === 2) return { label: 'Lusa', tone: 'upcoming' }
  return { label: `${days} hari lagi`, tone: 'upcoming' }
}

export const TONE_CLASS: Record<DueTone, string> = {
  late: 'bg-merah text-white',
  today: 'bg-merah text-white',
  soon: 'bg-kunyit text-ink',
  upcoming: 'bg-hijau text-white',
  normal: '',
}
