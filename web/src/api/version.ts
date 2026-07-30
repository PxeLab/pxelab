import type { ApiResponse, VersionInfo, DownloadResult } from './types'
import { request } from './http'

// ── Version Info ──

export function getVersionInfo(): Promise<ApiResponse<VersionInfo>> {
  return request<VersionInfo>('GET', '/version')
}

// ── Check for Updates ──

export function checkForUpdate(): Promise<ApiResponse<VersionInfo>> {
  return request<VersionInfo>('POST', '/version/check')
}

// ── Download Update ──

export function downloadUpdate(): Promise<ApiResponse<DownloadResult>> {
  return request<DownloadResult>('POST', '/version/download')
}
