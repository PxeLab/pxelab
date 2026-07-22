import { useState, useEffect, useCallback, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Input, Select, Textarea } from '../components/ui/FormControls'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type AnswerTemplate, type AnswerTemplateVersion } from '../api/client'

const TEMPLATE_TYPES = ['kickstart', 'preseed', 'subiquity', 'autoyast', 'autounattend']

const TYPE_LABELS: Record<string, string> = {
  kickstart: 'Kickstart (RHEL/CentOS/Rocky/Alma)',
  preseed: 'Preseed (Ubuntu/Debian)',
  subiquity: 'Subiquity (Ubuntu 20.04+)',
  autoyast: 'AutoYaST (SUSE)',
  autounattend: 'Autounattend (Windows)',
}

const TYPE_EXTENSIONS: Record<string, string> = {
  kickstart: 'ks',
  preseed: 'cfg',
  subiquity: 'yaml',
  autoyast: 'xml',
  autounattend: 'xml',
}

const HOST_VARS = [
  '{{.HostName}}', '{{.HostIP}}', '{{.HostMAC}}', '{{.HostCIDR}}',
  '{{.Gateway}}', '{{.DNSServers}}', '{{.Disk}}', '{{.KeyboardLayout}}',
  '{{.Arch}}',
]

const WINDOWS_VARS = [
  '{{.ProductKey}}', '{{.ComputerName}}', '{{.JoinDomain}}', '{{.DomainOU}}',
  '{{.AdminPassword}}', '{{.TimeZone}}',
]



