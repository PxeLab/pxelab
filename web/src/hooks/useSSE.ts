import { useEffect, useRef, useState, useCallback } from 'react'
import type { Event } from '../api/client'

interface UseSSEReturn {
  connected: boolean
  pause: () => void
  resume: () => void
  paused: boolean
}

export function useSSE(url: string, onEvent: (data: Event) => void): UseSSEReturn {
  const [connected, setConnected] = useState(false)
  const [paused, setPaused] = useState(false)
  const bufferRef = useRef<Event[]>([])
  const esRef = useRef<EventSource | null>(null)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(undefined)
  const onEventRef = useRef(onEvent)
  const pausedRef = useRef(paused)

  onEventRef.current = onEvent
  pausedRef.current = paused

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
          const data: Event = JSON.parse(e.data)
          if (pausedRef.current) {
            bufferRef.current.push(data)
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
        reconnectTimerRef.current = reconnectTimer
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
