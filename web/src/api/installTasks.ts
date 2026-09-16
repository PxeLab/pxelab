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
  batch_id?: string
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

// ── R7 批量装机 / 批次操作 / 失败解锁重试 ──

export interface BatchHostInput {
  mac: string
  name?: string
  sn?: string
  ip?: string
}

export interface BatchCreateInput {
  hosts: BatchHostInput[]
  distro_name: string
  version_codename: string
  arch?: string
  answer_template_id?: number | null
  extra_cmdline?: string
  profile_id?: string
}

export interface BatchSkipped {
  mac: string
  reason: string
}

export interface BatchCreateResult {
  batch_id: string
  tasks: InstallTask[]
  skipped: BatchSkipped[]
}

export function createInstallTaskBatch(data: BatchCreateInput): Promise<ApiResponse<BatchCreateResult>> {
  return request('POST', '/install-tasks/batch', data)
}

export function cancelInstallTaskBatch(batchId: string): Promise<ApiResponse<{ deleted: number; cancelled: number }>> {
  return request('POST', `/install-tasks/batch/${encodeURIComponent(batchId)}/cancel`)
}

export function retryFailedInstallTaskBatch(batchId: string): Promise<ApiResponse<{ retried: number }>> {
  return request('POST', `/install-tasks/batch/${encodeURIComponent(batchId)}/retry-failed`)
}

export function retryInstallTask(id: string): Promise<ApiResponse<InstallTask>> {
  return request('POST', `/install-tasks/${encodeURIComponent(id)}/retry`)
}
