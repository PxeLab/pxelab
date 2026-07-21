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
  type: 'local' | 'direct' | 'chain' | 'sanboot' | 'wds' | 'custom'
  kernel?: string
  initrd?: string
  cmdline?: string
  url?: string
  wim?: string
  script?: string
  is_default?: boolean
  san_action?: 'boot' | 'hook' | 'zap' | 'unhook'
  san_no_describe?: boolean
  san_drive?: string
  san_keep_san?: boolean
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

export interface AuditLog {
  id: string
  action: string
  resource: string
  resource_id: string
  remote_ip: string
  detail: string
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
  md5: string
}

export interface DNSRecord {
  id?: number
  name: string
  type: string
  value: string
  ttl: number
  enabled: boolean
  subnet?: string
  created_at?: string
  updated_at?: string
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
export async function getHostBootConfig(id: string, format: string): Promise<string> {
  const session = getSessionToken()
  const headers: Record<string, string> = {}
  if (session) {
    headers["Authorization"] = "Bearer " + session
  }
  const res = await fetch(baseURL + "/api/v1/hosts/" + encodeURIComponent(id) + "/boot-config?format=" + encodeURIComponent(format), { headers })
  if (res.status === 401) {
    clearSession()
    window.location.href = "/login"
    throw new Error("会话已过期，请重新登录")
  }
  if (!res.ok) {
    const json = await res.json().catch(() => ({}))
    throw new Error(json.error || "获取引导配置失败")
  }
  return res.text()
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

// ── Profiles from Netboot Catalog ──
export interface CreateProfileFromNetbootData {
  distro_name: string
  version_codename: string
  arch?: string
  profile_name: string
  description?: string
}

export function createProfileFromNetboot(data: CreateProfileFromNetbootData): Promise<ApiResponse<Profile>> {
  return request<Profile>('POST', '/profiles/from-netboot', data)
}

// ── Events ──
export function getEvents(params?: Record<string, unknown>): Promise<ApiResponse<{ events: Event[]; meta: { page: number; size: number; total: number } }>> {
  return request('GET', '/events' + buildQuery(params))
}

// ── Audit Logs ──
export function getAuditLogs(params?: Record<string, unknown>): Promise<ApiResponse<{ logs: AuditLog[]; meta: { page: number; size: number; total: number } }>> {
  return request('GET', '/audit-logs' + buildQuery(params))
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

export function getBootRootDir(): Promise<ApiResponse<{ root_dir: string }>> {
  return request<{ root_dir: string }>('GET', '/files/root')
}

// ── Leases ──
export function getLeases(): Promise<ApiResponse<Lease[]>> {
  return request<Lease[]>('GET', '/leases')
}

export interface LeaseStats {
  subnet_id: string
  dhcp_mode: string
  pool_size: number
  allocated: number
  available: number
  usage_pct: number
  active_only: number
}

export function getLeaseStats(): Promise<ApiResponse<LeaseStats[]>> {
  return request<LeaseStats[]>('GET', '/leases/stats')
}

export function deleteLease(mac: string): Promise<ApiResponse<{ message: string }>> {
  return request('DELETE', `/leases/${encodeURIComponent(mac)}`)
}

export function batchDeleteLeases(macs: string[]): Promise<ApiResponse<{ message: string; success_count: number; failed_count: number; failed_macs?: string[] }>> {
  return request('POST', '/leases/batch-delete', { macs })
}

export function pruneLeases(): Promise<ApiResponse<{ message: string }>> {
  return request('POST', '/leases/prune')
}

// ── Settings ──
export interface SubnetSettings {
  cidr: string
  dhcp_mode: string
  pools: string[]
  gateway: string
  dns_servers: string
  lease_time: number
  next_server: string
  chain_to_ipxe: boolean
}

export interface InterfaceSettings {
  name: string
  ip: string
  bootloader: string
  subnets: SubnetSettings[]
  // backward compat fields (deprecated, use subnets)
  subnet: string
  pools: string[]
  gateway: string
  dns_servers: string
  lease_time: number
  next_server: string
  tftp?: boolean
  http?: boolean
  dns?: boolean
}

export interface BootSettings {
  default_menu: {
    title: string
    timeout: number
    default: number
    list_all_profiles: boolean
    entries: MenuEntry[]
  }
  catalog_redirect: {
    enabled: boolean
    target_url: string
    detect_arch: boolean
    preamble: string
  }
  catalog_display: {
    title: string
  }
}

export interface SettingsData {
  server: { name: string; app_mode: boolean; token: string; listen_addr: string; token_set: boolean }
  dhcp: { enabled: boolean; range: string; gateway: string; subnet: string; lease_time: number; dns_servers: string }
  tftp: { enabled: boolean; port: number; root: string }
  dns: { enabled: boolean; port: number; upstream: string; local_domain?: string; default_record?: boolean }
  ipmi?: { enabled: boolean; timeout: number }
  netboot: { enabled: boolean; script_template?: string; boot: BootSettings }
  whitelist_enabled: boolean
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

// ── Settings sub-endpoints ──

export interface GeneralSettings {
  server_name: string
  app_mode: boolean
  token: string
  token_set: boolean
  listen_addr: string
  log_level: string
  data_dir: string
  whitelist_enabled: boolean
  script_template: string
  default_menu: BootSettings['default_menu']
  page_size: number
  migrate_boot?: boolean
}

export interface InterfaceSettingsList {
  interfaces: InterfaceSettings[]
}

export interface DHCPSettingsData {
  enabled: boolean
  range: string
  gateway: string
  subnet: string
  lease_time: number
  dns_servers: string
}

export interface TFTPSettingsData {
  enabled: boolean
  port: number
  timeout: number
  root: string
  pxe_config_file: string
  grub_config_file: string
}

export interface DNSSettingsData {
  enabled: boolean
  port: number
  upstream: string
  local_domain?: string
  default_record?: boolean
}

export interface NFSClientInfo {
  ip: string
  connected_at: string
  last_activity: string
}

export interface NFSMountPointData {
  label: string
  export_path: string
  local_dir: string
  read_only: boolean
  allow_ips: string[]
  connection_count: number
  clients?: NFSClientInfo[]
}

export interface NFSSettingsData {
  enabled: boolean
  running: boolean
  port: number
  rpcbind_port: number
  version: string
  mount_points: NFSMountPointData[]
}

export interface NetbootSettingsData {
  enabled: boolean
  proxy_https: boolean
  cache_enabled: boolean
  catalog_redirect: BootSettings['catalog_redirect']
  catalog_display: BootSettings['catalog_display']
}

export interface LoggingSettings {
  max_size_mb: number
  max_backups: number
  max_age_days: number
  compress: boolean
  cleanup_interval: number
}

export interface LogFileInfo {
  name: string
  size: number
  mod_time: string
}

export interface LogDiskUsage {
  dir: string
  size_bytes: number
}

export interface CacheStats {
  path: string
  size_bytes: number
  file_count: number
}

export function getCacheStats(): Promise<ApiResponse<CacheStats>> {
  return request<CacheStats>('GET', '/netboot/cache-stats')
}

export function getGeneralSettings(): Promise<ApiResponse<GeneralSettings>> {
  return request<GeneralSettings>('GET', '/settings/general')
}

export function updateGeneralSettings(data: GeneralSettings): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/general', data)
}

export function getInterfaceSettings(): Promise<ApiResponse<InterfaceSettingsList>> {
  return request<InterfaceSettingsList>('GET', '/settings/interfaces')
}

export function updateInterfaceSettings(data: InterfaceSettingsList): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/interfaces', data)
}

export function getDHCPSettings(): Promise<ApiResponse<DHCPSettingsData>> {
  return request<DHCPSettingsData>('GET', '/services/dhcp')
}

export function updateDHCPSettings(data: DHCPSettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/dhcp', data)
}

export function getTFTPSettings(): Promise<ApiResponse<TFTPSettingsData>> {
  return request<TFTPSettingsData>('GET', '/services/tftp')
}

export function updateTFTPSettings(data: TFTPSettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/tftp', data)
}

// ── ArchMap ──
export interface ArchEntryData {
  arch_code: number
  arch_name: string
  nbp: string         // "ipxe" | "pxelinux" | "grub2"
  chain_load: boolean // 是否链式加载到 iPXE
  ipxe: string
  pxelinux: string
  grub: string
  grub_config: string

  // Secure Boot 支持
  secure_boot: boolean
  ipxe_sb: string
  shim_ipxe: string
  grub_sb: string
  shim_grub: string
}

export interface ArchMapData {
  entries: ArchEntryData[]
}

export function getArchMap(): Promise<ApiResponse<ArchMapData>> {
  return request<ArchMapData>('GET', '/services/archmap')
}

export function updateArchMap(data: ArchMapData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/archmap', data)
}

export function getArchMapDefaults(): Promise<ApiResponse<ArchMapData>> {
  return request<ArchMapData>('GET', '/services/archmap/defaults')
}

// ── iPXE Script (DHCP Option 175) ──

export interface IPXEScriptSettings {
  enabled: boolean
  port: number
  path: string
  feature_flags: number
}

export function getIPXEScript(): Promise<ApiResponse<IPXEScriptSettings>> {
  return request<IPXEScriptSettings>('GET', '/services/ipxe-script')
}

export function updateIPXEScript(data: IPXEScriptSettings): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/ipxe-script', data)
}

export function getDNSSettings(): Promise<ApiResponse<DNSSettingsData>> {
  return request<DNSSettingsData>('GET', '/services/dns')
}

export function updateDNSSettings(data: DNSSettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/dns', data)
}

export function getNFSSettings(): Promise<ApiResponse<NFSSettingsData>> {
  return request<NFSSettingsData>('GET', '/services/nfs')
}

export function updateNFSSettings(data: NFSSettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/services/nfs', data)
}

export function validateNFSPath(path: string): Promise<ApiResponse<{ exists: boolean; writable: boolean; is_dir?: boolean; error?: string }>> {
  return request<{ exists: boolean; writable: boolean; is_dir?: boolean; error?: string }>('POST', '/services/nfs/validate-path', { path })
}

export interface BrowseNFSPathEntry {
  name: string
  path: string
  is_dir: boolean
}

export function browseNFSPath(path: string): Promise<ApiResponse<{ current: string; entries: BrowseNFSPathEntry[] }>> {
  return request<{ current: string; entries: BrowseNFSPathEntry[] }>('GET', `/services/nfs/browse-path?path=${encodeURIComponent(path)}`)
}

export function getNetbootSettings(): Promise<ApiResponse<NetbootSettingsData>> {
  return request<NetbootSettingsData>('GET', '/settings/netboot')
}

export function updateNetbootSettings(data: NetbootSettingsData): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/netboot', data)
}

