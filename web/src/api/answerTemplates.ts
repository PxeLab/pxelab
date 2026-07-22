import type { ApiResponse } from './types'
import { request } from './http'

// ── Answer Templates ──
export interface AnswerTemplate {
  id?: number
  name: string
  description?: string
  type: string // kickstart | preseed | subiquity | autoyast | autounattend
  content: string
  current_version?: number
  variables?: string[]
  created_at?: string
  updated_at?: string
}

export interface AnswerTemplateVersion {
  id: number
  template_id: number
  version: number
  content: string
  description?: string
  created_at: string
}

export function getAnswerTemplates(): Promise<ApiResponse<{ templates: AnswerTemplate[] }>> {
  return request('GET', '/netboot/answer-templates')
}

export function createAnswerTemplate(t: AnswerTemplate): Promise<ApiResponse<AnswerTemplate>> {
  return request('POST', '/netboot/answer-templates', t)
}

export function getAnswerTemplate(id: number): Promise<ApiResponse<AnswerTemplate>> {
  return request('GET', `/netboot/answer-templates/${id}`)
}

export function updateAnswerTemplate(id: number, t: AnswerTemplate): Promise<ApiResponse<AnswerTemplate>> {
  return request('PUT', `/netboot/answer-templates/${id}`, t)
}

export function deleteAnswerTemplate(id: number): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/netboot/answer-templates/${id}`)
}

export function getAnswerTemplateVersions(id: number): Promise<ApiResponse<{ versions: AnswerTemplateVersion[] }>> {
  return request('GET', `/netboot/answer-templates/${id}/versions`)
}

export function getAnswerTemplateVersion(id: number, version: number): Promise<ApiResponse<AnswerTemplateVersion>> {
  return request('GET', `/netboot/answer-templates/${id}/versions/${version}`)
}

export function rollbackAnswerTemplate(id: number, version: number): Promise<ApiResponse<AnswerTemplate>> {
  return request('POST', `/netboot/answer-templates/${id}/rollback/${version}`)
}

export function validateAnswerTemplate(content: string): Promise<ApiResponse<{ valid: boolean; error?: string }>> {
  return request('POST', '/netboot/answer-templates/validate', content)
}

export function validateAnswerTemplateById(id: number, content: string): Promise<ApiResponse<{ valid: boolean; error?: string }>> {
  return request('POST', `/netboot/answer-templates/${id}/validate`, content)
}

export function previewAnswerTemplate(id: number, vars: Record<string, string>): Promise<ApiResponse<{ rendered: string }>> {
  return request('POST', `/netboot/answer-templates/${id}/preview`, vars)
}

export function getAnswerTemplatePresets(type?: string): Promise<ApiResponse<{ presets: { type: string; count?: number }[] } | { presets: { name: string; description: string; content: string; variables: string[] }[] }>> {
  const q = type ? `?type=${encodeURIComponent(type)}` : ''
  return request('GET', `/netboot/answer-templates/presets${q}`)
}
