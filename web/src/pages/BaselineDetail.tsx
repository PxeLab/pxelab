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
import { api, createAndAddBaselineScript, type Baseline, type BaselineScriptAssignment, type scriptDTO, type SetScriptsItem } from '../api/client'

const SCRIPT_TYPE_COLORS: Record<string, string> = {
  shell: 'bg-accent-green/15 text-accent-green',
  bat: 'bg-blue-500/15 text-blue-400',
  powershell: 'bg-accent-yellow/15 text-accent-yellow',
}

export default function BaselineDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [baseline, setBaseline] = useState<Baseline | null>(null)
  const [assignments, setAssignments] = useState<BaselineScriptAssignment[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')

  // Edit baseline form
  const [editing, setEditing] = useState(false)
  const [editName, setEditName] = useState('')
  const [editDesc, setEditDesc] = useState('')
  const [editVars, setEditVars] = useState('')
  const [editSaving, setEditSaving] = useState(false)
  const [savingScripts, setSavingScripts] = useState(false)

  // Add script modal — pick from library
  const [showAddModal, setShowAddModal] = useState(false)
  const [availableScripts, setAvailableScripts] = useState<scriptDTO[]>([])
  const [selectedScriptIds, setSelectedScriptIds] = useState<Set<number>>(new Set())
  const [loadingAvailable, setLoadingAvailable] = useState(false)

  // Delete confirm
  const [deleteScriptTarget, setDeleteScriptTarget] = useState<number | null>(null)

  // Inline create — new library script appended to this baseline (L2)
  const [showInline, setShowInline] = useState(false)
  const [inlineName, setInlineName] = useState('')
  const [inlineType, setInlineType] = useState('shell')
  const [inlineContent, setInlineContent] = useState('')
  const [inlineDesc, setInlineDesc] = useState('')
  const [inlineSaving, setInlineSaving] = useState(false)
  const [inlineError, setInlineError] = useState('')

  function openInlineCreate() {
    setInlineName('')
    setInlineType('shell')
    setInlineContent('')
    setInlineDesc('')
    setInlineError('')
    setShowInline(true)
  }

  async function saveInlineCreate() {
    if (!id) return
    if (!inlineName.trim() || !inlineContent.trim()) {
      setInlineError(t('common.fieldRequired'))
      return
    }
    setInlineSaving(true)
    setInlineError('')
    try {
      await createAndAddBaselineScript(id, {
        name: inlineName.trim(),
        type: inlineType,
        content: inlineContent,
        description: inlineDesc.trim() || undefined,
      })
      success(t('baselines.scriptSaved'))
      setShowInline(false)
      loadBaseline()
    } catch (err: any) {
      setInlineError(err.message || t('common.saveFailed'))
    } finally {
      setInlineSaving(false)
    }
  }

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
      setAssignments(scriptsRes.data.sort((a, b) => a.seq - b.seq))
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

  // ── Script Library Selection ──

  async function openAddScript() {
    setShowAddModal(true)
    setSelectedScriptIds(new Set())
    setLoadingAvailable(true)
    try {
      // Load all available scripts
      const res = await api.getScripts()
      // Filter out already assigned ones
      const assignedIds = new Set(assignments.map(a => a.script_id))
      setAvailableScripts(res.data.filter(s => !assignedIds.has(s.id)))
    } catch (err: any) {
      showError(err.message || t('common.loadFailed'))
    } finally {
      setLoadingAvailable(false)
    }
  }

  function toggleSelectScript(id: number) {
    setSelectedScriptIds(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  async function confirmAddScripts() {
    if (!id) return

    // Build the new assignment list: keep existing + newly selected
    const items: SetScriptsItem[] = [
      ...assignments.map(a => ({ script_id: a.script_id, seq: a.seq })),
      ...Array.from(selectedScriptIds).map((scriptId, i) => ({
        script_id: scriptId,
        seq: assignments.length + i + 1,
      })),
    ]

    setSavingScripts(true)
    try {
      await api.setBaselineScripts(id, items)
      success(t('baselines.scriptSaved'))
      setShowAddModal(false)
      loadBaseline()
    } catch (err: any) {
      showError(err.message || t('common.saveFailed'))
    } finally {
      setSavingScripts(false)
    }
  }

  function handleRemoveScript(scriptId: number) {
    setDeleteScriptTarget(scriptId)
  }

  async function confirmRemoveScript() {
    if (!id || deleteScriptTarget === null) return
    const items: SetScriptsItem[] = assignments
      .filter(a => a.script_id !== deleteScriptTarget)
      .map((a, i) => ({ script_id: a.script_id, seq: i + 1 }))

    try {
      await api.setBaselineScripts(id, items)
      success(t('baselines.scriptDeleted'))
      setDeleteScriptTarget(null)
      loadBaseline()
    } catch (err: any) {
      showError(err.message || t('common.deleteFailed'))
    }
  }

  function moveScript(index: number, direction: -1 | 1) {
    const newAssignments = [...assignments]
    const target = index + direction
    if (target < 0 || target >= newAssignments.length) return
    ;[newAssignments[index], newAssignments[target]] = [newAssignments[target], newAssignments[index]]
    // Re-number seq
    newAssignments.forEach((a, i) => { a.seq = i + 1 })
    setAssignments(newAssignments)
  }

  async function saveOrder() {
    if (!id) return
    setSavingScripts(true)
    try {
      const items: SetScriptsItem[] = assignments.map(a => ({
        script_id: a.script_id,
        seq: a.seq,
      }))
      await api.setBaselineScripts(id, items)
      success(t('common.saved'))
    } catch (err: any) {
      showError(err.message || t('common.saveFailed'))
      loadBaseline()
    } finally {
      setSavingScripts(false)
    }
  }

  function formatTime(s: string) {
    try { return new Date(s).toLocaleString() } catch { return s }
  }

  const scriptColumns: Column<BaselineScriptAssignment>[] = [
    {
      key: 'seq',
      label: t('baselines.scriptSeq'),
      render: (item: BaselineScriptAssignment) => (
        <span className="text-xs font-mono text-[var(--text-muted)] w-8">{item.seq}</span>
      ),
    },
    {
      key: 'name',
      label: t('common.name'),
      render: (item: BaselineScriptAssignment) => (
        <span className="text-sm font-medium text-[var(--text-primary)]">{item.name}</span>
      ),
    },
    {
      key: 'type',
      label: t('common.type'),
      render: (item: BaselineScriptAssignment) => (
        <span className={`px-2 py-0.5 rounded text-xs font-medium ${SCRIPT_TYPE_COLORS[item.type] || 'text-[var(--text-muted)]'}`}>
          {item.type === 'shell' ? 'Shell' : item.type === 'bat' ? 'BAT' : item.type === 'powershell' ? 'PowerShell' : item.type}
        </span>
      ),
    },
    {
      key: 'content',
      label: t('common.content'),
      render: (item: BaselineScriptAssignment) => (
        <span className="block max-w-[300px] truncate text-xs text-[var(--text-muted)] font-mono">
          {item.content?.split('\n')[0] || '-'}
        </span>
      ),
    },
    {
      key: 'actions',
      label: '',
      className: 'text-right',
      render: (item: BaselineScriptAssignment) => {
        const idx = assignments.findIndex(a => a.script_id === item.script_id)
        return (
          <div className="flex items-center justify-end gap-1">
            <button
              onClick={() => moveScript(idx, -1)}
              disabled={idx <= 0}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)] transition-colors disabled:opacity-20"
              title={t('common.up')}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m18 15-6-6-6 6"/></svg>
            </button>
            <button
              onClick={() => moveScript(idx, 1)}
              disabled={idx >= assignments.length - 1}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-[var(--text-primary)] hover:bg-[var(--bg-card)] transition-colors disabled:opacity-20"
              title={t('common.down')}
            >
              <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round"><path d="m6 9 6 6 6-6"/></svg>
            </button>
            <button
              onClick={() => handleRemoveScript(item.script_id)}
              className="p-1.5 rounded-lg text-[var(--text-muted)] hover:text-accent-red hover:bg-accent-red/10 transition-colors"
              title={t('common.delete')}
            >
              <Trash2 size={14} />
            </button>
          </div>
        )
      },
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
          <div className="flex items-center gap-2">
            {assignments.length > 0 && (
              <Button variant="secondary" size="sm" onClick={saveOrder} disabled={savingScripts}>
                {t('common.save')}
              </Button>
            )}
            <Button variant="secondary" size="sm" onClick={openInlineCreate}>
              <Plus size={14} />
              {t('baselines.newScriptAndAdd')}
            </Button>
            <Button variant="primary" size="sm" onClick={openAddScript}>
              <Plus size={14} />
              {t('baselines.addScript')}
            </Button>
          </div>
        </div>

        {assignments.length === 0 ? (
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
              data={assignments}
              rowKey={(a) => String(a.script_id)}
            />
          </Card>
        )}
      </div>

      {/* Add Script from Library Modal */}
      <Modal
        open={showAddModal}
        onClose={() => setShowAddModal(false)}
        title={t('baselines.addScript')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowAddModal(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={confirmAddScripts} disabled={selectedScriptIds.size === 0 || savingScripts}>
              {savingScripts ? t('common.processing') : t('common.add')}
            </Button>
          </>
        }
      >
        <div className="space-y-3 max-h-80 overflow-y-auto pr-1">
          {loadingAvailable ? (
            <div className="flex items-center justify-center py-8">
              <div className="w-6 h-6 border-2 border-blue-500/30 border-t-blue-500 rounded-full animate-spin" />
            </div>
          ) : availableScripts.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)] text-center py-4">{t('scripts.noScripts')}</p>
          ) : (
            availableScripts.map(sc => (
              <label
                key={sc.id}
                className={`flex items-center gap-3 p-3 rounded-lg border cursor-pointer transition-colors ${
                  selectedScriptIds.has(sc.id)
                    ? 'border-blue-500/40 bg-blue-500/5'
                    : 'border-[var(--bg-border)] hover:bg-[var(--bg-hover)]'
                }`}
              >
                <input
                  type="checkbox"
                  checked={selectedScriptIds.has(sc.id)}
                  onChange={() => toggleSelectScript(sc.id)}
                  className="rounded border-[var(--bg-border)] text-blue-500 focus:ring-blue-500/30"
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-[var(--text-primary)] truncate">{sc.name}</span>
                    <span className={`px-1.5 py-0.5 rounded text-[10px] font-medium ${SCRIPT_TYPE_COLORS[sc.type] || ''}`}>
                      {sc.type}
                    </span>
                  </div>
                  {sc.description && (
                    <p className="text-xs text-[var(--text-muted)] truncate mt-0.5">{sc.description}</p>
                  )}
                </div>
              </label>
            ))
          )}
        </div>
      </Modal>

      {/* Inline Create & Add Modal (L2) */}
      <Modal
        open={showInline}
        onClose={() => setShowInline(false)}
        title={t('baselines.newScriptAndAdd')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowInline(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={saveInlineCreate} disabled={inlineSaving}>
              {inlineSaving ? t('common.processing') : t('common.save')}
            </Button>
          </>
        }
      >
        {inlineError && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">{inlineError}</div>
        )}
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.name')} *</label>
            <Input size="sm" type="text" value={inlineName} onChange={e => setInlineName(e.target.value)} placeholder="my-script" />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.type')}</label>
            <select
              value={inlineType}
              onChange={e => setInlineType(e.target.value)}
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] outline-none transition-all rounded-lg px-3.5 py-2 text-sm focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
            >
              <option value="shell">Shell</option>
              <option value="bat">BAT</option>
              <option value="powershell">PowerShell</option>
            </select>
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.content')} *</label>
            <Textarea size="sm" value={inlineContent} onChange={e => setInlineContent(e.target.value)} rows={10} className="font-mono text-sm" placeholder="#!/bin/bash" />
          </div>
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.description')}</label>
            <Textarea size="sm" value={inlineDesc} onChange={e => setInlineDesc(e.target.value)} rows={2} />
          </div>
        </div>
      </Modal>

      {/* Remove Script Confirm */}
      <ConfirmDialog
        open={deleteScriptTarget !== null}
        onClose={() => setDeleteScriptTarget(null)}
        onConfirm={confirmRemoveScript}
        title={t('common.delete')}
        message={t('baselines.deleteScriptConfirm')}
      />
    </div>
  )
}