// ── Logging Settings ──
export function getLoggingSettings(): Promise<ApiResponse<LoggingSettings>> {
  return request<LoggingSettings>('GET', '/settings/logging')
}

export function updateLoggingSettings(data: LoggingSettings): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/logging', data)
}

export function getLogFiles(): Promise<ApiResponse<{ files: LogFileInfo[]; dir: string }>> {
  return request('GET', '/logs/files')
}

export function getLogDiskUsage(): Promise<ApiResponse<LogDiskUsage>> {
  return request<LogDiskUsage>('GET', '/logs/disk-usage')
}

export function cleanupLogs(maxAgeDays?: number, maxBackups?: number): Promise<ApiResponse<{ removed: number }>> {
  return request('POST', '/logs/cleanup', { max_age_days: maxAgeDays || 30, max_backups: maxBackups || 5 })
}

// ── Netboot Catalog ──
export interface NetbootDistro {
  name: string
  enabled: boolean
  website?: string
  menu_group: string
  logo?: string
  mirror?: string
  kernel_params?: string
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
  boot_type?: string
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

// ── Netboot Overlays ──
export interface VersionOverride {
  codename: string
  arch: string
  enabled?: boolean
  remote_kernel?: string
  remote_initrd?: string
  cmdline?: string
  answer_param?: string
  answer_type?: string
}

export interface NetbootOverlay {
  id?: number
  distro_name: string
  enabled: boolean
  mirror?: string
  local_base?: string
  kernel_params?: string
  version_overrides?: VersionOverride[]
  created_at?: string
  updated_at?: string
}

export function getNetbootOverlays(): Promise<ApiResponse<{ overlays: NetbootOverlay[] }>> {
  return request('GET', '/netboot/overlays')
}

export function getNetbootOverlay(distro: string): Promise<ApiResponse<NetbootOverlay>> {
  return request('GET', `/netboot/overlays/${encodeURIComponent(distro)}`)
}

export function upsertNetbootOverlay(distro: string, overlay: NetbootOverlay): Promise<ApiResponse<NetbootOverlay>> {
  return request('PUT', `/netboot/overlays/${encodeURIComponent(distro)}`, overlay)
}

export function deleteNetbootOverlay(distro: string): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/netboot/overlays/${encodeURIComponent(distro)}`)
}

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

export function getDNSRecords(): Promise<ApiResponse<{ records: DNSRecord[]; local_domain?: string }>> {
  return request('GET', '/dns/records')
}

export function createDNSRecord(r: DNSRecord): Promise<ApiResponse<DNSRecord>> {
  return request('POST', '/dns/records', r)
}

export function getDNSRecord(id: number): Promise<ApiResponse<DNSRecord>> {
  return request('GET', `/dns/records/${id}`)
}

export function updateDNSRecord(id: number, r: DNSRecord): Promise<ApiResponse<DNSRecord>> {
  return request('PUT', `/dns/records/${id}`, r)
}

export function deleteDNSRecord(id: number): Promise<ApiResponse<unknown>> {
  return request('DELETE', `/dns/records/${id}`)
}

// ── Access Control: Blacklist / Whitelist ──
export interface BlacklistEntry {
  id: number
  mac: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export interface WhitelistEntry {
  id: number
  mac: string
  subnet_cidr: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export function getBlacklist(): Promise<ApiResponse<BlacklistEntry[]>> {
  return request<BlacklistEntry[]>('GET', '/access/blacklist')
}

export function createBlacklistEntry(data: { mac: string; reason?: string }): Promise<ApiResponse<BlacklistEntry>> {
  return request<BlacklistEntry>('POST', '/access/blacklist', data)
}

export function deleteBlacklistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/blacklist/${id}`)
}

