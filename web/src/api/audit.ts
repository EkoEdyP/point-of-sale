import { api } from './client'
import type { AuditEntity, AuditList } from './types'

// Riwayat perubahan data, terbaru dulu. Hanya admin. from/to: YYYY-MM-DD, keduanya inklusif.
export function listAuditLogs(params: {
  entity?: AuditEntity | ''
  entityId?: string
  actorId?: string
  from?: string
  to?: string
  page?: number
  limit?: number
}) {
  const query = new URLSearchParams()
  if (params.entity) query.set('entity', params.entity)
  if (params.entityId) query.set('entity_id', params.entityId)
  if (params.actorId) query.set('actor_id', params.actorId)
  if (params.from) query.set('from', params.from)
  if (params.to) query.set('to', params.to)
  query.set('page', String(params.page ?? 1))
  query.set('limit', String(params.limit ?? 20))
  return api<AuditList>(`/api/v1/audit-logs?${query}`)
}
