import type { ApiResponse, ServiceStatus } from './types'
import { request } from './http'

// ── Status ──
export function getStatus(): Promise<ApiResponse<ServiceStatus>> {
  return request<ServiceStatus>('GET', '/status')
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

// ── Port Check ──
export interface PortProcess {
  pid: number
  name: string
  path?: string
}

export interface PortCheckResult {
  port: number
  protocol: string
  occupied: boolean
  processes: PortProcess[]
  descriptor: string
}

export function portCheck(port: number, protocol: string): Promise<ApiResponse<PortCheckResult>> {
  return request<PortCheckResult>('GET', `/services/port-check?port=${port}&protocol=${encodeURIComponent(protocol)}`)
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

export function getMetrics(): Promise<ApiResponse<MetricsSnapshot>> {
  return request<MetricsSnapshot>('GET', '/metrics')
}
