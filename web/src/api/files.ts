import type { ApiResponse, FileInfo } from './types'
import { request } from './http'

// ── Files ──
export function getFiles(dir?: string): Promise<ApiResponse<FileInfo[]>> {
  const q = dir ? '?dir=' + encodeURIComponent(dir) : ''
  return request<FileInfo[]>('GET', '/files' + q)
}

export function uploadFile(file: File): Promise<ApiResponse<unknown>> {
  const fd = new FormData()
  fd.append('file', file)
  return request<unknown>('POST', '/files/upload', fd)
}

export function deleteFile(path: string): Promise<ApiResponse<void>> {
  return request<void>('DELETE', '/files?path=' + encodeURIComponent(path))
}

export function getBootRootDir(): Promise<ApiResponse<{ root_dir: string }>> {
  return request<{ root_dir: string }>('GET', '/files/root')
}
