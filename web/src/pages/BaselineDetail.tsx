import { useState, useEffect, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { FileCode, ArrowLeft, Plus, Trash2 } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Textarea } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type Baseline, type BaselineScript } from '../api/client'

export default function BaselineDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [baseline, setBaseline] = useState<Baseline | null>(null)
  const [scripts, setScripts] = useState<BaselineScript[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  // Edit baseline form
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [editVars, setEditVars] = useState('')
  const [editSaving, setEditSaving] = useState(false)

  // Script form modal
  const [scriptModalOpen, setScriptModalOpen] = useState(false)
  const [editScriptIdx, setEditScriptIdx] = useState<number | null>(null)
  const [scriptFilename, setScriptFilename] = useState('')
  const [scriptSeq, setScriptSeq] = useState(0)
  const [scriptContent, setScriptContent] = useState('')
  const [scriptSaving, setScriptSaving] = useState(false)

  // Delete confirm
  const [deleteScriptTarget, setDeleteScriptTarget] = useState<number | null>(null)

  const loadBaseline = useCallback(async () => {
    if (!id) return
    setLoading(true)
    setLoadError('')
    try {
      const [baselineRes, scriptsRes] = await Promise.all([
        api.getBaseline(id),
        api.getBaselineScripts(id),
      ])
      setBaseline(baselineRes.data)
      setScripts(scriptsRes.data)
    } catch (err: any) {
      setLoadError(err.message || t('common.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [id, t])

  useEffect(() => { loadBaseline() }, [loadBaseline])

  function startEdit() {
    if (!baseline) return
    setEditName(baseline.name)
    setEditDesc(baseline.description || '')
    setEditVars(baseline.variables ? JSON.stringify(baseline.variables, null, 2) : '')
    setEditing(true)
  }

  async function saveBaseline() {
    if (!id || !editName.trim()) return
    setEditSaving(true)
    try {
      let variables: Record<string, string> | undefined
      const vt = editVars.trim()
      if (vt) {
        try {
          const parsed = JSON.parse(vt)
          if (typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)) {
            variables = parsed
          }
        } catch { /* ignore invalid */ }
      }
      await api.updateBaseline(id, {
        name: editName.trim(),
        description: editDesc.trim() || undefined,
        variables,
      })
      success(t('baselines.updateSuccess'))
      setEditing(false)
      loadBaseline()
    } catch (err: any) {
      showError(err.message || t('common.saveFailed'))
    } finally {
      setEditSaving(false)
    }
  }

  // ── Script CRUD ──

  function openAddScript() {
    setEditScriptIdx(null)
    setScriptFilename('')
    setScriptSeq(scripts.length + 1)
    setScriptContent('')
    setScriptModalOpen(true)
  }

  function openEditScript(sc: BaselineScript, idx: number) {
    setEditScriptIdx(idx)
    setScriptFilename(sc.filename)
    setScriptSeq(sc.seq)
    setScriptContent(sc.content)
    setScriptModalOpen(true)
  }

  async function saveScript() {
    if (!id || !scriptFilename.trim()) return
    setScriptSaving(true)
    try {
      const data: BaselineScript = {
        baseline_id: id,
        filename: scriptFilename.trim(),
        seq: scriptSeq,
        content: scriptContent,
      }
      if (editScriptIdx !== null) {
        const existing = scripts[editScriptIdx]
        if (existing?.id) data.id = existing.id
      }
      await api.upsertBaselineScript(data)
      success(t('baselines.scriptSaved'))
      setScriptModalOpen(false)
      loadBaseline()
    } catch (err: any) {
      showError(err.message || t('common.saveFailed'))
    } finally {
      setScriptSaving(false)
    }
  }

  async function deleteScript(id: number) {
    setDeleteScriptTarget(id)
  }

  async function doDeleteScript() {
    if (deleteScriptTarget === null) return
    try {
      await api.deleteBaselineScript(deleteScriptTarget)
      setScripts(prev => prev.filter(s => s.id !== deleteScriptTarget))
      success(t('baselines.scriptDeleted'))
    } catch (err: any) {
      showError(err.message || t('common.deleteFailed'))
    } finally {
      setDeleteScriptTarget(null)
    }
  }

  function formatTime(s: string) {
    try { return new Date(s).toLocaleString() } catch { return s }
  }

  const scriptColumns: Column<BaselineScript>[] = [
    {
      key: 'seq',
      label: t('baselines.scriptSeq'),
      render: sc => <span className="text-xs font-mono text-[var(--text-muted)] w-8">{sc.seq}</span>,
    },
    {
      key: 'filename',
      label: t('baselines.scriptFilename'),
      render: sc => <span className="text-sm font-mono text-[var(--text-primary)]">{sc.filename}</span>,
    },
    {
      key: 'content',
      label: t('baselines.scriptContent'),
      render: sc => (
        <span className="block max-w-[400px] truncate text-xs text-[var(--text-muted)] font-mono">
          {sc.content?.split('\n')[0] || '-'}
        </span>
      ),
    },
    {
      key: 'actions',
      label: '',
      className: 'text-right',
      render: (sc) => (
        <div className="flex items-center justify-end gap-1">
          <button
            onClick={() => openEditScript(sc, -1)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)] transition-colors"
            title={t('common.edit')}
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="M17 3a2.85 2.83 0 1 1 4 4L7.5 20.5 2 22l1.5-5.5Z"/><path d="m15 5 4 4"/></svg>
          </button>
          <button
            onClick={() => sc.id && deleteScript(sc.id)}
            className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-accent-red hover:bg-accent-red/10 transition-colors"
            title={t('common.delete')}
          >
            <Trash2 size={14} />
          </button>
        </div>
      ),
    },
  ]

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="flex flex-col items-center gap-3 text-[var(--text-muted)]">
          <div className="w-8 h-8 border-2 border-blue-500/30 border-t-blue-500 rounded-full animate-spin" />
          <span className="text-sm">{t('common.loading')}</span>
        </div>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="space-y-5">
        <button onClick={() => navigate('/baselines')} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
          <ArrowLeft size={14} /> {t('common.back')}
        </button>
        <Card><div className="py-8 text-center text-sm text-accent-red">{loadError}</div></Card>
      </div>
    )
  }

  if (!baseline) return null

  return (
    <div className="space-y-5">
      {/* Back + Header */}
      <button onClick={() => navigate('/baselines')} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
        <ArrowLeft size={14} /> {t('common.back')}
      </button>

      <PageHeader
        title={baseline.name}
        className="mb-0"
        actions={
          editing ? (
            <div className="flex items-center gap-2">
              <Button variant="secondary" size="sm" onClick={() => setEditing(false)}>{t('common.cancel')}</Button>
              <Button variant="primary" size="sm" onClick={saveBaseline} disabled={editSaving}>
                {editSaving ? t('common.processing') : t('common.save')}
              </Button>
            </div>
          ) : (
            <Button variant="primary" size="sm" onClick={startEdit}>{t('common.edit')}</Button>
          )
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        {/* Info Card */}
        <Card>
          {editing ? (
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.name')}</label>
                <Input size="sm" value={editName} onChange={e => setEditName(e.target.value)} />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.description')}</label>
                <Textarea size="sm" value={editDesc} onChange={e => setEditDesc(e.target.value)} rows={2} />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.variables')}</label>
                <Textarea size="sm" value={editVars} onChange={e => setEditVars(e.target.value)} rows={4} className="font-mono text-xs" />
              </div>
            </div>
          ) : (
            <div className="space-y-4">
              <div>
                <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.description')}</span>
                <span className="text-sm text-[var(--text-primary)]">{baseline.description || '-'}</span>
              </div>
              {baseline.variables && Object.keys(baseline.variables).length > 0 && (
                <div>
                  <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.variables')}</span>
                  <div className="bg-[var(--bg-input)] rounded-lg p-3 space-y-1">
                    {Object.entries(baseline.variables).map(([k, v]) => (
                      <div key={k} className="flex items-center gap-2 text-xs">
                        <span className="font-mono text-blue-400">{k}</span>
                        <span className="text-[var(--text-muted)]">=</span>
                        <span className="font-mono text-[var(--text-primary)]">{String(v)}</span>
                      </div>
                    ))}
                  </div>
                </div>
              )}
              <div>
                <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.created')}</span>
                <span className="text-xs text-[var(--text-muted)]">{formatTime(baseline.created_at)}</span>
              </div>
            </div>
          )}
        </Card>

        {/* Profile Association Card */}
        <Card>
          <div className="space-y-3">
            <span className="block text-xs font-medium text-[var(--text-muted)]">{t('baselines.profileAssociation')}</span>
            <p className="text-sm text-[var(--text-muted)]">{t('baselines.noAssignedProfiles')}</p>
            <p className="text-xs text-[var(--text-muted)] opacity-60">
              {t('baselines.assignToProfileHint')}
            </p>
          </div>
        </Card>
      </div>

      {/* Scripts Section */}
      <div>
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-[var(--text-primary)]">{t('baselines.scripts')}</h3>
          <Button variant="primary" size="sm" onClick={openAddScript}>
            <Plus size={14} />
            {t('baselines.addScript')}
          </Button>
        </div>

        {scripts.length === 0 ? (
          <Card>
            <div className="py-8 text-center">
              <FileCode size={28} className="mx-auto mb-2 text-[var(--text-muted)] opacity-40" />
              <p className="text-sm text-[var(--text-muted)]">{t('baselines.noScripts')}</p>
            </div>
          </Card>
        ) : (
          <Card padding={false}>
            <DataTable
              columns={scriptColumns}
              data={scripts}
              rowKey={(sc) => String(sc.id ?? sc.filename)}
            />
          </Card>
        )}
      </div>

      {/* Script Editor Modal */}
      <Modal
        open={scriptModalOpen}
        onClose={() => setScriptModalOpen(false)}
        title={editScriptIdx !== null ? t('baselines.editScript') : t('baselines.addScript')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setScriptModalOpen(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={saveScript} disabled={scriptSaving}>
              {scriptSaving ? t('common.processing') : t('common.save')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="flex items-center gap-4">
            <div className="flex-1">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.scriptFilename')}</label>
              <Input size="sm" value={scriptFilename} onChange={e => setScriptFilename(e.target.value)}
                placeholder="init.sh" />
            </div>
            <div className="w-20">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.scriptSeq')}</label>
              <Input size="sm" type="number" min={0} value={String(scriptSeq)} onChange={e => setScriptSeq(Number(e.target.value))} />
            </div>
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('baselines.scriptContent')}</label>
            <Textarea
              size="sm"
              value={scriptContent}
              onChange={e => setScriptContent(e.target.value)}
              rows={16}
              className="font-mono text-xs"
              placeholder="#!/bin/bash&#10;echo 'Hello, World!'"
            />
          </div>
        </div>
      </Modal>

      {/* Delete Script Confirm */}
      <ConfirmDialog
        open={deleteScriptTarget !== null}
        onClose={() => setDeleteScriptTarget(null)}
        onConfirm={doDeleteScript}
        title={t('common.delete')}
        message={t('baselines.deleteScriptConfirm')}
      />
    </div>
  )
}