export function getWhitelist(): Promise<ApiResponse<WhitelistEntry[]>> {
  return request<WhitelistEntry[]>('GET', '/access/whitelist')
}

export function createWhitelistEntry(data: { mac: string; subnet_cidr?: string; reason?: string }): Promise<ApiResponse<WhitelistEntry>> {
  return request<WhitelistEntry>('POST', '/access/whitelist', data)
}

export function deleteWhitelistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/whitelist/${id}`)
}

// ── Unauthorized Devices ──
export interface UnauthorizedDevice {
  id: number
  mac: string
  subnet_cidr: string
  reason: string
  count: number
  last_seen: string
  created_at: string
  updated_at: string
}

export function getUnauthorizedDevices(): Promise<ApiResponse<UnauthorizedDevice[]>> {
  return request<UnauthorizedDevice[]>('GET', '/access/unauthorized')
}

export function addUnauthorizedToWhitelist(mac: string, subnet_cidr: string): Promise<ApiResponse<WhitelistEntry>> {
  return request<WhitelistEntry>('POST', '/access/unauthorized/add-to-whitelist', { mac, subnet_cidr })
}

export function addUnauthorizedToBlacklist(mac: string, subnet_cidr: string): Promise<ApiResponse<BlacklistEntry>> {
  return request<BlacklistEntry>('POST', '/access/unauthorized/add-to-blacklist', { mac, subnet_cidr })
}

export function deleteUnauthorizedDevice(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/unauthorized/${id}`)
}

