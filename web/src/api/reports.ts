import { api } from './client'
import type { DailyReport, ReportSource } from './types'

export function getDailyReport(from: string, to: string, source: ReportSource) {
  const query = new URLSearchParams()
  if (from) query.set('from', from)
  if (to) query.set('to', to)
  query.set('source', source)
  return api<DailyReport>(`/api/v1/reports/daily?${query}`)
}
