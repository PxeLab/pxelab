import type { ApiResponse, Profile } from './types'
import { request } from './http'

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