// ── BMC / OOB ──
export interface BMCConfig {
  id?: number
  host: string
  port: number
  username: string
  password?: string
  protocol: string   // "ipmi" | "redfish"
  vendor: string
  model: string
  serial: string
  mac: string
  name: string
  boot_mode: string  // "auto" | "uefi" | "legacy"
  next_boot_device: string  // "pxe" | "disk" | "cdrom" | "bios"
  has_password?: boolean
  created_at?: string
  updated_at?: string
}

export interface BMCProbeResult {
  name: string
  vendor: string
  model: string
  serial: string
  mac: string
}

export function getBMCConfigs(): Promise<ApiResponse<BMCConfig[]>> {
  return request<BMCConfig[]>('GET', '/bmc/configs')
}

export function getBMCConfig(id: number): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('GET', `/bmc/configs/${id}`)
}

export function createBMCConfig(data: Partial<BMCConfig>): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('POST', '/bmc/configs', data)
}

export function updateBMCConfig(id: number, data: Partial<BMCConfig>): Promise<ApiResponse<BMCConfig>> {
  return request<BMCConfig>('PUT', `/bmc/configs/${id}`, data)
}

export function deleteBMCConfig(id: number): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/bmc/configs/${id}`)
}

export function probeBMC(data: { host: string; port: number; username: string; password: string; protocol?: string }): Promise<ApiResponse<BMCProbeResult>> {
  return request<BMCProbeResult>('POST', '/bmc/probe', data)
}

export function refreshBMCConfig(id: number): Promise<ApiResponse<{ refreshed: boolean; result?: BMCProbeResult; error?: string }>> {
  return request('POST', `/bmc/${id}/refresh`)
}

export function bmcPowerOn(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/power-on`)
}

