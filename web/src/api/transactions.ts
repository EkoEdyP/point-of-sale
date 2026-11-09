import { idempotencyHeader } from '../lib/requestKey'
import { api } from './client'
import type { Transaction, TransactionInput, TransactionList } from './types'

// key: kunci idempotensi (lib/requestKey.ts), sama untuk setiap percobaan penjualan yang sama.
export function createTransaction(input: TransactionInput, key?: string) {
  return api<Transaction>('/api/v1/transactions', {
    method: 'POST',
    body: JSON.stringify(input),
    headers: idempotencyHeader(key),
  })
}

export function listTransactions(params: { from?: string; to?: string; status?: string; page?: number; limit?: number }) {
  const query = new URLSearchParams()
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  if (params.status) query.set('status', params.status)
  query.set('page', String(params.page ?? 1))
  query.set('limit', String(params.limit ?? 20))
  return api<TransactionList>(`/api/v1/transactions?${query}`)
}

export function getTransaction(id: string) {
  return api<Transaction>(`/api/v1/transactions/${id}`)
}

export function voidTransaction(id: string, reason: string) {
  return api<Transaction>(`/api/v1/transactions/${id}/void`, { method: 'POST', body: JSON.stringify({ reason }) })
}
