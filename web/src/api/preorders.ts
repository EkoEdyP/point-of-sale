import { idempotencyHeader } from '../lib/requestKey'
import { api } from './client'
import type {
  PaymentMethod,
  Preorder,
  PreorderInput,
  PreorderList,
  PreorderProduction,
  PreorderQuota,
  PreorderSummary,
  Transaction,
} from './types'

// Dipancarkan setiap kali pre-order berubah, supaya strip peringatan di header ikut diperbarui.
export const PREORDERS_CHANGED = 'preorders:changed'

function notifyChanged() {
  window.dispatchEvent(new Event(PREORDERS_CHANGED))
}

const json = (body: unknown) => JSON.stringify(body)

export function listPreorders(params: { status?: string; dueDate?: string; page?: number; limit?: number }) {
  const query = new URLSearchParams()
  query.set('status', params.status ?? 'open')
  if (params.dueDate) query.set('due_date', params.dueDate)
  query.set('page', String(params.page ?? 1))
  query.set('limit', String(params.limit ?? 20))
  return api<PreorderList>(`/api/v1/preorders?${query}`)
}

export function getPreorder(id: string) {
  return api<Preorder>(`/api/v1/preorders/${id}`)
}

export function getPreorderSummary() {
  return api<PreorderSummary>('/api/v1/preorders/summary')
}

// dueDate kosong = semua hari H yang masih berjalan.
export function getPreorderProduction(dueDate?: string) {
  const query = dueDate ? `?${new URLSearchParams({ due_date: dueDate })}` : ''
  return api<PreorderProduction>(`/api/v1/preorders/production${query}`)
}

// Kuota hari H. excludeId: pre-order yang sedang dipindah jadwalnya, supaya tidak terhitung dua kali.
export function getPreorderQuota(dueDate: string, excludeId?: string) {
  const query = new URLSearchParams({ due_date: dueDate })
  if (excludeId) query.set('exclude', excludeId)
  return api<PreorderQuota>(`/api/v1/preorders/quota?${query}`)
}

// Hanya admin. Berlaku untuk pesanan baru dan pemindahan jadwal; pesanan yang sudah ada tidak berubah.
export async function setPreorderDailyLimit(dailyLimit: number) {
  const result = await api<{ daily_limit: number }>('/api/v1/preorders/quota', {
    method: 'PUT',
    body: json({ daily_limit: dailyLimit }),
  })
  notifyChanged()
  return result
}

// key: kunci idempotensi (lib/requestKey.ts), sama untuk setiap percobaan penyimpanan yang sama.
export async function createPreorder(input: PreorderInput, key?: string) {
  const result = await api<Preorder>('/api/v1/preorders', {
    method: 'POST',
    body: json(input),
    headers: idempotencyHeader(key),
  })
  notifyChanged()
  return result
}

export async function addDownPayment(id: string, amount: number, method: PaymentMethod, key?: string) {
  const result = await api<Preorder>(`/api/v1/preorders/${id}/payments`, {
    method: 'POST',
    body: json({ amount, method }),
    headers: idempotencyHeader(key),
  })
  notifyChanged()
  return result
}

export interface PreorderPatch {
  customer_name?: string
  customer_phone?: string
  address?: string // hanya pre-order diantar
  due_date?: string
  due_time?: string
  notes?: string
  shipping_fee?: number
  other_fee?: number
  other_fee_note?: string
  version?: number // versi saat dialog ubah dibuka
}

export async function updatePreorder(id: string, patch: PreorderPatch) {
  const result = await api<Preorder>(`/api/v1/preorders/${id}`, { method: 'PATCH', body: json(patch) })
  notifyChanged()
  return result
}

export async function markPreorderReady(id: string) {
  const result = await api<Preorder>(`/api/v1/preorders/${id}/ready`, { method: 'POST' })
  notifyChanged()
  return result
}

// Mengembalikan transaksi yang terbentuk (untuk struk). Kuncinya per pre-order (completeKeyScope), jadi pelunasan
// dari detail pre-order maupun dari pengirimannya memakai kunci yang sama.
export async function completePreorder(
  id: string,
  input: { payment_method?: PaymentMethod; paid?: number; received_by?: string },
  key?: string,
) {
  const result = await api<Transaction>(`/api/v1/preorders/${id}/complete`, {
    method: 'POST',
    body: json(input),
    headers: idempotencyHeader(key),
  })
  notifyChanged()
  return result
}

export const completeKeyScope = (preorderId: string) => `complete:${preorderId}`

export async function cancelPreorder(id: string, reason: string) {
  const result = await api<Preorder>(`/api/v1/preorders/${id}/cancel`, { method: 'POST', body: json({ reason }) })
  notifyChanged()
  return result
}