export function bmcPowerOff(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/power-off`)
}

export function bmcRestart(id: number): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/restart`)
}

export function bmcPowerStatus(id: number): Promise<ApiResponse<{ config_id: number; status: string }>> {
  return request('GET', `/bmc/${id}/status`)
}

export function bmcSetBootDevice(id: number, device: string): Promise<ApiResponse<{ status: string }>> {
  return request('POST', `/bmc/${id}/boot-device`, { device })
}

export function bmcBatchPowerOn(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/power-on', { ids })
}

export function bmcBatchPowerOff(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/power-off', { ids })
}

export function bmcBatchRestart(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; status?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/restart', { ids })
}

export function bmcBatchStatus(ids: number[]): Promise<ApiResponse<Array<{ config_id: number; state?: string; error?: string }>>> {
  return request('POST', '/bmc/batch/status', { ids })
}

export function bmcImportCSV(csv: string): Promise<ApiResponse<{ success: number; failed: number; errors: string[] }>> {
  return request('POST', '/bmc/configs/import', { csv })
}

// ── DHCP Reservations ──
export interface DHCPReservation {
  id?: number
  interface_name: string
  subnet_cidr: string
  mac: string
  ip: string
  hostname: string
  description: string
  created_at?: string
  updated_at?: string
}

export function getDHCPReservations(subnetCIDR?: string): Promise<ApiResponse<{ reservations: DHCPReservation[] }>> {
  const q = subnetCIDR ? '?subnet_cidr=' + encodeURIComponent(subnetCIDR) : ''
  return request('GET', '/dhcp/reservations' + q)
}

export function getDHCPReservation(id: number): Promise<ApiResponse<DHCPReservation>> {
  return request<DHCPReservation>('GET', `/dhcp/reservations/${id}`)
}

export function createDHCPReservation(data: Partial<DHCPReservation>): Promise<ApiResponse<DHCPReservation>> {
  return request<DHCPReservation>('POST', '/dhcp/reservations', data)
}

export function updateDHCPReservation(id: number, data: Partial<DHCPReservation>): Promise<ApiResponse<DHCPReservation>> {
  return request<DHCPReservation>('PUT', `/dhcp/reservations/${id}`, data)
}

