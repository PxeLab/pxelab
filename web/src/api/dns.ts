import type { ApiResponse, DNSRecord } from './types'
import { request } from './http'

// ── DNS Records ──
export function getDNSRecords(): Promise<ApiResponse<{ records: DNSRecord[]; local_domain?: string }>> {
  return request('GET', '/dns/records')
}

export function createDNSRecord(r: DNSRecord): Promise<ApiResponse<DNSRecord>> {
  return request('POST', '/dns/records', r)
}

export function getDNSRecord(id: number): Promise<ApiResponse<DNSRecord>> {
  return request('GET', `/dns/records/${id}`)
}

export function updateDNSRecord(id: number, r: DNSRecord): Promise<ApiResponse<DNSRecord>> {
  return request('PUT', `/dns/records/${id}`, r)
}

export function deleteDNSRecord(id: number): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/dns/records/${id}`)
}
