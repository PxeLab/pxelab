import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, RefreshCw, Folder, File, Trash2, Search, Info } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Tag } from '../components/ui/Tag'
import { EmptyState } from '../components/ui/EmptyState'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { useToast } from '../components/ui/Toast'
import { api, type FileInfo } from '../api/client'

export default function Files({ hideHeader, rootPath }: { hideHeader?: boolean; rootPath?: string }) {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [files, setFiles] = useState<FileInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [currentDir, setCurrentDir] = useState('.')
  const [rootDir, setRootDir] = useState('')
  const [search, setSearch] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { loadFiles() }, [currentDir])

  useEffect(() => {
    api.getBootRootDir().then(res => setRootDir(res.data.root_dir)).catch(() => {})
  }, [])

  async function loadFiles() {
    setLoading(true)
    try {
      const res = await api.getFiles(currentDir)
      setFiles(res.data)
    } catch (err: any) { error(err.message) }
    finally { setLoading(false) }
  }

  async function handleUpload(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    try {
      await api.uploadFile(file)
      success( `${t('files.upload')}: ${file.name}`)
      loadFiles()
    } catch (err: any) { error(err.message) }
    if (fileInputRef.current) fileInputRef.current.value = ''
  }

  async function handleDelete(name: string) {
    setConfirmDelete(name)
  }

  async function doDelete() {
    if (!confirmDelete) return
    const path = currentDir === '.' ? confirmDelete : currentDir + '/' + confirmDelete
    try {
      await api.deleteFile(path)
      success( `${confirmDelete} ${t('common.deleted', '已删除')}`)
      loadFiles()
    } catch (err: any) { error(err.message) }
    setConfirmDelete(null)
  }

  function enterDir(name: string) {
    const dir = currentDir === '.' ? name : currentDir + '/' + name
    setCurrentDir(dir)
  }

  const filtered = search
    ? files.filter(f => f.name.toLowerCase().includes(search.toLowerCase()))
    : files

  const folders = filtered.filter(f => f.is_dir)
  const fileItems = filtered.filter(f => !f.is_dir)

  const sizeStr = (size: number) => {
    if (size < 1024) return size + ' B'
    if (size < 1024 * 1024) return (size / 1024).toFixed(1) + ' KB'
    return (size / (1024 * 1024)).toFixed(1) + ' MB'
  }

  const timeStr = (modtime: string) => {
    try {
      const d = new Date(modtime)
      const pad = (n: number) => n.toString().padStart(2, '0')
      return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
    } catch {
      return modtime
    }
  }

  const displayPath = currentDir === '.' ? (rootPath || '') : (rootPath ? rootPath + '/' + currentDir : currentDir)

  return (
    <div>
      {hideHeader ? (
        <div className="flex items-center justify-between mb-4 pt-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
            <input
              className="w-[280px] bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg py-2 pl-9 pr-3 text-sm text-[var(--text-primary)] placeholder-[var(--text-muted)] outline-none focus:border-blue-500"
              placeholder={t('common.search') + '...'}
              value={search} onChange={e => setSearch(e.target.value)}
            />
          </div>
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" onClick={loadFiles}>
              <RefreshCw size={14} /> {t('common.refresh', '刷新')}
            </Button>
            <Button variant="primary" size="sm" onClick={() => fileInputRef.current?.click()}>
              <Upload size={14} /> {t('files.upload')}
            </Button>
            <input ref={fileInputRef} type="file" className="hidden" onChange={handleUpload} />
          </div>
        </div>
      ) : (
        <>
          <div className="flex items-center justify-between mb-6">
            <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('files.title')}</h1>
            <div className="flex gap-2">
              <Button variant="secondary" size="sm" onClick={loadFiles}>
                <RefreshCw size={14} /> {t('common.refresh', '刷新')}
              </Button>
              <Button variant="primary" size="sm" onClick={() => fileInputRef.current?.click()}>
                <Upload size={14} /> {t('files.upload')}
              </Button>
              <input ref={fileInputRef} type="file" className="hidden" onChange={handleUpload} />
            </div>
          </div>
          <div className="flex items-center justify-between mb-6">
            <div className="flex items-center gap-3">
              <div className="relative">
                <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
                <input
                  className="w-[280px] bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg py-2 pl-9 pr-3 text-sm text-[var(--text-primary)] placeholder-[var(--text-muted)] outline-none focus:border-blue-500"
                  placeholder={t('common.search') + '...'}
                  value={search} onChange={e => setSearch(e.target.value)}
                />
              </div>
            </div>
          </div>
        </>
      )}

      {rootDir && (
        <div className="mb-3 flex items-center gap-2 px-3 py-2 rounded-lg bg-[var(--bg-input)] border border-[var(--bg-border)]">
          <Info size={14} className="text-blue-400 shrink-0" />
          <span className="text-xs text-[var(--text-muted)]">
            {t('files.rootDir')}: <span className="font-mono text-[var(--text-secondary)]">{rootDir}</span>
          </span>
          <span className="text-xs text-[var(--text-muted)]">— {t('files.rootDirHint')}</span>
        </div>
      )}

      <Card
        footer={<span className="text-[var(--text-muted)]">{files.length} {t('common.items', '个项目')}</span>}
      >
        {currentDir !== '.' && (
          <button onClick={() => setCurrentDir('.')} className="flex items-center gap-2 px-2 py-1.5 rounded text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors mb-1">
            {t('files.backToParent')}
          </button>
        )}
        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-9 bg-[var(--bg-card)] rounded animate-shimmer" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState title={t('files.empty')} />
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead className="border-b border-[var(--bg-border)]">
                <tr className="text-left text-xs font-semibold text-[var(--text-secondary)]">
                  <th className="px-3 py-2 w-8"></th>
                  <th className="px-3 py-2">{t('files.colName')}</th>
                  <th className="px-3 py-2 w-24 text-right">{t('files.colSize')}</th>
                  <th className="px-3 py-2 w-32">{t('files.colModified')}</th>
                  <th className="px-3 py-2 w-28">MD5</th>
                  <th className="px-3 py-2 w-10"></th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--bg-border)]">
                {folders.map(f => (
                  <tr key={f.name}
                    onClick={() => enterDir(f.name)}
                    className="cursor-pointer hover:bg-[var(--bg-hover)] transition-colors"
                  >
                    <td className="px-3 py-2"><Folder size={16} className="text-yellow-500" /></td>
                    <td className="px-3 py-2 text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]">{f.name}/</td>
                    <td className="px-3 py-2 text-right text-[var(--text-muted)] font-mono text-xs">—</td>
                    <td className="px-3 py-2 text-[var(--text-muted)] text-xs">{timeStr(f.modtime)}</td>
                    <td className="px-3 py-2 text-[var(--text-muted)] text-xs font-mono">—</td>
                    <td className="px-3 py-2"><Tag color="yellow">dir</Tag></td>
                  </tr>
                ))}
                {fileItems.map(f => (
                  <tr key={f.name} className="hover:bg-[var(--bg-hover)] transition-colors group">
                    <td className="px-3 py-2"><File size={16} className="text-blue-500" /></td>
                    <td className="px-3 py-2 text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]">{f.name}</td>
                    <td className="px-3 py-2 text-right text-[var(--text-muted)] font-mono text-xs">{sizeStr(f.size)}</td>
                    <td className="px-3 py-2 text-[var(--text-muted)] text-xs">{timeStr(f.modtime)}</td>
                    <td className="px-3 py-2">
                      {f.md5 ? (
                        <span className="text-[10px] text-[var(--text-muted)] font-mono" title={f.md5}>{f.md5.slice(0, 16)}</span>
                      ) : (
                        <span className="text-[10px] text-[var(--text-muted)]">—</span>
                      )}
                    </td>
                    <td className="px-3 py-2">
                      <button onClick={() => handleDelete(f.name)}
                        className="opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-red-400 transition-all"
                      >
                        <Trash2 size={14} />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        {rootPath && (
          <p className="px-1 pt-3 text-xs text-[var(--text-muted)] font-mono">{t('files.path')}:{displayPath}</p>
        )}
      </Card>
      <ConfirmDialog
        open={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={doDelete}
        title={t('files.deleteTitle')}
        message={t('files.deleteConfirm')}
      />
    </div>
  )
}
