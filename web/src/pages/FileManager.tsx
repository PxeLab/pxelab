import { useTranslation } from 'react-i18next'
import Files from './Files'

export default function FileManager() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('files.title')}</h1>
      </div>

      <Files hideHeader />
    </div>
  )
}