export default function AnswerTemplates() {
  const { t } = useTranslation()
  const [templates, setTemplates] = useState<AnswerTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<AnswerTemplate | null>(null)
  const [showEditor, setShowEditor] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const importRef = useRef<HTMLInputElement>(null)

  // Validation state
  const [validationResult, setValidationResult] = useState<{ valid: boolean; error?: string } | null>(null)

  // Preview state
  const [showPreview, setShowPreview] = useState(false)
  const [previewResult, setPreviewResult] = useState('')

  // Preset state
  const [showPresets, setShowPresets] = useState(false)
  const [presets, setPresets] = useState<{ name: string; description: string; content: string; variables: string[] }[]>([])
  const [presetType, setPresetType] = useState('preseed')

  // Version history state
  const [showVersions, setShowVersions] = useState(false)
  const [versionTemplateId, setVersionTemplateId] = useState<number | null>(null)
  const [versions, setVersions] = useState<AnswerTemplateVersion[]>([])
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [diffVerA, setDiffVerA] = useState<number | ''>('')
  const [diffVerB, setDiffVerB] = useState<number | ''>('')
  const [confirmBatchDelete, setConfirmBatchDelete] = useState(false)
  const [confirmRollback, setConfirmRollback] = useState<number | null>(null)

  const load = useCallback(async () => {
    try {
      const res = await api.getAnswerTemplates()
      setTemplates(res.data.templates)
      setError('')
    } catch (err: any) {
      setError(err.message || t('answerTemplates.loadFailed'))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => { load() }, [load])

  const runValidation = async (content: string) => {
    setValidationResult(null)
    try {
      const res = await api.validateAnswerTemplate(content)
      setValidationResult(res.data)
    } catch (err: any) {
      setValidationResult({ valid: false, error: err.message })
    }
  }

  const save = async () => {
    if (!editing) return
    // Validate before save
    try {
      const res = await api.validateAnswerTemplate(editing.content)
      if (!res.data.valid) {
        setError('Template syntax error: ' + (res.data.error || 'unknown'))
        return
      }
    } catch (err: any) {
      setError('Validation failed: ' + err.message)
      return
    }
    try {
      if (editing.id) {
        await api.updateAnswerTemplate(editing.id, editing)
      } else {
        await api.createAnswerTemplate(editing)
      }
      setShowEditor(false)
      setEditing(null)
      setValidationResult(null)
      await load()
    } catch (err: any) {
      setError(err.message || t('answerTemplates.saveFailed'))
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteAnswerTemplate(id)
      setSelected(prev => { const next = new Set(prev); next.delete(id); return next })
      await load()
    } catch (err: any) {
      setError(err.message || t('answerTemplates.deleteFailed'))
    }
  }

  const removeBatch = async () => {
    if (selected.size === 0) return
    setConfirmBatchDelete(true)
  }

  const doBatchDelete = async () => {
    for (const id of selected) {
      try { await api.deleteAnswerTemplate(id) } catch {}
    }
    setSelected(new Set())
    setConfirmBatchDelete(false)
    await load()
  }

  const variablesForType = (type: string) => {
    const vars = [...HOST_VARS]
    if (type === 'autounattend') vars.push(...WINDOWS_VARS)
    return vars
  }

  const openNew = () => {
    setEditing({ name: '', description: '', type: 'preseed', content: '' })
    setValidationResult(null)
    setShowEditor(true)
  }

  const openNewWithContent = (content: string, name: string) => {
    setEditing({ name, description: '', type: 'preseed', content })
    setValidationResult(null)
    setShowEditor(true)
  }

  const openPresets = async (type: string) => {
    setPresetType(type)
    try {
      const res = await api.getAnswerTemplatePresets(type) as any
      setPresets((res.data as any).presets || [])
      setShowPresets(true)
    } catch (err: any) {
      setError(err.message || 'Failed to load presets')
    }
  }

  const applyPreset = (preset: { name: string; description: string; content: string; variables: string[] }) => {
    setEditing({
      name: preset.name,
      description: preset.description,
      type: presetType,
      content: preset.content,
    })
    setShowPresets(false)
    setValidationResult(null)
  }

  const renderTemplate = () => {
    if (!editing?.content) return ''
    let rendered = editing.content
    const mapping: [string, string][] = [
      ['{{.HostName}}', 'node-01'],
      ['{{.HostIP}}', '192.168.1.100'],
      ['{{.HostMAC}}', '00:11:22:33:44:55'],
      ['{{.HostCIDR}}', '192.168.1.0/24'],
      ['{{.Gateway}}', '192.168.1.1'],
      ['{{.DNSServers}}', '192.168.1.1'],
      ['{{.Disk}}', '/dev/sda'],
      ['{{.KeyboardLayout}}', 'us'],
      ['{{.Arch}}', 'amd64'],
      ['{{.ProductKey}}', 'XXXXX-XXXXX-XXXXX-XXXXX-XXXXX'],
      ['{{.ComputerName}}', 'WIN-NODE-01'],
      ['{{.JoinDomain}}', 'example.local'],
      ['{{.DomainOU}}', 'OU=Servers,DC=example,DC=local'],
      ['{{.AdminPassword}}', 'P@ssw0rd'],
      ['{{.TimeZone}}', 'UTC'],
    ]
    for (const [placeholder, value] of mapping) {
      rendered = rendered.replace(new RegExp(placeholder.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), 'g'), value)
    }
    return rendered
  }

  const openPreview = () => {
    setPreviewResult(renderTemplate())
    setShowPreview(true)
  }

  const openVersions = async (tplId: number) => {
    setVersionTemplateId(tplId)
    setShowVersions(true)
    setVersionsLoading(true)
    setDiffVerA('')
    setDiffVerB('')
    try {
      const res = await api.getAnswerTemplateVersions(tplId)
      setVersions(res.data.versions || [])
    } catch {
      setVersions([])
    } finally {
      setVersionsLoading(false)
    }
  }

  const handleRollback = async (version: number) => {
    if (!versionTemplateId) return
    setConfirmRollback(version)
  }

  const doRollback = async () => {
    if (!versionTemplateId || confirmRollback === null) return
    try {
      await api.rollbackAnswerTemplate(versionTemplateId, confirmRollback)
      setShowVersions(false)
      setConfirmRollback(null)
      await load()
    } catch (err: any) {
      setError(err.message || t('answerTemplates.rollbackFailed'))
      setConfirmRollback(null)
    }
  }

  function computeDiff(a: string, b: string): { type: 'add' | 'remove' | 'keep'; line: string }[] {
    const linesA = a.split('\n')
    const linesB = b.split('\n')
    const m = linesA.length, n = linesB.length
    const dp: number[][] = Array.from({ length: m + 1 }, () => Array(n + 1).fill(0))
    for (let i = 1; i <= m; i++) {
      for (let j = 1; j <= n; j++) {
        dp[i][j] = linesA[i - 1] === linesB[j - 1] ? dp[i - 1][j - 1] + 1 : Math.max(dp[i - 1][j], dp[i][j - 1])
      }
    }
    const lcs: string[] = []
    let i = m, j = n
    while (i > 0 && j > 0) {
      if (linesA[i - 1] === linesB[j - 1]) { lcs.unshift(linesA[i - 1]); i--; j-- }
      else if (dp[i - 1][j] > dp[i][j - 1]) i--
      else j--
    }
    const result: { type: 'add' | 'remove' | 'keep'; line: string }[] = []
    i = 0; j = 0
    for (const common of lcs) {
      while (i < m && linesA[i] !== common) { result.push({ type: 'remove', line: linesA[i++] }) }
      while (j < n && linesB[j] !== common) { result.push({ type: 'add', line: linesB[j++] }) }
      result.push({ type: 'keep', line: linesA[i] }); i++; j++
    }
    while (i < m) { result.push({ type: 'remove', line: linesA[i++] }) }
    while (j < n) { result.push({ type: 'add', line: linesB[j++] }) }
    return result
  }

  const getVersionContent = (version: number): string => versions.find(v => v.version === version)?.content || ''

  const openEdit = (tpl: AnswerTemplate) => {
    setEditing({ ...tpl })
    setValidationResult(null)
    setShowEditor(true)
  }

  const toggleSelect = (id: number) => {
    setSelected(prev => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    if (selected.size === templates.length) {
      setSelected(new Set())
    } else {
      setSelected(new Set(templates.filter(tp => tp.id).map(tp => tp.id!)))
    }
  }

  const exportSingle = (tpl: AnswerTemplate) => {
    const ext = TYPE_EXTENSIONS[tpl.type] || 'txt'
    const blob = new Blob([tpl.content], { type: 'text/plain;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${tpl.name}.${ext}`
    a.click()
    URL.revokeObjectURL(url)
  }

  const exportBatch = () => {
    if (selected.size === 0) return
    const items = templates.filter(tp => tp.id && selected.has(tp.id))
    const json = JSON.stringify(items, null, 2)
    const blob = new Blob([json], { type: 'application/json;charset=utf-8' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `answer-templates-${Date.now()}.json`
    a.click()
    URL.revokeObjectURL(url)
  }

  const handleImport = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) return
    const reader = new FileReader()
    reader.onload = async (ev) => {
      const content = ev.target?.result as string
      if (file.name.endsWith('.json')) {
        try {
          const items: AnswerTemplate[] = JSON.parse(content)
          let count = 0
          for (const item of items) {
            if (!item.name || !item.content) continue
            try {
              await api.createAnswerTemplate({
                name: item.name,
                description: item.description,
                type: item.type || 'preseed',
                content: item.content,
              })
              count++
            } catch {}
          }
          await load()
        } catch {
          setError(t('answerTemplates.importJsonError'))
        }
        return
      }
      const name = file.name.replace(/\.[^.]+$/, '')
      openNewWithContent(content, name)
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  const selectAllCheckbox = (
    <input
      type="checkbox"
      checked={selected.size === templates.length && templates.length > 0}
      onChange={toggleSelectAll}
      className="accent-blue-500"
    />
  )

  const columns: Column<AnswerTemplate>[] = [
    {
      key: 'select',
      label: selectAllCheckbox,
      width: '2.5rem',
      className: 'text-center',
      render: (tpl) => (
        <input
          type="checkbox"
          checked={tpl.id ? selected.has(tpl.id) : false}
          onChange={() => tpl.id && toggleSelect(tpl.id)}
          className="accent-blue-500"
        />
      ),
    },
    {
      key: 'name',
      label: t('answerTemplates.name'),
      render: (tpl) => (
        <>
          <span className="text-sm font-medium text-[var(--text-primary)]">{tpl.name}</span>
          {tpl.current_version && tpl.current_version > 1 && (
            <span className="ml-2 inline-flex items-center px-1.5 py-0.5 text-[10px] font-mono rounded bg-blue-500/10 text-blue-400">
              v{tpl.current_version}
            </span>
          )}
        </>
      ),
    },
    {
      key: 'type',
      label: t('answerTemplates.type'),
      render: (tpl) => (
        <span className="inline-block px-1.5 py-0.5 text-[10px] font-semibold rounded bg-[var(--hover)] text-[var(--text-muted)]">
          {tpl.type}
        </span>
      ),
    },
    {
      key: 'description',
      label: t('answerTemplates.description'),
      render: (tpl) => (
        <span className="text-xs text-[var(--text-muted)]">{tpl.description || '-'}</span>
      ),
    },
    {
      key: 'updated_at',
      label: t('answerTemplates.updatedAt'),
      render: (tpl) => (
        <span className="text-xs text-[var(--text-muted)]">{tpl.updated_at ? new Date(tpl.updated_at).toLocaleString() : '-'}</span>
      ),
    },
    {
      key: 'actions',
      label: t('answerTemplates.actions'),
      className: 'text-right',
      render: (tpl) => (
        <>
          <Button variant="ghost" size="sm" onClick={() => openEdit(tpl)}>{t('common.edit')}</Button>
          <Button variant="ghost" size="sm" onClick={() => exportSingle(tpl)}>{t('answerTemplates.export')}</Button>
          <Button variant="ghost" size="sm" onClick={() => tpl.id && openVersions(tpl.id)}>{t('answerTemplates.versions')}</Button>
          <Button variant="danger" size="sm" onClick={() => tpl.id && remove(tpl.id)}>{t('common.delete')}</Button>
        </>
      ),
    },
  ]

  return (
    <div>
      <PageHeader
        title={t('answerTemplates.title')}
        className="mb-5"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={load}>{t('common.refresh')}</Button>
            <input ref={importRef} type="file" accept=".json,.cfg,.ks,.preseed,.xml,.yaml,.yml,.txt" onChange={handleImport} className="hidden" />
            <Button variant="secondary" size="sm" onClick={() => importRef.current?.click()}>{t('answerTemplates.import')}</Button>
            <Button variant="primary" size="sm" onClick={openNew}>+ {t('answerTemplates.newTemplate')}</Button>
          </>
        }
      />

      {error && (
        <div className="mb-4 px-4 py-3 rounded-lg bg-accent-red/10 border border-accent-red/20 text-sm text-accent-red">
          {error}
          <button onClick={() => setError('')} className="float-right text-accent-red/60 hover:text-accent-red">✕</button>
        </div>
      )}

      {selected.size > 0 && (
        <div className="mb-3 px-4 py-2.5 rounded-lg bg-blue-500/10 border border-blue-500/20 flex items-center justify-between">
          <span className="text-xs text-blue-400">{t('answerTemplates.selectedCount', { count: selected.size })}</span>
          <div className="flex gap-2">
            <Button variant="ghost" size="sm" onClick={exportBatch}>{t('answerTemplates.exportSelected')}</Button>
            <Button variant="danger" size="sm" onClick={removeBatch}>{t('answerTemplates.deleteSelected')}</Button>
            <Button variant="ghost" size="sm" onClick={() => setSelected(new Set())}>{t('answerTemplates.deselect')}</Button>
          </div>
        </div>
      )}

      {/* Editor Modal */}
      <Modal
        open={showEditor && !!editing}
        onClose={() => { setShowEditor(false); setValidationResult(null) }}
        title={editing?.id ? t('answerTemplates.editTemplate') : t('answerTemplates.newTemplate')}
        width="780px"
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => openPreview()}>
              {t('answerTemplates.preview')}
            </Button>
            <Button variant="ghost" size="sm" onClick={() => runValidation(editing?.content || '')}>
              {t('answerTemplates.validate')}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setShowEditor(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={save} disabled={!editing?.name || !editing?.content}>{t('common.save')}</Button>
          </>
        }
      >
        {editing && (
          <>
            <div className="grid grid-cols-3 gap-4 mb-4">
              <div className="col-span-2">
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('answerTemplates.name')}</label>
                <Input
                  size="sm"
                  type="text" value={editing.name}
                  onChange={e => setEditing({ ...editing, name: e.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('answerTemplates.type')}</label>
                <div className="flex gap-1">
                  <Select
                    size="sm"
                    className="flex-1"
                    value={editing.type}
                    onChange={e => setEditing({ ...editing, type: e.target.value })}
                  >
                    {TEMPLATE_TYPES.map(tp => (
                      <option key={tp} value={tp}>{TYPE_LABELS[tp] || tp}</option>
                    ))}
                  </Select>
                  <button
                    onClick={() => openPresets(editing.type)}
                    className="px-2 py-1 text-[10px] rounded bg-blue-500/10 text-blue-400 hover:bg-blue-500/20"
                    title="From preset"
                  >
                    Presets
                  </button>
                </div>
              </div>
            </div>

            <div className="mb-4">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('answerTemplates.description')}</label>
              <Input
                size="sm"
                type="text" value={editing.description || ''}
                onChange={e => setEditing({ ...editing, description: e.target.value })}
              />
            </div>

            {/* Validation result */}
            {validationResult && (
              <div className={`mb-4 px-3 py-2 rounded-lg text-xs flex items-center gap-2 ${
                validationResult.valid
                  ? 'bg-accent-green/10 text-accent-green border border-accent-green/20'
                  : 'bg-accent-red/10 text-accent-red border border-accent-red/20'
              }`}>
                <span>{validationResult.valid ? t('answerTemplates.validTemplate') : t('answerTemplates.validationError')}</span>
                {validationResult.error && <code className="ml-1 font-mono text-[10px] opacity-70">{validationResult.error}</code>}
              </div>
            )}

            <div className="mb-4">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('answerTemplates.content')}</label>
              <Textarea
                size="sm"
                className="font-mono"
                value={editing.content}
                onChange={e => { setEditing({ ...editing, content: e.target.value }); setValidationResult(null) }}
                rows={16}
              />
            </div>

            {/* Variable hints */}
            <div className="mb-4 p-3 rounded-lg bg-blue-500/5 border border-blue-500/10">
              <p className="text-xs font-medium text-blue-400 mb-2">{t('answerTemplates.availableVars')}</p>
              <div className="flex flex-wrap gap-1.5">
                {variablesForType(editing.type).map(v => (
                  <button
                    key={v}
                    onClick={() => {
                      const ta = document.querySelector('textarea') as HTMLTextAreaElement
                      if (ta) {
                        const start = ta.selectionStart
                        const end = ta.selectionEnd
                        const newContent = editing.content.substring(0, start) + v + editing.content.substring(end)
                        setEditing({ ...editing, content: newContent })
                        setTimeout(() => { ta.selectionStart = ta.selectionEnd = start + v.length; ta.focus() }, 0)
                      } else {
                        navigator.clipboard.writeText(v)
                      }
                    }}
                    className="px-2 py-0.5 text-[11px] font-mono rounded bg-blue-500/10 text-blue-300 hover:bg-blue-500/20 transition-colors"
                  >
                    {v}
                  </button>
                ))}
              </div>
            </div>
          </>
        )}
      </Modal>

      {/* Preview Modal */}
      <Modal
        open={showPreview}
        onClose={() => setShowPreview(false)}
        title={t('answerTemplates.templatePreview')}
        width="740px"
        footer={
          <Button variant="secondary" size="sm" onClick={() => setShowPreview(false)}>{t('answerTemplates.close')}</Button>
        }
      >
        <div className="rounded-lg border border-[var(--bg-border)] overflow-hidden">
          <pre className="p-3 max-h-80 overflow-y-auto text-[11px] font-mono whitespace-pre-wrap text-[var(--text-primary)]">
            {previewResult || editing?.content || ''}
          </pre>
        </div>
      </Modal>

      {/* Presets Modal */}
      <Modal
        open={showPresets}
        onClose={() => setShowPresets(false)}
        title={`${t('answerTemplates.presets')} — ${TYPE_LABELS[presetType] || presetType}`}
        width="640px"
        footer={<Button variant="secondary" size="sm" onClick={() => setShowPresets(false)}>{t('answerTemplates.close')}</Button>}
      >
        <div className="space-y-3">
          {presets.length === 0 ? (
            <p className="text-sm text-[var(--text-muted)]">{t('answerTemplates.noPresets')}</p>
          ) : (
            presets.map((p, i) => (
              <div key={i} className="p-4 rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)]">
                <div className="flex items-start justify-between mb-2">
                  <div>
                    <p className="text-sm font-semibold text-[var(--text-primary)]">{p.name}</p>
                    <p className="text-[11px] text-[var(--text-muted)]">{p.description}</p>
                  </div>
                  <Button variant="primary" size="sm" onClick={() => applyPreset(p)}>{t('answerTemplates.use')}</Button>
                </div>
                {p.variables?.length > 0 && (
                  <div className="flex flex-wrap gap-1 mt-2">
                    {p.variables.map(v => (
                      <span key={v} className="px-1.5 py-0.5 text-[10px] font-mono rounded bg-accent-yellow/10 text-accent-yellow">
                        {v}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            ))
          )}
        </div>
      </Modal>

      {/* Template list */}
      <Card padding={false}>
        <DataTable
          columns={columns}
          data={templates}
          loading={loading}
          emptyText={t('answerTemplates.empty')}
          rowKey={(tpl) => String(tpl.id ?? tpl.name)}
        />
      </Card>

      {/* Version History Modal */}
      <Modal
        open={showVersions}
        onClose={() => setShowVersions(false)}
        title={t('answerTemplates.versionHistory')}
        width="700px"
        footer={<Button variant="secondary" size="sm" onClick={() => setShowVersions(false)}>{t('common.close')}</Button>}
      >
        {versionsLoading ? (
          <div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('common.loading')}</div>
        ) : versions.length === 0 ? (
          <div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('answerTemplates.noVersions')}</div>
        ) : (
          <div className="space-y-2">
            {versions.map(v => (
              <div key={v.id} className="flex items-center justify-between px-4 py-3 rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)]">
                <div className="flex items-center gap-3">
                  <span className="text-xs font-mono font-bold text-blue-400 bg-blue-500/10 px-1.5 py-0.5 rounded">
                    v{v.version}
                  </span>
                  <span className="text-xs text-[var(--text-muted)]">{v.description || '-'}</span>
                </div>
                <div className="flex items-center gap-3">
                  <span className="text-[11px] text-[var(--text-muted)] font-mono">
                    {new Date(v.created_at).toLocaleString()}
                  </span>
                  <button
                    onClick={() => handleRollback(v.version)}
                    className="px-2 py-1 text-[11px] font-medium rounded bg-accent-yellow/10 text-accent-yellow hover:bg-accent-yellow/20 transition-colors"
                  >
                    {t('answerTemplates.rollback')}
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}

        {versions.length >= 2 && (
          <div className="mt-4">
            <div className="flex items-center gap-3 mb-3">
              <span className="text-xs font-medium text-[var(--text-muted)]">{t('answerTemplates.compare')}</span>
              <select
                value={diffVerA}
                onChange={e => setDiffVerA(e.target.value ? Number(e.target.value) : '')}
                className="px-2 py-1 text-xs rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
              >
                <option value="">{t('answerTemplates.oldVersion')}</option>
                {versions.filter(v => v.version !== diffVerB).map(v => (
                  <option key={v.version} value={v.version}>v{v.version}</option>
                ))}
              </select>
              <span className="text-[10px] text-[var(--text-muted)]">→</span>
              <select
                value={diffVerB}
                onChange={e => setDiffVerB(e.target.value ? Number(e.target.value) : '')}
                className="px-2 py-1 text-xs rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
              >
                <option value="">{t('answerTemplates.newVersion')}</option>
                {versions.filter(v => v.version !== diffVerA).map(v => (
                  <option key={v.version} value={v.version}>v{v.version}</option>
                ))}
              </select>
            </div>

            {diffVerA !== '' && diffVerB !== '' && (
              <div className="rounded-lg border border-[var(--bg-border)] overflow-hidden">
                <div className="max-h-80 overflow-y-auto font-mono text-[11px] leading-5">
                  {(() => {
                    const contentA = getVersionContent(diffVerA)
                    const contentB = getVersionContent(diffVerB)
                    const diff = computeDiff(contentA, contentB)
                    return diff.map((d, idx) => (
                      <div
                        key={idx}
                        className={`px-3 whitespace-pre-wrap ${
                          d.type === 'add' ? 'bg-accent-green/10 text-accent-green border-l-2 border-accent-green' :
                          d.type === 'remove' ? 'bg-accent-red/10 text-accent-red border-l-2 border-accent-red' :
                          'text-[var(--text-muted)] border-l-2 border-transparent'
                        }`}
                      >
                        {d.type === 'add' ? '+ ' : d.type === 'remove' ? '- ' : '  '}{d.line}
                      </div>
                    ))
                  })()}
                </div>
              </div>
            )}
          </div>
        )}
      </Modal>
      <ConfirmDialog
        open={confirmBatchDelete}
        onClose={() => setConfirmBatchDelete(false)}
        onConfirm={doBatchDelete}
        title={t('answerTemplates.batchDelete')}
        message={t('answerTemplates.batchDeleteConfirm', { count: selected.size })}
      />
      <ConfirmDialog
        open={confirmRollback !== null}
        onClose={() => setConfirmRollback(null)}
        onConfirm={doRollback}
        title={t('answerTemplates.rollbackVersion')}
        message={t('answerTemplates.rollbackConfirm', { version: confirmRollback })}
      />
    </div>
  )
}
