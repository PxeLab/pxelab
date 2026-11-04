import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Languages } from 'lucide-react'

const languages = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en', label: 'English' },
]

export function LangSwitch() {
  const { t, i18n } = useTranslation()

  const current = i18n.language?.startsWith('zh') ? 'zh-CN' : 'en'

  // 读屏按文档语言选择发音规则；切换语言时同步 <html lang>
  useEffect(() => {
    document.documentElement.lang = current
  }, [current])

  const cycleLang = () => {
    const codes = languages.map(l => l.code)
    const idx = codes.indexOf(current)
    const next = codes[(idx + 1) % codes.length]
    i18n.changeLanguage(next)
  }

  return (
    <button
      onClick={cycleLang}
      title={languages.find(l => l.code === current)?.label}
      aria-label={t('langSwitch.label')}
      className="flex items-center gap-1.5 rounded-lg px-2 sm:px-3 py-2 text-sm text-[var(--text-muted)] hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)] transition-colors"
    >
      <Languages size={16} />
      <span className="hidden sm:inline">{languages.find(l => l.code === current)?.label}</span>
    </button>
  )
}
