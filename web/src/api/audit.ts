import type { ApiResponse, AuditLog } from './types'
import { buildQuery, request } from './http'

// ── Audit Logs ──
export function getAuditLogs(params?: Record<string, unknown>): Promise<ApiResponse<{ logs: AuditLog[]; meta: { page: number; size: number; total: number } }>> {
  return request('GET', '/audit-logs' + buildQuery(params))
}
