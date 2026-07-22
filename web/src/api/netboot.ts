import type { ApiResponse } from './types'
import { request } from './http'

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
