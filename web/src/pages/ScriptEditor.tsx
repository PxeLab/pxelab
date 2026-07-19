import { useState, useEffect, useRef, useCallback } from 'react'
import {
  Code, Clock, RefreshCw, RotateCcw, Trash2,
  FileCode, History, Eye, EyeOff, Upload,
} from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import {
  type ScriptMeta, type ScriptVersion, type ScriptDetail, type Profile,
} from '../api/client'
import { api } from '../api/client'

export default function ScriptEditor() {
  const { success, error } = useToast()
  const editorRef = useRef<HTMLTextAreaElement>(null)

  const [scripts, setScripts] = useState<ScriptMeta[]>([])
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [selectedID, setSelectedID] = useState<string | null>(null)
  const [detail, setDetail] = useState<ScriptDetail | null>(null)
  const [content, setContent] = useState('')
  const [originalContent, setOriginalContent] = useState('')
  const [versions, setVersions] = useState<ScriptVersion[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [comment, setComment] = useState('')
  const [showVersions, setShowVersions] = useState(false)
  const [diffVer, setDiffVer] = useState<string | null>(null)
  const [diffContent, setDiffContent] = useState('')
  const [syncOpen, setSyncOpen] = useState(false)
  const [previewMode, setPreviewMode] = useState(false)

  const hasChanges = content !== originalContent

  const loadScripts = useCallback(async () => {
    setLoading(true)
    try {
      const [sRes, pRes] = await Promise.all([
        api.getScripts(),
        api.getProfiles().catch(() => ({ data: [] as Profile[] })),
      ])
      setScripts(sRes.data)
      setProfiles(pRes.data)
    } catch { error('Failed to load scripts') }
    finally { setLoading(false) }
  }, [error])

  useEffect(() => { loadScripts() }, [loadScripts])

  const selectScript = async (id: string) => {
    setSelectedID(id)
    setShowVersions(false)
    setDiffVer(null)
    try {
      const d = await api.getScript(id)
      setDetail(d.data)
      setContent(d.data.content)
      setOriginalContent(d.data.content)
      const v = await api.getScriptVersions(id)
      setVersions(v.data)
    } catch { error('Failed to load script') }
  }

  const handleSave = async () => {
    if (!selectedID) return
    setSaving(true)
    try {
      await api.saveScript(selectedID, content, comment || undefined)
      success('Script saved')
      setOriginalContent(content)
      setComment('')
      const v = await api.getScriptVersions(selectedID)
      setVersions(v.data)
    } catch (err: any) { error(err.message) }
    finally { setSaving(false) }
  }

  const handleRollback = async (verID: string) => {
    if (!selectedID) return
    try {
      const r = await api.rollbackScript(selectedID, verID)
      setContent(r.data.content)
      setOriginalContent(r.data.content)
      success('Rolled back')
      const v = await api.getScriptVersions(selectedID)
      setVersions(v.data)
    } catch (err: any) { error(err.message) }
  }

  const showDiff = async (verID: string) => {
    if (!selectedID) return
    setDiffVer(verID)
    try {
      const d = await api.getScriptDiff(selectedID, verID)
      setDiffContent(d.data.diff)
    } catch { error('Failed to load diff') }
  }

  const syncAllFromProfiles = async () => {
    setSyncOpen(false)
    let synced = 0
    for (const p of profiles) {
      const entries = p.menu?.entries || []
      for (let i = 0; i < entries.length; i++) {
        const e = entries[i]
        let scriptContent = ''
        if (e.type === 'custom' && e.script) {
          scriptContent = e.script
        } else if (e.type === 'direct') {
          scriptContent = `#!ipxe\nkernel ${e.kernel || ''} ${e.cmdline || ''}\ninitrd ${e.initrd || ''}\nboot`
        }
        if (scriptContent) {
          try {
            await api.syncScript(p.id, e.label || p.name, e.type, scriptContent)
            synced++
          } catch {}
        }
      }
    }
    success(`Synced ${synced} scripts from profiles`)
    loadScripts()
  }

  const currentLabel = detail?.meta?.label || selectedID?.split(':')[1] || selectedID

  return (
    <div className="h-full flex flex-col gap-4">
      <div className="flex items-center justify-between shrink-0">
        <h1 className="text-lg font-bold text-[var(--text-primary)] flex items-center gap-2">
          <Code size={18} />
          Script Editor
        </h1>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={() => setSyncOpen(true)} disabled={loading}>
            <Upload size={14} /> Sync
          </Button>
          <Button variant="secondary" size="sm" onClick={loadScripts} disabled={loading}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          </Button>
        </div>
      </div>

      <div className="flex gap-4 flex-1 min-h-0">
        <div className="w-60 shrink-0 flex flex-col gap-2 overflow-hidden">
          <input placeholder="Search scripts..." className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 shrink-0" onChange={e => {
            const q = e.target.value.toLowerCase()
            document.querySelectorAll('[data-script-item]').forEach(el => {
              const item = el as HTMLElement
              item.style.display = item.textContent?.toLowerCase().includes(q) ? '' : 'none'
            })
          }} />
          <div className="flex-1 overflow-y-auto space-y-0.5">
            {scripts.length === 0 && !loading && (
              <div className="text-xs text-[var(--text-muted)] text-center py-4">No scripts yet.<br />Click Sync to import.</div>
            )}
            {scripts.map(s => (
              <button key={s.id} data-script-item
                onClick={() => selectScript(s.id)}
                className={`w-full text-left px-3 py-2 rounded-lg text-xs transition-colors ${
                  selectedID === s.id
                    ? 'bg-blue-500/10 text-blue-400 border border-blue-500/30'
                    : 'text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] border border-transparent'
                }`}
              >
                <div className="font-medium truncate">{s.label}</div>
                <div className="flex items-center gap-2 mt-0.5">
                  <Tag color={s.type === 'custom' ? 'purple' : 'cyan'}>{s.type}</Tag>
                  <span className="text-[10px] text-[var(--text-muted)]">{s.updated_at?.slice(0, 10)}</span>
                </div>
              </button>
            ))}
          </div>
        </div>

        <div className="flex-1 flex flex-col min-w-0 gap-3">
          {selectedID && detail ? (
            <>
              <div className="flex items-center justify-between shrink-0">
                <div className="flex items-center gap-3">
                  <span className="text-sm font-semibold text-[var(--text-primary)]">{currentLabel}</span>
                  <span className="font-mono text-[10px] text-[var(--text-muted)]">{selectedID}</span>
                  {hasChanges && <Tag color="yellow">unsaved</Tag>}
                </div>
                <div className="flex items-center gap-2">
                  <button onClick={() => setPreviewMode(!previewMode)}
                    className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] flex items-center gap-1">
                    {previewMode ? <EyeOff size={13} /> : <Eye size={13} />}
                    {previewMode ? 'Edit' : 'Preview'}
                  </button>
                  <button onClick={() => setShowVersions(!showVersions)}
                    className="text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] flex items-center gap-1">
                    <History size={13} />
                    Versions
                  </button>
                </div>
              </div>

              <div className="flex-1 flex gap-3 min-h-0">
                <div className="flex-1 flex flex-col min-w-0">
                  {previewMode ? (
                    <pre className="flex-1 p-4 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-xs font-mono text-[var(--text-primary)] whitespace-pre-wrap break-all overflow-auto">
                      {content || '(empty)'}
                    </pre>
                  ) : (
                    <textarea ref={editorRef}
                      value={content}
                      onChange={e => setContent(e.target.value)}
                      className="flex-1 w-full resize-none p-4 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-xs font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 leading-relaxed"
                      spellCheck={false}
                      placeholder="#!ipxe"
                    />
                  )}
                </div>

                {showVersions && (
                  <div className="w-56 shrink-0 flex flex-col border border-[var(--bg-border)] rounded-lg bg-[var(--bg-card)] overflow-hidden">
                    <div className="text-xs font-semibold text-[var(--text-muted)] px-3 py-2 border-b border-[var(--bg-border)] flex items-center gap-1.5">
                      <Clock size={12} /> Version History
                    </div>
                    <div className="flex-1 overflow-y-auto">
                      {versions.map(v => (
                        <div key={v.id} className="px-3 py-2 border-b border-[var(--bg-border)] text-xs">
                          <div className="flex items-center justify-between gap-1">
                            <span className="font-mono text-[10px] text-[var(--text-muted)] truncate">{v.id.slice(0, 18)}...</span>
                            <div className="flex gap-1 shrink-0">
                              <button onClick={() => showDiff(v.id)} title="Diff"
                                className="text-[var(--text-muted)] hover:text-blue-400"><Eye size={11} /></button>
                              <button onClick={() => handleRollback(v.id)} title="Rollback"
                                className="text-[var(--text-muted)] hover:text-amber-400"><RotateCcw size={11} /></button>
                            </div>
                          </div>
                          <div className="text-[10px] text-[var(--text-muted)] mt-0.5">{v.created_at.slice(0, 19).replace('T', ' ')}</div>
                          {v.comment && <div className="text-[10px] text-[var(--text-secondary)] italic mt-0.5 truncate">{v.comment}</div>}
                        </div>
                      ))}
                      {versions.length === 0 && (
                        <div className="text-xs text-[var(--text-muted)] text-center py-4">No history</div>
                      )}
                    </div>
                  </div>
                )}
              </div>

              {hasChanges && (
                <div className="flex items-center gap-3 shrink-0">
                  <input value={comment} onChange={e => setComment(e.target.value)}
                    placeholder="Optional change comment..."
                    className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" />
                  <Button variant="secondary" size="sm" onClick={() => { setContent(originalContent); setComment('') }}>
                    <Trash2 size={13} /> Discard
                  </Button>
                  <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
                    {saving ? 'Saving...' : 'Save'}
                  </Button>
                </div>
              )}
            </>
          ) : (
            <div className="flex-1 flex items-center justify-center">
              <div className="text-center text-[var(--text-muted)]">
                <FileCode size={40} className="mx-auto mb-3 opacity-30" />
                <p className="text-sm">Select a script from the left panel</p>
                <p className="text-xs mt-1">or sync profiles to import existing scripts</p>
              </div>
            </div>
          )}
        </div>
      </div>

      <Modal open={!!diffVer && diffContent !== ''} onClose={() => { setDiffVer(null); setDiffContent('') }} title="Diff" width="700px">
        <pre className="p-4 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-xs font-mono whitespace-pre-wrap break-all max-h-96 overflow-auto leading-relaxed">
          {diffContent.split('\n').map((line, i) => {
            let cls = 'text-[var(--text-primary)]'
            if (line.startsWith('+ ')) cls = 'text-green-400'
            else if (line.startsWith('- ')) cls = 'text-red-400'
            else if (line.startsWith('  ')) cls = 'text-[var(--text-muted)]'
            return <div key={i} className={cls}>{line || ' '}</div>
          })}
        </pre>
      </Modal>

      <Modal open={syncOpen} onClose={() => setSyncOpen(false)} title="Sync Scripts from Profiles" width="480px">
        <div className="space-y-3">
          <p className="text-xs text-[var(--text-secondary)]">
            This will scan all profiles and register their boot scripts with the version manager.
            Existing scripts will be updated only if content changed.
          </p>
          <div className="max-h-60 overflow-y-auto space-y-1">
            {profiles.map(p => (
              <div key={p.id} className="flex items-center gap-3 px-3 py-2 bg-[var(--bg-input)] rounded-lg">
                <FileCode size={14} className="text-[var(--text-muted)] shrink-0" />
                <div className="flex-1 min-w-0">
                  <div className="text-xs font-medium text-[var(--text-primary)] truncate">{p.name}</div>
                  <div className="text-[10px] text-[var(--text-muted)]">{p.menu?.entries?.length || 0} entries</div>
                </div>
                <Tag color={p.menu?.entries?.some(e => e.type === 'custom') ? 'purple' : 'cyan'}>
                  {p.menu?.entries?.map(e => e.type).join(', ') || 'none'}
                </Tag>
              </div>
            ))}
          </div>
          <div className="flex justify-end gap-2">
            <Button variant="ghost" size="sm" onClick={() => setSyncOpen(false)}>Cancel</Button>
            <Button variant="primary" size="sm" onClick={syncAllFromProfiles}>
              <Upload size={13} /> Sync All
            </Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
