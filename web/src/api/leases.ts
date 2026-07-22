import type { ApiResponse, Lease } from './types'
import { request } from './http'

// ── Leases ──
export function getLeases(): Promise<ApiResponse<Lease[]>> {
  return request<Lease[]>('GET', '/leases')
}

export interface LeaseStats {
  subnet_id: string
  dhcp_mode: string
  pool_size: number
  allocated: number
  available: number
  usage_pct: number
  active_only: number
}

export function getLeaseStats(): Promise<ApiResponse<LeaseStats[]>> {
  return request<LeaseStats[]>('GET', '/leases/stats')
}

export function deleteLease(mac: string): Promise<ApiResponse<{ message: string }>> {
  return request('DELETE', `/leases/${encodeURIComponent(mac)}`)
}

export function batchDeleteLeases(macs: string[]): Promise<ApiResponse<{ message: string; success_count: number; failed_count: number; failed_macs?: string[] }>> {
  return request('POST', '/leases/batch-delete', { macs })
}

export function pruneLeases(): Promise<ApiResponse<{ message: string }>> {
  return request('POST', '/leases/prune')
}
