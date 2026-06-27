import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Power, PowerOff, RefreshCw, Activity, Zap, HardDrive } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
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
        <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[var(--bg-card)] via-[var(--bg-hover)] to-[var(--bg-card)] bg-[length:200%_100%] h-32" />
      ))}
    </div>
  }

  if (!host) return <p className="text-[var(--text-muted)]">{t('common.notFound', '未找到')}</p>

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
            <span className="font-mono text-xs text-[var(--text-muted)]">{host.bmc_addr || t('common.notConfigured', '未配置')}</span>
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
              <span>安装任务</span>
            </div>
            <button
              onClick={() => {
                setShowCreateTask(true)
                setNewTask({ distro_name: '', version_codename: '', extra_cmdline: '' })
              }}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 transition-colors"
            >
              + 分配任务
            </button>
          </div>
        }>
          {taskLoading ? (
            <div className="py-8 text-center text-sm text-[var(--text-muted)]">加载中...</div>
          ) : hostTasks.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">暂无安装任务</p>
          ) : (
            <div className="overflow-x-auto -mx-5">
              <table className="w-full text-sm">
                <thead>
                  <tr>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">发行版</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">版本</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">状态</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">额外参数</th>
                    <th className="text-left px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">创建时间</th>
                    <th className="text-right px-4 py-2 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">操作</th>
                  </tr>
                </thead>
                <tbody>
                  {hostTasks.map(task => (
                    <tr key={task.id} className="hover:bg-white/[0.02]">
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
                          <button
                            onClick={async () => {
                              if (!confirm('确定删除此任务？')) return
                              try {
                                await api.deleteInstallTask(task.id!)
                                setHostTasks(prev => prev.filter(t => t.id !== task.id))
                                success('任务已删除')
                              } catch { showError('删除失败') }
                            }}
                            className="px-2 py-1 text-xs font-medium rounded bg-red-500/10 text-red-400 hover:bg-red-500/20"
                          >
                            删除
                          </button>
                        )}
                        {task.status === 'failed' && task.error_msg && (
                          <span className="text-[10px] text-red-400/60" title={task.error_msg}>失败</span>
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
      {showCreateTask && host && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowCreateTask(false)}>
          <div className="max-w-lg w-full mx-4 rounded-xl bg-[var(--bg-card)] border border-[var(--bg-border)] p-5" onClick={e => e.stopPropagation()}>
            <h3 className="text-sm font-bold text-[var(--text-primary)] mb-4">分配安装任务</h3>

            {taskError && (
              <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-xs text-red-400">{taskError}</div>
            )}

            <div className="space-y-4">
              {/* Distro select */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">发行版</label>
                <select
                  value={newTask.distro_name || ''}
                  onChange={e => {
                    setNewTask({ ...newTask, distro_name: e.target.value, version_codename: '' })
                  }}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                >
                  <option value="">请选择发行版</option>
                  {distros.filter(d => d.enabled).map(d => (
                    <option key={d.name} value={d.name}>{d.name}</option>
                  ))}
                </select>
              </div>

              {/* Version select */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">版本</label>
                <select
                  value={newTask.version_codename || ''}
                  onChange={e => {
                    const codename = e.target.value
                    // Look up arch from selected distro's versions
                    const distro = distros.find(d => d.name === newTask.distro_name)
                    const ver = distro?.versions.find(v => v.codename === codename)
                    setNewTask({ ...newTask, version_codename: codename, arch: ver?.arch || '' })
                  }}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                >
                  <option value="">请选择版本</option>
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

              {/* Answer template select */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">应答模板（可选）</label>
                <select
                  value={newTask.answer_template_id || ''}
                  onChange={e => setNewTask({ ...newTask, answer_template_id: e.target.value ? Number(e.target.value) : null })}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                >
                  <option value="">无</option>
                  {templates.map(t => (
                    <option key={t.id} value={t.id}>{t.name} ({t.type})</option>
                  ))}
                </select>
              </div>

              {/* Extra cmdline */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">额外 Kernel 参数</label>
                <input
                  type="text"
                  value={newTask.extra_cmdline || ''}
                  onChange={e => setNewTask({ ...newTask, extra_cmdline: e.target.value })}
                  placeholder="net.ifnames=0 biosdevname=0"
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                />
              </div>
            </div>

            <div className="flex justify-end gap-2 mt-5">
              <button onClick={() => setShowCreateTask(false)} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)]">
                取消
              </button>
              <button
                onClick={async () => {
                  if (!newTask.distro_name || !newTask.version_codename) {
                    setTaskError('请选择发行版和版本')
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
                    success('安装任务已创建')
                  } catch (err: any) {
                    setTaskError(err.message || '创建失败')
                  } finally {
                    setTaskSaving(false)
                  }
                }}
                disabled={taskSaving || !newTask.distro_name || !newTask.version_codename}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors"
              >
                {taskSaving ? '创建中...' : '创建任务'}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
