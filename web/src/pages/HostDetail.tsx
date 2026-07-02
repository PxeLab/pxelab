import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Power, PowerOff, RefreshCw, Activity, Zap, HardDrive } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { Tag } from '../components/ui/Tag'
import { StatusDot } from '../components/ui/StatusDot'
import { useToast } from '../components/ui/Toast'
import { api, type Host, type Event, type InstallTask, type AnswerTemplate, type NetbootDistro } from '../api/client'

export default function HostDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [host, setHost] = useState<Host | null>(null)
  const [events, setEvents] = useState<Event[]>([])
  const [loading, setLoading] = useState(true)
  const [powerLoading, setPowerLoading] = useState<string | null>(null)

  // Install task state
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [templates, setTemplates] = useState<AnswerTemplate[]>([])
  const [hostTasks, setHostTasks] = useState<InstallTask[]>([])
  const [taskLoading, setTaskLoading] = useState(true)
  const [showCreateTask, setShowCreateTask] = useState(false)
  const [newTask, setNewTask] = useState<Partial<InstallTask>>({
    distro_name: '', version_codename: '', arch: '', extra_cmdline: '',
  })
  const [taskSaving, setTaskSaving] = useState(false)
  const [taskError, setTaskError] = useState('')
  const [confirmPower, setConfirmPower] = useState<string | null>(null)
  const [confirmDeleteTask, setConfirmDeleteTask] = useState<InstallTask | null>(null)

  useEffect(() => {
    if (!id) return
    async function load() {
      setTaskLoading(true)
      try {
        const [h, e, catRes, tmplRes, taskRes] = await Promise.all([
          api.getHost(id!),
          api.getEvents({ page: '1', size: '10' }),
          api.getNetbootCatalog(),
          api.getAnswerTemplates(),
          api.getInstallTasks(),
        ])
        setHost(h.data)
        setEvents(e.data.events)
        setDistros(catRes.data?.distros || [])
        setTemplates(tmplRes.data?.templates || [])
        setHostTasks((taskRes.data?.tasks || []).filter(t => t.host_id === id))
      } catch (err: any) {
        showError(err.message)
      } finally {
        setLoading(false)
        setTaskLoading(false)
      }
    }
    load()
  }, [id])

  async function handlePower(action: string) {
    if (!host) return
    if (action !== 'status') {
      setConfirmPower(action)
      return
    }
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

  async function doPower() {
    if (!host || !confirmPower) return
    setPowerLoading(confirmPower)
    try {
      const res = await api.powerHost(host.id, confirmPower)
      success( `${t('hosts.detail.power' + confirmPower)}: ${(res.data as any).status}`)
    } catch (err: any) {
      showError(err.message)
    } finally {
      setPowerLoading(null)
      setConfirmPower(null)
    }
  }

  if (loading) {
    return <div className="space-y-6">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 animate-shimmer h-32" />
      ))}
    </div>
  }

  if (!host) return <p className="text-[var(--text-muted)]">{t('common.notFound')}</p>

  const hostEvents = events.filter(e => e.mac === host.mac)

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-3">
          <Button variant="ghost" onClick={() => navigate('/hosts')}>
            <ArrowLeft size={16} /> {t('common.back')}
          </Button>
          <StatusDot color={host.last_online ? 'green' : 'red'} />
          <span className="text-lg font-bold">{host.name || t('common.unnamed')}</span>
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
              { label: 'Profile', value: host.profile_id || t('common.default') },
              { label: t('common.created'), value: new Date(host.created_at).toLocaleString(), mono: true },
            ].map((item, i) => (
              <div key={i}>
                <div className="text-[11px] font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-1">{item.label}</div>
                <div className={`text-sm font-medium text-[var(--text-primary)] ${item.mono ? 'font-mono text-xs' : ''}`}>{item.value}</div>
              </div>
            ))}
          </div>
        </Card>

        {/* IPMI Power Control */}
        <Card title={t('hosts.detail.powerControl')}>
          <div className="flex items-center gap-3 mb-4 px-3.5 py-2.5 bg-[var(--bg-input)] rounded-lg border border-[var(--bg-border)] text-sm">
            <StatusDot color={host.bmc_addr ? 'green' : 'yellow'} />
            <span className="text-[var(--text-secondary)]">BMC</span>
            <span className="font-mono text-xs text-[var(--text-muted)]">{host.bmc_addr || t('common.notConfigured')}</span>
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
                className={`flex flex-col items-center gap-2 py-4 rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] cursor-pointer transition-all hover:bg-[var(--bg-hover)] disabled:opacity-50 ${color}`}
              >
                <Icon size={22} />
                <span className="text-xs font-semibold text-[var(--text-secondary)]">{label}</span>
              </button>
            ))}
          </div>
        </Card>
      </div>

      {/* Boot History */}
      <div className="mt-5">
        <Card title={t('hosts.detail.bootHistory')}>
          {hostEvents.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">{t('events.noEvents')}</p>
          ) : (
            <div className="overflow-x-auto -mx-5">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('events.time')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('events.type')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('events.message')}</th>
                  </tr>
                </thead>
                <tbody>
                  {hostEvents.map((e, i) => (
                    <tr key={i}>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] font-mono text-xs text-[var(--text-muted)]">
                        {new Date(e.timestamp).toLocaleString()}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <Tag color={e.type.includes('dhcp') ? 'cyan' : e.type.includes('tftp') ? 'orange' : e.type.includes('boot') ? 'green' : 'blue'}>
                          {e.type}
                        </Tag>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-[var(--text-secondary)]">{e.message}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>

      {/* Install Task */}
      <div className="mt-5">
        <Card title={
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              <HardDrive size={16} />
              <span>{t('hosts.detail.installTasks')}</span>
            </div>
            <Button
              variant="primary"
              size="sm"
              onClick={() => {
                setShowCreateTask(true)
                setNewTask({ distro_name: '', version_codename: '', extra_cmdline: '' })
              }}
            >
              + {t('hosts.detail.assignTask')}
            </Button>
          </div>
        }>
          {taskLoading ? (
            <div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('common.loading')}</div>
          ) : hostTasks.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">{t('hosts.detail.noTasks')}</p>
          ) : (
            <div className="overflow-x-auto -mx-5">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.distro')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.version')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.status')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.extraParams')}</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.createdAt')}</th>
                    <th className="text-right px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">{t('hosts.detail.actions')}</th>
                  </tr>
                </thead>
                <tbody>
                  {hostTasks.map(task => (
                    <tr key={task.id} className="hover:bg-[var(--bg-hover)]/50">
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-sm text-[var(--text-primary)]">{task.distro_name}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)]">{task.version_codename}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <Tag color={task.status === 'done' ? 'green' : task.status === 'failed' ? 'red' : task.status === 'installing' ? 'orange' : 'blue'}>
                          {task.status}
                        </Tag>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs font-mono text-[var(--text-muted)] max-w-[200px] truncate">{task.extra_cmdline || '-'}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)]">{new Date(task.created_at!).toLocaleString()}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-right">
                        {task.status === 'pending' && (
                          <Button
                            variant="danger"
                            size="sm"
                            onClick={() => setConfirmDeleteTask(task)}
                          >
                            {t('common.delete')}
                          </Button>
                        )}
                        {task.status === 'failed' && task.error_msg && (
                          <span className="text-[10px] text-red-400/60" title={task.error_msg}>{t('common.error')}</span>
                        )}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Card>
      </div>

      {/* Create Task Modal */}
      <Modal
        open={showCreateTask && !!host}
        onClose={() => setShowCreateTask(false)}
        title={t('hosts.detail.assignTaskTitle')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowCreateTask(false)}>{t('common.cancel')}</Button>
            <Button
              variant="primary"
              size="sm"
              onClick={async () => {
                if (!host || !newTask.distro_name || !newTask.version_codename) {
                  setTaskError(t('hosts.detail.selectDistro'))
                  return
                }
                setTaskSaving(true)
                setTaskError('')
                try {
                  const res = await api.createInstallTask({
                    host_id: host.id,
                    distro_name: newTask.distro_name,
                    version_codename: newTask.version_codename,
                    arch: newTask.arch,
                    answer_template_id: newTask.answer_template_id || null,
                    extra_cmdline: newTask.extra_cmdline || '',
                  })
                  setHostTasks(prev => [res.data, ...prev])
                  setShowCreateTask(false)
                  success(t('hosts.detail.taskCreated'))
                } catch (err: any) {
                  setTaskError(err.message || t('hosts.detail.taskCreateFailed'))
                } finally {
                  setTaskSaving(false)
                }
              }}
              disabled={taskSaving || !newTask.distro_name || !newTask.version_codename}
            >
              {taskSaving ? t('hosts.detail.creating') : t('hosts.detail.createTask')}
            </Button>
          </>
        }
      >
        {taskError && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-xs text-red-400">{taskError}</div>
        )}

        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.distro')}</label>
            <select
              value={newTask.distro_name || ''}
              onChange={e => setNewTask({ ...newTask, distro_name: e.target.value, version_codename: '' })}
              className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
            >
              <option value="">{t('hosts.detail.selectDistro')}</option>
              {distros.filter(d => d.enabled).map(d => (
                <option key={d.name} value={d.name}>{d.name}</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.version')}</label>
            <select
              value={newTask.version_codename || ''}
              onChange={e => {
                const codename = e.target.value
                const distro = distros.find(d => d.name === newTask.distro_name)
                const ver = distro?.versions.find(v => v.codename === codename)
                setNewTask({ ...newTask, version_codename: codename, arch: ver?.arch || '' })
              }}
              className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
            >
              <option value="">{t('hosts.detail.selectVersion')}</option>
              {distros
                .filter(d => d.name === newTask.distro_name)
                .flatMap(d => d.versions)
                .filter(v => v.enabled)
                .map(v => (
                  <option key={v.codename} value={v.codename}>{v.name} ({v.arch})</option>
                ))
              }
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.answerTemplate')}</label>
            <select
              value={newTask.answer_template_id || ''}
              onChange={e => setNewTask({ ...newTask, answer_template_id: e.target.value ? Number(e.target.value) : null })}
              className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
            >
              <option value="">{t('hosts.detail.none')}</option>
              {templates.map(t => (
                <option key={t.id} value={t.id}>{t.name} ({t.type})</option>
              ))}
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.extraKernelParams')}</label>
            <input
              type="text"
              value={newTask.extra_cmdline || ''}
              onChange={e => setNewTask({ ...newTask, extra_cmdline: e.target.value })}
              placeholder="net.ifnames=0 biosdevname=0"
              className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
            />
          </div>
        </div>
      </Modal>
      <ConfirmDialog
        open={!!confirmPower}
        onClose={() => setConfirmPower(null)}
        onConfirm={doPower}
        title={t('hosts.detail.powerConfirm')}
        message={`${t('hosts.detail.power' + confirmPower)}?`}
      />
      <ConfirmDialog
        open={!!confirmDeleteTask}
        onClose={() => setConfirmDeleteTask(null)}
        onConfirm={async () => {
          if (!confirmDeleteTask) return
          try {
            await api.deleteInstallTask(confirmDeleteTask.id!)
            setHostTasks(prev => prev.filter(t => t.id !== confirmDeleteTask.id))
            success(t('hosts.detail.taskDeleted'))
          } catch { showError(t('hosts.detail.taskDeleteFailed')) }
          setConfirmDeleteTask(null)
        }}
        title={t('hosts.detail.deleteTaskConfirm')}
        message={t('hosts.detail.deleteTaskMessage')}
      />
    </div>
  )
}
