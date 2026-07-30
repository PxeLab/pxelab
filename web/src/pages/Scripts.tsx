import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Textarea } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type scriptDTO } from '../api/client'

const SCRIPT_TYPES = [
  { value: 'shell', label: 'Shell' },
  { value: 'bat', label: 'BAT' },
  { value: 'powershell', label: 'PowerShell' },
]

function formatTime(s: string) {
  try { return new Date(s).toLocaleString() } catch { return s }
}

export default function Scripts() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [scripts, setScripts] = useState<scriptDTO[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  // Create modal state
  const [showForm, setShowForm] = useState(false)
  const [formName, setFormName] = useState('')
  const [formType, setFormType] = useState('shell')
  const [formContent, setFormContent] = useState('')
  const [formDesc, setFormDesc] = useState('')
  const [formSaving, setFormSaving] = useState(false)
  const [formError, setFormError] = useState('')

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<scriptDTO | null>(null)

  const loadScripts = async () => {
    setLoading(true)
    setLoadError('')
    try {
      const res = await api.getScripts()
      setScripts(res.data)
    } catch (err: any) {
      setLoadError(err.message || t('common.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadScripts() }, [])

  function openCreate() {
    setFormName('')
    setFormType('shell')
    setFormContent('')
    setFormDesc('')
    setFormError('')
    setShowForm(true)
  }

  async function handleSave() {
    if (!formName.trim()) { setFormError(t('common.fieldRequired')); return }
    if (!formContent.trim()) { setFormError(t('common.fieldRequired')); return }
    setFormSaving(true)
    setFormError('')
    try {
      await api.createScript({ name: formName, type: formType, content: formContent, description: formDesc })
      success(t('common.created'))
      setShowForm(false)
      loadScripts()
    } catch (err: any) {
      setFormError(err.message || t('common.saveFailed'))
    } finally {
      setFormSaving(false)
    }
  }

  async function handleDelete(sc: scriptDTO) {
    try {
      await api.deleteScript(sc.id)
      success(t('common.deleted'))
      setDeleteTarget(null)
      loadScripts()
    } catch (err: any) {
      showError(err.message || t('common.deleteFailed'))
    }
  }

  const columns: Column<scriptDTO>[] = [
    {
      key: 'name',
      label: t('common.name'),
      sortable: true,
      render: (item: scriptDTO) => (
        <button
          onClick={() => navigate(`/scripts/${item.id}`)}
          className="text-sm font-medium text-blue-400 hover:text-blue-300 transition-colors"
        >
          {item.name}
        </button>
      ),
    },
    {
      key: 'type',
      label: t('common.type'),
      render: (item: scriptDTO) => {
        const st = SCRIPT_TYPES.find(s => s.value === item.type)
        return (
          <span className={`px-2 py-0.5 rounded text-xs font-medium ${
            item.type === 'shell' ? 'bg-accent-green/15 text-accent-green' :
            item.type === 'bat' ? 'bg-blue-500/15 text-blue-400' :
            'bg-accent-yellow/15 text-accent-yellow'
          }`}>
            {st ? st.label : item.type}
          </span>
        )
      },
    },
    {
      key: 'description',
      label: t('common.description'),
      render: (item: scriptDTO) => (
        <span className="text-xs text-[var(--text-muted)] max-w-[240px] truncate block">
          {item.description || '-'}
        </span>
      ),
    },
    {
      key: 'created_at',
      label: t('common.createdAt'),
      sortable: true,
      render: (item: scriptDTO) => (
        <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">{formatTime(item.created_at)}</span>
      ),
    },
    {
      key: 'actions',
      label: '',
      className: 'text-right',
      render: (item: scriptDTO) => (
        <div className="flex items-center justify-end gap-1">
          <button
            onClick={(e) => { e.stopPropagation(); setDeleteTarget(item) }}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-accent-red hover:bg-accent-red/10 transition-colors"
            title={t('common.delete')}
          >
            <Trash2 size={14} />
          </button>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('scripts.title')}
        description={t('scripts.description')}
        className="mb-0"
        actions={
          <Button variant="primary" size="sm" onClick={openCreate}>
            <Plus size={14} />
            {t('scripts.create')}
          </Button>
        }
      />

      {loadError && (
        <Card>
          <div className="py-8 text-center text-sm text-accent-red">{loadError}</div>
        </Card>
      )}

      {loading && (
        <Card padding={false}>
          <div className="p-6 space-y-3">
            {[1,2,3].map(i => (
              <div key={i} className="h-10 rounded bg-[var(--bg-hover)] animate-pulse" />
            ))}
          </div>
        </Card>
      )}

      {!loading && !loadError && scripts.length === 0 && (
        <Card>
          <div className="py-12 text-center">
            <p className="text-sm text-[var(--text-muted)]">{t('scripts.noScripts')}</p>
          </div>
        </Card>
      )}

      {!loading && !loadError && scripts.length > 0 && (
        <Card padding={false}>
          <DataTable
            columns={columns}
            data={scripts}
            onRowClick={(row) => navigate(`/scripts/${row.id}`)}
            rowKey={sc => String(sc.id)}
          />
        </Card>
      )}

      {/* Create Modal */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={t('scripts.create')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowForm(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleSave} disabled={formSaving}>
              {formSaving ? t('common.processing') : t('common.save')}
            </Button>
          </>
        }
      >
        {formError && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">{formError}</div>
        )}
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.name')} *</label>
            <Input size="sm" value={formName} onChange={e => setFormName(e.target.value)} placeholder="my-script" />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.type')}</label>
            <select
              value={formType}
              onChange={e => setFormType(e.target.value)}
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] outline-none transition-all rounded-lg px-3.5 py-2 text-sm focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            >
              {SCRIPT_TYPES.map(st => (
                <option key={st.value} value={st.value}>{st.label}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.content')} *</label>
            <Textarea size="sm" value={formContent} onChange={e => setFormContent(e.target.value)} rows={10} className="font-mono text-sm" placeholder="#!/bin/bash" />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.description')}</label>
            <Textarea size="sm" value={formDesc} onChange={e => setFormDesc(e.target.value)} rows={2} />
          </div>
        </div>
      </Modal>

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => deleteTarget && handleDelete(deleteTarget)}
        title={t('common.delete')}
        message={deleteTarget ? t('scripts.deleteConfirm', { name: deleteTarget.name }) : ''}
      />
    </div>
  )
}
