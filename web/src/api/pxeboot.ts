import type { ApiResponse } from './types'
import { request } from './http'

// ── PXE Boot Records / Host Claim ──

export interface PxeBootRecord {
  mac: string
  ip: string
  loader: string
  last_context: string
  first_seen: string
  last_seen: string
  count: number
  claimed_host_id: string
  claimed_host: string
}

export function getPxeBootRecords(): Promise<ApiResponse<{ records: PxeBootRecord[] }>> {
  return request('GET', '/pxe-boot/records')
}

export function deletePxeBootRecord(mac: string): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/pxe-boot/records/${encodeURIComponent(mac)}`)
}

export function clearPxeBootRecords(): Promise<ApiResponse<unknown>> {
  return request('POST', '/pxe-boot/records/clear')
}

export function claimPxeBootRecord(mac: string, hostId: string): Promise<ApiResponse<{ mac: string; claimed_host_id: string }>> {
  return request('POST', `/pxe-boot/records/${encodeURIComponent(mac)}/claim`, { host_id: hostId })
}
