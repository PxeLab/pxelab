import { useTranslation } from 'react-i18next'
import { Languages } from 'lucide-react'

const languages = [
  { code: 'zh-CN', label: '中文' },
  { code: 'en', label: 'English' },
]

export function LangSwitch() {
  const { i18n } = useTranslation()

  const current = i18n.language?.startsWith('zh') ? 'zh-CN' : 'en'

  const cycleLang = () => {
    const codes = languages.map(l => l.code)
    const idx = codes.indexOf(current)
    const next = codes[(idx + 1) % codes.length]
    i18n.changeLanguage(next)
  }

  return (
    <button
      onClick={cycleLang}
      className="flex items-center gap-1.5 rounded-lg px-3 py-2 text-sm text-gray-400 hover:bg-gray-800 hover:text-gray-100 transition-colors"
    >
      <Languages size={16} />
      <span>{languages.find(l => l.code === current)?.label}</span>
    </button>
  )
}
