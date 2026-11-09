import { api } from './client'
import type { Delivery, DeliveryList, DeliveryPatch, DeliverySummary } from './types'

// Dipancarkan setiap kali pengiriman berubah, supaya strip peringatan di header ikut diperbarui.
export const DELIVERIES_CHANGED = 'deliveries:changed'

function notifyChanged() {
  window.dispatchEvent(new Event(DELIVERIES_CHANGED))
}

const json = (body: unknown) => JSON.stringify(body)

export function listDeliveries(params: { status?: string; page?: number; limit?: number }) {
  const query = new URLSearchParams()
  query.set('status', params.status ?? 'active')
  query.set('page', String(params.page ?? 1))
  query.set('limit', String(params.limit ?? 20))
  return api<DeliveryList>(`/api/v1/deliveries?${query}`)
}

export function getDelivery(id: string) {
  return api<Delivery>(`/api/v1/deliveries/${id}`)
}

export function getDeliverySummary() {
  return api<DeliverySummary>('/api/v1/deliveries/summary')
}

export async function updateDelivery(id: string, patch: DeliveryPatch) {
  const result = await api<Delivery>(`/api/v1/deliveries/${id}`, { method: 'PATCH', body: json(patch) })
  notifyChanged()
  return result
}

export async function dispatchDelivery(id: string, courier: string) {
  const result = await api<Delivery>(`/api/v1/deliveries/${id}/dispatch`, {
    method: 'POST',
    body: json(courier ? { courier } : {}),
  })
  notifyChanged()
  return result
}

export async function markFailed(id: string, reason: string) {
  const result = await api<Delivery>(`/api/v1/deliveries/${id}/fail`, { method: 'POST', body: json({ reason }) })
  notifyChanged()
  return result
}

export async function cancelDelivery(id: string, reason: string) {
  const result = await api<Delivery>(`/api/v1/deliveries/${id}/cancel`, { method: 'POST', body: json({ reason }) })
  notifyChanged()
  return result
}
