export interface Host {
  id: string
  name: string
  mac: string
  ip: string
  profile_id?: string
  bmc_addr?: string
  bmc_user?: string
  bmc_pass?: string
  last_online?: string
  boot_count: number
  created_at: string
  updated_at: string
}

export interface Profile {
  id: string
  name: string
  description?: string
  menu: BootMenu
  is_default: boolean
  arch?: string
  created_at: string
  updated_at: string
}

export interface BootMenu {
  entries: MenuEntry[]
}

export interface MenuEntry {
  label: string
  type: 'local' | 'direct' | 'chain' | 'sanboot' | 'wds'
  kernel?: string
  initrd?: string
  cmdline?: string
  url?: string
  wim?: string
}

export interface Event {
  id: string
  type: string
  level: string
  message: string
  mac?: string
  ip?: string
  detail?: Record<string, unknown>
  timestamp: string
}

export interface Lease {
  mac: string
  ip: string
  subnet_id: string
  hostname?: string
  expires_at: string
  created_at: string
}

export interface FileInfo {
  name: string
  size: number
  is_dir: boolean
  modtime: string
}

export interface ApiResponse<T> {
  success: boolean
  data: T
  error?: string
  meta?: { page: number; size: number; total: number }
}

export interface ServiceStatus {
  status: string
  version: string
  uptime: number
  services: Record<string, string>
}

let baseURL = ''

// ── Auth token management ──

function loadToken(): string | null {
  try {
    return localStorage.getItem('pxego_auth_token')
  } catch {
    return null
  }
}

function saveToken(token: string) {
  authToken = token
  try {
    localStorage.setItem('pxego_auth_token', token)
  } catch {}
}

let authToken: string | null = loadToken()

export function setAuthToken(token: string) {
  saveToken(token)
}

export function clearAuthToken() {
  authToken = null
  try {
    localStorage.removeItem('pxego_auth_token')
  } catch {}
}

export function getAuthToken(): string | null {
  return authToken || loadToken()
}

export function setBaseURL(url: string) {
  baseURL = url
}

export function getBaseURL(): string {
  return baseURL
}

function buildQuery(params?: Record<string, unknown>): string {
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

async function request<T>(method: string, path: string, body?: unknown): Promise<ApiResponse<T>> {
  const headers: Record<string, string> = {}
  const token = getAuthToken()
  if (token) {
    headers['Authorization'] = 'Bearer ' + token
  }
  const opts: RequestInit = { method, headers }
  if (body instanceof FormData) {
    opts.body = body
  } else if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
    opts.body = JSON.stringify(body)
  }
  const res = await fetch(baseURL + '/api/v1' + path, opts)
  if (res.status === 204) return { success: true } as ApiResponse<T>
  const json = await res.json()
  if (!res.ok) throw new Error(json.error || `HTTP ${res.status}`)
  return json
}

// ── Status ──
export function getStatus(): Promise<ApiResponse<ServiceStatus>> {
  return request<ServiceStatus>('GET', '/status')
}

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

// ── Profiles ──
export function getProfiles(): Promise<ApiResponse<Profile[]>> {
  return request<Profile[]>('GET', '/profiles')
}

export function createProfile(data: Partial<Profile>): Promise<ApiResponse<Profile>> {
  return request<Profile>('POST', '/profiles', data)
}

export function updateProfile(id: string, data: Partial<Profile>): Promise<ApiResponse<Profile>> {
  return request<Profile>('PUT', '/profiles/' + encodeURIComponent(id), data)
}

export function deleteProfile(id: string): Promise<ApiResponse<void>> {
  return request<void>('DELETE', '/profiles/' + encodeURIComponent(id))
}

// ── Events ──
export function getEvents(params?: Record<string, unknown>): Promise<ApiResponse<{ events: Event[]; meta: { page: number; size: number; total: number } }>> {
  return request('GET', '/events' + buildQuery(params))
}

// ── Files ──
export function getFiles(dir?: string): Promise<ApiResponse<FileInfo[]>> {
  const q = dir ? '?dir=' + encodeURIComponent(dir) : ''
  return request<FileInfo[]>('GET', '/files' + q)
}

