import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, Trash2, RefreshCw, Disc, FileArchive, HardDrive, Search, AlertCircle, CheckCircle, Clock, FolderInput, Pencil, FolderOpen, Folder, ArrowUp, CloudDownload } from 'lucide-react'
import { PieChart, Pie, Cell } from 'recharts'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Input, Select } from '../components/ui/FormControls'
import { Pagination } from '../components/ui/Pagination'
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
  const [typeFilter, setTypeFilter] = useState<'all' | DistroKind>('all')
  const [page, setPage] = useState(1)
  const [sortField, setSortField] = useState('created_at')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('desc')
  const [deleteTarget, setDeleteTarget] = useState<OSImage | null>(null)
  const [deleteFileToo, setDeleteFileToo] = useState(false)
  const [unmountTarget, setUnmountTarget] = useState<OSImage | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [importDir, setImportDir] = useState('')
  const [importRecursive, setImportRecursive] = useState(false)
  const [importing, setImporting] = useState(false)
  const [browseState, setBrowseState] = useState<{ path: string; parent: string; dirs: string[] } | null>(null)
  const [editTarget, setEditTarget] = useState<OSImage | null>(null)
  const [editForm, setEditForm] = useState({ name: '', distro: '', version: '', arch: '' })
  const [wdsTarget, setWdsTarget] = useState<OSImage | null>(null)
  const [wdsName, setWdsName] = useState('')

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
      await api.deleteOSImage(deleteTarget.id, deleteTarget.source_path ? deleteFileToo : false)
      success(t('osImages.deleted'))
      setDeleteTarget(null)
      setDeleteFileToo(false)
      // 删除的是当前页最后一条时回退一页，避免困在空页
      if (paged.length === 1 && page > 1) setPage(page - 1)
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

  const openCreateWds = (img: OSImage) => {
    setWdsName(`${img.name?.replace(/[^A-Za-z0-9\-_ ]/g, '') || img.filename} PE`.trim())
    setWdsTarget(img)
  }

  const handleCreateWds = async () => {
    if (!wdsTarget?.id) return
    const name = wdsName.trim()
    if (!name) { showError(t('osImages.wdsNameRequired')); return }
    try {
      await api.createWdsProfileFromOSImage(wdsTarget.id, name)
      success(t('osImages.wdsCreated', { name }))
      setWdsTarget(null)
    } catch (err: any) { showError(err.message) }
  }

  const handleSetCatalogLocal = async (id: number) => {
    try {
      await api.setCatalogLocal(id)
      success(t('osImages.setCatalogLocalDone'))
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
      setBrowseState(null)
      await loadImages()
    } catch (err: any) { showError(err.message) }
    finally { setImporting(false) }
  }

  const browseTo = async (path: string) => {
    try {
      const res = await api.browseFs(path)
      setBrowseState(res.data)
    } catch (err: any) { showError(err.message) }
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
    (!search || img.name.toLowerCase().includes(search.toLowerCase()) ||
      img.distro?.toLowerCase().includes(search.toLowerCase()) ||
      img.version?.toLowerCase().includes(search.toLowerCase())) &&
    (typeFilter === 'all' || distroKind(img.distro) === typeFilter)
  )

  const sorted = [...filtered].sort((a, b) => {
    let cmp = 0
    switch (sortField) {
      case 'name': cmp = a.name.localeCompare(b.name); break
      case 'distro': cmp = (a.distro || '').localeCompare(b.distro || ''); break
      case 'size': cmp = (a.size || 0) - (b.size || 0); break
      default: cmp = (a.created_at || '').localeCompare(b.created_at || '')
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

  const PAGE_SIZE = 10
  const paged = sorted.slice((page - 1) * PAGE_SIZE, page * PAGE_SIZE)

  const handleSort = (field: string) => {
    if (sortField === field) setSortDir(d => d === 'asc' ? 'desc' : 'asc')
    else { setSortField(field); setSortDir('asc') }
  }

  const stats = computeStats(images)
  const pieData = [
    { name: 'Linux', value: stats.linux, color: '#3b82f6' },
    { name: 'Windows', value: stats.windows, color: 'var(--color-accent-cyan)' },
    { name: t('osImages.statsOther'), value: stats.other, color: 'var(--foreground-muted)' },
  ].filter(d => d.value > 0)

  const actionBtn = 'flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] transition-colors'

  const columns: Column<OSImage>[] = [
    {
      key: 'name', label: t('osImages.colName'), sortable: true,
      render: (img) => (
        <div className="flex items-center gap-2 min-w-0">
          <Disc size={15} className="shrink-0 text-[var(--text-muted)]" />
          <span className="text-sm font-medium text-[var(--text-primary)] truncate">{img.name}</span>
        </div>
      ),
    },
    {
      key: 'distro', label: t('osImages.distro'), sortable: true,
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
    { key: 'size', label: t('osImages.size'), sortable: true, render: (img) => <span className="font-mono text-xs">{formatSize(img.size || 0)}</span> },
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
    { key: 'created_at', label: t('osImages.colCreated'), sortable: true, render: (img) => <span className="text-[10px] text-[var(--text-muted)]">{formatTime(img.created_at || '')}</span> },
    {
      key: 'actions', label: t('osImages.colActions'), className: 'text-right',
      render: (img) => (
        <div className="flex items-center justify-end gap-1.5">
          {(img.status === 'error' || (img.status === 'ready' && !img.distro)) && (
            <button onClick={() => handleReprocess(img.id!)} className={`${actionBtn} hover:bg-blue-500/15 hover:text-blue-400`}>
              <RefreshCw size={11} /> {t('osImages.reprocess')}
            </button>
          )}
          {img.status === 'ready' && (
            <>
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
          {distroKind(img.distro) === 'windows' && img.extracted_to && (
            <button onClick={() => openCreateWds(img)} className={`${actionBtn} hover:bg-violet-500/15 hover:text-violet-400`}>
              <Disc size={11} /> {t('osImages.createWdsProfile')}
            </button>
          )}
          {distroKind(img.distro) === 'windows' && img.extracted_to && (
            <button onClick={() => handleSetCatalogLocal(img.id!)} className={`${actionBtn} hover:bg-accent-green/15 hover:text-accent-green`} title={t('osImages.setCatalogLocalHint')}>
              <CloudDownload size={11} /> {t('osImages.setCatalogLocal')}
            </button>
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

      {/* 搜索 + 类型过滤 */}
      <div className="flex gap-2">
        <div className="relative flex-1">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)] z-10" />
          <input
            value={search}
            onChange={e => { setSearch(e.target.value); setPage(1) }}
            placeholder={t('osImages.searchPlaceholder')}
            className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg pl-9 pr-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
          />
        </div>
        <Select
          value={typeFilter}
          onChange={e => { setTypeFilter(e.target.value as 'all' | DistroKind); setPage(1) }}
          className="w-36! shrink-0"
        >
          <option value="all">{t('osImages.filterAllTypes')}</option>
          <option value="linux">Linux</option>
          <option value="windows">Windows</option>
          <option value="other">{t('osImages.statsOther')}</option>
        </Select>
      </div>

      <Card padding={false}>
        <DataTable
          columns={columns}
          data={paged}
          loading={loading}
          emptyText={t('osImages.noImages')}
          rowKey={(img) => String(img.id)}
          sortField={sortField}
          sortDir={sortDir}
          onSort={handleSort}
        />
        <div className="px-4 pb-2">
          <Pagination page={page} total={sorted.length} size={PAGE_SIZE} onChange={setPage} />
        </div>
      </Card>

      <Modal open={!!deleteTarget} onClose={() => { setDeleteTarget(null); setDeleteFileToo(false) }} title={t('osImages.deleteTitle')}>
        <p className="text-sm text-[var(--text-secondary)] mb-4">
          {t('osImages.deleteConfirmPre')} <strong>{deleteTarget?.name}</strong>{t('osImages.deleteConfirmPost')}
        </p>
        {deleteTarget?.source_path ? (
          <div className="mb-4 space-y-2">
            <p className="text-xs text-[var(--text-muted)]">{t('osImages.deleteExternalNote')}</p>
            <label className="flex items-center gap-2.5 text-sm text-[var(--text-secondary)] cursor-pointer">
              <input type="checkbox" checked={deleteFileToo} onChange={e => setDeleteFileToo(e.target.checked)} className="rounded border-[var(--bg-border)] w-4 h-4" />
              {t('osImages.deleteFileToo')}
            </label>
            {deleteFileToo && (
              <p className="text-[11px] font-mono text-accent-red break-all">{deleteTarget.source_path}</p>
            )}
          </div>
        ) : (
          <p className="text-xs text-[var(--text-muted)] mb-4">{t('osImages.deleteManagedNote')}</p>
        )}
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={() => { setDeleteTarget(null); setDeleteFileToo(false) }}>{t('osImages.cancel')}</Button>
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

      {/* 从本地 Windows ISO 创建 wds Profile */}
      <Modal open={!!wdsTarget} onClose={() => setWdsTarget(null)} title={t('osImages.createWdsTitle')}>
        <div className="space-y-4">
          {wdsTarget?.extracted_to && (
            <div className="text-xs text-[var(--text-muted)] leading-relaxed">
              {t('osImages.createWdsHint')}
              <div className="mt-1.5 font-mono text-[11px] text-[var(--text-primary)] break-all">
                wimboot: {window.location.origin}/netboot/menu/wimboot<br />
                win base: {window.location.origin}/boot/isos/{wdsTarget.extracted_to.split(/[\\/]/).pop()}/
              </div>
            </div>
          )}
          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('osImages.wdsNameLabel')}</label>
            <Input value={wdsName} onChange={e => setWdsName(e.target.value)} placeholder={t('osImages.wdsNamePlaceholder')} />
            <p className="text-[10px] text-[var(--text-muted)] mt-1">{t('osImages.wdsNameAsciiHint')}</p>
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <Button variant="secondary" size="sm" onClick={() => setWdsTarget(null)}>{t('common.cancel')}</Button>
            <Button size="sm" onClick={handleCreateWds}>{t('osImages.createWdsConfirm')}</Button>
          </div>
        </div>
      </Modal>

      {/* 导入目录 */}
      <Modal open={importOpen} onClose={() => { setImportOpen(false); setBrowseState(null) }} title={t('osImages.importDirTitle')}>
        <div className="space-y-4">
          <p className="text-xs text-[var(--text-muted)]">{t('osImages.importDirHint')}</p>
          <div className="flex gap-2">
            <Input
              value={importDir}
              onChange={e => setImportDir(e.target.value)}
              placeholder={t('osImages.importDirPlaceholder')}
              className="font-mono"
            />
            <Button variant="secondary" onClick={() => browseState ? setBrowseState(null) : browseTo(importDir.trim())}>
              <FolderOpen size={14} /> {t('osImages.browse')}
            </Button>
          </div>
          {browseState && (
            <div className="border border-[var(--border)] rounded-lg overflow-hidden">
              <div className="flex items-center justify-between px-3 py-2 bg-[var(--hover)] border-b border-[var(--border)]">
                <span className="text-[11px] font-mono text-[var(--foreground-secondary)] truncate">{browseState.path || t('osImages.browseRoot')}</span>
                <button
                  onClick={() => { setImportDir(browseState.path); setBrowseState(null) }}
                  disabled={!browseState.path}
                  className="text-[11px] text-blue-400 hover:text-blue-300 font-medium shrink-0 ml-2 disabled:opacity-40"
                >
                  {t('osImages.browseSelect')}
                </button>
              </div>
              <div className="max-h-[200px] overflow-y-auto">
                {browseState.parent !== undefined && browseState.path && (
                  <button onClick={() => browseTo(browseState.parent)} className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-[var(--foreground-secondary)] hover:bg-[var(--hover)]">
                    <ArrowUp size={12} className="text-[var(--foreground-muted)]" /> ..
                  </button>
                )}
                {browseState.dirs.length === 0 && (
                  <p className="px-3 py-3 text-xs text-[var(--foreground-muted)] text-center">{t('osImages.browseEmpty')}</p>
                )}
                {browseState.dirs.map(d => (
                  <button
                    key={d}
                    onClick={() => browseTo(browseState.path ? `${browseState.path.replace(/[\\/]+$/, '')}/${d}` : d)}
                    className="w-full flex items-center gap-2 px-3 py-1.5 text-xs text-[var(--foreground-primary)] hover:bg-[var(--hover)]"
                  >
                    <Folder size={12} className="text-accent-yellow shrink-0" /> {d}
                  </button>
                ))}
              </div>
            </div>
          )}
          <label className="flex items-center gap-2.5 text-sm text-[var(--text-secondary)] cursor-pointer">
            <input type="checkbox" checked={importRecursive} onChange={e => setImportRecursive(e.target.checked)} className="rounded border-[var(--bg-border)] w-4 h-4" />
            {t('osImages.importRecursive')}
          </label>
          <div className="flex justify-end gap-2">
            <Button variant="secondary" onClick={() => { setImportOpen(false); setBrowseState(null) }}>{t('osImages.cancel')}</Button>
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
