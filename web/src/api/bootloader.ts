import type { ApiResponse } from './types'
import { request } from './http'

// ── Bootloader Management ──
export interface BootFileInfo {
  name: string
  description: string
  required: boolean
  present: boolean
  size?: number
  checksum?: string
  mod_time?: string
}

export interface BootloaderCheckResult {
  root_dir: string
  total: number
  present: number
  missing: number
  all_ok: boolean
  files: BootFileInfo[]
}

export interface BootDirEntry {
  name: string
  size: number
  mod_time: string
  is_dir: boolean
}

export function getBootloaderCheck(): Promise<ApiResponse<BootloaderCheckResult>> {
  return request<BootloaderCheckResult>('GET', '/bootloader/check')
}

export function getBootloaderFiles(): Promise<ApiResponse<BootDirEntry[]>> {
  return request<BootDirEntry[]>('GET', '/bootloader/files')
}

export function checkBootloaderFile(name: string): Promise<ApiResponse<BootFileInfo>> {
  return request<BootFileInfo>('POST', '/bootloader/check-file', { name })
}
