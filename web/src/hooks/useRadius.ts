import { useState, useEffect, useCallback } from 'react'

/**
 * 圆角偏好：调节全局 --radius-base，Tailwind rounded-* 全站联动（见 index.css @theme）。
 * 只有用户显式选择过（localStorage 有值）才写内联 style，
 * 否则保持 CSS 预设生效（如 html.theme-macos 的 --radius-base: 0.75rem）。
 */
export type RadiusPreset = 'none' | 'sm' | 'md' | 'lg' | 'xl'

const STORAGE_KEY = 'PxeLab-radius'

export const RADIUS_PRESETS: Record<RadiusPreset, string> = {
  none: '0rem',
  sm: '0.25rem',
  md: '0.5rem',
  lg: '0.75rem',
  xl: '1rem',
}

/** 用户显式存储过的值；未设置过返回 null（此时不写内联 style） */
function getStoredRadius(): RadiusPreset | null {
  if (typeof window === 'undefined') return null
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored && stored in RADIUS_PRESETS) return stored as RadiusPreset
  return null
}

function applyRadius(preset: RadiusPreset) {
  document.documentElement.style.setProperty('--radius-base', RADIUS_PRESETS[preset])
}

export function useRadius() {
  // UI 选中态：无存储时显示默认 'md'，但不覆盖 CSS 预设
  const [radius, setRadiusState] = useState<RadiusPreset>(() => getStoredRadius() ?? 'md')

  useEffect(() => {
    const stored = getStoredRadius()
    if (stored) applyRadius(stored)
  }, [])

  const setRadius = useCallback((r: RadiusPreset) => {
    setRadiusState(r)
    localStorage.setItem(STORAGE_KEY, r)
    applyRadius(r)
  }, [])

  return { radius, setRadius }
}
