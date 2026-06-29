import { useTranslation } from 'react-i18next'
import { Cpu } from 'lucide-react'
import { Card } from '../components/ui/Card'

export default function BmcView() {
  const { t } = useTranslation()

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-xl bg-blue-500/10 flex items-center justify-center">
            <Cpu size={20} className="text-blue-400" />
          </div>
          <div>
            <h1 className="text-xl font-bold tracking-tight">{t('nav.bmc')}</h1>
            <p className="text-sm text-[var(--text-muted)]">Out-of-Band Management</p>
          </div>
        </div>
      </div>
      <Card>
        <div className="flex items-center justify-center py-20 text-[var(--text-muted)]">
          <p>BMC management interface — coming soon</p>
        </div>
      </Card>
    </div>
  )
}
