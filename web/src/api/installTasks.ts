import type { ApiResponse } from './types'
import { request } from './http'

// ── Install Tasks ──
export interface InstallTask {
  id?: string
  host_id: string
  distro_name: string
  version_codename: string
  arch?: string
  answer_template_id?: number | null
  extra_cmdline?: string
  status?: string
  error_msg?: string
  created_at?: string
  updated_at?: string
}

export function getInstallTasks(): Promise<ApiResponse<{ tasks: InstallTask[] }>> {
  return request('GET', '/netboot/tasks')
}

export function createInstallTask(task: InstallTask): Promise<ApiResponse<InstallTask>> {
  return request('POST', '/netboot/tasks', task)
}

export function getInstallTask(id: string): Promise<ApiResponse<InstallTask>> {
  return request('GET', `/netboot/tasks/${encodeURIComponent(id)}`)
}

export function updateInstallTask(id: string, task: InstallTask): Promise<ApiResponse<InstallTask>> {
  return request('PUT', `/netboot/tasks/${encodeURIComponent(id)}`, task)
}

export function deleteInstallTask(id: string): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/netboot/tasks/${encodeURIComponent(id)}`)
}
