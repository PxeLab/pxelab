import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Upload, Trash2, RefreshCw, Disc, FileArchive, HardDrive, Search, AlertCircle, CheckCircle, Clock } from 'lucide-react'
import { Button } from '../components/ui/Button'
import { Card } from '../components/ui/Card'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
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

function statusColor(status: string) {
  switch (status) {
    case 'ready': return 'green'
    case 'uploading':
    case 'validating': return 'yellow'
    case 'error': return 'red'
    default: return 'gray'
  }
}

function statusIcon(status: string) {
  switch (status) {
    case 'ready': return <CheckCircle size={14} className="text-accent-green" />
    case 'uploading':
    case 'validating': return <Clock size={14} className="text-accent-yellow" />
    case 'error': return <AlertCircle size={14} className="text-accent-red" />
    default: return <Clock size={14} className="text-gray-400" />
  }
}

export default function OSImages() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [images, setImages] = useState<OSImage[]>([])
  const [loading, setLoading] = useState(true)
  const [uploading, setUploading] = useState(false)
  const [search, setSearch] = useState('')
  const [deleteTarget, setDeleteTarget] = useState<OSImage | null>(null)

  const fileRef = useRef<HTMLInputElement>(null)
  const dropRef = useRef<HTMLDivElement>(null)
  const [dragOver, setDragOver] = useState(false)

  const loadImages = async () => {
    setLoading(true)
    try {
      const res = await api.getOSImages()
      setImages(res.data)
    } catch (err: any) { showError(err.message) }
    finally { setLoading(false) }
  }

  useEffect(() => { loadImages() }, [])

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
        await api.uploadOSImage(f)
        success(t('osImages.uploaded', { name: f.name }))
      }
      await loadImages()
    } catch (err: any) { showError(err.message) }
    finally { setUploading(false) }
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

  const [unmountTarget, setUnmountTarget] = useState<OSImage | null>(null)

  const handleUnmount = async (id: number) => {
    try {
      await api.unmountOSImage(id)
      success(t('osImages.unmounted'))
      await loadImages()
    } catch (err: any) { showError(err.message) }
  }

  const filtered = images.filter(img =>
    !search || img.name.toLowerCase().includes(search.toLowerCase()) ||
    img.distro?.toLowerCase().includes(search.toLowerCase()) ||
    img.version?.toLowerCase().includes(search.toLowerCase())
  )

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('osImages.title')}
        className="mb-0"
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={loadImages} disabled={loading}>
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
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

      <div
        ref={dropRef}
        onDragOver={e => { e.preventDefault(); setDragOver(true) }}
        onDragLeave={() => setDragOver(false)}
        onDrop={e => { e.preventDefault(); setDragOver(false); handleUpload(e.dataTransfer.files) }}
        className={`border-2 border-dashed rounded-xl p-8 text-center transition-colors cursor-pointer
          ${dragOver ? 'border-blue-400 bg-blue-500/5' : 'border-[var(--bg-border)] hover:border-blue-400/50'}`}
        onClick={() => fileRef.current?.click()}
      >
        <Upload size={32} className="mx-auto mb-2 text-[var(--text-muted)]" />
        <p className="text-sm text-[var(--text-secondary)]">{t('osImages.dropHint')}</p>
        <p className="text-xs text-[var(--text-muted)] mt-1">{t('osImages.dropSub')}</p>
        {uploading && <p className="text-xs text-blue-400 mt-2">{t('osImages.uploading')}</p>}
      </div>

      <div className="relative">
        <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
        <input
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder={t('osImages.searchPlaceholder')}
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg pl-9 pr-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
        />
      </div>

      {loading ? (
        <div className="flex items-center justify-center py-12">
          <RefreshCw size={24} className="animate-spin text-[var(--text-muted)]" />
        </div>
      ) : filtered.length === 0 ? (
        <Card><div className="py-12 text-center text-sm text-[var(--text-muted)]">{t('osImages.noImages')}</div></Card>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {filtered.map(img => (
            <Card key={img.id} padding={false}>
              <div className="p-4 space-y-3">
                <div className="flex items-start justify-between">
                  <div className="flex items-center gap-2 min-w-0">
                    <Disc size={18} className="shrink-0 text-[var(--text-muted)]" />
                    <div className="min-w-0">
                      <p className="text-sm font-semibold text-[var(--text-primary)] truncate">{img.name}</p>
                      <p className="text-[10px] text-[var(--text-muted)] font-mono truncate">{img.filename}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-1 shrink-0">
                    {statusIcon(img.status || '')}
                    <span className="text-[10px] font-medium" style={{ color: `var(--${statusColor(img.status || '')})` }}>{img.status}</span>
                  </div>
                </div>

                <div className="grid grid-cols-2 gap-2 text-[11px]">
                  {img.distro && (
                    <div>
                      <span className="text-[var(--text-muted)]">{t('osImages.distro')}</span>
                      <p className="text-[var(--text-primary)] font-medium">{img.distro}</p>
                    </div>
                  )}
                  {img.version && (
                    <div>
                      <span className="text-[var(--text-muted)]">{t('osImages.version')}</span>
                      <p className="text-[var(--text-primary)] font-medium">{img.version}</p>
                    </div>
                  )}
                  {img.arch && (
                    <div>
                      <span className="text-[var(--text-muted)]">{t('osImages.arch')}</span>
                      <Tag color="blue">{img.arch}</Tag>
                    </div>
                  )}
                  <div>
                    <span className="text-[var(--text-muted)]">{t('osImages.size')}</span>
                    <p className="text-[var(--text-primary)] font-mono text-[10px]">{formatSize(img.size || 0)}</p>
                  </div>
                  {img.mount_point && (
                    <div className="col-span-2">
                      <span className="text-[var(--text-muted)]">{t('osImages.mountPoint')}</span>
                      <p className="text-[var(--text-primary)] font-mono text-[10px] truncate">{img.mount_point}</p>
                    </div>
                  )}
                  {img.error_message && (
                    <div className="col-span-2">
                      <span className="text-accent-red">{t('osImages.error')}</span>
                      <p className="text-accent-red text-[10px]">{img.error_message}</p>
                    </div>
                  )}
                </div>

                <div className="text-[10px] text-[var(--text-muted)]">{formatTime(img.created_at || '')}</div>

                <div className="flex items-center gap-1.5 pt-1 border-t border-[var(--bg-border)]">
                  {img.status === 'ready' && (
                    <>
                      {!img.extracted_to && (
                        <button onClick={() => handleExtract(img.id!)}
                          className="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:bg-orange-500/15 hover:text-orange-400 transition-colors">
                          <FileArchive size={11} /> {t('osImages.extract')}
                        </button>
                      )}
                      {!img.mount_point ? (
                        <button onClick={() => handleMount(img.id!)}
                          className="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:bg-blue-500/15 hover:text-blue-400 transition-colors">
                          <HardDrive size={11} /> {t('osImages.mount')}
                        </button>
                      ) : (
                        <button onClick={() => setUnmountTarget(img)}
                          className="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:bg-accent-yellow/15 hover:text-accent-yellow transition-colors">
                          <HardDrive size={11} /> {t('osImages.unmount')}
                        </button>
                      )}
                    </>
                  )}
                  <button onClick={() => setDeleteTarget(img)}
                    className="flex items-center gap-1 px-2 py-1 rounded text-[10px] font-medium bg-[var(--bg-hover)] text-[var(--text-secondary)] hover:bg-accent-red/15 hover:text-accent-red transition-colors ml-auto">
                    <Trash2 size={11} /> {t('osImages.delete')}
                  </button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}

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
        onConfirm={() => {
          const id = unmountTarget?.id
          setUnmountTarget(null)
          if (id != null) handleUnmount(id)
        }}
        message={t('osImages.confirmUnmount', { name: unmountTarget?.name })}
      />
    </div>
  )
}
