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
  baselines?: string[]
  variables?: Record<string, string>
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

// ── Version / Update ──

export interface ReleaseInfo {
	latest_version: string
	release_date: string
	release_notes_url: string
	download_url: string
	checksums_url: string
	min_upgrade_version: string
}

export interface CheckResult {
	current_version: string
	latest_version: string
	update_available: boolean
	release_info?: ReleaseInfo
	checked_at: string
	error?: string
}

export interface VersionInfo {
	current_version: string
	check?: CheckResult
}

export interface DownloadResult {
	file_path: string
	file_name: string
	file_size: number
	version: string
}
