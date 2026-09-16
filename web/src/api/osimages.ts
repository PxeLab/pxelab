import type { ApiResponse } from './types'
import { request, getSessionToken, getBaseURL } from './http'

// ── OS Images ──
export interface OSImage {
  id?: number
  name: string
  filename: string
  source_path?: string
  size?: number
  distro?: string
  version?: string
  arch?: string
  status?: string
  mount_point?: string
  extracted_to?: string
  kernel_path?: string
  initrd_path?: string
  file_path?: string
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

// 带上传进度的 XHR 版本（fetch 无法监听 upload progress）
export function uploadOSImageWithProgress(
  file: File,
  onProgress: (percent: number) => void,
  name?: string,
): Promise<ApiResponse<OSImage>> {
  return new Promise((resolve, reject) => {
    const form = new FormData()
    form.append('file', file)
    if (name) form.append('name', name)

    const xhr = new XMLHttpRequest()
    xhr.open('POST', getBaseURL() + '/api/v1/os-images/upload')
    const session = getSessionToken()
    if (session) xhr.setRequestHeader('Authorization', 'Bearer ' + session)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable) onProgress(Math.round((e.loaded / e.total) * 100))
    }
    xhr.onload = () => {
      if (xhr.status === 401) {
        window.location.href = '/login'
        reject(new Error('Session expired'))
        return
      }
      try {
        const json = JSON.parse(xhr.responseText)
        if (xhr.status >= 200 && xhr.status < 300) resolve(json)
        else reject(new Error(json.error || `HTTP ${xhr.status}`))
      } catch {
        reject(new Error(`HTTP ${xhr.status}`))
      }
    }
    xhr.onerror = () => reject(new Error('Network error'))
    xhr.send(form)
  })
}

export function deleteOSImage(id: number, deleteFile = false): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/os-images/${id}${deleteFile ? '?delete_file=1' : ''}`)
}

export function extractOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/extract`)
}

export function reprocessOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/reprocess`)
}

export function updateOSImage(id: number, data: Partial<Pick<OSImage, 'name' | 'distro' | 'version' | 'arch' | 'kernel_path' | 'initrd_path'>>): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('PUT', `/os-images/${id}`, data)
}

export function importOSImages(dir: string, recursive = false): Promise<ApiResponse<{ imported: number; skipped: number }>> {
  return request<{ imported: number; skipped: number }>('POST', '/os-images/import', { dir, recursive })
}

export interface FsBrowseResult {
  path: string
  parent: string
  dirs: string[]
}

export function browseFs(path: string): Promise<ApiResponse<FsBrowseResult>> {
  return request<FsBrowseResult>('GET', '/fs/browse' + (path ? `?path=${encodeURIComponent(path)}` : ''))
}

export function mountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/mount`)
}

export function unmountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/unmount`)
}

// 把已提取的本地 Windows ISO 设为 Netboot Catalog 的 Windows PE 本地源
// （离线可引导，且安装任务的应答文件注入不受影响）
export function setCatalogLocal(id: number): Promise<ApiResponse<{ catalog_local: string; wimboot: string }>> {
  return request<{ catalog_local: string; wimboot: string }>('POST', `/os-images/${id}/set-catalog-local`)
}

// 把已提取的本地 Windows ISO 一键生成为 wds 类型 Profile（本机 wimboot + 本机提取目录）
export function createWdsProfileFromOSImage(
  osImageId: number,
  profileName: string,
  description?: string,
): Promise<ApiResponse<Record<string, unknown>>> {
  return request<Record<string, unknown>>('POST', '/profiles/from-os-image', {
    os_image_id: osImageId,
    profile_name: profileName,
    description,
  })
}
