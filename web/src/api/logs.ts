import type { ApiResponse } from './types'
import { request } from './http'

// ── Logging Settings ──
export interface LoggingSettings {
  max_size_mb: number
  max_backups: number
  max_age_days: number
  compress: boolean
  cleanup_interval: number
}

export interface LogFileInfo {
  name: string
  size: number
  mod_time: string
}

export interface LogDiskUsage {
  dir: string
  size_bytes: number
}

export function getLoggingSettings(): Promise<ApiResponse<LoggingSettings>> {
  return request<LoggingSettings>('GET', '/settings/logging')
}

export function updateLoggingSettings(data: LoggingSettings): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/logging', data)
}

export function getLogFiles(): Promise<ApiResponse<{ files: LogFileInfo[]; dir: string }>> {
  return request('GET', '/logs/files')
}

export function getLogDiskUsage(): Promise<ApiResponse<LogDiskUsage>> {
  return request<LogDiskUsage>('GET', '/logs/disk-usage')
}

export function cleanupLogs(maxAgeDays?: number, maxBackups?: number): Promise<ApiResponse<{ removed: number }>> {
  return request('POST', '/logs/cleanup', { max_age_days: maxAgeDays || 30, max_backups: maxBackups || 5 })
}
