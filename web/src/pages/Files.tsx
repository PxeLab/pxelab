import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, RefreshCw, Folder, File, Trash2, Search } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Tag } from '../components/ui/Tag'
import { EmptyState } from '../components/ui/EmptyState'
import { useToast } from '../components/ui/Toast'
import { api, type FileInfo } from '../api/client'

export default function Files() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [files, setFiles] = useState<FileInfo[]>([])
  const [loading, setLoading] = useState(true)
  const [currentDir, setCurrentDir] = useState('.')
  const [search, setSearch] = useState('')
  const fileInputRef = useRef<HTMLInputElement>(null)

  useEffect(() => { loadFiles() }, [currentDir])

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
    if (!confirm(t('files.deleteConfirm'))) return
    const path = currentDir === '.' ? name : currentDir + '/' + name
    try {
      await api.deleteFile(path)
      success( `${name} ${t('common.deleted', '已删除')}`)
      loadFiles()
    } catch (err: any) { error(err.message) }
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

  return (
    <div>
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

      <Card
        title={'/' + (currentDir === '.' ? '' : currentDir)}
        footer={<span className="text-[var(--text-muted)]">{files.length} {t('common.items', '个项目')}</span>}
      >
        {currentDir !== '.' && (
          <button onClick={() => setCurrentDir('.')} className="flex items-center gap-2 px-2 py-1.5 rounded text-sm text-[var(--text-secondary)] hover:bg-[var(--bg-hover)] hover:text-[var(--text-primary)] transition-colors mb-1">
            .. 返回上级
          </button>
        )}
        {loading ? (
          <div className="space-y-2">
            {Array.from({ length: 5 }).map((_, i) => (
              <div key={i} className="h-9 bg-[var(--bg-card)] rounded animate-[shimmer_1.5s_infinite] bg-gradient-to-r from-[var(--bg-card)] via-[var(--bg-hover)] to-[var(--bg-card)] bg-[length:200%_100%]" />
            ))}
          </div>
        ) : filtered.length === 0 ? (
          <EmptyState title={t('files.empty')} />
        ) : (
          <div className="divide-y divide-[#232738]">
            {folders.map(f => (
              <div key={f.name} className="flex items-center gap-3 px-2 py-2 rounded cursor-pointer hover:bg-[var(--bg-hover)] transition-colors group" onClick={() => enterDir(f.name)}>
                <Folder size={16} className="text-yellow-500 shrink-0" />
                <span className="flex-1 text-sm text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]">{f.name}/</span>
                <span className="text-[11px] text-[var(--text-muted)] font-mono">—</span>
                <Tag color="yellow">dir</Tag>
              </div>
            ))}
            {fileItems.map(f => (
              <div key={f.name} className="flex items-center gap-3 px-2 py-2 rounded hover:bg-[var(--bg-hover)] transition-colors group">
                <File size={16} className="text-blue-500 shrink-0" />
                <span className="flex-1 text-sm text-[var(--text-secondary)] group-hover:text-[var(--text-primary)]">{f.name}</span>
                <span className="text-[11px] text-[var(--text-muted)] font-mono">{sizeStr(f.size)}</span>
                <button onClick={() => handleDelete(f.name)} className="opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-red-400 transition-all">
                  <Trash2 size={14} />
                </button>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
