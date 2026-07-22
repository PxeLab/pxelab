import { useEffect, useRef, useState, useCallback } from 'react'

interface UseFullscreenReturn<T extends HTMLElement> {
  ref: React.RefObject<T | null>
  isFullscreen: boolean
  toggle: () => void
}

/** 兼容 webkit 前缀的 Fullscreen API */
interface FullscreenDocument extends Document {
  webkitFullscreenElement?: Element | null
  webkitExitFullscreen?: () => Promise<void>
}

interface FullscreenElement extends HTMLElement {
  webkitRequestFullscreen?: () => Promise<void>
}

function getFullscreenElement(): Element | null {
  const doc = document as FullscreenDocument
  return doc.fullscreenElement ?? doc.webkitFullscreenElement ?? null
}

export function useFullscreen<T extends HTMLElement>(): UseFullscreenReturn<T> {
  const ref = useRef<T | null>(null)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const isFullscreenRef = useRef(false)

  useEffect(() => {
    function onChange() {
      const active = getFullscreenElement() === ref.current
      isFullscreenRef.current = active
      setIsFullscreen(active)
    }

    document.addEventListener('fullscreenchange', onChange)
    document.addEventListener('webkitfullscreenchange', onChange)

    return () => {
      document.removeEventListener('fullscreenchange', onChange)
      document.removeEventListener('webkitfullscreenchange', onChange)
      // 卸载时若仍处于全屏则退出
      if (isFullscreenRef.current) {
        const doc = document as FullscreenDocument
        if (doc.exitFullscreen) {
          doc.exitFullscreen()
        } else {
          doc.webkitExitFullscreen?.()
        }
      }
    }
  }, [])

  const toggle = useCallback(() => {
    const el = ref.current as FullscreenElement | null
    if (!el) return
    const doc = document as FullscreenDocument

    if (getFullscreenElement()) {
      if (doc.exitFullscreen) {
        doc.exitFullscreen()
      } else {
        doc.webkitExitFullscreen?.()
      }
    } else if (el.requestFullscreen) {
      el.requestFullscreen()
    } else {
      el.webkitRequestFullscreen?.()
    }
  }, [])

  return { ref, isFullscreen, toggle }
}
