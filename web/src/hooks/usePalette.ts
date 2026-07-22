import { useState, useEffect, useCallback } from 'react'

export type Palette = 'ocean' | 'forest' | 'sunset' | 'violet' | 'rose' | 'midnight'

const STORAGE_KEY = 'PxeLab-palette'

/**
 * 调色板配置：每个主题只需声明一个主色（500 档）。
 * 完整色阶（50..900）由 oklch 派生——固定 lightness 档位 + 主色的色相/彩度。
 * 新增主题色 = 在这里加一行。
 */
export const PALETTES: Record<Palette, { primary: string }> = {
  ocean: { primary: '#3b82f6' },
  forest: { primary: '#14b8a6' },
  sunset: { primary: '#f59e0b' },
  violet: { primary: '#8b5cf6' },
  rose: { primary: '#f43f5e' },
  midnight: { primary: '#2563eb' },
}

/* ── hex → oklch ── */

interface Oklch { l: number; c: number; h: number }

function hexToOklch(hex: string): Oklch {
  const n = parseInt(hex.slice(1), 16)
  const toLinear = (v: number) => {
    v /= 255
    return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4)
  }
  const r = toLinear((n >> 16) & 0xff)
  const g = toLinear((n >> 8) & 0xff)
  const b = toLinear(n & 0xff)

  const l_ = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m_ = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s_ = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)

  const L = 0.2104542553 * l_ + 0.793617785 * m_ - 0.0040720468 * s_
  const a = 1.9779984951 * l_ - 2.428592205 * m_ + 0.4505937099 * s_
  const b2 = 0.0259040371 * l_ + 0.7827717662 * m_ - 0.808675766 * s_

  const c = Math.sqrt(a * a + b2 * b2)
  let h = (Math.atan2(b2, a) * 180) / Math.PI
  if (h < 0) h += 360
  return { l: L, c, h }
}

function hexToRgbString(hex: string): string {
  const n = parseInt(hex.slice(1), 16)
  return `${(n >> 16) & 0xff},${(n >> 8) & 0xff},${n & 0xff}`
}

/* ── 色阶派生 ── */

// 各档位的 lightness 目标（500 档取主色自身 L）与彩度系数
const STOPS: { stop: number; l: (lp: number) => number; cf: number }[] = [
  { stop: 50, l: () => 0.97, cf: 0.12 },
  { stop: 100, l: () => 0.93, cf: 0.22 },
  { stop: 200, l: () => 0.87, cf: 0.42 },
  { stop: 300, l: () => 0.8, cf: 0.62 },
  { stop: 400, l: lp => Math.min(0.72, lp * 1.12), cf: 0.85 },
  { stop: 500, l: lp => lp, cf: 1 },
  { stop: 600, l: lp => lp * 0.88, cf: 0.95 },
  { stop: 700, l: lp => lp * 0.77, cf: 0.88 },
  { stop: 800, l: lp => lp * 0.66, cf: 0.78 },
  { stop: 900, l: lp => lp * 0.56, cf: 0.68 },
]

function applyPalette(palette: Palette) {
  const { primary } = PALETTES[palette]
  const { l, c, h } = hexToOklch(primary)
  const style = document.documentElement.style
  for (const { stop, l: lf, cf } of STOPS) {
    style.setProperty(`--color-blue-${stop}`, `oklch(${lf(l).toFixed(3)} ${(c * cf).toFixed(3)} ${h.toFixed(1)})`)
  }
  const rgb = hexToRgbString(primary)
  style.setProperty('--glow-blue', `0 0 20px -4px rgba(${rgb},0.35)`)
  style.setProperty('--glow-green', `0 0 16px -4px rgba(${rgb},0.3)`)
}

function getInitialPalette(): Palette {
  if (typeof window === 'undefined') return 'ocean'
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored && stored in PALETTES) return stored as Palette
  return 'ocean'
}

export function usePalette() {
  const [palette, setPaletteState] = useState<Palette>(getInitialPalette)

  useEffect(() => {
    applyPalette(palette)
  }, [palette])

  const setPalette = useCallback((p: Palette) => {
    setPaletteState(p)
    localStorage.setItem(STORAGE_KEY, p)
    applyPalette(p)
  }, [])

  return { palette, setPalette }
}
