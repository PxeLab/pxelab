import { useState, useEffect } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Download } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { getStoreItem, importStoreItem, type StoreItemDetail } from '../api/store'

export default function StoreDetail() {
  const { type, id } = useParams<{ type: string; id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [item, setItem] = useState<StoreItemDetail | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState('')
  const [importing, setImporting] = useState(false)

  useEffect(() => {
    if (!type || !id) return
    let cancelled = false
    setLoading(true)
    setLoadError('')
    getStoreItem(type, id)
      .then(res => { if (!cancelled) setItem(res.data) })
      .catch(err => { if (!cancelled) setLoadError(err.message || t('common.loadFailed')) })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [type, id, t])

  function formatTime(s?: string) {
    if (!s) return '-'
    try { return new Date(s).toLocaleString() } catch { return s }
  }

  function handleDownload() {
    if (!item) return
    const blob = new Blob([JSON.stringify(item, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = `${item.type}-${item.id}.json`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  }

  async function handleImport() {
    if (!item || importing) return
    setImporting(true)
    try {
      const res = await importStoreItem(item.id, item.type)
      success(t('store.importSuccess', { name: item.name }))
      if (item.type === 'baseline') {
        navigate(`/baselines/${res.data.id}`)
      }
    } catch (err: any) {
      showError(err.message || t('store.importFailed', { error: '' }))
    } finally {
      setImporting(false)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="flex flex-col items-center gap-3 text-[var(--text-muted)]">
          <div className="w-8 h-8 border-2 border-blue-500/30 border-t-blue-500 rounded-full animate-spin" />
          <span className="text-sm">{t('common.loading')}</span>
        </div>
      </div>
    )
  }

  if (loadError) {
    return (
      <div className="space-y-5">
        <button onClick={() => navigate('/store')} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
          <ArrowLeft size={14} /> {t('store.detail.backToStore')}
        </button>
        <Card><div className="py-8 text-center text-sm text-accent-red">{loadError}</div></Card>
      </div>
    )
  }

  if (!item) {
    return (
      <div className="space-y-5">
        <button onClick={() => navigate('/store')} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
          <ArrowLeft size={14} /> {t('store.detail.backToStore')}
        </button>
        <Card><div className="py-8 text-center text-sm text-[var(--text-muted)]">{t('store.detail.notFound')}</div></Card>
      </div>
    )
  }

  return (
    <div className="space-y-5">
      <button onClick={() => navigate('/store')} className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
        <ArrowLeft size={14} /> {t('store.detail.backToStore')}
      </button>

      <PageHeader
        title={item.name}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="secondary" size="sm" onClick={handleDownload}>
              <Download size={14} />
              {t('store.detail.download')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleImport} disabled={importing}>
              {importing ? t('store.importing') : t('store.detail.import')}
            </Button>
          </div>
        }
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-5">
        <Card>
          <div className="space-y-4">
            <div>
              <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.detail.content')}</span>
              <span className="text-sm text-[var(--text-primary)]">{item.description || '-'}</span>
            </div>
            <div>
              <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.version')}</span>
              <span className="text-sm text-[var(--text-primary)]">{item.version}</span>
            </div>
            <div>
              <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.author')}</span>
              <span className="text-sm text-[var(--text-primary)]">{item.author}</span>
            </div>
            {item.tags && item.tags.length > 0 && (
              <div>
                <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.tags')}</span>
                <div className="flex flex-wrap gap-1.5">
                  {item.tags.map(tag => (
                    <Tag key={tag}>{tag}</Tag>
                  ))}
                </div>
              </div>
            )}
            {item.created_at && (
              <div>
                <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.created')}</span>
                <span className="text-xs text-[var(--text-muted)]">{formatTime(item.created_at)}</span>
              </div>
            )}
          </div>
        </Card>

        <Card>
          <div className="space-y-4">
            <div>
              <span className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('store.updated')}</span>
              <span className="text-xs text-[var(--text-muted)]">{formatTime(item.updated_at)}</span>
            </div>
          </div>
        </Card>
      </div>

      <Card>
        <div className="space-y-3">
          <span className="block text-xs font-medium text-[var(--text-muted)]">{t('store.detail.content')}</span>
          <pre className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg p-4 text-xs font-mono text-[var(--text-primary)] overflow-auto max-h-96 whitespace-pre">
            {JSON.stringify(item.content, null, 2)}
          </pre>
        </div>
      </Card>
    </div>
  )
}
