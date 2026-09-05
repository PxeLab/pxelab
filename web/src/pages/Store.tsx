import { useState, useEffect, useRef } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Package, Download, Search, RefreshCw, AlertCircle, RotateCw, Upload } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { EmptyState } from '../components/ui/EmptyState'
import { Input } from '../components/ui/FormControls'
import { useToast } from '../components/ui/Toast'
import { getStoreCatalog, importStoreItem, importLocalStoreItem, type StoreItem, type StoreCatalog } from '../api/store'

type TabKey = 'all' | 'baseline' | 'boot_template' | 'netboot_distro'

export default function Store() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { success, error: showError } = useToast()

  const [catalog, setCatalog] = useState<StoreCatalog | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [search, setSearch] = useState('')
  const [activeTab, setActiveTab] = useState<TabKey>('all')

  // Import confirm
  const [importTarget, setImportTarget] = useState<StoreItem | null>(null)
  const [importing, setImporting] = useState(false)
  const [importingFile, setImportingFile] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const loadCatalog = async () => {
    setLoading(true)
    setLoadError('')
    try {
      const res = await getStoreCatalog()
      setCatalog(res.data.catalog)
    } catch (err: any) {
      setLoadError(err?.message || t('store.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { loadCatalog() }, [])

  const items = catalog?.items || []
  const filtered = items.filter(item => {
    if (activeTab !== 'all' && item.type !== activeTab) return false
    if (search) {
      const q = search.toLowerCase()
      return item.name.toLowerCase().includes(q) ||
        item.description.toLowerCase().includes(q) ||
        item.tags?.some(t => t.toLowerCase().includes(q))
    }
    return true
  })

  const tabs: { key: TabKey; label: string; count: number }[] = [
    { key: 'all', label: t('store.type_all'), count: items.length },
    { key: 'baseline', label: t('store.type_baseline'), count: items.filter(i => i.type === 'baseline').length },
    { key: 'boot_template', label: t('store.type_boot_template'), count: items.filter(i => i.type === 'boot_template').length },
    { key: 'netboot_distro', label: t('store.type_netboot_distro'), count: items.filter(i => i.type === 'netboot_distro').length },
  ]

  async function handleImport(item: StoreItem) {
    setImporting(true)
    try {
      const res = await importStoreItem(item.id, item.type)
      if (res.data.type === 'netboot_distro') {
        success(t('store.importedToCatalog', { name: res.data.name }))
        setImportTarget(null)
        navigate('/netboot-catalog')
        return
      }
      success(t('store.importSuccess', { name: res.data.name }))
      setImportTarget(null)
      if (res.data.type === 'baseline') {
        navigate(`/baselines/${res.data.id}`)
      }
    } catch (err: any) {
      showError(t('store.importFailed', { error: err?.message || '' }))
    } finally {
      setImporting(false)
    }
  }

  async function handleFileImport(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0]
    if (!file) return
    setImportingFile(true)
    try {
      const text = await file.text()
      const data = JSON.parse(text)
      const res = await importLocalStoreItem(data)
      success(t('store.detail.importLocalSuccess', { name: res.data.name }))
      loadCatalog()
    } catch (err: any) {
      showError(t('store.detail.importLocalFailed', { error: err?.message || '' }))
    } finally {
      setImportingFile(false)
      if (fileInputRef.current) fileInputRef.current.value = ''
    }
  }

  // ── Render ──

  if (loading) {
    return (
      <div className="p-6 space-y-6">
        <PageHeader title={t('store.title')} description={t('store.description')} />
        <div className="flex items-center justify-center py-20">
          <div className="flex flex-col items-center gap-3 text-[var(--text-muted)]">
            <RotateCw size={24} className="animate-spin" />
            <span className="text-sm">{t('common.loading')}</span>
          </div>
        </div>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="p-6 space-y-6">
        <PageHeader title={t('store.title')} description={t('store.description')} />
        <Card className="p-12">
          <div className="flex flex-col items-center gap-4 text-center">
            <div className="w-12 h-12 rounded-full bg-accent-red/10 flex items-center justify-center">
              <AlertCircle size={24} className="text-accent-red" />
            </div>
            <div>
              <h3 className="text-base font-semibold text-[var(--text-primary)]">{t('store.storeUnavailable')}</h3>
              <p className="text-sm text-[var(--text-muted)] mt-1">{t('store.storeUnavailableDesc')}</p>
            </div>
            <Button variant="primary" onClick={loadCatalog}>
              <RefreshCw size={14} />
              {t('store.retry')}
            </Button>
          </div>
        </Card>
      </div>
    )
  }

  return (
    <div className="p-6 space-y-6">
      <PageHeader
        title={t('store.title')}
        description={t('store.description')}
        actions={
          <>
            <Button variant="secondary" size="sm" onClick={() => fileInputRef.current?.click()} disabled={importingFile}>
              <Upload size={14} className={importingFile ? 'animate-spin' : ''} />
              {importingFile ? t('common.loading') : t('store.importFromFile')}
            </Button>
            <Button variant="secondary" onClick={loadCatalog} disabled={loading}>
              <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
              {t('store.retry')}
            </Button>
          </>
        }
      />

      {/* Search + Tabs */}
      <div className="flex flex-col gap-4">
        <div className="relative max-w-md">
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-[var(--text-muted)]" />
          <Input
            size="md"
            type="text"
            value={search}
            onChange={e => setSearch(e.target.value)}
            placeholder={t('store.searchPlaceholder')}
            className="pl-9"
          />
        </div>

        <div className="flex items-center gap-1 border-b border-[var(--bg-border)]">
          {tabs.map(tab => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors -mb-px ${
                activeTab === tab.key
                  ? 'border-blue-500 text-blue-400'
                  : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-secondary)]'
              }`}
            >
              {tab.label}
              {tab.count > 0 && (
                <span className="ml-1.5 text-[10px] opacity-60">({tab.count})</span>
              )}
            </button>
          ))}
        </div>
      </div>

      {/* Grid */}
      {filtered.length === 0 ? (
        <EmptyState
          icon="📦"
          title={t('store.noItems')}
        />
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {filtered.map(item => (
            <StoreCard
              key={`${item.type}-${item.id}`}
              item={item}
              onImport={() => setImportTarget(item)}
            />
          ))}
        </div>
      )}

      {/* Hidden file input */}
      <input
        ref={fileInputRef}
        type="file"
        accept=".json"
        style={{ display: 'none' }}
        onChange={handleFileImport}
      />

      {/* Import Confirm */}
      <ConfirmDialog
        open={!!importTarget}
        onClose={() => !importing && setImportTarget(null)}
        onConfirm={() => importTarget && handleImport(importTarget)}
        title={t('store.confirmImportTitle', { name: importTarget?.name || '' })}
        message={t('store.confirmImportMsg', { name: importTarget?.name || '' })}
        confirmLabel={importing ? t('store.importing') : t('store.import')}
        loading={importing}
        danger={false}
      />
    </div>
  )
}

// ── Sub-components ──

function ShieldCheckIcon() {
  return (
    <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-emerald-500 to-teal-600 flex items-center justify-center shadow-lg shadow-emerald-500/20 shrink-0">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
      </svg>
    </div>
  )
}

function TerminalIcon() {
  return (
    <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-violet-500 to-purple-600 flex items-center justify-center shadow-lg shadow-violet-500/20 shrink-0">
      <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="white" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
        <polyline points="4 17 10 11 4 5" />
        <line x1="12" y1="19" x2="20" y2="19" />
      </svg>
    </div>
  )
}

function StoreCard({ item, onImport }: { item: StoreItem; onImport: () => void }) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  return (
    <Card className="p-5 flex flex-col gap-4 hover:shadow-md transition-shadow duration-200 group">
      {/* Clickable body */}
      <div
        className="cursor-pointer flex flex-col gap-4"
        onClick={() => navigate(`/store/item/${item.type}/${item.id}`)}
      >
        {/* Header */}
        <div className="flex items-start gap-3">
          {item.type === 'baseline' ? <ShieldCheckIcon /> : item.type === 'boot_template' ? <TerminalIcon /> : item.type === 'netboot_distro' ? <TerminalIcon /> : (
            <div className="w-10 h-10 rounded-xl bg-gradient-to-br from-blue-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-blue-500/20 shrink-0">
              <Package size={18} className="text-white" />
            </div>
          )}
          <div className="min-w-0 flex-1">
            <h3 className="text-sm font-semibold text-[var(--text-primary)] truncate">
              {item.name}
            </h3>
            {item.version && (
              <span className="text-[10px] text-[var(--text-muted)] font-mono">
                v{item.version}
              </span>
            )}
          </div>
        </div>

        {/* Description */}
        <p className="text-xs text-[var(--text-secondary)] leading-relaxed line-clamp-2">
          {item.description}
        </p>

        {/* Tags */}
        {item.tags && item.tags.length > 0 && (
          <div className="flex flex-wrap gap-1">
            {item.tags.slice(0, 4).map(tag => (
              <Tag key={tag}>{tag}</Tag>
            ))}
            {item.tags.length > 4 && (
              <span className="text-[10px] text-[var(--text-muted)]">+{item.tags.length - 4}</span>
            )}
          </div>
        )}

        {/* Meta */}
        <div className="flex items-center justify-between text-[10px] text-[var(--text-muted)]">
          {item.author && (
            <span>{t('store.author')}: {item.author}</span>
          )}
          {(item.downloads ?? 0) > 0 && (
            <span className="flex items-center gap-1">
              <Download size={10} />
              {item.downloads}
            </span>
          )}
        </div>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-2 pt-1">
        <Button variant="primary" size="sm" onClick={onImport} className="flex-1">
          <Download size={13} />
          {t('store.import')}
        </Button>
      </div>
    </Card>
  )
}