export function uploadFile(file: File): Promise<ApiResponse<unknown>> {
  const fd = new FormData()
  fd.append('file', file)
  return request<unknown>('POST', '/files/upload', fd)
}

export function deleteFile(path: string): Promise<ApiResponse<void>> {
  return request<void>('DELETE', '/files?path=' + encodeURIComponent(path))
}

// ── Leases ──
export function getLeases(): Promise<ApiResponse<Lease[]>> {
  return request<Lease[]>('GET', '/leases')
}

// ── Settings ──
export interface InterfaceSettings {
  name: string
  ip: string
  dhcp_mode: string
  bootloader: string
  chain_to_ipxe: boolean
  subnet: string
  pools: string[]
  gateway: string
  dns_servers: string
  lease_time: number
  next_server: string
  tftp: boolean
  http: boolean
  dns: boolean
}

export interface BootSettings {
  default_menu: {
    title: string
    timeout: number
    default: number
    entries: MenuEntry[]
  }
  profile_behavior: {
    append_local: boolean
    append_netboot: boolean
    append_position: 'first' | 'last'
  }
  catalog_redirect: {
    enabled: boolean
    target_url: string
    detect_arch: boolean
    preamble: string
  }
  catalog_display: {
    title: string
    groups: {
      name: string
      title: string
      enabled: boolean
      order: number
    }[]
  }
}

export interface SettingsData {
  server: { name: string; app_mode: boolean; token: string }
  dhcp: { enabled: boolean; range: string; gateway: string; subnet: string; lease_time: number; dns_servers: string }
  tftp: { enabled: boolean; port: number; root: string }
  dns: { enabled: boolean; port: number; upstream: string }
  http: { port: number; boot_dir: string }
  ipmi?: { enabled: boolean; timeout: number }
  netboot: { enabled: boolean; script_template?: string; boot: BootSettings }
  log_level: string
  data_dir: string
  interfaces: InterfaceSettings[]
}

export interface InterfaceInfo {
  name: string
  mac: string
  ipv4: string[]
  ipv6: string[]
  up: boolean
}

export function getInterfaces(): Promise<ApiResponse<InterfaceInfo[]>> {
  return request<InterfaceInfo[]>('GET', '/interfaces')
}

export function getSettings(): Promise<ApiResponse<SettingsData>> {
  return request<SettingsData>('GET', '/settings')
}

export function updateSettings(data: SettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings', data)
}

// ── Netboot Catalog ──
export interface NetbootDistro {
  name: string
  enabled: boolean
  website?: string
  menu_group: string
  logo?: string
  mirror?: string
  versions: NetbootVersion[]
}

export interface NetbootVersion {
  codename: string
  name: string
  arch: string
  enabled: boolean
  local?: { kernel: string; initrd: string }
  remote?: { kernel: string; initrd: string }
  cmdline?: string
  install_type?: string
}

export interface NetbootGroup {
  name: string
  distros: NetbootDistro[]
}

export interface FileStatus {
  distro: string
  version: string
  arch: string
  has_local: boolean
}

export function getNetbootCatalog(): Promise<ApiResponse<{ distros: NetbootDistro[] }>> {
  return request('GET', '/netboot/catalog')
}

export function getNetbootGroups(): Promise<ApiResponse<NetbootGroup[]>> {
  return request('GET', '/netboot/groups')
}

export function getNetbootDistro(name: string): Promise<ApiResponse<NetbootDistro>> {
  return request('GET', `/netboot/catalog/${encodeURIComponent(name)}`)
}

export function getNetbootFileStatus(): Promise<ApiResponse<FileStatus[]>> {
  return request('GET', '/netboot/check-files')
}

// ── Convenience namespace (backward-compat) ──
export const api = {
  getStatus,
  getHosts,
  getHost,
  createHost,
  updateHost,
  deleteHost,
  wakeHost,
  powerHost,
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  getEvents,
  getFiles,
  uploadFile,
  deleteFile,
  getLeases,
  getSettings,
  updateSettings,
  getInterfaces,
  getNetbootCatalog,
  getNetbootGroups,
  getNetbootDistro,
  getNetbootFileStatus,
}
