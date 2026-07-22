import { useTranslation } from 'react-i18next'
import { PageHeader } from '../components/ui/PageHeader'
import Files from './Files'

export default function FileManager() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      <PageHeader title={t('files.title')} className="mb-0" />

      <Files hideHeader />
    </div>
  )
}
