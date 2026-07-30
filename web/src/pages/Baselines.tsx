import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { FileCode, Plus, Trash2, Edit3, Eye } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Textarea } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type Baseline } from '../api/client'

export default function Baselines() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [baselines, setBaselines] = useState<Baseline[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  // Create / Edit modal state
  const [showForm, setShowForm] = useState(false)
  const [editingId, setEditingId] = useState<string | null>(null)
  const [formName, setFormName] = useState('')
  const [formDesc, setFormDesc] = useState('')
  const [formVars, setFormVars] = useState<string>('')
  const [formSaving, setFormSaving] = useState(false)
  const [formError, setFormError] = useState('')

  // Delete confirm
  const [deleteTarget, setDeleteTarget] = useState<Baseline | null>(null)

  const loadBaselines = async () => {
    setLoading(true)
    setLoadError('')
    try {
      const res = await api.getBaselines()
      setBaselines(res.data)
    } catch (err: any) {
      setLoadError(err.message || t('common.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadBaselines() }, [])

  function openCreate() {
    setEditingId(null)
    setFormName('')
    setFormDesc('')
    setFormVars('')
    setFormError('')
    setShowForm(true)
  }

  function openEdit(bl: Baseline) {
    setEditingId(bl.id)
    setFormName(bl.name)
    setFormDesc(bl.description || '')
    setFormVars(bl.variables ? JSON.stringify(bl.variables, null, 2) : '')
    setFormError('')
    setShowForm(true)
  }

  async function handleSave() {
    const trimmedName = formName.trim()
    if (!trimmedName) {
      setFormError(t('common.fieldRequired'))
      return
    }

    // Parse variables JSON
    let variables: Record<string, string> | undefined
    const varsTrimmed = formVars.trim()
    if (varsTrimmed) {
      try {
        const parsed = JSON.parse(varsTrimmed)
        if (typeof parsed !== 'object' || parsed === null || Array.isArray(parsed)) {
          setFormError(t('baselines.variables') + ': ' + t('common.invalidJSON'))
          return
        }
        variables = parsed
      } catch {
        setFormError(t('baselines.variables') + ': ' + t('common.invalidJSON'))
        return
      }
    }

    setFormSaving(true)
    setFormError('')
    try {
      if (editingId) {
        await api.updateBaseline(editingId, { name: trimmedName, description: formDesc.trim() || undefined, variables })
        success(t('baselines.updateSuccess'))
      } else {
        await api.createBaseline({ name: trimmedName, description: formDesc.trim() || undefined, variables })
        success(t('baselines.createSuccess'))
      }
      setShowForm(false)
      loadBaselines()
    } catch (err: any) {
      setFormError(err.message || t('common.saveFailed'))
    } finally {
      setFormSaving(false)
    }
  }

  async function handleDelete(bl: Baseline) {
    setDeleteTarget(bl)
  }

  async function doDelete() {
    if (!deleteTarget) return
    try {
      await api.deleteBaseline(deleteTarget.id)
      setBaselines(prev => prev.filter(b => b.id !== deleteTarget.id))
      success(t('baselines.deleteSuccess'))
    } catch (err: any) {
      showError(err.message || t('common.deleteFailed'))
    } finally {
      setDeleteTarget(null)
    }
  }

  function formatTime(s: string) {
    try { return new Date(s).toLocaleString() } catch { return s }
  }

  const columns: Column<Baseline>[] = [
    {
      key: 'name',
      label: t('baselines.name'),
      render: bl => (
        <button
          onClick={() => navigate(`/baselines/${bl.id}`)}
          className="text-sm font-medium text-blue-400 hover:text-blue-300 transition-colors"
        >
          {bl.name}
        </button>
      ),
    },
    {
      key: 'description',
      label: t('baselines.description'),
      render: bl => (
        <span className="text-xs text-[var(--text-muted)] max-w-[240px] truncate block">
          {bl.description || '-'}
        </span>
      ),
    },
    {
      key: 'variables',
      label: t('baselines.variables'),
      render: bl => (
        <span className="text-xs font-mono text-[var(--text-muted)]">
          {bl.variables ? `${Object.keys(bl.variables).length} vars` : '-'}
        </span>
      ),
    },
    {
      key: 'created_at',
      label: t('baselines.created'),
      render: bl => (
        <span className="text-xs text-[var(--text-muted)] whitespace-nowrap">{formatTime(bl.created_at)}</span>
      ),
    },
    {
      key: 'actions',
      label: t('baselines.actions'),
      className: 'text-right',
      render: bl => (
        <div className="flex items-center justify-end gap-1">
          <button
            onClick={() => navigate(`/baselines/${bl.id}`)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-blue-400 hover:bg-blue-500/10 transition-colors"
            title={t('baselines.scripts')}
          >
            <Eye size={14} />
          </button>
          <button
            onClick={() => openEdit(bl)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)] transition-colors"
            title={t('common.edit')}
          >
            <Edit3 size={14} />
          </button>
          <button
            onClick={() => handleDelete(bl)}
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
        title={t('baselines.listTitle')}
        className="mb-0"
        actions={
          <Button variant="primary" size="sm" onClick={openCreate}>
            <Plus size={14} />
            {t('baselines.createTitle')}
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

      {!loading && !loadError && baselines.length === 0 && (
        <Card>
          <div className="py-12 text-center">
            <FileCode size={32} className="mx-auto mb-3 text-[var(--text-muted)] opacity-40" />
            <p className="text-sm text-[var(--text-muted)]">{t('baselines.noBaselines')}</p>
          </div>
        </Card>
      )}

      {!loading && !loadError && baselines.length > 0 && (
        <Card padding={false}>
          <DataTable
            columns={columns}
            data={baselines}
            rowKey={bl => bl.id}
          />
        </Card>
      )}

      {/* Create / Edit Modal */}
      <Modal
        open={showForm}
        onClose={() => setShowForm(false)}
        title={editingId ? t('baselines.editTitle') : t('baselines.createTitle')}
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
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.name')} *</label>
            <Input size="sm" type="text" value={formName} onChange={e => setFormName(e.target.value)}
              placeholder={t('baselines.name')} />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.description')}</label>
            <Textarea size="sm" value={formDesc} onChange={e => setFormDesc(e.target.value)}
              placeholder={t('baselines.description')} rows={2} />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.variables')}</label>
            <Textarea size="sm" value={formVars} onChange={e => setFormVars(e.target.value)}
              placeholder='{"key": "value"}' rows={4} className="font-mono text-xs" />
          </div>
        </div>
      </Modal>

      {/* Delete Confirm */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={doDelete}
        title={t('baselines.deleteConfirmTitle')}
        message={deleteTarget ? t('baselines.deleteConfirmMsg') : ''}
      />
    </div>
  )
}
