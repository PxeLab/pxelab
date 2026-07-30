import type { ApiResponse } from './types'
import { request } from './http'

// ── Types ──

export interface Baseline {
  id: string
  name: string
  description?: string
  variables?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface BaselineScript {
  id?: number
  baseline_id: string
  filename: string
  content: string
  seq: number
  created_at?: string
  updated_at?: string
}

// ── Baselines ──

export function getBaselines(): Promise<ApiResponse<Baseline[]>> {
  return request<Baseline[]>('GET', '/baselines')
}

export function getBaseline(id: string): Promise<ApiResponse<Baseline>> {
  return request<Baseline>('GET', `/baselines/${id}`)
}

export function createBaseline(data: { name: string; description?: string; variables?: Record<string, string> }): Promise<ApiResponse<Baseline>> {
  return request<Baseline>('POST', '/baselines', data)
}

export function updateBaseline(id: string, data: { name?: string; description?: string; variables?: Record<string, string> }): Promise<ApiResponse<Baseline>> {
  return request<Baseline>('PUT', `/baselines/${id}`, data)
}

export function deleteBaseline(id: string): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/baselines/${id}`)
}

// ── Baseline Scripts ──

export function getBaselineScripts(baselineId: string): Promise<ApiResponse<BaselineScript[]>> {
  return request<BaselineScript[]>('GET', `/baselines/${baselineId}/scripts`)
}

export function getBaselineScript(id: number): Promise<ApiResponse<BaselineScript>> {
  return request<BaselineScript>('GET', `/baselines/scripts/${id}`)
}

export function upsertBaselineScript(data: BaselineScript): Promise<ApiResponse<BaselineScript>> {
  return request<BaselineScript>('POST', '/baselines/scripts', data)
}

export function deleteBaselineScript(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/baselines/scripts/${id}`)
}
