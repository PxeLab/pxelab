import type { ApiResponse } from './types'
import { request } from './http'

// ── BMC / OOB ──
export interface BMCConfig {
  id?: number
  host: string
  port: number
  username: string
  password?: string
  protocol: string   // "ipmi" | "redfish"
  vendor: string
  model: string
  serial: string
  mac: string
  name: string
  boot_mode: string  // "auto" | "uefi" | "legacy"
  next_boot_device: string  // "pxe" | "disk" | "cdrom" | "bios"
  has_password?: boolean
  created_at?: string
  updated_at?: string
}

export interface BMCProbeResult {
  name: string
  vendor: string
  model: string
  serial: string
  mac: string
}

export function getBMCConfigs(): Promise<ApiResponse<BMCConfig[]>> {
  return request<BMCConfig[]>('GET', '/bmc/configs')
}

export function getBMCConfig(id: number): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('GET', `/bmc/configs/${id}`)
}

export function createBMCConfig(data: Partial<BMCConfig>): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('POST', '/bmc/configs', data)
}

export function updateBMCConfig(id: number, data: Partial<BMCConfig>): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('PUT', `/bmc/configs/${id}`, data)
}

export function deleteBMCConfig(id: number): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/bmc/configs/${id}`)
}

export function probeBMC(data: { host: string; port: number; username: string; password: string; protocol?: string }): Promise<ApiResponse<BMCProbeResult>> {
  return request<BMCProbeResult>('POST', '/bmc/probe', data)
}

export function refreshBMCConfig(id: number): Promise<ApiResponse<{ refreshed: boolean; result?: BMCProbeResult; error?: string }>> {
  return request('POST', `/bmc/${id}/refresh`)
}

export function bmcPowerOn(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/power-on`)
}

export function bmcPowerOff(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/power-off`)
}

export function bmcRestart(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/restart`)
}

export function bmcPowerStatus(id: number): Promise<ApiResponse<{ config_id: number; status: string }>> {
  return request('GET', `/bmc/${id}/status`)
}

export function bmcSetBootDevice(id: number, device: string): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/boot-device`, { device })
}

export function bmcBatchPowerOn(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/power-on', { ids })
}

export function bmcBatchPowerOff(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/power-off', { ids })
}

export function bmcBatchRestart(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/restart', { ids })
}

export function bmcBatchStatus(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; state?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/status', { ids })
}

export function bmcImportCSV(csv: string): Promise<ApiResponse<{ success: number; failed: number; errors: string[] }>> {
  return request('POST', '/bmc/configs/import', { csv })
}
