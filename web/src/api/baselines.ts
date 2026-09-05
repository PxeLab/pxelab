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

export interface BaselineScriptAssignment {
  script_id: number
  seq: number
  name: string
  type: string
  content: string
  description?: string
  created_at: string
  updated_at: string
}

export interface SetScriptsItem {
  script_id: number
  seq: number
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

// ── Baseline Script Associations ──

export function getBaselineScripts(baselineId: string): Promise<ApiResponse<BaselineScriptAssignment[]>> {
  return request<BaselineScriptAssignment[]>('GET', `/baselines/${baselineId}/scripts`)
}

export function setBaselineScripts(baselineId: string, scripts: SetScriptsItem[]): Promise<ApiResponse<{ scripts: SetScriptsItem[] }>> {
  return request<{ scripts: SetScriptsItem[] }>('PUT', `/baselines/${baselineId}/scripts`, { scripts })
}

export interface InlineCreatedScript {
  id: number
  name: string
  type: string
  content: string
  description?: string
  created_at: string
  updated_at: string
}

export interface InlineCreateScriptResult {
  baseline_id: string
  seq: number
  script: InlineCreatedScript
}

/** 一步动作：在共享脚本库中新建脚本，并自动追加到该脚本集末尾（L2 inline create）。 */
export function createAndAddBaselineScript(
  baselineId: string,
  data: { name: string; type?: string; content: string; description?: string }
): Promise<ApiResponse<InlineCreateScriptResult>> {
  return request<InlineCreateScriptResult>('POST', `/baselines/${baselineId}/scripts`, data)
}

// ── Assigned Scripts (machine pull) ──

export function getAssignedBaselineScripts(mac: string): Promise<ApiResponse<BaselineScriptAssignment[]>> {
  return request<BaselineScriptAssignment[]>('GET', `/baselines/assigned?mac=${encodeURIComponent(mac)}`)
}
