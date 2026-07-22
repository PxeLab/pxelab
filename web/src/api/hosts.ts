import type { ApiResponse, Host } from './types'
import { buildQuery, clearSession, getBaseURL, getSessionToken, request } from './http'

// ── Hosts ──
export function getHosts(params?: Record<string, unknown>): Promise<ApiResponse<{ hosts: Host[]; meta: { page: number; size: number; total: number } }>> {
  return request('GET', '/hosts' + buildQuery(params))
}

export function getHost(id: string): Promise<ApiResponse<Host>> {
  return request<Host>('GET', '/hosts/' + encodeURIComponent(id))
}

export function createHost(data: Partial<Host>): Promise<ApiResponse<Host>> {
  return request<Host>('POST', '/hosts', data)
}

export function updateHost(id: string, data: Partial<Host>): Promise<ApiResponse<Host>> {
  return request<Host>('PUT', '/hosts/' + encodeURIComponent(id), data)
}

export function deleteHost(id: string): Promise<ApiResponse<void>> {
  return request<void>('DELETE', '/hosts/' + encodeURIComponent(id))
}

export function wakeHost(id: string): Promise<ApiResponse<unknown>> {
  return request<unknown>('POST', '/hosts/' + encodeURIComponent(id) + '/wake')
}

export function powerHost(id: string, action: string): Promise<ApiResponse<unknown>> {
  return request<unknown>('POST', '/hosts/' + encodeURIComponent(id) + '/power', { action })
}
export async function getHostBootConfig(id: string, format: string): Promise<string> {
  const session = getSessionToken()
  const headers: Record<string, string> = {}
  if (session) {
    headers["Authorization"] = "Bearer " + session
  }
  const res = await fetch(getBaseURL() + "/api/v1/hosts/" + encodeURIComponent(id) + "/boot-config?format=" + encodeURIComponent(format), { headers })
  if (res.status === 401) {
    clearSession()
    window.location.href = "/login"
    throw new Error("Session expired")
  }
  if (!res.ok) {
    const json = await res.json().catch(() => ({}))
    throw new Error(json.error || `Failed to fetch boot config (HTTP ${res.status})`)
  }
  return res.text()
}