export function deleteDHCPReservation(id: number): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/dhcp/reservations/${id}`)
}

// ── Services ──
export interface ServiceInfo {
  name: string
  display: string
  status: 'running' | 'stopped' | 'error'
  auto_start: boolean
  protected: boolean
  port: number
  protocol: string
  error_msg: string
  started_at: string | null
}

export function getServices(): Promise<ApiResponse<ServiceInfo[]>> {
  return request<ServiceInfo[]>('GET', '/services')
}

export function startService(name: string): Promise<ApiResponse<ServiceInfo>> {
  return request<ServiceInfo>('POST', `/services/${encodeURIComponent(name)}/start`)
}

export function stopService(name: string): Promise<ApiResponse<ServiceInfo>> {
  return request<ServiceInfo>('POST', `/services/${encodeURIComponent(name)}/stop`)
}

export function restartService(name: string): Promise<ApiResponse<ServiceInfo>> {
  return request<ServiceInfo>('POST', `/services/${encodeURIComponent(name)}/restart`)
}

export function batchService(action: string, names: string[]): Promise<ApiResponse<{ success: boolean; result: Record<string, string> }>> {
  return request('POST', `/services/batch/${action}`, { names })
}

export function updateAutoStart(name: string, enabled: boolean): Promise<ApiResponse<ServiceInfo>> {
  return request<ServiceInfo>('PUT', `/services/${encodeURIComponent(name)}/auto-start`, { enabled })
}

// ── Metrics ──
export interface TimeBucket {
  t: number
  v: number
}

export interface ServiceMetricSnapshot {
  requests: number
  errors: number
  bytesIn: number
  bytesOut: number
  activeConns: number
  rejected: number
  requestRate: TimeBucket[]
  errorRate: TimeBucket[]
  bandwidth: TimeBucket[]
  latencyMs: TimeBucket[]
}

export interface DHCPMetricExtra {
  offers: number
  acks: number
  naks: number
  declines: number
  discovers: number
  requests: number
  unauthorized: number
  activeLeases: number
  archBreakdown: Record<string, number>
  platformBreakdown: Record<string, number>
}

export interface HTTPMetricExtra {
  status2xx: number
  status3xx: number
  status4xx: number
  status5xx: number
  duration: TimeBucket[]
}

export interface MetricServiceData {
  metrics: ServiceMetricSnapshot
  dhcp?: DHCPMetricExtra
  http?: HTTPMetricExtra
}

export interface MetricsSnapshot {
  services: Record<string, MetricServiceData>
}

// ── WOL ──
export interface WOLHistoryRecord {
  id: number
  mac: string
  host_name: string
  broadcast: string
  source_ip: string
  interface: string
  success: boolean
  error_msg?: string
  created_at: string
}

export interface WOLSchedule {
  id: number
  mac: string
  host_name: string
  schedule_at: string
  cron_expr?: string
  repeat_type: string
  enabled: boolean
  last_run?: string
  created_at: string
  updated_at: string
}

export interface WOLInterface {
  name: string
  ips: string[]
}

export interface BatchWakeResult {
  mac: string
  host_name: string
  success: boolean
  broadcast: string
  error?: string
}

export function batchWakeHosts(req: { ids?: string[]; macs?: string[]; interface?: string; broadcast?: string }): Promise<ApiResponse<{ results: BatchWakeResult[]; success_count: number; total: number }>> {
  return request('POST', '/hosts/batch/wake', req)
}

export function getWOLHistory(page = 1, size = 20): Promise<ApiResponse<{ records: WOLHistoryRecord[]; total: number; page: number; size: number }>> {
  return request('GET', `/wol/history?page=${page}&size=${size}`)
}

export function getWOLHistoryByMAC(mac: string): Promise<ApiResponse<{ records: WOLHistoryRecord[] }>> {
  return request('GET', `/wol/history/${encodeURIComponent(mac)}`)
}

export function deleteWOLHistory(id: number): Promise<ApiResponse<{ message: string }>> {
  return request('DELETE', `/wol/history/${id}`)
}

export function deleteAllWOLHistory(): Promise<ApiResponse<{ message: string }>> {
  return request('DELETE', '/wol/history')
}

export function createWOLSchedule(mac: string, scheduleAt: string, cronExpr?: string, repeatType = 'once', weekday?: number, scheduleTime?: string, customBroadcast?: string): Promise<ApiResponse<WOLSchedule>> {
  return request('POST', '/wol/schedule', { mac, schedule_at: scheduleAt, cron_expr: cronExpr, repeat_type: repeatType, weekday, schedule_time: scheduleTime, custom_broadcast: customBroadcast })
}

export function getWOLSchedules(): Promise<ApiResponse<{ schedules: WOLSchedule[] }>> {
  return request('GET', '/wol/schedules')
}

export function deleteWOLSchedule(id: number): Promise<ApiResponse<{ message: string }>> {
  return request('DELETE', `/wol/schedule/${id}`)
}

export function getWOLInterfaces(): Promise<ApiResponse<WOLInterface[]>> {
  return request('GET', '/wol/interfaces')
}

export function getMetrics(): Promise<ApiResponse<MetricsSnapshot>> {
  return request<MetricsSnapshot>('GET', '/metrics')
}

// ── OS Images ──
export interface OSImage {
  id?: number
  name: string
  filename: string
  size?: number
  distro?: string
  version?: string
  arch?: string
  status?: string
  mount_point?: string
  extracted_to?: string
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

export function deleteOSImage(id: number): Promise<ApiResponse<void>> {
  return request<void>('DELETE', `/os-images/${id}`)
}

export function extractOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/extract`)
}

