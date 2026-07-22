import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { HardDrive, Plus, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type InstallTask, type Host, type NetbootDistro, type AnswerTemplate } from '../api/client'

export default function InstallTasks() {
  const navigate = useNavigate()
  const { t } = useTranslation()
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
  const [confirmDeleteTask, setConfirmDeleteTask] = useState<InstallTask | null>(null)

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
        setLoadError(err.message || t('installTasks.loadFailed'))
      } finally {
        setLoading(false)
      }
    }
    load()
  }, [])

  async function handleDelete(task: InstallTask) {
    setConfirmDeleteTask(task)
  }

  async function doDeleteTask() {
    if (!confirmDeleteTask) return
    try {
      await api.deleteInstallTask(confirmDeleteTask.id!)
      setTasks(prev => prev.filter(t => t.id !== confirmDeleteTask.id))
      success(t('installTasks.deleted'))
    } catch (err: any) {
      showError(err.message || t('installTasks.deleteFailed'))
    } finally {
      setConfirmDeleteTask(null)
    }
  }

  async function handleCreate() {
    if (!newTask.host_id || !newTask.distro_name || !newTask.version_codename) {
      setTaskError(t('installTasks.selectRequired'))
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
      success(t('installTasks.created'))
    } catch (err: any) {
      setTaskError(err.message || t('installTasks.createFailed'))
    } finally {
      setTaskSaving(false)
    }
  }

  const columns: Column<InstallTask>[] = [
    {
      key: 'host',
      label: t('installTasks.colHost'),
      render: task => {
        const host = task.host_id ? hostMap.get(task.host_id) : undefined
        return host ? (
          <button
            onClick={() => navigate(`/hosts/${host.id}`)}
            className="text-sm font-medium text-blue-400 hover:text-blue-300 transition-colors"
          >
            {host.name || host.id}
          </button>
        ) : (
          <span className="text-sm text-[var(--text-muted)]">{task.host_id}</span>
        )
      },
    },
    {
      key: 'mac',
      label: t('installTasks.colMac'),
      render: task => {
        const host = task.host_id ? hostMap.get(task.host_id) : undefined
        return <span className="text-xs font-mono text-[var(--text-muted)]">{host?.mac || '-'}</span>
      },
    },
    {
      key: 'distro_name',
      label: t('installTasks.colDistro'),
      render: task => <span className="text-sm text-[var(--text-primary)]">{task.distro_name}</span>,
    },
    {
      key: 'version_codename',
      label: t('installTasks.colVersion'),
      render: task => <span className="text-xs text-[var(--text-muted)]">{task.version_codename}</span>,
    },
    {
      key: 'arch',
      label: t('installTasks.colArch'),
      render: task => <span className="text-xs text-[var(--text-muted)]">{task.arch || '-'}</span>,
    },
    {
      key: 'status',
      label: t('installTasks.colStatus'),
      render: task => (
        <Tag color={task.status === 'done' ? 'green' : task.status === 'failed' ? 'red' : task.status === 'installing' ? 'orange' : 'blue'}>
          {task.status}
        </Tag>
      ),
    },
    {
      key: 'template',
      label: t('installTasks.colTemplate'),
      render: task => {
        const tmpl = task.answer_template_id
          ? templates.find(tp => tp.id === task.answer_template_id)
          : undefined
        return <span className="text-xs text-[var(--text-muted)]">{tmpl ? `${tmpl.name} (${tmpl.type})` : '-'}</span>
      },
    },
    {
      key: 'extra_cmdline',
      label: t('installTasks.colExtra'),
      render: task => (
        <span className="block max-w-[160px] truncate text-xs font-mono text-[var(--text-muted)]">
          {task.extra_cmdline || '-'}
        </span>
      ),
    },
    {
      key: 'created_at',
      label: t('installTasks.colCreated'),
      render: task => (
        <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">
          {task.created_at ? new Date(task.created_at).toLocaleString() : '-'}
        </span>
      ),
    },
    {
      key: 'actions',
      label: t('installTasks.colActions'),
      className: 'text-right',
      render: task => (
        <>
          {task.status === 'pending' && (
            <Button
              variant="danger"
              size="sm"
              onClick={() => handleDelete(task)}
            >
              <Trash2 size={12} />
              {t('common.delete')}
            </Button>
          )}
          {task.status === 'failed' && task.error_msg && (
            <span className="text-[10px] text-accent-red/60" title={task.error_msg}>{t('common.error')}</span>
          )}
        </>
      ),
    },
  ]

  return (
    <div className="space-y-5">
      {/* Header */}
      <PageHeader
        title={t('installTasks.title')}
        className="mb-0"
        actions={
          <Button
            variant="primary"
            size="sm"
            onClick={() => {
              setNewTask({ distro_name: '', version_codename: '', extra_cmdline: '' })
              setShowCreate(true)
            }}
          >
            <Plus size={14} />
            {t('installTasks.addTask')}
          </Button>
        }
      />

      {/* Error state */}
      {loadError && (
        <Card>
          <div className="py-8 text-center text-sm text-accent-red">{loadError}</div>
        </Card>
      )}

      {/* Loading state */}
      {loading && (
        <Card padding={false}>
          <div className="p-6 space-y-3">
            {[1,2,3,4].map(i => (
              <div key={i} className="h-10 rounded bg-[var(--bg-hover)] animate-pulse" />
            ))}
          </div>
        </Card>
      )}

      {/* Empty state */}
      {!loading && !loadError && tasks.length === 0 && (
        <Card>
          <div className="py-12 text-center">
            <HardDrive size={32} className="mx-auto mb-3 text-[var(--text-muted)] opacity-40" />
            <p className="text-sm text-[var(--text-muted)]">{t('installTasks.empty')}</p>
          </div>
        </Card>
      )}

      {/* Table */}
      {!loading && !loadError && tasks.length > 0 && (
        <Card padding={false}>
          <DataTable
            columns={columns}
            data={tasks}
            rowKey={task => String(task.id)}
          />
        </Card>
      )}

      {/* Create Task Modal */}
      <Modal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        title={t('installTasks.addTask')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowCreate(false)}>{t('common.cancel')}</Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleCreate}
              disabled={taskSaving || !newTask.host_id || !newTask.distro_name || !newTask.version_codename}
            >
              {taskSaving ? t('common.processing') : t('hosts.detail.createTask')}
            </Button>
          </>
        }
      >
        {taskError && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">{taskError}</div>
        )}

        <div className="space-y-4">
          {/* Host select */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('installTasks.colHost')}</label>
            <Select
              size="sm"
              value={newTask.host_id || ''}
              onChange={e => setNewTask({ ...newTask, host_id: e.target.value })}
            >
              <option value="">{t('hosts.detail.selectDistro')}</option>
              {hosts.map(h => (
                <option key={h.id} value={h.id}>{h.name || h.id} ({h.mac})</option>
              ))}
            </Select>
          </div>

          {/* Distro select */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('installTasks.colDistro')}</label>
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

          {/* Version select */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('installTasks.colVersion')}</label>
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

          {/* Answer template select */}
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('hosts.detail.answerTemplate')}</label>
            <Select
              size="sm"
              value={newTask.answer_template_id ?? ''}
              onChange={e => setNewTask({ ...newTask, answer_template_id: e.target.value ? Number(e.target.value) : null })}
            >
              <option value="">{t('hosts.detail.none')}</option>
              {templates.map(t => (
                <option key={t.id} value={t.id}>{t.name} ({t.type})</option>
              ))}
            </Select>
          </div>

          {/* Extra cmdline */}
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
      <ConfirmDialog
        open={!!confirmDeleteTask}
        onClose={() => setConfirmDeleteTask(null)}
        onConfirm={doDeleteTask}
        title={t('hosts.detail.deleteTaskConfirm')}
        message={t('hosts.detail.deleteTaskMessage')}
      />
    </div>
  )
}
