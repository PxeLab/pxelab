import type { ApiResponse } from './types'
import { request } from './http'

// ── Types ──

export interface scriptDTO {
  id: number
  name: string
  type: string // shell / bat / powershell
  content: string
  description?: string
  created_at: string
  updated_at: string
}

export interface ScriptReq {
  name: string
  type?: string
  content: string
  description?: string
}

// ── Scripts ──

export async function getScripts(): Promise<ApiResponse<scriptDTO[]>> {
  const res = await request<{ scripts: scriptDTO[] }>('GET', '/scripts')
  return { ...res, data: res.data?.scripts ?? [] }
}

export function getScript(id: number): Promise<ApiResponse<scriptDTO>> {
  return request<scriptDTO>('GET', `/scripts/${id}`)
}

export function createScript(data: ScriptReq): Promise<ApiResponse<scriptDTO>> {
  return request<scriptDTO>('POST', '/scripts', data)
}

export function updateScript(id: number, data: Partial<ScriptReq>): Promise<ApiResponse<scriptDTO>> {
  return request<scriptDTO>('PUT', `/scripts/${id}`, data)
}

export function deleteScript(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/scripts/${id}`)
}
