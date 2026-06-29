import { useState, useEffect, useCallback, useRef } from 'react'
import { Card } from '../components/ui/Card'
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
  const [templates, setTemplates] = useState<AnswerTemplate[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState<AnswerTemplate | null>(null)
  const [showEditor, setShowEditor] = useState(false)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const importRef = useRef<HTMLInputElement>(null)

  // Version history state
  const [showVersions, setShowVersions] = useState(false)
  const [versionTemplateId, setVersionTemplateId] = useState<number | null>(null)
  const [versions, setVersions] = useState<AnswerTemplateVersion[]>([])
  const [versionsLoading, setVersionsLoading] = useState(false)
  const [diffVerA, setDiffVerA] = useState<number | ''>('')
  const [diffVerB, setDiffVerB] = useState<number | ''>('')

  const load = useCallback(async () => {
    try {
      const res = await api.getAnswerTemplates()
      setTemplates(res.data.templates)
      setError('')
    } catch (err: any) {
      setError(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { load() }, [load])

  const save = async () => {
    if (!editing) return
    try {
      if (editing.id) {
        await api.updateAnswerTemplate(editing.id, editing)
      } else {
        await api.createAnswerTemplate(editing)
      }
      setShowEditor(false)
      setEditing(null)
      await load()
    } catch (err: any) {
      setError(err.message || '保存失败')
    }
  }

  const remove = async (id: number) => {
    try {
      await api.deleteAnswerTemplate(id)
      setSelected(prev => { const next = new Set(prev); next.delete(id); return next })
      await load()
    } catch (err: any) {
      setError(err.message || '删除失败')
    }
  }

  const removeBatch = async () => {
    if (selected.size === 0) return
    if (!confirm(`确定删除选中的 ${selected.size} 个模板？`)) return
    for (const id of selected) {
      try { await api.deleteAnswerTemplate(id) } catch {}
    }
    setSelected(new Set())
    await load()
  }

  const variablesForType = (type: string) => {
    const vars = [...HOST_VARS]
    if (type === 'autounattend') vars.push(...WINDOWS_VARS)
    return vars
  }

  const openNew = () => {
    setEditing({ name: '', description: '', type: 'preseed', content: '' })
    setShowEditor(true)
  }

  const openNewWithContent = (content: string, name: string) => {
    setEditing({ name, description: '', type: 'preseed', content })
    setShowEditor(true)
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
    if (!confirm(`确定回滚到版本 ${version}？当前内容将被保存为新版本。`)) return
    try {
      await api.rollbackAnswerTemplate(versionTemplateId, version)
      setShowVersions(false)
      await load()
    } catch (err: any) {
      setError(err.message || '回滚失败')
    }
  }

  // Simple LCS-based line diff
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
      setSelected(new Set(templates.filter(t => t.id).map(t => t.id!)))
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
    const items = templates.filter(t => t.id && selected.has(t.id))
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
      // Handle JSON batch import (from multi-export)
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
          if (count > 0) {
            setError('')
          }
        } catch {
          setError('JSON 格式错误，导入失败')
        }
        return
      }
      // Single file import — pre-fill editor
      const name = file.name.replace(/\.[^.]+$/, '')
      openNewWithContent(content, name)
    }
    reader.readAsText(file)
    e.target.value = ''
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-5">
        <div>
          <h2 className="text-base font-bold text-[var(--text-primary)]">应答模板管理</h2>
        </div>
        <div className="flex items-center gap-2">
          <button onClick={load} className="px-3 py-1.5 text-xs font-medium rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] transition-colors">
            刷新
          </button>
          <input ref={importRef} type="file" accept=".json,.cfg,.ks,.preseed,.xml,.yaml,.yml,.txt" onChange={handleImport} className="hidden" />
          <button onClick={() => importRef.current?.click()} className="px-3 py-1.5 text-xs font-medium rounded-lg border border-[var(--bg-border)] bg-[var(--bg-card)] text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] transition-colors">
            导入
          </button>
          <button onClick={openNew} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 transition-colors">
            + 新建模板
          </button>
        </div>
      </div>

      {error && (
        <div className="mb-4 px-4 py-3 rounded-lg bg-red-500/10 border border-red-500/20 text-sm text-red-400">
          {error}
          <button onClick={() => setError('')} className="float-right text-red-400/60 hover:text-red-400">✕</button>
        </div>
      )}

      {/* Selection toolbar */}
      {selected.size > 0 && (
        <div className="mb-3 px-4 py-2.5 rounded-lg bg-blue-500/10 border border-blue-500/20 flex items-center justify-between">
          <span className="text-xs text-blue-400">已选择 {selected.size} 项</span>
          <div className="flex gap-2">
            <button onClick={exportBatch} className="px-3 py-1 text-xs font-medium rounded bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 transition-colors">
              导出选中
            </button>
            <button onClick={removeBatch} className="px-3 py-1 text-xs font-medium rounded bg-red-500/10 text-red-400 hover:bg-red-500/20 transition-colors">
              删除选中
            </button>
            <button onClick={() => setSelected(new Set())} className="px-3 py-1 text-xs font-medium rounded bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] transition-colors">
              取消选择
            </button>
          </div>
        </div>
      )}

      {/* Template editor modal */}
      {showEditor && editing && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowEditor(false)}>
          <div className="max-w-3xl w-full mx-4 max-h-[90vh] overflow-y-auto rounded-xl bg-[var(--bg-card)] border border-[var(--bg-border)] p-5" onClick={e => e.stopPropagation()}>
            <h3 className="text-sm font-bold text-[var(--text-primary)] mb-4">
              {editing.id ? '编辑模板' : '新建模板'}
            </h3>

            <div className="grid grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">名称</label>
                <input
                  type="text" value={editing.name}
                  onChange={e => setEditing({ ...editing, name: e.target.value })}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">类型</label>
                <select
                  value={editing.type}
                  onChange={e => setEditing({ ...editing, type: e.target.value })}
                  className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                >
                  {TEMPLATE_TYPES.map(t => (
                    <option key={t} value={t}>{TYPE_LABELS[t] || t}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="mb-4">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">说明</label>
              <input
                type="text" value={editing.description || ''}
                onChange={e => setEditing({ ...editing, description: e.target.value })}
                className="w-full px-3 py-2 text-xs rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
              />
            </div>

            <div className="mb-4">
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">模板内容</label>
              <textarea
                value={editing.content}
                onChange={e => setEditing({ ...editing, content: e.target.value })}
                rows={16}
                className="w-full px-3 py-2 text-xs font-mono rounded-lg border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)] resize-y"
              />
            </div>

            {/* Variable hints panel */}
            <div className="mb-4 p-3 rounded-lg bg-blue-500/5 border border-blue-500/10">
              <p className="text-xs font-medium text-blue-400 mb-2">可用变量（点击复制）</p>
              <div className="flex flex-wrap gap-1.5">
                {variablesForType(editing.type).map(v => (
                  <button
                    key={v}
                    onClick={() => navigator.clipboard.writeText(v)}
                    className="px-2 py-0.5 text-[11px] font-mono rounded bg-blue-500/10 text-blue-300 hover:bg-blue-500/20 transition-colors"
                  >
                    {v}
                  </button>
                ))}
              </div>
            </div>

            <div className="flex justify-end gap-2">
              <button onClick={() => setShowEditor(false)} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)]">
                取消
              </button>
              <button onClick={save} disabled={!editing.name || !editing.content} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-blue-500/15 text-blue-400 hover:bg-blue-500/25 disabled:opacity-40 transition-colors">
                保存
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Template list */}
      <Card>
        {loading ? (
          <div className="py-12 text-center text-sm text-[var(--text-muted)]">加载中...</div>
        ) : templates.length === 0 ? (
          <div className="py-12 text-center text-sm text-[var(--text-muted)]">暂无模板，点击"新建模板"创建</div>
        ) : (
          <div className="overflow-x-auto -mx-5">
            <table className="w-full text-sm">
              <thead>
                <tr>
                  <th className="w-10 px-2 py-3 border-b border-[var(--bg-border)]">
                    <input
                      type="checkbox"
                      checked={selected.size === templates.length && templates.length > 0}
                      onChange={toggleSelectAll}
                      className="accent-blue-500"
                    />
                  </th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">名称</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">类型</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">说明</th>
                  <th className="text-left px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">更新时间</th>
                  <th className="text-right px-4 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--text-muted)] border-b border-[var(--bg-border)]">操作</th>
                </tr>
              </thead>
              <tbody>
                {templates.map(tpl => (
                  <tr key={tpl.id} className="hover:bg-white/[0.02] transition-colors">
                    <td className="px-2 py-3 border-b border-[var(--bg-border)] text-center">
                      <input
                        type="checkbox"
                        checked={tpl.id ? selected.has(tpl.id) : false}
                        onChange={() => tpl.id && toggleSelect(tpl.id)}
                        className="accent-blue-500"
                      />
                    </td>
                    <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                      <span className="text-sm font-medium text-[var(--text-primary)]">{tpl.name}</span>
                      {tpl.current_version && tpl.current_version > 1 && (
                        <span className="ml-2 inline-flex items-center px-1.5 py-0.5 text-[10px] font-mono rounded bg-blue-500/10 text-blue-400">
                          v{tpl.current_version}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                      <span className="inline-block px-1.5 py-0.5 text-[10px] font-semibold rounded bg-[var(--bg-muted)] text-[var(--text-muted)]">
                        {tpl.type}
                      </span>
                    </td>
                    <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                      <span className="text-xs text-[var(--text-muted)]">{tpl.description || '-'}</span>
                    </td>
                    <td className="px-4 py-3 border-b border-[var(--bg-border)]">
                      <span className="text-xs text-[var(--text-muted)]">{tpl.updated_at ? new Date(tpl.updated_at).toLocaleString() : '-'}</span>
                    </td>
                    <td className="px-4 py-3 border-b border-[var(--bg-border)] text-right">
                      <button onClick={() => openEdit(tpl)} className="px-2 py-1 text-xs font-medium rounded bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] mr-1">
                        编辑
                      </button>
                      <button onClick={() => exportSingle(tpl)} className="px-2 py-1 text-xs font-medium rounded bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] mr-1">
                        导出
                      </button>
                      <button onClick={() => tpl.id && openVersions(tpl.id)} className="px-2 py-1 text-xs font-medium rounded bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:text-[var(--text-primary)] mr-1">
                        版本
                      </button>
                      <button onClick={() => tpl.id && remove(tpl.id)} className="px-2 py-1 text-xs font-medium rounded bg-red-500/10 text-red-400 hover:bg-red-500/20">
                        删除
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {/* Version History Modal */}
      {showVersions && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={() => setShowVersions(false)}>
          <div className="max-w-2xl w-full mx-4 max-h-[80vh] overflow-y-auto rounded-xl bg-[var(--bg-card)] border border-[var(--bg-border)] p-5" onClick={e => e.stopPropagation()}>
            <h3 className="text-sm font-bold text-[var(--text-primary)] mb-4">版本历史</h3>

            {versionsLoading ? (
              <div className="py-8 text-center text-sm text-[var(--text-muted)]">加载中...</div>
            ) : versions.length === 0 ? (
              <div className="py-8 text-center text-sm text-[var(--text-muted)]">暂无版本记录</div>
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
                        className="px-2 py-1 text-[11px] font-medium rounded bg-yellow-500/10 text-yellow-400 hover:bg-yellow-500/20 transition-colors"
                      >
                        回滚
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* Diff comparison */}
            {versions.length >= 2 && (
              <div className="mt-4">
                <div className="flex items-center gap-3 mb-3">
                  <span className="text-xs font-medium text-[var(--text-muted)]">对比</span>
                  <select
                    value={diffVerA}
                    onChange={e => setDiffVerA(e.target.value ? Number(e.target.value) : '')}
                    className="px-2 py-1 text-xs rounded border border-[var(--bg-border)] bg-[var(--bg-input)] text-[var(--text-primary)]"
                  >
                    <option value="">旧版本</option>
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
                    <option value="">新版本</option>
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
                              d.type === 'add' ? 'bg-green-500/10 text-green-300 border-l-2 border-green-500' :
                              d.type === 'remove' ? 'bg-red-500/10 text-red-300 border-l-2 border-red-500' :
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

            <div className="flex justify-end mt-4">
              <button onClick={() => setShowVersions(false)} className="px-3 py-1.5 text-xs font-medium rounded-lg bg-[var(--bg-hover)] text-[var(--text-secondary)]">
                关闭
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
