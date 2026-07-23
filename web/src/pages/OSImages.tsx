import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, Trash2, RefreshCw, Disc, FileArchive, HardDrive, Search, AlertCircle, CheckCircle, Clock, FolderInput, Pencil } from 'lucide-react'
import { PieChart, Pie, Cell } from 'recharts'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Input } from '../components/ui/FormControls'
import { useToast } from '../components/ui/Toast'
import { api, type OSImage } from '../api/client'

function formatTime(s: string) {
  try { return new Date(s).toLocaleString() } catch { return s }
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function statusIcon(status: string) {
  switch (status) {
    case 'ready': return <CheckCircle size={14} className="text-accent-green" />
    case 'uploading':
    case 'validating': return <Clock size={14} className="text-accent-yellow" />
    case 'missing':
    case 'error': return <AlertCircle size={14} className="text-accent-red" />
    default: return <Clock size={14} className="text-[var(--text-muted)]" />
  }
}

function statusColorClass(status: string) {
  switch (status) {
    case 'ready': return 'text-accent-green'
    case 'uploading':
    case 'validating': return 'text-accent-yellow'
    case 'missing':
    case 'error': return 'text-accent-red'
    default: return 'text-[var(--text-muted)]'
  }
}

const LINUX_DISTROS = ['ubuntu', 'debian', 'centos', 'rocky', 'almalinux', 'fedora', 'rhel', 'openeuler', 'kylin', 'anolis', 'uos', 'live']

type DistroKind = 'linux' | 'windows' | 'other'

function distroKind(distro?: string): DistroKind {
  const d = (distro || '').toLowerCase()
  if (LINUX_DISTROS.includes(d)) return 'linux'
  if (d === 'windows') return 'windows'
  return 'other'
}

interface Stats {
  total: number
  totalSize: number
  linux: number
  windows: number
  other: number
  extracted: number
  notExtracted: number
  mounted: number
  notMounted: number
}

function computeStats(images: OSImage[]): Stats {
  const s: Stats = { total: images.length, totalSize: 0, linux: 0, windows: 0, other: 0, extracted: 0, notExtracted: 0, mounted: 0, notMounted: 0 }
  for (const img of images) {
    s.totalSize += img.size || 0
    s[distroKind(img.distro)]++
    if (img.extracted_to) s.extracted++; else s.notExtracted++
    if (img.mount_point) s.mounted++; else s.notMounted++
  }
  return s
}

export default function OSImages() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [images, setImages] = useState<OSImage[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [uploadProgress, setUploadProgress] = useState<{ name: string; percent: number } | null>(null)
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<OSImage | null>(null)
  const [unmountTarget, setUnmountTarget] = useState<OSImage | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importDir, setImportDir] = useState('')
  const [importRecursive, setImportRecursive] = useState(false)
  const [importing, setImporting] = useState(false)
  const [editTarget, setEditTarget] = useState<OSImage | null>(null)
  const [editForm, setEditForm] = useState({ name: '', distro: '', version: '', arch: '' })

  const fileRef = useRef<HTMLInputElement>(null)
  const [dragOver, setDragOver] = useState(false)

  const loadImages = async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const res = await api.getOSImages()
      setImages(res.data ?? [])
    } catch (err: any) { showError(err.message) }
    finally { if (!silent) setLoading(false) }
  }

  useEffect(() => { loadImages() }, [])

  // 有镜像处于处理中（上传后异步校验/识别）时，每 3 秒静默刷新直到全部就绪
  const hasPending = images.some(img => img.status === 'uploading' || img.status === 'validating')
  useEffect(() => {
    if (!hasPending) return
    const iv = setInterval(() => loadImages(true), 3000)
    return () => clearInterval(iv)
  }, [hasPending])

  const handleUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return
    setUploading(true)
    try {
      for (let i = 0; i < files.length; i++) {
        const f = files[i]
        if (!f.name.toLowerCase().endsWith('.iso')) {
          showError(t('osImages.onlyIso', { name: f.name }))
          continue
        }
        setUploadProgress({ name: f.name, percent: 0 })
        await api.uploadOSImageWithProgress(f, (p) => setUploadProgress({ name: f.name, percent: p }))
        success(t('osImages.uploaded', { name: f.name }))
      }
      await loadImages()
    } catch (err: any) { showError(err.message) }
    finally { setUploading(false); setUploadProgress(null) }
  }

  const handleDelete = async () => {
    if (!deleteTarget?.id) return
    try {
      await api.deleteOSImage(deleteTarget.id)
      success(t('osImages.deleted'))
      setDeleteTarget(null)
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const handleExtract = async (id: number) => {
    try {
      await api.extractOSImage(id)
      success(t('osImages.extracted'))
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const handleMount = async (id: number) => {
    try {
      await api.mountOSImage(id)
      success(t('osImages.mounted'))
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const handleUnmount = async (id: number) => {
    try {
      await api.unmountOSImage(id)
      success(t('osImages.unmounted'))
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const handleReprocess = async (id: number) => {
    try {
      await api.reprocessOSImage(id)
      success(t('osImages.reprocessing'))
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const handleImport = async () => {
    if (!importDir.trim()) return
    setImporting(true)
    try {
      const res = await api.importOSImages(importDir.trim(), importRecursive)
      success(t('osImages.importResult', { imported: res.data.imported, skipped: res.data.skipped }))
      setImportOpen(false)
      setImportDir('')
      await loadImages()
    } catch (err: any) { showError(err.message) }
    finally { setImporting(false) }
  }

  const openEdit = (img: OSImage) => {
    setEditForm({ name: img.name, distro: img.distro || '', version: img.version || '', arch: img.arch || '' })
    setEditTarget(img)
  }

  const handleEditSave = async () => {
    if (!editTarget?.id) return
    try {
      await api.updateOSImage(editTarget.id, editForm)
      success(t('osImages.saved'))
      setEditTarget(null)
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const filtered = images.filter(img =>
    !search || img.name.toLowerCase().includes(search.toLowerCase()) ||
    img.distro?.toLowerCase().includes(search.toLowerCase()) ||
    img.version?.toLowerCase().includes(search.toLowerCase())
  )

  const stats = computeStats(images)
  const pieData = [
    { name: 'Linux', value: stats.linux, color: '#3b82f6' },
    { name: 'Windows', value: stats.windows, color: 'var(--color-accent-cyan)' },
    { name: t('osImages.statsOther'), value: stats.other, color: 'var(--foreground-muted)' },
  ].filter(d => d.value > 0)

  const actionBtn = 'flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors'

  const columns: Column<OSImage>[] = [
    {
      key: 'name', label: t('osImages.colName'),
      render: (img) => (
        <div className="flex items-center gap-2 min-w-0">
          <Disc size={15} className="shrink-0 text-[var(--text-muted)]" />
          <span className="text-sm font-medium text-[var(--text-primary)] truncate">{img.name}</span>
        </div>
      ),
    },
    {
      key: 'distro', label: t('osImages.distro'),
      render: (img) => (
        <span className="text-xs text-[var(--text-primary)]">{img.distro || '—'}{img.version ? ` ${img.version}` : ''}</span>
      ),
    },
    {
      key: 'arch', label: t('osImages.arch'),
      render: (img) => img.arch ? <Tag color="blue">{img.arch}</Tag> : <span className="text-[var(--text-muted)]">—</span>,
    },
    {
      key: 'path', label: t('osImages.colPath'),
      render: (img) => <span className="font-mono text-[10px] text-[var(--text-muted)] block max-w-[220px] truncate" title={img.file_path || img.filename}>{img.file_path || img.filename}</span>,
    },
    { key: 'size', label: t('osImages.size'), render: (img) => <span className="font-mono text-xs">{formatSize(img.size || 0)}</span> },
    {
      key: 'status', label: t('osImages.colStatus'),
      render: (img) => (
        <div className="flex items-center gap-1.5">
          {statusIcon(img.status || '')}
          <span className={`text-[10px] font-medium ${statusColorClass(img.status || '')}`}>{img.status}</span>
          {img.error_message && <span className="text-[10px] text-accent-red truncate max-w-[160px]" title={img.error_message}>{img.error_message}</span>}
        </div>
      ),
    },
    {
      key: 'state', label: t('osImages.colState'),
      render: (img) => (
        <div className="flex items-center gap-1.5">
          {img.extracted_to ? <Tag color="green">{t('osImages.extractedTag')}</Tag> : <Tag>{t('osImages.notExtractedTag')}</Tag>}
          {img.mount_point ? <Tag color="blue">{t('osImages.mountedTag')}</Tag> : <Tag>{t('osImages.notMountedTag')}</Tag>}
        </div>
      ),
    },
    {
      key: 'installSource', label: t('osImages.installSource'),
      render: (img) => img.extracted_to
        ? <span className="text-accent-green font-mono text-[10px]">{`${window.location.origin}/boot/isos/${img.extracted_to.split(/[\\/]/).pop()}/`}</span>
        : <span className="text-[var(--text-muted)]">—</span>,
    },
    { key: 'created_at', label: t('osImages.colCreated'), render: (img) => <span className="text-[10px] text-[var(--text-muted)]">{formatTime(img.created_at || '')}</span> },
    {
      key: 'actions', label: t('osImages.colActions'), className: 'text-right',
      render: (img) => (
        <div className="flex items-center justify-end gap-1.5">
          {img.status === 'ready' && (
            <>
              {!img.distro && (
                <button onClick={() => handleReprocess(img.id!)} className={`${actionBtn} hover:bg-blue-500/15 hover:text-blue-400`}>
                  <RefreshCw size={11} /> {t('osImages.reprocess')}
                </button>
              )}
              {!img.extracted_to && (
                <button onClick={() => handleExtract(img.id!)} className={`${actionBtn} hover:bg-accent-orange/15 hover:text-accent-orange`}>
                  <FileArchive size={11} /> {t('osImages.extract')}
                </button>
              )}
              {!img.mount_point ? (
                <button onClick={() => handleMount(img.id!)} className={`${actionBtn} hover:bg-blue-500/15 hover:text-blue-400`}>
                  <HardDrive size={11} /> {t('osImages.mount')}
                </button>
              ) : (
                <button onClick={() => setUnmountTarget(img)} className={`${actionBtn} hover:bg-accent-yellow/15 hover:text-accent-yellow`}>
                  <HardDrive size={11} /> {t('osImages.unmount')}
                </button>
              )}
            </>
          )}
          <button onClick={() => openEdit(img)} className={`${actionBtn} hover:bg-blue-500/15 hover:text-blue-400`}>
            <Pencil size={11} /> {t('osImages.edit')}
          </button>
          <button onClick={() => setDeleteTarget(img)} className={`${actionBtn} hover:bg-accent-red/15 hover:text-accent-red`}>
            <Trash2 size={11} /> {t('osImages.delete')}
          </button>
        </div>
      ),
    },
  ]

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('osImages.title')}
        className="mb-0"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => loadImages()} disabled={loading}>
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            </Button>
            <Button variant="secondary" size="sm" onClick={() => setImportOpen(true)}>
              <FolderInput size={14} /> {t('osImages.importDir')}
            </Button>
            <Button variant="secondary" size="sm" onClick={() => fileRef.current?.click()}>
              <Upload size={14} /> {t('osImages.uploadIso')}
            </Button>
          </>
        }
      />

      <input
        ref={fileRef}
        type="file"
        accept=".iso"
        multiple
        className="hidden"
        onChange={e => handleUpload(e.target.files)}
      />

      {/* 顶部：左侧统计 + 右侧上传 */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
        {/* 统计面板 */}
        <Card className="lg:col-span-2">
          <div className="flex flex-wrap items-center gap-x-8 gap-y-4">
            {/* 类型分布：环形图 + 图例 */}
            <div className="flex items-center gap-4 shrink-0">
              <PieChart width={104} height={104}>
                <Pie
                  data={pieData}
                  dataKey="value"
                  innerRadius={30}
                  outerRadius={48}
                  paddingAngle={2}
                  strokeWidth={0}
                  isAnimationActive={false}
                >
                  {pieData.map(d => <Cell key={d.name} fill={d.color} />)}
                </Pie>
              </PieChart>
              <div className="space-y-1.5 text-[11px]">
                <p className="font-semibold text-[var(--text-secondary)]">{t('osImages.statsByType')}</p>
                <div className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-blue-500 shrink-0" />Linux <span className="text-[var(--text-muted)]">{stats.linux}</span></div>
                <div className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-accent-cyan shrink-0" />Windows <span className="text-[var(--text-muted)]">{stats.windows}</span></div>
                <div className="flex items-center gap-1.5"><span className="w-2 h-2 rounded-full bg-[var(--text-muted)] shrink-0" />{t('osImages.statsOther')} <span className="text-[var(--text-muted)]">{stats.other}</span></div>
              </div>
            </div>
            {/* 分隔线 */}
            <div className="hidden sm:block w-px self-stretch bg-[var(--bg-border)]" />
            {/* 统计数字：每个指标全面板只出现一次 */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-x-8 gap-y-4 flex-1">
              <div>
                <p className="text-2xl font-bold text-[var(--text-primary)] leading-tight">{stats.total}</p>
                <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('osImages.statsTotal')}</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-[var(--text-primary)] leading-tight">{formatSize(stats.totalSize)}</p>
                <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('osImages.statsSize')}</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-[var(--text-primary)] leading-tight">
                  {stats.extracted}<span className="text-sm font-normal text-[var(--text-muted)]"> / {stats.total}</span>
                </p>
                <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('osImages.statsExtracted')}</p>
              </div>
              <div>
                <p className="text-2xl font-bold text-[var(--text-primary)] leading-tight">
                  {stats.mounted}<span className="text-sm font-normal text-[var(--text-muted)]"> / {stats.total}</span>
                </p>
                <p className="text-[11px] text-[var(--text-muted)] mt-0.5">{t('osImages.statsMounted')}</p>
              </div>
            </div>
          </div>
        </Card>

        {/* 紧凑上传区 */}
        <div
          onDragOver={e => { e.preventDefault(); setDragOver(true) }}
          onDragLeave={() => setDragOver(false)}
          onDrop={e => { e.preventDefault(); setDragOver(false); handleUpload(e.dataTransfer.files) }}
          className={`border-2 border-dashed rounded-xl p-4 text-center transition-colors cursor-pointer flex flex-col items-center justify-center
            ${dragOver ? 'border-blue-400 bg-blue-500/5' : 'border-[var(--bg-border)] hover:border-blue-400/50'}`}
          onClick={() => fileRef.current?.click()}
        >
          <Upload size={24} className="mb-1.5 text-[var(--text-muted)]" />
          <p className="text-xs text-[var(--text-secondary)]">{t('osImages.dropHint')}</p>
          <p className="text-[10px] text-[var(--text-muted)] mt-0.5">{t('osImages.dropSub')}</p>
          {uploading && uploadProgress && (
            <div className="mt-3 w-full text-left" onClick={e => e.stopPropagation()}>
              <div className="flex items-center justify-between text-[10px] mb-1">
                <span className="text-[var(--text-secondary)] font-mono truncate mr-2">{uploadProgress.name}</span>
                <span className="text-blue-400 font-medium shrink-0">{uploadProgress.percent}%</span>
              </div>
              <div className="h-1.5 rounded-full bg-[var(--bg-input)] overflow-hidden">
                <div className="h-full rounded-full bg-blue-500 transition-all duration-200" style={{ width: `${uploadProgress.percent}%` }} />
              </div>
            </div>
          )}
        </div>
      </div>

      {/* 列表 */}
      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] z-10" />
        <input
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder={t('osImages.searchPlaceholder')}
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg pl-9 pr-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        />
      </div>

      <Card padding={false}>
        <DataTable
          columns={columns}
          data={filtered}
          loading={loading}
          emptyText={t('osImages.noImages')}
          rowKey={(img) => String(img.id)}
        />
      </Card>

      <Modal open={!!deleteTarget} onClose={() => setDeleteTarget(null)} title={t('osImages.deleteTitle')}>
        <p className="text-sm text-[var(--text-secondary)] mb-4">
          {t('osImages.deleteConfirmPre')} <strong>{deleteTarget?.name}</strong>{t('osImages.deleteConfirmPost')}
        </p>
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => setDeleteTarget(null)}>{t('osImages.cancel')}</Button>
          <Button variant="danger" onClick={handleDelete}>{t('osImages.delete')}</Button>
        </div>
      </Modal>

      <ConfirmDialog
        open={!!unmountTarget}
        onClose={() => setUnmountTarget(null)}
        onConfirm={() => { if (unmountTarget?.id) handleUnmount(unmountTarget.id) }}
        title={t('osImages.unmount')}
        message={t('osImages.confirmUnmount', { name: unmountTarget?.name })}
      />

      {/* 导入目录 */}
      <Modal open={importOpen} onClose={() => setImportOpen(false)} title={t('osImages.importDirTitle')}>
        <div className="space-y-4">
          <p className="text-xs text-[var(--text-muted)]">{t('osImages.importDirHint')}</p>
          <Input
            value={importDir}
            onChange={e => setImportDir(e.target.value)}
            placeholder={t('osImages.importDirPlaceholder')}
            className="font-mono"
          />
          <label className="flex items-center gap-2.5 text-sm text-[var(--text-secondary)] cursor-pointer">
            <input type="checkbox" checked={importRecursive} onChange={e => setImportRecursive(e.target.checked)} className="rounded border-[var(--bg-border)] w-4 h-4" />
            {t('osImages.importRecursive')}
          </label>
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => setImportOpen(false)}>{t('osImages.cancel')}</Button>
            <Button onClick={handleImport} disabled={importing || !importDir.trim()}>
              {importing ? t('osImages.importing') : t('osImages.importDir')}
            </Button>
          </div>
        </div>
      </Modal>

      {/* 编辑镜像信息 */}
      <Modal open={!!editTarget} onClose={() => setEditTarget(null)} title={t('osImages.editTitle')}>
        <div className="space-y-3">
          {(['name', 'distro', 'version', 'arch'] as const).map(field => (
            <div key={field}>
              <label className="block text-xs text-[var(--text-muted)] mb-1">{t(`osImages.field_${field}`)}</label>
              <Input
                value={editForm[field]}
                onChange={e => setEditForm({ ...editForm, [field]: e.target.value })}
              />
            </div>
          ))}
          <div className="flex justify-end gap-2 pt-2">
            <Button variant="secondary" onClick={() => setEditTarget(null)}>{t('osImages.cancel')}</Button>
            <Button onClick={handleEditSave}>{t('osImages.save')}</Button>
          </div>
        </div>
      </Modal>
    </div>
  )
}
