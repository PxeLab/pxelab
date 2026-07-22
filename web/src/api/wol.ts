import type { ApiResponse } from './types'
import { request } from './http'

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
