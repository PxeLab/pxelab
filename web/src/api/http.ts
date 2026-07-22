import type { ApiResponse } from './types'

let baseURL = ''

// ── Session token management ──

function loadSession(): string | null {
  try {
    return localStorage.getItem('PxeLab_session')
  } catch {
    return null
  }
}

function saveSession(token: string) {
  sessionToken = token
  try {
    localStorage.setItem('PxeLab_session', token)
  } catch {}
}

let sessionToken: string | null = loadSession()

export function setSessionToken(token: string) {
  saveSession(token)
}

export function clearSession() {
  sessionToken = null
  try {
    localStorage.removeItem('PxeLab_session')
    localStorage.removeItem('PxeLab_auth_token')
  } catch {}
}

export function getSessionToken(): string | null {
  return sessionToken || loadSession()
}

// ── Login / Logout ──

export async function login(masterToken: string): Promise<string> {
  const res = await fetch(baseURL + '/api/v1/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ token: masterToken }),
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: '登录失败' }))
    throw new Error(err.error || '登录失败')
  }
  const json = await res.json()
  const sessionToken = json.data?.session_token
  if (!sessionToken) throw new Error('登录失败：未获取到会话令牌')
  saveSession(sessionToken)
  return sessionToken
}

export async function logout() {
  const session = getSessionToken()
  if (session) {
    try {
      await fetch(baseURL + '/api/v1/auth/logout', {
        method: 'POST',
        headers: { 'Authorization': 'Bearer ' + session },
      })
    } catch {}
  }
  clearSession()
}

export async function checkSession(): Promise<boolean> {
  const session = getSessionToken()
  if (!session) return false
  try {
    const res = await fetch(baseURL + '/api/v1/auth/session', {
      headers: { 'Authorization': 'Bearer ' + session },
    })
    const data = await res.json()
    return data.data?.valid === true
  } catch {
    return false
  }
}

export function setBaseURL(url: string) {
  baseURL = url
}

export function getBaseURL(): string {
  return baseURL
}

export function buildQuery(params?: Record<string, unknown>): string {
  if (!params) return ''
  const sp = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      sp.set(key, String(value))
    }
  }
  const qs = sp.toString()
  return qs ? '?' + qs : ''
}

export async function request<T>(method: string, path: string, body?: unknown): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = {}
  const session = getSessionToken()
  if (session) {
    headers['Authorization'] = 'Bearer ' + session
  }
  const opts: RequestInit = { method, headers }
  if (body instanceof FormData) {
    opts.body = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(baseURL + '/api/v1' + path, opts)
  if (res.status === 401) {
    clearSession()
    window.location.href = '/login'
    throw new Error('会话已过期，请重新登录')
  }
  if (res.status === 204) return { success: true } as ApiResponse<T>
  const json = await res.json()
  if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`)
  return json
}
