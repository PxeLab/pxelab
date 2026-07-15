import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Wifi, Plus, Trash2, RefreshCw, Clock, History, List } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { useToast } from '../components/ui/Toast'
import { api, type WOLHistoryRecord, type WOLSchedule, type WOLInterface } from '../api/client'

function formatTime(s: string) {
  try { return new Date(s).toLocaleString() } catch { return s }
}

export default function WolView() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [, setLoading] = useState(true)
  const [history, setHistory] = useState<WOLHistoryRecord[]>([])
  const [schedules, setSchedules] = useState<WOLSchedule[]>([])
  const [interfaces, setInterfaces] = useState<WOLInterface[]>([])
  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const [wakeMac, setWakeMac] = useState('')
  const [wakeIface, setWakeIface] = useState('')
  const [waking, setWaking] = useState(false)

  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [scheduleMac, setScheduleMac] = useState('')
  const [scheduleAt, setScheduleAt] = useState('')
  const [repeatType, setRepeatType] = useState('once')
  const [creatingSchedule, setCreatingSchedule] = useState(false)

  const [tab, setTab] = useState<'history' | 'schedules'>('history')

  const loadData = async () => {
    setLoading(true)
    try {
      const [h, s, ifs] = await Promise.all([
        api.getWOLHistory(page),
        api.getWOLSchedules(),
        api.getWOLInterfaces(),
      ])
      setHistory(h.data.records)
      setTotal(h.data.total)
      setSchedules(s.data.schedules)
      setInterfaces(ifs.data)
    } catch { /* ignore */ }
    setLoading(false)
  }

  useEffect(() => { loadData() }, [page])

  const doQuickWake = async () => {
    const mac = wakeMac.trim().replace(/[^a-fA-F0-9:]/g, '').toUpperCase()
    if (!mac) { showError(t('wol.invalidMac')); return }
    setWaking(true)
    try {
      const res = await api.batchWakeHosts({ macs: [mac], interface: wakeIface || undefined })
      if (res.data.results[0]?.success) {
        success(t('wol.wakeSuccess', { mac }))
        setWakeMac('')
        loadData()
      } else {
        showError(res.data.results[0]?.error || t('wol.wakeFailed'))
      }
    } catch (e: any) { showError(e?.message || t('wol.wakeFailed')) }
    setWaking(false)
  }

  const doCreateSchedule = async () => {
    const mac = scheduleMac.trim().replace(/[^a-fA-F0-9:]/g, '').toUpperCase()
    if (!mac || !scheduleAt) { showError(t('wol.invalidSchedule')); return }
    setCreatingSchedule(true)
    try {
      await api.createWOLSchedule(mac, new Date(scheduleAt).toISOString(), undefined, repeatType)
      success(t('wol.scheduleCreated'))
      setScheduleOpen(false)
      setScheduleMac('')
      setScheduleAt('')
      setRepeatType('once')
      loadData()
    } catch (e: any) { showError(e?.message || t('wol.scheduleFailed')) }
    setCreatingSchedule(false)
  }

  const doDeleteSchedule = async (id: number) => {
    try {
      await api.deleteWOLSchedule(id)
      success(t('wol.scheduleDeleted'))
      loadData()
    } catch (e: any) { showError(e?.message || t('common.error')) }
  }

  return (
    <div className="space-y-6 animate-fade-in">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-[var(--text-primary)]">{t('wol.title')}</h1>
          <p className="text-sm text-[var(--text-muted)] mt-1">{t('wol.description')}</p>
        </div>
        <Button onClick={loadData} variant="secondary" size="sm">
          <RefreshCw size={14} className="mr-1.5" />{t('common.refresh')}
        </Button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        <Card className="p-4 lg:col-span-2">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3 flex items-center gap-2">
            <Wifi size={14} className="text-blue-400" />{t('wol.quickWake')}
          </h3>
          <div className="flex flex-wrap items-end gap-3">
            <div className="flex-1 min-w-[200px]">
              <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.macAddress')}</label>
              <input
                value={wakeMac}
                onChange={e => setWakeMac(e.target.value)}
                placeholder="AA:BB:CC:DD:EE:FF"
                className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono"
                onKeyDown={e => e.key === 'Enter' && doQuickWake()}
              />
            </div>
            <div className="min-w-[140px]">
              <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.interface')}</label>
              <select
                value={wakeIface}
                onChange={e => setWakeIface(e.target.value)}
                className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
              >
                <option value="">{t('wol.auto')}</option>
                {interfaces.map(iface => iface.ips.map(ip => (
                  <option key={`${iface.name}-${ip}`} value={ip}>{iface.name} ({ip})</option>
                )))}
              </select>
            </div>
            <Button onClick={doQuickWake} disabled={waking}>
              {waking ? '...' : t('wol.wake')}
            </Button>
          </div>
        </Card>

        <Card className="p-4">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3 flex items-center gap-2">
            <List size={14} className="text-green-400" />{t('wol.availableInterfaces')}
          </h3>
          {interfaces.length === 0 ? (
            <p className="text-xs text-[var(--text-muted)]">{t('wol.noInterfaces')}</p>
          ) : (
            <div className="space-y-2">
              {interfaces.map(iface => (
                <div key={iface.name} className="text-xs">
                  <span className="font-medium text-[var(--text-primary)]">{iface.name}</span>
                  <div className="text-[var(--text-muted)] font-mono mt-0.5">{iface.ips.join(', ')}</div>
                </div>
              ))}
            </div>
          )}
        </Card>
      </div>

      <Card className="p-4">
        <div className="flex items-center justify-between mb-4">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setTab('history')}
              className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${tab === 'history' ? 'bg-blue-500/15 text-blue-400' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}
            >
              <History size={14} />{t('wol.history')}
            </button>
            <button
              onClick={() => setTab('schedules')}
              className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${tab === 'schedules' ? 'bg-blue-500/15 text-blue-400' : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'}`}
            >
              <Clock size={14} />{t('wol.schedules')}
            </button>
          </div>
          {tab === 'schedules' && (
            <Button onClick={() => setScheduleOpen(true)} size="sm">
              <Plus size={14} className="mr-1" />{t('wol.newSchedule')}
            </Button>
          )}
        </div>

        {tab === 'history' ? (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-[var(--bg-border)] text-[var(--text-muted)]">
                  <th className="text-left py-2 px-3 font-medium">{t('wol.mac')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.host')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.broadcast')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.source')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.status')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.time')}</th>
                </tr>
              </thead>
              <tbody>
                {history.map(r => (
                  <tr key={r.id} className="border-b border-[var(--bg-border)]/50 hover:bg-[var(--bg-hover)]/30">
                    <td className="py-2 px-3 font-mono text-[var(--text-primary)]">{r.mac}</td>
                    <td className="py-2 px-3 text-[var(--text-secondary)]">{r.host_name || '-'}</td>
                    <td className="py-2 px-3 font-mono text-[var(--text-secondary)]">{r.broadcast}</td>
                    <td className="py-2 px-3 font-mono text-[var(--text-secondary)]">{r.source_ip || '-'}</td>
                    <td className="py-2 px-3">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium ${r.success ? 'bg-green-500/15 text-green-400' : 'bg-red-500/15 text-red-400'}`}>
                        {r.success ? t('wol.success') : (r.error_msg || t('wol.failed'))}
                      </span>
                    </td>
                    <td className="py-2 px-3 text-[var(--text-muted)]">{formatTime(r.created_at)}</td>
                  </tr>
                ))}
                {history.length === 0 && (
                  <tr><td colSpan={6} className="py-8 text-center text-[var(--text-muted)] text-xs">{t('wol.noHistory')}</td></tr>
                )}
              </tbody>
            </table>
            {total > 20 && (
              <div className="flex items-center justify-between pt-3">
                <span className="text-xs text-[var(--text-muted)]">{t('wol.pageInfo', { page, total })}</span>
                <div className="flex gap-2">
                  <Button variant="secondary" size="sm" disabled={page <= 1} onClick={() => setPage(p => p - 1)}>{t('common.prev')}</Button>
                  <Button variant="secondary" size="sm" disabled={page * 20 >= total} onClick={() => setPage(p => p + 1)}>{t('common.next')}</Button>
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-xs">
              <thead>
                <tr className="border-b border-[var(--bg-border)] text-[var(--text-muted)]">
                  <th className="text-left py-2 px-3 font-medium">{t('wol.mac')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.host')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.scheduleAt')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.repeat')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.status')}</th>
                  <th className="text-left py-2 px-3 font-medium">{t('wol.lastRun')}</th>
                  <th className="text-right py-2 px-3 font-medium">{t('common.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {schedules.map(s => (
                  <tr key={s.id} className="border-b border-[var(--bg-border)]/50 hover:bg-[var(--bg-hover)]/30">
                    <td className="py-2 px-3 font-mono text-[var(--text-primary)]">{s.mac}</td>
                    <td className="py-2 px-3 text-[var(--text-secondary)]">{s.host_name || '-'}</td>
                    <td className="py-2 px-3 text-[var(--text-secondary)]">{formatTime(s.schedule_at)}</td>
                    <td className="py-2 px-3 text-[var(--text-secondary)]">{s.cron_expr || s.repeat_type}</td>
                    <td className="py-2 px-3">
                      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium ${s.enabled ? 'bg-green-500/15 text-green-400' : 'bg-yellow-500/15 text-yellow-400'}`}>
                        {s.enabled ? t('wol.enabled') : t('wol.disabled')}
                      </span>
                    </td>
                    <td className="py-2 px-3 text-[var(--text-muted)]">{s.last_run ? formatTime(s.last_run) : '-'}</td>
                    <td className="py-2 px-3 text-right">
                      <button onClick={() => doDeleteSchedule(s.id)} className="p-1.5 rounded-lg text-red-400 hover:bg-red-500/15 transition-colors">
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
                {schedules.length === 0 && (
                  <tr><td colSpan={7} className="py-8 text-center text-[var(--text-muted)] text-xs">{t('wol.noSchedules')}</td></tr>
                )}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      <Modal open={scheduleOpen} onClose={() => setScheduleOpen(false)} title={t('wol.newSchedule')}>
        <div className="space-y-4 p-1">
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.macAddress')}</label>
            <input
              value={scheduleMac}
              onChange={e => setScheduleMac(e.target.value)}
              placeholder="AA:BB:CC:DD:EE:FF"
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono"
            />
          </div>
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.scheduleAt')}</label>
            <input
              type="datetime-local"
              value={scheduleAt}
              onChange={e => setScheduleAt(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
            />
          </div>
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.repeat')}</label>
            <select
              value={repeatType}
              onChange={e => setRepeatType(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
            >
              <option value="once">{t('wol.once')}</option>
              <option value="daily">{t('wol.daily')}</option>
              <option value="weekly">{t('wol.weekly')}</option>
              <option value="weekday">{t('wol.weekday')}</option>
            </select>
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setScheduleOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={doCreateSchedule} disabled={creatingSchedule}>
              {creatingSchedule ? '...' : t('common.create')}
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
