import { useState, useEffect, useCallback } from 'react'

export type Palette = 'ocean' | 'forest' | 'sunset' | 'violet' | 'rose' | 'midnight'

const STORAGE_KEY = 'PxeLab-palette'

function getInitialPalette(): Palette {
  if (typeof window === 'undefined') return 'ocean'
  const stored = localStorage.getItem(STORAGE_KEY)
  if (['ocean', 'forest', 'sunset', 'violet', 'rose', 'midnight'].includes(stored || '')) {
    return stored as Palette
  }
  return 'ocean'
}

function applyPalette(palette: Palette) {
  const html = document.documentElement
  html.classList.remove('palette-ocean', 'palette-forest', 'palette-sunset', 'palette-violet', 'palette-rose', 'palette-midnight')
  if (palette !== 'ocean') {
    html.classList.add(`palette-${palette}`)
  }
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