export function mountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/mount`)
}

export function unmountOSImage(id: number): Promise<ApiResponse<OSImage>> {
  return request<OSImage>('POST', `/os-images/${id}/unmount`)
}

// ── Network Diagnostics ──
export interface PingOptions {
  host: string
  count?: number
  timeout_ms?: number
  interval_ms?: number
  size?: number
  ttl?: number
  interface?: string
}

export interface PingPacket {
  seq: number
  rtt: number
  ttl: number
  bytes: number
  error?: string
}

export interface PingResult {
  host: string
  ip: string
  sent: number
  received: number
  lost: number
  min_rtt: number
  max_rtt: number
  avg_rtt: number
  stddev_rtt: number
  packets: PingPacket[]
  reachable: boolean
  error?: string
}

export interface TracerouteOptions {
  host: string
  max_hops?: number
  timeout_ms?: number
  probes?: number
  interface?: string
}

export interface TracerouteHop {
  ttl: number
  ip: string
  host: string
  rtt: number
  rtts: number[]
  timeout: boolean
  avg_rtt?: number
}

export interface TracerouteResult {
  host: string
  ip: string
  hops: TracerouteHop[]
  error?: string
}

export interface NetworkInterface {
  name: string
  ips: string[]
}

export function networkPing(options: PingOptions): Promise<ApiResponse<PingResult>> {
  return request<PingResult>('POST', '/network/ping', options)
}

export function networkPingStream(options: PingOptions, onPacket: (pkt: PingPacket | { type: string; sent: number; received: number; lost: number; min_rtt: number; max_rtt: number; avg_rtt: number }) => void): EventSource {
  const session = getSessionToken()
  const url = getBaseURL() + '/api/v1/network/ping/stream'
  
  // Use fetch with streaming for POST SSE
  const controller = new AbortController()
  
  fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(session ? { 'Authorization': 'Bearer ' + session } : {}),
    },
    body: JSON.stringify(options),
    signal: controller.signal,
  }).then(async (res) => {
    const reader = res.body?.getReader()
    const decoder = new TextDecoder()
    if (!reader) return
    
    let buffer = ''
    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      
      for (const line of lines) {
        if (line.startsWith('data: ')) {
          try {
            const data = JSON.parse(line.slice(6))
            onPacket(data)
          } catch {}
        }
      }
    }
  }).catch(() => {})
  
  // Return a fake EventSource-like object with abort capability
  return { close: () => controller.abort() } as unknown as EventSource
}

export function networkTraceroute(options: TracerouteOptions): Promise<ApiResponse<TracerouteResult>> {
  return request<TracerouteResult>('POST', '/network/traceroute', options)
}

export function getNetworkInterfaces(): Promise<ApiResponse<NetworkInterface[]>> {
  return request<NetworkInterface[]>('GET', '/network/interfaces')
}

// ── Bootloader Management ──
export interface BootFileInfo {
  name: string
  description: string
  required: boolean
  present: boolean
  size?: number
  checksum?: string
  mod_time?: string
}

export interface BootloaderCheckResult {
  root_dir: string
  total: number
  present: number
  missing: number
  all_ok: boolean
  files: BootFileInfo[]
}

export interface BootDirEntry {
  name: string
  size: number
  mod_time: string
  is_dir: boolean
}

export function getBootloaderCheck(): Promise<ApiResponse<BootloaderCheckResult>> {
  return request<BootloaderCheckResult>('GET', '/bootloader/check')
}

export function getBootloaderFiles(): Promise<ApiResponse<BootDirEntry[]>> {
  return request<BootDirEntry[]>('GET', '/bootloader/files')
}

export function checkBootloaderFile(name: string): Promise<ApiResponse<BootFileInfo>> {
  return request<BootFileInfo>('POST', '/bootloader/check-file', { name })
}

// ── Profile Script Versions ──
export interface ProfileScriptVersion {
  id: number
  profile_id: string
  content: string
  checksum: string
  comment?: string
  created_at: string
}

export function getScriptVersions(profileId: string): Promise<ApiResponse<ProfileScriptVersion[]>> {
  return request<ProfileScriptVersion[]>('GET', `/profiles/${encodeURIComponent(profileId)}/script-versions`)
}

export function getScriptVersion(profileId: string, verId: number): Promise<ApiResponse<ProfileScriptVersion>> {
  return request<ProfileScriptVersion>('GET', `/profiles/${encodeURIComponent(profileId)}/script-versions/${verId}`)
}

export function getScriptDiff(profileId: string, verId: number): Promise<ApiResponse<{ diff: string }>> {
  return request<{ diff: string }>('GET', `/profiles/${encodeURIComponent(profileId)}/script-diff/${verId}`)
}

export function rollbackScriptVersion(profileId: string, verId: number): Promise<ApiResponse<Profile>> {
  return request<Profile>('POST', `/profiles/${encodeURIComponent(profileId)}/script-rollback/${verId}`)
}

// ── Convenience namespace (backward-compat) ──
export const api = {
  getStatus,
  getMetrics,
  getHosts,
  getHost,
  getHostBootConfig,
  batchWakeHosts,
  getWOLHistory,
  getWOLHistoryByMAC,
  deleteWOLHistory,
  deleteAllWOLHistory,
  createWOLSchedule,
  getWOLSchedules,
  deleteWOLSchedule,
  getWOLInterfaces,
  getOSImages,
  getOSImage,
  uploadOSImage,
  deleteOSImage,
  extractOSImage,
  mountOSImage,
  unmountOSImage,
  createHost,
  updateHost,
  deleteHost,
  wakeHost,
  powerHost,
  getProfiles,
  createProfile,
  updateProfile,
  deleteProfile,
  createProfileFromNetboot,
  getEvents,
  getAuditLogs,
  getFiles,
  uploadFile,
  deleteFile,
  getBootRootDir,
  getLeases,
  getLeaseStats,
  deleteLease,
  batchDeleteLeases,
  pruneLeases,
  getSettings,
  updateSettings,
  getInterfaces,
  getNetbootCatalog,
  getNetbootGroups,
  getNetbootDistro,
  getNetbootFileStatus,
  getNetbootOverlays,
  getNetbootOverlay,
  upsertNetbootOverlay,
  deleteNetbootOverlay,
  getAnswerTemplates,
  createAnswerTemplate,
  getAnswerTemplate,
  updateAnswerTemplate,
  deleteAnswerTemplate,
  validateAnswerTemplate,
  validateAnswerTemplateById,
  previewAnswerTemplate,
  getAnswerTemplatePresets,
  getAnswerTemplateVersions,
  getAnswerTemplateVersion,
  rollbackAnswerTemplate,
  getInstallTasks,
  createInstallTask,
  getInstallTask,
  getDNSRecords,
  getDNSRecord,
  createDNSRecord,
  updateDNSRecord,
  deleteDNSRecord,
  updateInstallTask,
  deleteInstallTask,
  getServices,
  startService,
  stopService,
  restartService,
  batchService,
  updateAutoStart,
  getBlacklist,
  createBlacklistEntry,
  deleteBlacklistEntry,
  getWhitelist,
  createWhitelistEntry,
  deleteWhitelistEntry,
  getUnauthorizedDevices,
  addUnauthorizedToWhitelist,
  addUnauthorizedToBlacklist,
  deleteUnauthorizedDevice,
  getGeneralSettings,
  updateGeneralSettings,
  getInterfaceSettings,
  updateInterfaceSettings,
  getDHCPSettings,
  updateDHCPSettings,
  getTFTPSettings,
  updateTFTPSettings,
  getArchMap,
  updateArchMap,
  getArchMapDefaults,
  getIPXEScript,
  updateIPXEScript,
  getDNSSettings,
  updateDNSSettings,
  getNFSSettings,
  updateNFSSettings,
  validateNFSPath,
  browseNFSPath,
  getNetbootSettings,
  updateNetbootSettings,
  getLoggingSettings,
  updateLoggingSettings,
  getLogFiles,
  getLogDiskUsage,
  cleanupLogs,
  getBMCConfigs,
  getBMCConfig,
  createBMCConfig,
  updateBMCConfig,
  deleteBMCConfig,
  probeBMC,
  refreshBMCConfig,
  bmcPowerOn,
  bmcPowerOff,
  bmcRestart,
  bmcPowerStatus,
  bmcSetBootDevice,
  bmcBatchPowerOn,
  bmcBatchPowerOff,
  bmcBatchRestart,
  bmcBatchStatus,
  bmcImportCSV,
  getDHCPReservations,
  getDHCPReservation,
  createDHCPReservation,
  updateDHCPReservation,
  deleteDHCPReservation,
  networkPing,
  networkPingStream,
  networkTraceroute,
  getNetworkInterfaces,
  getBootloaderCheck,
  getBootloaderFiles,
  checkBootloaderFile,
  getScriptVersions,
  getScriptVersion,
  getScriptDiff,
  rollbackScriptVersion,
}
