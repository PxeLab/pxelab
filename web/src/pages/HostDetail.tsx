import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Power, PowerOff, RefreshCw, Activity, HardDrive, Wifi, FileText } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { Tag } from '../components/ui/Tag'
import { StatusDot } from '../components/ui/StatusDot'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import { Toggle } from '../components/ui/Toggle'
import { ChecklistPicker } from '../components/ui/ChecklistPicker'
import { DataTable } from '../components/ui/DataTable'
import { api, type Host, type Event, type InstallTask, type AnswerTemplate, type NetbootDistro, type WOLHistoryRecord, type Profile, type Baseline, type scriptDTO } from '../api/client'

// 电源操作 action → i18n key 显式映射（key 不是 action 的简单拼接）
const powerLabelKeys: Record<string, string> = {
  on: 'powerOn',
  off: 'powerOff',
  cycle: 'powerRestart',
  status: 'powerStatus',
}

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

  // Boot config preview state
  const [showBootConfig, setShowBootConfig] = useState(false)
  const [bootConfigFormat, setBootConfigFormat] = useState('pxelinux')
  const [bootConfigContent, setBootConfigContent] = useState('')
  const [bootConfigLoading, setBootConfigLoading] = useState(false)
  const [bootConfigError, setBootConfigError] = useState('')

  // Edit host state
  const [showEdit, setShowEdit] = useState(false)
  const [editForm, setEditForm] = useState<{
    name: string; mac: string; ip: string; sn: string; profile_id: string
    bmc_addr: string; bmc_user: string
    baseline_ids: string[]; script_ids: number[]
  }>({ name: '', mac: '', ip: '', sn: '', profile_id: '', bmc_addr: '', bmc_user: '', baseline_ids: [], script_ids: [] })
  const [editSaving, setEditSaving] = useState(false)
  const [editShowBaselines, setEditShowBaselines] = useState(false)
  const [editShowScripts, setEditShowScripts] = useState(false)
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [allBaselines, setAllBaselines] = useState<Baseline[]>([])
  const [allScripts, setAllScripts] = useState<scriptDTO[]>([])

  function toggleArr<T>(arr: T[], v: T): T[] {
    return arr.includes(v) ? arr.filter(x => x !== v) : [...arr, v]
  }

  // WOL state
  const [wolHistory, setWolHistory] = useState<WOLHistoryRecord[]>([])
  const [wakingWOL, setWakingWOL] = useState(false)
  const [wolHistoryLoaded, setWolHistoryLoaded] = useState(false)

  useEffect(() => {
    if (!id) return
    async function load() {
      setTaskLoading(true)
      try {
        const [h, catRes, tmplRes, taskRes] = await Promise.all([
          api.getHost(id!),
          api.getNetbootCatalog(),
          api.getAnswerTemplates(),
          api.getInstallTasks(),
        ])
        setHost(h.data)
        setDistros(catRes.data?.distros || [])
        setTemplates(tmplRes.data?.templates || [])
        setHostTasks((taskRes.data?.tasks || []).filter(t => t.host_id === id))
        // 启动历史按主机 MAC 过滤，避免全局最近 10 条中没有该主机时显示为空
        const e = await api.getEvents({ page: '1', size: '10', mac: h.data.mac })
        setEvents(e.data.events)
      } catch (err: any) {
        showError(err.message)
      } finally {
        setLoading(false)
        setTaskLoading(false)
      }
    }
    load()
  }, [id])

  useEffect(() => {
    api.getProfiles().then(res => setProfiles(res.data)).catch(() => {})
    api.getBaselines().then(res => setAllBaselines(res.data ?? [])).catch(() => {})
    api.getScripts().then(res => setAllScripts(res.data ?? [])).catch(() => {})
  }, [])

  async function handlePower(action: string) {
    if (!host) return
    if (action !== 'status') {
      setConfirmPower(action)
      return
    }
    setPowerLoading(action)
    try {
      const res = await api.powerHost(host.id, action)
      success(`${t('hosts.detail.' + powerLabelKeys[action])}: ${(res.data as any).status}`)
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
      success(`${t('hosts.detail.' + powerLabelKeys[confirmPower])}: ${(res.data as any).status}`)
    } catch (err: any) {
      showError(err.message)
    } finally {
      setPowerLoading(null)
      setConfirmPower(null)
    }
  }

  async function doWOLWake() {
    if (!host) return
    setWakingWOL(true)
    try {
      await api.wakeHost(host.id)
      success(t('hosts.detail.wolSent', { mac: host.mac }))
      loadWOLHistory()
    } catch (e: any) {
      showError(e?.message || t('hosts.detail.wolFailed'))
    }
    setWakingWOL(false)
  }

  async function loadWOLHistory() {
    if (!host) return
    try {
      const res = await api.getWOLHistoryByMAC(host.mac)
      setWolHistory(res.data.records)
      setWolHistoryLoaded(true)
    } catch { /* ignore */ }
  }

  async function handlePreviewBootConfig() {
    if (!host) return
    setBootConfigLoading(true)
    setBootConfigError('')
    setBootConfigContent('')
    try {
      const text = await api.getHostBootConfig(host.id, bootConfigFormat)
      setBootConfigContent(text)
      setShowBootConfig(true)
    } catch (e: any) {
      setBootConfigError(e?.message || t('hosts.detail.bootConfigFailed'))
      setShowBootConfig(true)
    } finally {
      setBootConfigLoading(false)
    }
  }

  async function handleSaveEdit() {
    if (!host) return
    setEditSaving(true)
    try {
      const res = await api.updateHost(host.id, {
        ...host,
        name: editForm.name,
        mac: editForm.mac,
        ip: editForm.ip,
        sn: editForm.sn || undefined,
        profile_id: editForm.profile_id || undefined,
        bmc_addr: editForm.bmc_addr,
        bmc_user: editForm.bmc_user,
        baseline_ids: editForm.baseline_ids,
        script_ids: editForm.script_ids,
      })
      setHost(res.data)
      setShowEdit(false)
      success(t('hosts.updated'))
    } catch (err: any) {
      showError(err.message)
    } finally {
      setEditSaving(false)
    }
  }

  // 初始化脚本选择（P1）：Profile 继承的基线只读展示，主机可再追加
  const editProfile = profiles.find(p => p.id === editForm.profile_id)
  const editInherited: string[] = editProfile?.baselines ?? []

  if (loading) {
    return <div className="space-y-6">
      {Array.from({ length: 3 }).map((_, i) => (
        <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 animate-shimmer h-32" />
      ))}
    </div>
  }

  if (!host) return <p className="text-[var(--text-muted)]">{t('common.notFound')}</p>

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
          <Button variant="secondary" size="sm" onClick={handlePreviewBootConfig} disabled={bootConfigLoading}>
            <FileText size={14} /> {bootConfigLoading ? '...' : t('hosts.detail.bootConfig')}
          </Button>
          <Button variant="secondary" size="sm" onClick={doWOLWake} disabled={wakingWOL}>
            <Wifi size={14} /> {wakingWOL ? '...' : t('hosts.detail.wolWake')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => {
            setEditForm({
              name: host.name || '',
              mac: host.mac || '',
              ip: host.ip || '',
              sn: host.sn || '',
              profile_id: host.profile_id || '',
              bmc_addr: host.bmc_addr || '',
              bmc_user: host.bmc_user || '',
              baseline_ids: host.baseline_ids ?? [],
              script_ids: host.script_ids ?? [],
            })
            setEditShowBaselines((host.baseline_ids ?? []).length > 0)
            setEditShowScripts((host.script_ids ?? []).length > 0)
            setShowEdit(true)
          }}>
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
              { action: 'on', label: t('hosts.detail.powerOn'), icon: Power, color: 'text-accent-green hover:border-accent-green/30' },
              { action: 'off', label: t('hosts.detail.powerOff'), icon: PowerOff, color: 'text-accent-red hover:border-accent-red/30' },
              { action: 'cycle', label: t('hosts.detail.powerRestart'), icon: RefreshCw, color: 'text-accent-yellow hover:border-accent-yellow/30' },
              { action: 'status', label: t('hosts.detail.powerStatus'), icon: Activity, color: 'text-blue-400 hover:border-blue-500/30' },
            ].map(({ action, label, icon: Icon, color }) => {
              const bmcMissing = !host.bmc_addr
              return (
                <button
                  key={action}
                  onClick={() => handlePower(action)}
                  disabled={powerLoading === action || bmcMissing}
                  title={bmcMissing ? t('hosts.detail.bmcNotConfigured', 'BMC 未配置') : undefined}
                  className={`flex flex-col items-center gap-2 py-4 rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] cursor-pointer transition-all hover:bg-[var(--bg-hover)] disabled:opacity-40 disabled:cursor-not-allowed ${bmcMissing ? '' : color}`}
                >
                  <Icon size={22} />
                  <span className="text-xs font-semibold text-[var(--text-secondary)]">{label}</span>
                </button>
              )
            })}
          </div>
        </Card>
      </div>

      {/* Boot History */}
      <div className="mt-5">
        <Card title={t('hosts.detail.bootHistory')}>
          <div className="-mx-5">
            <DataTable
              columns={[
                {
                  key: 'time',
                  label: t('events.time'),
                  render: (e: Event) => (
                    <span className="font-mono text-xs text-[var(--text-muted)]">{new Date(e.timestamp).toLocaleString()}</span>
                  ),
                },
                {
                  key: 'type',
                  label: t('events.type'),
                  render: (e: Event) => (
                    <Tag color={e.type.includes('dhcp') ? 'cyan' : e.type.includes('tftp') ? 'orange' : e.type.includes('boot') ? 'green' : 'blue'}>
                      {e.type}
                    </Tag>
                  ),
                },
                {
                  key: 'message',
                  label: t('events.message'),
                  render: (e: Event) => <span className="text-[var(--text-secondary)]">{e.message}</span>,
                },
              ]}
              data={events}
              emptyText={t('events.noEvents')}
            />
          </div>
        </Card>
      </div>

      {/* WOL History */}
      <div className="mt-5">
        <Card title={
          <div className="flex items-center justify-between w-full">
            <div className="flex items-center gap-2">
              <Wifi size={16} />
              <span>{t('hosts.detail.wolHistory')}</span>
            </div>
            <Button variant="secondary" size="sm" onClick={loadWOLHistory}>
              <RefreshCw size={12} className="mr-1" /> {t('common.refresh')}
            </Button>
          </div>
        }>
          {!wolHistoryLoaded ? (
            <div className="py-8 text-center text-sm text-[var(--text-muted)]">
              <Button variant="ghost" size="sm" onClick={loadWOLHistory}>{t('hosts.detail.loadWOLHistory')}</Button>
            </div>
          ) : (
            <div className="-mx-5">
              <DataTable
                columns={[
                  {
                    key: 'broadcast',
                    label: t('wol.broadcast'),
                    render: (r: WOLHistoryRecord) => <span className="font-mono text-xs text-[var(--text-secondary)]">{r.broadcast}</span>,
                  },
                  {
                    key: 'source',
                    label: t('wol.source'),
                    render: (r: WOLHistoryRecord) => <span className="font-mono text-xs text-[var(--text-secondary)]">{r.source_ip || '-'}</span>,
                  },
                  {
                    key: 'status',
                    label: t('wol.status'),
                    render: (r: WOLHistoryRecord) => (
                      <Tag color={r.success ? 'green' : 'red'}>{r.success ? t('wol.success') : (r.error_msg || t('wol.failed'))}</Tag>
                    ),
                  },
                  {
                    key: 'time',
                    label: t('wol.time'),
                    render: (r: WOLHistoryRecord) => <span className="font-mono text-xs text-[var(--text-muted)]">{new Date(r.created_at).toLocaleString()}</span>,
                  },
                ]}
                data={wolHistory}
                emptyText={t('hosts.detail.noWOLHistory')}
                rowKey={(r: WOLHistoryRecord) => String(r.id)}
              />
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
          <div className="-mx-5">
            <DataTable
              columns={[
                {
                  key: 'distro',
                  label: t('hosts.detail.distro'),
                  render: (task: InstallTask) => <span className="text-sm text-[var(--text-primary)]">{task.distro_name}</span>,
                },
                {
                  key: 'version',
                  label: t('hosts.detail.version'),
                  render: (task: InstallTask) => <span className="text-xs text-[var(--text-muted)]">{task.version_codename}</span>,
                },
                {
                  key: 'status',
                  label: t('hosts.detail.status'),
                  render: (task: InstallTask) => (
                    <Tag color={task.status === 'done' ? 'green' : task.status === 'failed' ? 'red' : task.status === 'installing' ? 'orange' : 'blue'}>
                      {task.status}
                    </Tag>
                  ),
                },
                {
                  key: 'extra',
                  label: t('hosts.detail.extraParams'),
                  render: (task: InstallTask) => (
                    <span className="block text-xs font-mono text-[var(--text-muted)] max-w-[200px] truncate">{task.extra_cmdline || '-'}</span>
                  ),
                },
                {
                  key: 'created',
                  label: t('hosts.detail.createdAt'),
                  render: (task: InstallTask) => <span className="text-xs text-[var(--text-muted)]">{new Date(task.created_at!).toLocaleString()}</span>,
                },
                {
                  key: 'actions',
                  label: t('hosts.detail.actions'),
                  render: (task: InstallTask) => (
                    <div className="text-right">
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
                        <span className="text-[10px] text-accent-red/60" title={task.error_msg}>{t('common.error')}</span>
                      )}
                    </div>
                  ),
                },
              ]}
              data={hostTasks}
              loading={taskLoading}
              emptyText={t('hosts.detail.noTasks')}
              rowKey={(task: InstallTask) => String(task.id)}
            />
          </div>
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
          <div className="mb-4 px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">{taskError}</div>
        )}

        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.distro')}</label>
            <Select
              size="sm"
              value={newTask.distro_name || ''}
              onChange={e => setNewTask({ ...newTask, distro_name: e.target.value, version_codename: '' })}
            >
              <option value="">{t('hosts.detail.selectDistro')}</option>
              {distros.filter(d => d.enabled).map(d => (
                <option key={d.name} value={d.name}>{d.name}</option>
              ))}
            </Select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.version')}</label>
            <Select
              size="sm"
              value={newTask.version_codename || ''}
              onChange={e => {
                const codename = e.target.value
                const distro = distros.find(d => d.name === newTask.distro_name)
                const ver = distro?.versions.find(v => v.codename === codename)
                setNewTask({ ...newTask, version_codename: codename, arch: ver?.arch || '' })
              }}
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
            </Select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.answerTemplate')}</label>
            <Select
              size="sm"
              value={newTask.answer_template_id || ''}
              onChange={e => setNewTask({ ...newTask, answer_template_id: e.target.value ? Number(e.target.value) : null })}
            >
              <option value="">{t('hosts.detail.none')}</option>
              {templates.map(t => (
                <option key={t.id} value={t.id}>{t.name} ({t.type})</option>
              ))}
            </Select>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.extraKernelParams')}</label>
            <Input
              size="sm"
              type="text"
              value={newTask.extra_cmdline || ''}
              onChange={e => setNewTask({ ...newTask, extra_cmdline: e.target.value })}
              placeholder="net.ifnames=0 biosdevname=0"
            />
          </div>
        </div>
      </Modal>

      {/* Boot Config Preview Modal */}
      <Modal open={showBootConfig} onClose={() => setShowBootConfig(false)} title={t('hosts.detail.bootConfig')}>
        <div className="space-y-4">
          <div className="flex gap-2 items-center">
            <select
              value={bootConfigFormat}
              onChange={e => setBootConfigFormat(e.target.value)}
              className="px-3 py-1.5 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
            >
              <option value="pxelinux">pxelinux</option>
              <option value="grub2">grub2</option>
            </select>
            <Button size="sm" variant="primary" onClick={handlePreviewBootConfig} disabled={bootConfigLoading}>
              {bootConfigLoading ? '...' : t('hosts.detail.bootConfigRefresh')}
            </Button>
          </div>

          {bootConfigError && (
            <div className="px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">
              {bootConfigError}
            </div>
          )}

          {bootConfigContent && (
            <pre className="p-4 rounded-lg bg-[var(--bg-card)] border border-[var(--bg-border)] text-xs font-mono text-[var(--text-primary)] overflow-x-auto max-h-[500px] overflow-y-auto whitespace-pre">
              {bootConfigContent}
            </pre>
          )}

          {!bootConfigContent && !bootConfigError && !bootConfigLoading && (
            <div className="py-8 text-center text-sm text-[var(--text-muted)]">
              {t('hosts.detail.bootConfigHint')}
            </div>
          )}
        </div>
      </Modal>

      {/* Edit Host Modal */}
      <Modal
        open={showEdit && !!host}
        onClose={() => setShowEdit(false)}
        title={t('hosts.editHost')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowEdit(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleSaveEdit} disabled={editSaving}>
              {editSaving ? t('common.processing') : t('common.save')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.mac')}</label>
            <Input placeholder="00:11:22:33:44:55" value={editForm.mac} onChange={e => setEditForm({ ...editForm, mac: e.target.value })} />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.hostname')}</label>
            <Input placeholder="node-01" value={editForm.name} onChange={e => setEditForm({ ...editForm, name: e.target.value })} />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('hosts.columns.ip')}</label>
              <Input placeholder="192.168.1.100" value={editForm.ip} onChange={e => setEditForm({ ...editForm, ip: e.target.value })} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('profiles.title')}</label>
              <Select
                size="sm"
                value={editForm.profile_id}
                onChange={e => setEditForm({ ...editForm, profile_id: e.target.value })}
              >
                <option value="">—</option>
                {profiles.map(p => (
                  <option key={p.id} value={p.id}>{p.name}</option>
                ))}
              </Select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('bmc.addr')}</label>
              <Input placeholder="192.168.1.10" value={editForm.bmc_addr} onChange={e => setEditForm({ ...editForm, bmc_addr: e.target.value })} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('bmc.username')}</label>
              <Input placeholder="admin" value={editForm.bmc_user} onChange={e => setEditForm({ ...editForm, bmc_user: e.target.value })} />
            </div>
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">SN</label>
            <Input placeholder={t('hosts.snPlaceholder', 'Serial Number（可选，配合 global.identity_attr=sn 使用）')} value={editForm.sn} onChange={e => setEditForm({ ...editForm, sn: e.target.value })} />
          </div>

          {/* 初始化脚本：勾选即装机后自动执行 */}
          <div className="rounded-lg border border-[var(--bg-border)] p-3 space-y-3">
            <div>
              <p className="text-xs font-semibold text-[var(--text-primary)]">{t('hosts.initSection')}</p>
              <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('hosts.initHint')}</p>
            </div>
            {editInherited.length > 0 && (
              <div className="flex flex-wrap items-center gap-1.5">
                <span className="text-[10px] uppercase tracking-wide text-[var(--text-muted)]">{t('hosts.inheritedBaselines')}</span>
                {editInherited.map(id => {
                  const b = allBaselines.find(x => x.id === id)
                  return (
                    <span key={id} className="px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 text-[10px] font-medium">{b ? b.name : id}</span>
                  )
                })}
              </div>
            )}
            <div>
              <Toggle
                checked={editShowBaselines}
                onChange={v => {
                  setEditShowBaselines(v)
                  if (!v) setEditForm({ ...editForm, baseline_ids: [] })
                }}
                label={t('hosts.addBaselines')}
              />
              {editShowBaselines && (
                <div className="mt-1.5">
                  <ChecklistPicker
                    items={allBaselines.map(b => ({ id: b.id, name: b.name }))}
                    selected={editForm.baseline_ids}
                    disabledIds={editInherited}
                    onToggle={id => setEditForm({ ...editForm, baseline_ids: toggleArr(editForm.baseline_ids, id) })}
                    emptyText={t('baselines.noBaselines')}
                  />
                </div>
              )}
            </div>
            <div>
              <Toggle
                checked={editShowScripts}
                onChange={v => {
                  setEditShowScripts(v)
                  if (!v) setEditForm({ ...editForm, script_ids: [] })
                }}
                label={t('hosts.addScripts')}
              />
              {editShowScripts && (
                <div className="mt-1.5">
                  <ChecklistPicker
                    items={allScripts.map(sc => ({ id: sc.id, name: sc.name, badge: sc.type }))}
                    selected={editForm.script_ids}
                    onToggle={id => setEditForm({ ...editForm, script_ids: toggleArr(editForm.script_ids, id) })}
                    emptyText={t('scripts.noScripts')}
                  />
                </div>
              )}
            </div>
          </div>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!confirmPower}
        onClose={() => setConfirmPower(null)}
        onConfirm={doPower}
        title={t('hosts.detail.powerConfirm')}
        message={confirmPower ? `${t('hosts.detail.' + powerLabelKeys[confirmPower])}?` : ''}
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
