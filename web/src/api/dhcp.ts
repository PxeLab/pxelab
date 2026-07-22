import type { ApiResponse } from './types'
import { request } from './http'

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
