import type { ApiResponse, MenuEntry } from './types'
import { request } from './http'

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
