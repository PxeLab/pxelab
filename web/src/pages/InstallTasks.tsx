import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { HardDrive, Plus, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { api, type InstallTask, type Host, type NetbootDistro, type AnswerTemplate } from '../api/client'

export default function InstallTasks() {
  const navigate = useNavigate()
  const { success, error: showError } = useToast()

  const [tasks, setTasks] = useState<InstallTask[]>([])
  const [hosts, setHosts] = useState<Host[]>([])
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [templates, setTemplates] = useState<AnswerTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  const [showCreate, setShowCreate] = useState(false)
  const [newTask, setNewTask] = useState<Partial<InstallTask>>({ distro_name: '', version_codename: '', extra_cmdline: '' })
  const [taskSaving, setTaskSaving] = useState(false)
  const [taskError, setTaskError] = useState('')

  // Build a host lookup map
  const hostMap = new Map<string, Host>()
  hosts.forEach(h => hostMap.set(h.id, h))

  useEffect(() => {
    async function load() {
      setLoading(true)
      setLoadError('')
      try {
        const [taskRes, hostRes, catRes, tmplRes] = await Promise.all([
          api.getInstallTasks(),
          api.getHosts({ page: '1', size: '9999' }),
          api.getNetbootCatalog(),
          api.getAnswerTemplates(),
        ])
        setTasks(taskRes.data?.tasks || [])
        setHosts(hostRes.data?.hosts || [])
        setDistros(catRes.data?.distros || [])
        setTemplates(tmplRes.data?.templates || [])
      } catch (err: any) {
        setLoadError(err.message || '加载失败')
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  async function handleDelete(task: InstallTask) {
    if (!confirm('确定删除此任务？')) return
    try {
      await api.deleteInstallTask(task.id!)
      setTasks(prev => prev.filter(t => t.id !== task.id))
      success('任务已删除')
    } catch (err: any) {
      showError(err.message || '删除失败')
    }
  }

  async function handleCreate() {
    if (!newTask.host_id || !newTask.distro_name || !newTask.version_codename) {
      setTaskError('请选择主机、发行版和版本')
      return
    }
    setTaskSaving(true)
    setTaskError('')
    try {
      const res = await api.createInstallTask({
        host_id: newTask.host_id,
        distro_name: newTask.distro_name,
        version_codename: newTask.version_codename,
        arch: newTask.arch,
        answer_template_id: newTask.answer_template_id || null,
        extra_cmdline: newTask.extra_cmdline || '',
      })
      setTasks(prev => [res.data, ...prev])
      setShowCreate(false)
      success('安装任务已创建')
    } catch (err: any) {
      setTaskError(err.message || '创建失败')
    } finally {
      setTaskSaving(false)
    }
  }

  return (
    <div className="space-y-5">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">安装任务</h1>
        <button
          onClick={() => {
            setNewTask({ distro_name: '', version_codename: '', extra_cmdline: '' })
            setShowCreate(true)
          }}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 transition-colors"
        >
          <Plus size={14} />
          新建任务
        </button>
      </div>

      {/* Error state */}
      {loadError && (
        <Card>
          <div className="py-8 text-center text-sm text-red-400">{loadError}</div>
        </Card>
      )}

      {/* Loading state */}
      {loading && (
        <Card padding={false}>
          <div className="p-6 space-y-3">
            {[1,2,3,4].map(i => (
              <div key={i} className="h-10 rounded bg-white/[0.03] animate-pulse" />
            ))}
          </div>
        </Card>
      )}

      {/* Empty state */}
      {!loading && !loadError && tasks.length === 0 && (
        <Card>
          <div className="py-12 text-center">
            <HardDrive size={32} className="mx-auto mb-3 text-[var(--text-muted)] opacity-40" />
            <p className="text-sm text-[var(--text-muted)]">暂无安装任务</p>
          </div>
        </Card>
      )}

      {/* Table */}
      {!loading && !loadError && tasks.length > 0 && (
        <Card padding={false}>
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">主机</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">MAC 地址</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">发行版</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">版本</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">架构</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">状态</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">应答模板</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">额外参数</th>
                  <th className="text-left px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">创建时间</th>
                  <th className="text-right px-4 py-2.5 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">操作</th>
                </tr>
              </thead>
              <tbody>
                {tasks.map(task => {
                  const host = task.host_id ? hostMap.get(task.host_id) : undefined
                  const tmpl = task.answer_template_id
                    ? templates.find(t => t.id === task.answer_template_id)
                    : undefined
                  return (
                    <tr key={task.id} className="hover:bg-white/[0.02]">
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        {host ? (
                          <button
                            onClick={() => navigate(`/hosts/${host.id}`)}
                            className="text-sm font-medium text-blue-400 hover:text-blue-300 transition-colors"
                          >
                            {host.name || host.id}
                          </button>
                        ) : (
                          <span className="text-sm text-[var(--text-muted)]">{task.host_id}</span>
                        )}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs font-mono text-[var(--text-muted)]">
                        {host?.mac || '-'}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-sm text-[var(--text-primary)]">{task.distro_name}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)]">{task.version_codename}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)]">{task.arch || '-'}</td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                        <Tag color={task.status === 'done' ? 'green' : task.status === 'failed' ? 'red' : task.status === 'installing' ? 'orange' : 'blue'}>
                          {task.status}
                        </Tag>
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)]">
                        {tmpl ? `${tmpl.name} (${tmpl.type})` : '-'}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs font-mono text-[var(--text-muted)] max-w-[160px] truncate">
                        {task.extra_cmdline || '-'}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-xs text-[var(--text-muted)] whitespace-nowrap">
                        {task.created_at ? new Date(task.created_at).toLocaleString() : '-'}
                      </td>
                      <td className="px-4 py-3 border-b border-[var(--bg-border)] text-right">
                        {task.status === 'pending' && (
                          <button
                            onClick={() => handleDelete(task)}
                            className="inline-flex items-center gap-1 px-2 py-1 text-xs font-medium rounded bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors"
                          >
                            <Trash2 size={12} />
                            删除
                          </button>
                        )}
                        {task.status === 'failed' && task.error_msg && (
                          <span className="text-[10px] text-red-400/60" title={task.error_msg}>失败</span>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </Card>
      )}

      {/* Create Task Modal */}
      {showCreate && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowCreate(false)}>
          <div className="max-w-lg w-full mx-4 rounded-xl bg-[var(--bg-card)] border border-[var(--bg-border)] p-5" onClick={e => e.stopPropagation()}>
            <h3 className="text-sm font-bold text-[var(--text-primary)] mb-4">新建安装任务</h3>

            {taskError && (
              <div className="mb-4 px-3 py-2 rounded-lg bg-red-500/10 border border-red-500/20 text-xs text-red-400">{taskError}</div>
            )}

            <div className="space-y-4">
              {/* Host select */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">目标主机</label>
                <select
                  value={newTask.host_id || ''}
                  onChange={e => setNewTask({ ...newTask, host_id: e.target.value })}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                >
                  <option value="">请选择主机</option>
                  {hosts.map(h => (
                    <option key={h.id} value={h.id}>{h.name || h.id} ({h.mac})</option>
                  ))}
                </select>
              </div>

              {/* Distro select */}
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">发行版</label>
                <select
                  value={newTask.distro_name || ''}
                  onChange={e => setNewTask({ ...newTask, distro_name: e.target.value, version_codename: '' })}
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
                  value={newTask.answer_template_id ?? ''}
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
              <button
                onClick={() => setShowCreate(false)}
                className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)]"
              >
                取消
              </button>
              <button
                onClick={handleCreate}
                disabled={taskSaving || !newTask.host_id || !newTask.distro_name || !newTask.version_codename}
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
