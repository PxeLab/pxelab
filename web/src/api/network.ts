import type { ApiResponse } from './types'
import { getBaseURL, getSessionToken, request } from './http'

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
