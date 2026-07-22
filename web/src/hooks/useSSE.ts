import { useEffect, useRef, useState, useCallback } from 'react'

interface UseSSEOptions {
  /** 暂停时缓冲区最多保留的最新事件数，超出丢弃最旧的；默认无上限 */
  bufferLimit?: number
}

interface UseSSEReturn {
  connected: boolean
  pause: () => void
  resume: () => void
  paused: boolean
}

export function useSSE<T>(url: string, onEvent: (data: T) => void, options?: UseSSEOptions): UseSSEReturn {
  const [connected, setConnected] = useState(false)
  const [paused, setPaused] = useState(false)
  const bufferRef = useRef<T[]>([])
  const esRef = useRef<EventSource | null>(null)
  const onEventRef = useRef(onEvent)
  const pausedRef = useRef(paused)
  const optionsRef = useRef(options)

  onEventRef.current = onEvent
  pausedRef.current = paused
  optionsRef.current = options

  useEffect(() => {
    let reconnectTimer: ReturnType<typeof setTimeout> | undefined

    function connect() {
      if (esRef.current) {
        esRef.current.close()
      }

      const es = new EventSource(url)
      esRef.current = es

      es.onopen = () => {
        setConnected(true)
      }

      es.onmessage = (e: MessageEvent) => {
        try {
          const data: T = JSON.parse(e.data)
          if (pausedRef.current) {
            const limit = optionsRef.current?.bufferLimit
            bufferRef.current.push(data)
            if (limit !== undefined && bufferRef.current.length > limit) {
              bufferRef.current.splice(0, bufferRef.current.length - limit)
            }
          } else {
            onEventRef.current(data)
          }
        } catch {
          // skip malformed events
        }
      }

      es.onerror = () => {
        setConnected(false)
        es.close()
        reconnectTimer = setTimeout(connect, 3000)
      }
    }

    connect()

    return () => {
      esRef.current?.close()
      esRef.current = null
      clearTimeout(reconnectTimer)
    }
  }, [url])

  const pause = useCallback(() => {
    setPaused(true)
  }, [])

  const resume = useCallback(() => {
    setPaused(false)
    const buffer = bufferRef.current
    bufferRef.current = []
    for (const event of buffer) {
      onEventRef.current(event)
    }
  }, [])

  return { connected, pause, resume, paused }
}
