import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, RefreshCw, Folder, File, Trash2, Search, Info } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Tag } from '../components/ui/Tag'
import { DataTable, type Column } from '../components/ui/DataTable'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
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

  const columns: Column<FileInfo>[] = [
    {
      key: 'icon',
      label: '',
      width: '2rem',
      render: f => f.is_dir
        ? <Folder size={16} className="text-accent-yellow" />
        : <File size={16} className="text-blue-500" />,
    },
    {
      key: 'name',
      label: t('files.colName'),
      render: f => f.is_dir ? `${f.name}/` : f.name,
    },
    {
      key: 'size',
      label: t('files.colSize'),
      width: '6rem',
      className: 'text-right text-[var(--text-muted)] font-mono text-xs',
      render: f => f.is_dir ? '—' : sizeStr(f.size),
    },
    {
      key: 'modtime',
      label: t('files.colModified'),
      width: '8rem',
      className: 'text-[var(--text-muted)] text-xs',
      render: f => timeStr(f.modtime),
    },
    {
      key: 'md5',
      label: 'MD5',
      width: '7rem',
      render: f => {
        if (f.is_dir) return <span className="text-xs text-[var(--text-muted)] font-mono">—</span>
        return f.md5 ? (
          <span className="text-[10px] text-[var(--text-muted)] font-mono" title={f.md5}>{f.md5.slice(0, 16)}</span>
        ) : (
          <span className="text-[10px] text-[var(--text-muted)]">—</span>
        )
      },
    },
    {
      key: 'actions',
      label: '',
      width: '2.5rem',
      className: 'group',
      render: f => f.is_dir ? (
        <Tag color="yellow">dir</Tag>
      ) : (
        <button
          onClick={e => { e.stopPropagation(); handleDelete(f.name) }}
          className="opacity-0 group-hover:opacity-100 text-[var(--text-muted)] hover:text-accent-red transition-all"
        >
          <Trash2 size={14} />
        </button>
      ),
    },
  ]

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
          <PageHeader
            title={t('files.title')}
            actions={
              <>
                <Button variant="secondary" size="sm" onClick={loadFiles}>
                  <RefreshCw size={14} /> {t('common.refresh', '刷新')}
                </Button>
                <Button variant="primary" size="sm" onClick={() => fileInputRef.current?.click()}>
                  <Upload size={14} /> {t('files.upload')}
                </Button>
                <input ref={fileInputRef} type="file" className="hidden" onChange={handleUpload} />
              </>
            }
          />
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
        <DataTable
          columns={columns}
          data={[...folders, ...fileItems]}
          loading={loading}
          onRowClick={f => { if (f.is_dir) enterDir(f.name) }}
          emptyText={t('files.empty')}
          rowKey={f => (f.is_dir ? 'd:' : 'f:') + f.name}
        />
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
