import type { ApiResponse } from './types'
import { request } from './http'

// ── Access Control: Blacklist / Whitelist ──
export interface BlacklistEntry {
  id: number
  mac: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export interface WhitelistEntry {
  id: number
  mac: string
  subnet_cidr: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export function getBlacklist(): Promise<ApiResponse<BlacklistEntry[]>> {
  return request<BlacklistEntry[]>('GET', '/access/blacklist')
}

export function createBlacklistEntry(data: { mac: string; reason?: string }): Promise<ApiResponse<BlacklistEntry>> {
  return request<BlacklistEntry>('POST', '/access/blacklist', data)
}

export function deleteBlacklistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/blacklist/${id}`)
}

export function getWhitelist(): Promise<ApiResponse<WhitelistEntry[]>> {
  return request<WhitelistEntry[]>('GET', '/access/whitelist')
}

export function createWhitelistEntry(data: { mac: string; subnet_cidr?: string; reason?: string }): Promise<ApiResponse<WhitelistEntry>> {
  return request<WhitelistEntry>('POST', '/access/whitelist', data)
}

export function deleteWhitelistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/whitelist/${id}`)
}

// ── Unauthorized Devices ──
export interface UnauthorizedDevice {
  id: number
  mac: string
  subnet_cidr: string
  reason: string
  count: number
  last_seen: string
  created_at: string
  updated_at: string
}

export function getUnauthorizedDevices(): Promise<ApiResponse<UnauthorizedDevice[]>> {
  return request<UnauthorizedDevice[]>('GET', '/access/unauthorized')
}

export function addUnauthorizedToWhitelist(mac: string, subnet_cidr: string): Promise<ApiResponse<WhitelistEntry>> {
  return request<WhitelistEntry>('POST', '/access/unauthorized/add-to-whitelist', { mac, subnet_cidr })
}

export function addUnauthorizedToBlacklist(mac: string, subnet_cidr: string): Promise<ApiResponse<BlacklistEntry>> {
  return request<BlacklistEntry>('POST', '/access/unauthorized/add-to-blacklist', { mac, subnet_cidr })
}

export function deleteUnauthorizedDevice(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/unauthorized/${id}`)
}
