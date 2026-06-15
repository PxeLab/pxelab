import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Power, PowerOff, RefreshCw, Activity, Zap } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Tag } from '../components/ui/Tag'
import { StatusDot } from '../components/ui/StatusDot'
import { useToast } from '../components/ui/Toast'
import { api, type Host, type Event } from '../api/client'

export default function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [host, setHost] = useState<Host | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)
  const [powerLoading, setPowerLoading] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    async function load() {
      try {
        const [h, e] = await Promise.all([
          api.getHost(id!),
          api.getEvents({ page: '1', size: '10' }),
        ])
        setHost(h.data)
        setEvents(e.data.events)
      } catch (err: any) {
        showError(err.message)
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [id])

  async function handlePower(action: string) {
    if (!host) return
    if (action !== 'status' && !confirm(`${t('hosts.detail.power' + action)}?`)) return
    setPowerLoading(action)
    try {
      const res = await api.powerHost(host.id, action)
      success( `${t('hosts.detail.power' + action)}: ${(res.data as any).status}`)
    } catch (err: any) {
      showError(err.message)
    } finally {
      setPowerLoading(null)
    }
  }

  if (loading) {
    return <div className="space-y-6">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="bg-[#16181f] border border-[#232738] rounded-xl p-5 animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[#16181f] via-[#1c1f2c] to-[#16181f] bg-[length:200%_100%] h-32" />
      ))}
    </div>
  }

  if (!host) return <p className="text-[#6b7294]">{t('common.notFound', '未找到')}</p>

  const hostEvents = events.filter(e => e.mac === host.mac)

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" onClick={() => navigate('/hosts')}>
            <ArrowLeft size={16} /> {t('common.back', '返回')}
          </Button>
          <StatusDot color={host.last_online ? 'green' : 'red'} />
          <span className="text-lg font-bold">{host.name || t('common.unnamed', '未命名主机')}</span>
          <Tag color={host.last_online ? 'green' : 'red'}>{host.last_online ? 'online' : 'offline'}</Tag>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => handlePower('status')}>
            <Zap size={14} /> {t('hosts.detail.wake')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => navigate('/hosts/' + host.id + '/edit')}>
            {t('common.edit')}
          </Button>
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Basic Info */}
        <Card title={t('hosts.detail.basicInfo')}>
          <div className="grid grid-cols-2 gap-4">
            {[
              { label: 'MAC', value: host.mac, mono: true },
              { label: 'IP', value: host.ip, mono: true },
              { label: t('hosts.columns.bootCount'), value: String(host.boot_count) },
              { label: t('hosts.columns.lastOnline'), value: host.last_online ? new Date(host.last_online).toLocaleString() : '—', mono: true },
              { label: 'Profile', value: host.profile_id || t('common.default', '默认') },
              { label: t('common.created', '创建时间'), value: new Date(host.created_at).toLocaleString(), mono: true },
            ].map((item, i) => (
              <div key={i}>
                <div className="text-[11px] font-semibold uppercase tracking-wider text-[#6b7294] mb-1">{item.label}</div>
                <div className={`text-sm font-medium text-[#e8eaed] ${item.mono ? 'font-mono text-xs' : ''}`}>{item.value}</div>
              </div>
            ))}
          </div>
        </Card>

        {/* IPMI Power Control */}
        <Card title={t('hosts.detail.powerControl')}>
          <div className="flex items-center gap-3 mb-4 px-3.5 py-2.5 bg-[#1a1d2e] rounded-lg border border-[#232738] text-sm">
            <StatusDot color={host.bmc_addr ? 'green' : 'yellow'} />
            <span className="text-[#9aa0ab]">BMC</span>
            <span className="font-mono text-xs text-[#6b7294]">{host.bmc_addr || t('common.notConfigured', '未配置')}</span>
          </div>
          <div className="grid grid-cols-2 gap-2">
            {[
              { action: 'on', label: t('hosts.detail.powerOn'), icon: Power, color: 'text-green-400 hover:border-green-500/30' },
              { action: 'off', label: t('hosts.detail.powerOff'), icon: PowerOff, color: 'text-red-400 hover:border-red-500/30' },
              { action: 'cycle', label: t('hosts.detail.powerRestart'), icon: RefreshCw, color: 'text-yellow-400 hover:border-yellow-500/30' },
              { action: 'status', label: t('hosts.detail.powerStatus'), icon: Activity, color: 'text-blue-400 hover:border-blue-500/30' },
            ].map(({ action, label, icon: Icon, color }) => (
              <button
                key={action}
                onClick={() => handlePower(action)}
                disabled={powerLoading === action}
                className={`flex flex-col items-center gap-2 py-4 rounded-lg border border-[#232738] bg-[#1a1d2e] cursor-pointer transition-all hover:bg-[#1c1f2c] disabled:opacity-50 ${color}`}
              >
                <Icon size={22} />
                <span className="text-xs font-semibold text-[#9aa0ab]">{label}</span>
              </button>
            ))}
          </div>
        </Card>
      </div>

      {/* Boot History */}
      <div className="mt-5">
        <Card title={t('hosts.detail.bootHistory')}>
          {hostEvents.length === 0 ? (
            <p className="text-sm text-[#6b7294] text-center py-4">{t('events.noEvents')}</p>
          ) : (
            <div className="overflow-x-auto -mx-5">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[#6b7294] border-b border-[#232738]">{t('events.time')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[#6b7294] border-b border-[#232738]">{t('events.type')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[#6b7294] border-b border-[#232738]">{t('events.message')}</th>
                  </tr>
                </thead>
                <tbody>
                  {hostEvents.map((e, i) => (
                    <tr key={i}>
                      <td className="px-4 py-3 border-b border-[#232738] font-mono text-xs text-[#6b7294]">
                        {new Date(e.timestamp).toLocaleString()}
                      </td>
                      <td className="px-4 py-3 border-b border-[#232738]">
                        <Tag color={e.type.includes('dhcp') ? 'cyan' : e.type.includes('tftp') ? 'orange' : e.type.includes('boot') ? 'green' : 'blue'}>
                          {e.type}
                        </Tag>
                      </td>
                      <td className="px-4 py-3 border-b border-[#232738] text-[#9aa0ab]">{e.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>
    </div>
  )
}
