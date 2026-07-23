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

// postSSE 以 POST + fetch streaming 实现 SSE（EventSource 不支持 POST）
// onError：网络失败或非 2xx 响应时回调（正常流结束不触发）
function postSSE(url: string, body: unknown, onData: (data: any) => void, onError?: (err: Error) => void): EventSource {
  const session = getSessionToken()
  const controller = new AbortController()

  const reportError = (err: unknown) => {
    if (controller.signal.aborted) return
    onError?.(err instanceof Error ? err : new Error(String(err)))
  }

  fetch(url, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      ...(session ? { 'Authorization': 'Bearer ' + session } : {}),
    },
    body: JSON.stringify(body),
    signal: controller.signal,
  }).then(async (res) => {
    if (!res.ok) {
      let msg = `HTTP ${res.status}`
      try {
        const j = await res.json()
        if (j?.message) msg = j.message
      } catch {}
      reportError(new Error(msg))
      return
    }
    const reader = res.body?.getReader()
    const decoder = new TextDecoder()
    if (!reader) {
      reportError(new Error('empty response body'))
      return
    }

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
            onData(JSON.parse(line.slice(6)))
          } catch {}
        }
      }
    }
  }).catch(reportError)

  // Return a fake EventSource-like object with abort capability
  return { close: () => controller.abort() } as unknown as EventSource
}

export interface StreamError {
  type: 'error'
  message: string
}

export function networkPingStream(
  options: PingOptions,
  onPacket: (pkt: PingPacket | { type: string; sent: number; received: number; lost: number; min_rtt: number; max_rtt: number; avg_rtt: number } | StreamError) => void,
  onError?: (err: Error) => void,
): EventSource {
  return postSSE(getBaseURL() + '/api/v1/network/ping/stream', options, onPacket, onError)
}

export interface TracerouteSummary {
  type: 'summary'
  host: string
  ip: string
  hops: number
}

export function networkTracerouteStream(options: TracerouteOptions, onData: (data: TracerouteHop | TracerouteSummary | StreamError) => void, onError?: (err: Error) => void): EventSource {
  return postSSE(getBaseURL() + '/api/v1/network/traceroute/stream', options, onData, onError)
}

export function networkTraceroute(options: TracerouteOptions): Promise<ApiResponse<TracerouteResult>> {
  return request<TracerouteResult>('POST', '/network/traceroute', options)
}

export function getNetworkInterfaces(): Promise<ApiResponse<NetworkInterface[]>> {
  return request<NetworkInterface[]>('GET', '/network/interfaces')
}
