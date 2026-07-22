import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Wifi, Plus, Trash2, RefreshCw, Clock, History } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { DataTable, type Column } from '../components/ui/DataTable'
import { useToast } from '../components/ui/Toast'
import { api, type WOLHistoryRecord, type WOLSchedule } from '../api/client'

function formatTime(s: string) {
  try { return new Date(s).toLocaleString() } catch { return s }
}

export default function WolView() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [, setLoading] = useState(true)
  const [history, setHistory] = useState<WOLHistoryRecord[]>([])
  const [schedules, setSchedules] = useState<WOLSchedule[]>([])

  const [page, setPage] = useState(1)
  const [total, setTotal] = useState(0)

  const [wakeMac, setWakeMac] = useState('')
  const [waking, setWaking] = useState(false)
  const [quickAdvancedMode, setQuickAdvancedMode] = useState(false)
  const [quickBroadcast, setQuickBroadcast] = useState('')
  const [batchOpen, setBatchOpen] = useState(false)
  const [batchMacs, setBatchMacs] = useState('')

  const [scheduleOpen, setScheduleOpen] = useState(false)
  const [scheduleMac, setScheduleMac] = useState('')
  const [scheduleDate, setScheduleDate] = useState('')
  const [scheduleTime, setScheduleTime] = useState('09:00')
  const [scheduleWeekday, setScheduleWeekday] = useState('1')
  const [repeatType, setRepeatType] = useState('once')
  const [creatingSchedule, setCreatingSchedule] = useState(false)
  const [advancedMode, setAdvancedMode] = useState(false)
  const [customBroadcast, setCustomBroadcast] = useState('')

  const [tab, setTab] = useState<'history' | 'schedules'>('history')

  const loadData = async () => {
    setLoading(true)
    try {
      const [h, s] = await Promise.all([
        api.getWOLHistory(page),
        api.getWOLSchedules(),
      ])
      setHistory(h.data.records)
      setTotal(h.data.total)
      setSchedules(s.data.schedules)
    } catch { /* ignore */ }
    setLoading(false)
  }

  useEffect(() => { loadData() }, [page])

  const parseMACs = (text: string): string[] => {
    return text.split(/[\n,;]+/).map(m => m.trim().replace(/[^a-fA-F0-9:]/g, '').toUpperCase()).filter(m => m.length === 17 || (m.length === 12 && /^[0-9A-F]{12}$/.test(m)))
  }

  const doQuickWake = async () => {
    const macs = parseMACs(wakeMac)
    if (!macs.length) { showError(t('wol.invalidMac')); return }
    setWaking(true)
    try {
      const res = await api.batchWakeHosts({ macs, broadcast: quickAdvancedMode && quickBroadcast ? quickBroadcast : undefined })
      const successCount = res.data.results.filter(r => r.success).length
      if (successCount > 0) {
        if (macs.length === 1) {
          success(t('wol.wakeSent', { mac: macs[0] }))
        } else {
          success(t('wol.wakeMultiSent', { count: successCount, total: macs.length }))
        }
        setWakeMac('')
        loadData()
      } else {
        showError(t('wol.wakeFailed'))
      }
    } catch (e: any) { showError(e?.message || t('wol.wakeFailed')) }
    setWaking(false)
  }

  const doBatchWake = async () => {
    const macs = parseMACs(batchMacs)
    if (!macs.length) { showError(t('wol.invalidMac')); return }
    setWaking(true)
    try {
      const res = await api.batchWakeHosts({ macs, broadcast: quickAdvancedMode && quickBroadcast ? quickBroadcast : undefined })
      const successCount = res.data.results.filter(r => r.success).length
      if (successCount > 0) {
        success(t('wol.wakeMultiSent', { count: successCount, total: macs.length }))
        setBatchOpen(false)
        setBatchMacs('')
        loadData()
      } else {
        showError(t('wol.wakeFailed'))
      }
    } catch (e: any) { showError(e?.message || t('wol.wakeFailed')) }
    setWaking(false)
  }

  const doCreateSchedule = async () => {
    const mac = scheduleMac.trim().replace(/[^a-fA-F0-9:]/g, '').toUpperCase()
    if (!mac) { showError(t('wol.invalidMac')); return }

    let scheduleAt = ''
    if (repeatType === 'once') {
      if (!scheduleDate || !scheduleTime) { showError(t('wol.invalidSchedule')); return }
      scheduleAt = `${scheduleDate}T${scheduleTime}:00`
    } else {
      if (!scheduleTime) { showError(t('wol.invalidSchedule')); return }
      const today = new Date()
      const dateStr = today.toISOString().split('T')[0]
      scheduleAt = `${dateStr}T${scheduleTime}:00`
    }

    setCreatingSchedule(true)
    try {
      const cronExpr = repeatType === 'weekly' ? `0 ${scheduleTime.split(':')[1]} ${scheduleTime.split(':')[0]} * * ${scheduleWeekday}` : undefined
      await api.createWOLSchedule(mac, new Date(scheduleAt).toISOString(), cronExpr, repeatType, parseInt(scheduleWeekday), scheduleTime, advancedMode && customBroadcast ? customBroadcast : undefined)
      success(t('wol.scheduleCreated'))
      setScheduleOpen(false)
      setScheduleMac('')
      setScheduleDate('')
      setScheduleTime('09:00')
      setScheduleWeekday('1')
      setRepeatType('once')
      setAdvancedMode(false)
      setCustomBroadcast('')
      loadData()
    } catch (e: any) { showError(e?.message || t('wol.scheduleFailed')) }
    setCreatingSchedule(false)
  }

  const [confirmDelete, setConfirmDelete] = useState<{ kind: 'history' | 'schedule' | 'allHistory'; id?: number } | null>(null)

  const doDeleteSchedule = async (id: number) => {
    try {
      await api.deleteWOLSchedule(id)
      success(t('wol.scheduleDeleted'))
      loadData()
    } catch (e: any) { showError(e?.message || t('common.error')) }
  }

  const doWakeAgain = async (record: WOLHistoryRecord) => {
    setWaking(true)
    try {
      const res = await api.batchWakeHosts({ macs: [record.mac], interface: record.interface || undefined, broadcast: record.broadcast || undefined })
      if (res.data.results[0]?.success) {
        success(t('wol.wakeSent', { mac: record.mac }))
        loadData()
      } else {
        showError(t('wol.wakeFailed'))
      }
    } catch (e: any) { showError(e?.message || t('wol.wakeFailed')) }
    setWaking(false)
  }

  const doWakeAllHistory = async () => {
    const uniqueMacs = [...new Set(history.filter(r => r.success).map(r => r.mac))]
    if (!uniqueMacs.length) { showError(t('wol.noHistory')); return }
    setWaking(true)
    try {
      const res = await api.batchWakeHosts({ macs: uniqueMacs })
      const successCount = res.data.results.filter(r => r.success).length
      success(t('wol.wakeMultiSent', { count: successCount, total: uniqueMacs.length }))
      loadData()
    } catch (e: any) { showError(e?.message || t('wol.wakeFailed')) }
    setWaking(false)
  }

  const doDeleteHistory = async (id: number) => {
    try {
      await api.deleteWOLHistory(id)
      success(t('wol.historyDeleted'))
      loadData()
    } catch (e: any) { showError(e?.message || t('common.error')) }
  }

  const doDeleteAllHistory = async () => {
    try {
      await api.deleteAllWOLHistory()
      success(t('wol.allHistoryDeleted'))
      loadData()
    } catch (e: any) { showError(e?.message || t('common.error')) }
  }

  const confirmDeleteMessages = {
    history: t('wol.confirmDeleteHistory'),
    schedule: t('wol.confirmDeleteSchedule'),
    allHistory: t('wol.confirmDeleteAllHistory'),
  }

  const handleConfirmDelete = async () => {
    const c = confirmDelete
    setConfirmDelete(null)
    if (!c) return
    if (c.kind === 'history' && c.id != null) await doDeleteHistory(c.id)
    else if (c.kind === 'schedule' && c.id != null) await doDeleteSchedule(c.id)
    else if (c.kind === 'allHistory') await doDeleteAllHistory()
  }

  const historyColumns: Column<WOLHistoryRecord>[] = [
    { key: 'mac', label: t('wol.mac'), render: r => <span className="font-mono text-[var(--text-primary)]">{r.mac}</span> },
    { key: 'host', label: t('wol.host'), render: r => r.host_name || '-' },
    { key: 'broadcast', label: t('wol.broadcast'), render: r => <span className="font-mono">{r.broadcast}</span> },
    { key: 'interface', label: t('wol.interface'), render: r => <span className="font-mono">{r.interface || r.source_ip || '-'}</span> },
    { key: 'status', label: t('wol.status'), render: r => (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium ${r.success ? 'bg-accent-green/15 text-accent-green' : 'bg-accent-red/15 text-accent-red'}`}>
        {r.success ? t('wol.success') : (r.error_msg || t('wol.failed'))}
      </span>
    )},
    { key: 'time', label: t('wol.time'), render: r => <span className="text-[var(--text-muted)]">{formatTime(r.created_at)}</span> },
    { key: 'actions', label: t('common.actions'), render: r => (
      <div className="flex items-center justify-end gap-1">
        <button onClick={() => doWakeAgain(r)} className="p-1.5 rounded-lg text-blue-400 hover:bg-blue-500/15 transition-colors" title={t('wol.wakeAgain')}>
          <Wifi size={14} />
        </button>
        <button onClick={() => setConfirmDelete({ kind: 'history', id: r.id })} className="p-1.5 rounded-lg text-accent-red hover:bg-accent-red/15 transition-colors" title={t('common.delete')}>
          <Trash2 size={14} />
        </button>
      </div>
    )},
  ]

  const scheduleColumns: Column<WOLSchedule>[] = [
    { key: 'mac', label: t('wol.mac'), render: s => <span className="font-mono text-[var(--text-primary)]">{s.mac}</span> },
    { key: 'host', label: t('wol.host'), render: s => s.host_name || '-' },
    { key: 'schedule_at', label: t('wol.scheduleAt'), render: s => formatTime(s.schedule_at) },
    { key: 'repeat', label: t('wol.repeat'), render: s => s.cron_expr || s.repeat_type },
    { key: 'status', label: t('wol.status'), render: s => (
      <span className={`inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-medium ${s.enabled ? 'bg-accent-green/15 text-accent-green' : 'bg-accent-yellow/15 text-accent-yellow'}`}>
        {s.enabled ? t('wol.enabled') : t('wol.disabled')}
      </span>
    )},
    { key: 'last_run', label: t('wol.lastRun'), render: s => <span className="text-[var(--text-muted)]">{s.last_run ? formatTime(s.last_run) : '-'}</span> },
    { key: 'actions', label: t('common.actions'), render: s => (
      <div className="flex items-center justify-end">
        <button onClick={() => setConfirmDelete({ kind: 'schedule', id: s.id })} className="p-1.5 rounded-lg text-accent-red hover:bg-accent-red/15 transition-colors">
          <Trash2 size={14} />
        </button>
      </div>
    )},
  ]

  return (
    <div className="space-y-6 animate-fade-in">
      <PageHeader
        title={t('wol.title')}
        description={t('wol.description')}
        className="mb-0"
        actions={
          <Button onClick={loadData} variant="secondary" size="sm">
            <RefreshCw size={14} className="mr-1.5" />{t('common.refresh')}
          </Button>
        }
      />

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
            <Button onClick={doQuickWake} disabled={waking || !wakeMac.trim()}>
              {waking ? '...' : t('wol.wake')}
            </Button>
            <Button variant="secondary" onClick={() => setBatchOpen(true)}>
              {t('wol.batchInput')}
            </Button>
          </div>
          <div className="mt-3 border-t border-[var(--bg-border)] pt-3">
            <button
              type="button"
              onClick={() => setQuickAdvancedMode(!quickAdvancedMode)}
              className="flex items-center gap-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
            >
              <span className={`transform transition-transform ${quickAdvancedMode ? 'rotate-90' : ''}`}>▶</span>
              {t('wol.advanced')}
            </button>
            {quickAdvancedMode && (
              <div className="mt-3">
                <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.customBroadcast')}</label>
                <input
                  value={quickBroadcast}
                  onChange={e => setQuickBroadcast(e.target.value)}
                  placeholder="192.168.2.255"
                  className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono"
                />
                <p className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.broadcastHint')}</p>
              </div>
            )}
          </div>
        </Card>

        <Card className="p-4">
          <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3 flex items-center gap-2">
            <History size={14} className="text-blue-400" />{t('wol.wakeStats')}
          </h3>
          <div className="grid grid-cols-4 gap-2">
            <div className="bg-[var(--bg-input)] rounded-lg p-3">
              <div className="text-2xl font-bold text-[var(--text-primary)]">{history.length || 0}</div>
              <div className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.statsTotal')}</div>
            </div>
            <div className="bg-[var(--bg-input)] rounded-lg p-3">
              <div className="text-2xl font-bold text-accent-green">{history.filter(r => r.success).length}</div>
              <div className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.statsSuccess')}</div>
            </div>
            <div className="bg-[var(--bg-input)] rounded-lg p-3">
              <div className="text-2xl font-bold text-accent-red">{history.filter(r => !r.success).length}</div>
              <div className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.statsFailed')}</div>
            </div>
            <div className="bg-[var(--bg-input)] rounded-lg p-3">
              <div className="text-2xl font-bold text-[var(--text-primary)]">{schedules.filter(s => s.enabled).length}</div>
              <div className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.activeSchedules')}</div>
            </div>
          </div>
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
          <div className="flex items-center gap-2">
            {tab === 'history' && (
              <>
                <Button variant="secondary" size="sm" onClick={doWakeAllHistory}>
                  <Wifi size={12} className="mr-1" />{t('wol.wakeAllHistory')}
                </Button>
                <Button variant="secondary" size="sm" onClick={() => setConfirmDelete({ kind: 'allHistory' })} className="text-accent-red">
                  <Trash2 size={12} className="mr-1" />{t('wol.deleteAllHistory')}
                </Button>
              </>
            )}
            {tab === 'schedules' && (
              <Button onClick={() => setScheduleOpen(true)} size="sm">
                <Plus size={14} className="mr-1" />{t('wol.newSchedule')}
              </Button>
            )}
          </div>
        </div>

        {tab === 'history' ? (
          <div>
            <DataTable<WOLHistoryRecord>
              columns={historyColumns}
              data={history}
              emptyText={t('wol.noHistory')}
              rowKey={r => String(r.id)}
            />
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
          <DataTable<WOLSchedule>
            columns={scheduleColumns}
            data={schedules}
            emptyText={t('wol.noSchedules')}
            rowKey={s => String(s.id)}
          />
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
          
          {repeatType === 'once' && (
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.scheduleDate')}</label>
              <input
                type="date"
                value={scheduleDate}
                onChange={e => setScheduleDate(e.target.value)}
                className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
              />
            </div>
          )}
          
          {repeatType === 'weekly' && (
            <div>
              <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.weekday')}</label>
              <select
                value={scheduleWeekday}
                onChange={e => setScheduleWeekday(e.target.value)}
                className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
              >
                <option value="1">{t('wol.monday')}</option>
                <option value="2">{t('wol.tuesday')}</option>
                <option value="3">{t('wol.wednesday')}</option>
                <option value="4">{t('wol.thursday')}</option>
                <option value="5">{t('wol.friday')}</option>
                <option value="6">{t('wol.saturday')}</option>
                <option value="0">{t('wol.sunday')}</option>
              </select>
            </div>
          )}
          
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.scheduleTime')}</label>
            <input
              type="time"
              value={scheduleTime}
              onChange={e => setScheduleTime(e.target.value)}
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors"
            />
          </div>

          <div className="border-t border-[var(--bg-border)] pt-3">
            <button
              type="button"
              onClick={() => setAdvancedMode(!advancedMode)}
              className="flex items-center gap-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
            >
              <span className={`transform transition-transform ${advancedMode ? 'rotate-90' : ''}`}>▶</span>
              {t('wol.advanced')}
            </button>
            {advancedMode && (
              <div className="mt-3">
                <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.customBroadcast')}</label>
                <input
                  value={customBroadcast}
                  onChange={e => setCustomBroadcast(e.target.value)}
                  placeholder="192.168.2.255"
                  className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono"
                />
                <p className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.broadcastHint')}</p>
              </div>
            )}
          </div>

          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setScheduleOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={doCreateSchedule} disabled={creatingSchedule}>
              {creatingSchedule ? '...' : t('common.create')}
            </Button>
          </div>
        </div>
      </Modal>

      <Modal open={batchOpen} onClose={() => setBatchOpen(false)} title={t('wol.batchWakeTitle')}>
        <div className="space-y-4">
          <div>
            <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.batchMacsHint')}</label>
            <textarea
              value={batchMacs}
              onChange={e => setBatchMacs(e.target.value)}
              placeholder={"AA:BB:CC:DD:EE:FF\n11:22:33:44:55:66\nAA:BB:CC:DD:EE:01,AA:BB:CC:DD:EE:02"}
              rows={8}
              className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono resize-none"
            />
            {batchMacs.trim() && (
              <p className="text-[10px] text-[var(--text-muted)] mt-1">
                {t('wol.macCount', { count: parseMACs(batchMacs).length })}
              </p>
            )}
          </div>
          <div className="border-t border-[var(--bg-border)] pt-3">
            <button
              type="button"
              onClick={() => setQuickAdvancedMode(!quickAdvancedMode)}
              className="flex items-center gap-2 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
            >
              <span className={`transform transition-transform ${quickAdvancedMode ? 'rotate-90' : ''}`}>▶</span>
              {t('wol.advanced')}
            </button>
            {quickAdvancedMode && (
              <div className="mt-3">
                <label className="block text-xs text-[var(--text-muted)] mb-1">{t('wol.customBroadcast')}</label>
                <input
                  value={quickBroadcast}
                  onChange={e => setQuickBroadcast(e.target.value)}
                  placeholder="192.168.2.255"
                  className="w-full px-3 py-2 text-sm rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-primary)] outline-none focus:border-blue-500/50 transition-colors font-mono"
                />
                <p className="text-[10px] text-[var(--text-muted)] mt-1">{t('wol.broadcastHint')}</p>
              </div>
            )}
          </div>
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setBatchOpen(false)}>{t('common.cancel')}</Button>
            <Button onClick={doBatchWake} disabled={waking || !batchMacs.trim()}>
              {waking ? '...' : t('wol.wake')}
            </Button>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={handleConfirmDelete}
        message={confirmDelete ? confirmDeleteMessages[confirmDelete.kind] : ''}
      />
    </div>
  )
}
