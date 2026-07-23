import { useState, useEffect, useCallback, useRef } from 'react'
import { api, type Event } from '../api/client'

export interface Notification {
  id: string
  type: 'info' | 'success' | 'warning' | 'error'
  title: string
  message: string
  time: string
  read: boolean
  link?: string
}

const READ_KEY = 'pxelab-notifications-read'
const READ_LIMIT = 200
const POLL_INTERVAL = 30_000
const FETCH_SIZE = 50

// Event.level: ERROR / WARN / INFO（见 internal/models/event.go）
// BOOT 类事件代表一次引导流程完成，归为 success
export function mapSeverity(e: Event): Notification['type'] {
  const level = (e.level || '').toUpperCase()
  if (level === 'ERROR') return 'error'
  if (level === 'WARN') return 'warning'
  if ((e.type || '').toUpperCase() === 'BOOT') return 'success'
  return 'info'
}

// 事件本身不直接带 host_id，优先从 detail.host_id 取；install 类事件跳安装任务页
export function mapLink(e: Event): string | undefined {
  const hostId = e.detail?.host_id
  if (typeof hostId === 'string' && hostId) return `/hosts/${hostId}`
  if ((e.type || '').toLowerCase().includes('install')) return '/install-tasks'
  if ((e.type || '').toUpperCase() === 'OS_IMAGE') return '/os-images'
  return undefined
}

export function eventToNotification(e: Event, readIds: Set<string>): Notification {
  const id = e.id || `${e.timestamp || ''}-${e.message || ''}`
  return {
    id,
    type: mapSeverity(e),
    title: e.type || 'EVENT',
    message: e.message || '',
    time: e.timestamp || '',
    read: readIds.has(id),
    link: mapLink(e),
  }
}

function loadReadIds(): Set<string> {
  try {
    const raw = localStorage.getItem(READ_KEY)
    if (!raw) return new Set()
    const arr: unknown = JSON.parse(raw)
    return new Set(Array.isArray(arr) ? arr.filter((x): x is string => typeof x === 'string') : [])
  } catch {
    return new Set()
  }
}

function saveReadIds(ids: Set<string>) {
  try {
    // 滚动裁剪，最多保留最近 READ_LIMIT 条
    localStorage.setItem(READ_KEY, JSON.stringify([...ids].slice(-READ_LIMIT)))
  } catch {
    // localStorage 不可用时静默降级（已读状态仅保留在内存）
  }
}

export function useNotifications() {
  const [notifications, setNotifications] = useState<Notification[]>([])
  const [readIds, setReadIds] = useState<Set<string>>(loadReadIds)
  const readIdsRef = useRef(readIds)
  readIdsRef.current = readIds
  const notificationsRef = useRef(notifications)
  notificationsRef.current = notifications

  const load = useCallback(async () => {
    // 页面不可见时跳过轮询，回到前台后由 visibilitychange 触发补拉
    if (document.visibilityState !== 'visible') return
    try {
      const res = await api.getEvents({ size: FETCH_SIZE })
      const events = res.data.events || []
      setNotifications(events.map(e => eventToNotification(e, readIdsRef.current)))
    } catch (err) {
      // 轮询失败静默：下一次轮询会重试，铃铛不显示错误态
      console.warn('notifications poll failed:', err)
    }
  }, [])

  useEffect(() => {
    load()
    const iv = setInterval(load, POLL_INTERVAL)
    const onVisible = () => { if (document.visibilityState === 'visible') load() }
    document.addEventListener('visibilitychange', onVisible)
    return () => {
      clearInterval(iv)
      document.removeEventListener('visibilitychange', onVisible)
    }
  }, [load])

  const markRead = useCallback((id: string) => {
    if (readIdsRef.current.has(id)) return
    const next = new Set(readIdsRef.current).add(id)
    readIdsRef.current = next
    saveReadIds(next)
    setReadIds(next)
    setNotifications(prev => prev.map(n => (n.id === id ? { ...n, read: true } : n)))
  }, [])

  const markAllRead = useCallback(() => {
    const next = new Set(readIdsRef.current)
    notificationsRef.current.forEach(n => next.add(n.id))
    readIdsRef.current = next
    saveReadIds(next)
    setReadIds(next)
    setNotifications(prev => prev.map(n => ({ ...n, read: true })))
  }, [])

  const unreadCount = notifications.filter(n => !n.read).length

  return { notifications, unreadCount, markRead, markAllRead }
}
