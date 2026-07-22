import type { ApiResponse } from './types'
import { request } from './http'

// ── OS Images ──
export interface OSImage {
  id?: number
  name: string
  filename: string
  size?: number
  distro?: string
  version?: string
  arch?: string
  status?: string
  mount_point?: string
  extracted_to?: string
  checksum?: string
  error_message?: string
  created_at?: string
  updated_at?: string
}

export function getOSImages(): Promise<ApiResponse<OSImage[]>> {
  return request<OSImage[]>('GET', '/os-images')
}

export function getOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('GET', `/os-images/${id}`)
}

export function uploadOSImage(file: File, name?: string): Promise<ApiResponse<OSImage>> {
  const form = new FormData()
  form.append('file', file)
  if (name) form.append('name', name)
  return request<OSImage>('POST', '/os-images/upload', form)
}

export function deleteOSImage(id: number): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/os-images/${id}`)
}

export function extractOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/extract`)
}

export function mountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/mount`)
}

export function unmountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/unmount`)
}
