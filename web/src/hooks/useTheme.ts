import { useState, useEffect, useCallback } from 'react'

export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'PxeLab-theme'

const media = () => window.matchMedia('(prefers-color-scheme: dark)')

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'system'
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'light' || stored === 'dark' || stored === 'system') return stored
  return 'system'
}

function resolveTheme(theme: Theme): 'light' | 'dark' {
  if (theme === 'system') return media().matches ? 'dark' : 'light'
  return theme
}

function applyTheme(resolved: 'light' | 'dark') {
  document.documentElement.classList.toggle('dark', resolved === 'dark')
}

/**
 * 带 View Transitions 圆形扩散动画的主题应用。
 * origin 为点击坐标（px），不支持或用户偏好减少动态时直接切换。
 */
function applyThemeAnimated(resolved: 'light' | 'dark', origin?: { x: number; y: number }) {
  const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches
  if (!origin || reduced || typeof document.startViewTransition !== 'function') {
    applyTheme(resolved)
    return
  }
  const el = document.documentElement
  el.style.setProperty('--theme-x', `${origin.x}px`)
  el.style.setProperty('--theme-y', `${origin.y}px`)
  document.startViewTransition(() => applyTheme(resolved))
}

export function useTheme() {
  const [theme, setThemeState] = useState<Theme>(getInitialTheme)
  const resolved = resolveTheme(theme)

  useEffect(() => {
    applyTheme(resolved)
  }, [resolved])

  // system 模式下跟随系统主题实时切换
  useEffect(() => {
    if (theme !== 'system') return
    const mq = media()
    const onChange = () => applyTheme(mq.matches ? 'dark' : 'light')
    mq.addEventListener('change', onChange)
    return () => mq.removeEventListener('change', onChange)
  }, [theme])

  const setTheme = useCallback((t: Theme, origin?: { x: number; y: number }) => {
    localStorage.setItem(STORAGE_KEY, t)
    applyThemeAnimated(resolveTheme(t), origin)
    setThemeState(t)
  }, [])

  return { theme, setTheme }
}
