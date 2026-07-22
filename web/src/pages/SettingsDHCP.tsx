import { useSearchParams } from 'react-router-dom'
import { PageHeader } from '../components/ui/PageHeader'
import { useTranslation } from 'react-i18next'
import { DHCPConfigTab } from './settings-dhcp/DHCPConfigTab'
import { LeasesTab } from './settings-dhcp/LeasesTab'
import { ReservationTab } from './settings-dhcp/ReservationTab'

// ── Main Page ──

export default function SettingsInterfaces() {
  const { t } = useTranslation()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as 'dhcp' | 'leases' | 'reservations') || 'dhcp'

  const tabs = [
    { key: 'dhcp' as const, label: t('settings.dhcpInterfaces') },
    { key: 'leases' as const, label: t('settings.dhcpLeases') },
    { key: 'reservations' as const, label: t('settings.dhcpReservations') },
  ]

  return (
    <div>
      <PageHeader title={t('settings.dhcpTitle')} />
      <div className="flex gap-1 mb-6 border-b border-[var(--bg-border)]">
        {tabs.map(tab => (
          <button key={tab.key}
            onClick={() => setSearchParams({ tab: tab.key })}
            className={`px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-[1px] ${
              activeTab === tab.key
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {activeTab === 'dhcp' ? <DHCPConfigTab /> : activeTab === 'leases' ? <LeasesTab /> : <ReservationTab />}
    </div>
  )
}
