import { useState, useEffect } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, Save } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { Input, Textarea } from '../components/ui/FormControls'
import { useToast } from '../components/ui/Toast'
import { api } from '../api/client'

const SCRIPT_TYPES = [
  { value: 'shell', label: 'Shell' },
  { value: 'bat', label: 'BAT' },
  { value: 'powershell', label: 'PowerShell' },
]

export default function ScriptDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const isNew = id === 'new'

  const [loading, setLoading] = useState(!isNew)
  const [saving, setSaving] = useState(false)
  const [fetchError, setFetchError] = useState('')

  const [name, setName] = useState('')
  const [type, setType] = useState('shell')
  const [content, setContent] = useState('')
  const [description, setDescription] = useState('')

  useEffect(() => {
    if (!isNew && id) {
      setLoading(true)
      setFetchError('')
      api.getScript(Number(id)).then(res => {
        const s = res.data
        setName(s.name)
        setType(s.type)
        setContent(s.content)
        setDescription(s.description || '')
      }).catch((err: any) => {
        setFetchError(err.message || t('common.loadFailed'))
      }).finally(() => {
        setLoading(false)
      })
    }
  }, [id])

  async function handleSave() {
    if (!name.trim()) { showError(t('common.fieldRequired')); return }
    if (!content.trim()) { showError(t('common.fieldRequired')); return }
    setSaving(true)
    try {
      if (isNew) {
        const res = await api.createScript({ name: name.trim(), type, content: content.trim(), description: description.trim() || undefined })
        success(t('common.created'))
        navigate(`/scripts/${res.data.id}`, { replace: true })
      } else if (id) {
        await api.updateScript(Number(id), { name: name.trim(), type, content: content.trim(), description: description.trim() || undefined })
        success(t('common.saved'))
      }
    } catch (err: any) {
      showError(err.message || t('common.saveFailed'))
    } finally {
      setSaving(false)
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

  if (fetchError && !isNew) {
    return (
      <div className="flex flex-col gap-4 py-10 items-center">
        <p className="text-sm text-accent-red">{fetchError}</p>
        <Button variant="secondary" size="sm" onClick={() => navigate('/scripts')}>
          {t('common.back')}
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 max-w-3xl">
      <PageHeader
        title={isNew ? t('scripts.create') : name}
        description={isNew ? t('scripts.createDescription') : t('scripts.editDescription')}
        actions={
          <div className="flex items-center gap-2">
            <Button variant="ghost" size="sm" onClick={() => navigate('/scripts')}>
              <ArrowLeft className="w-4 h-4 mr-1" /> {t('common.back')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
              <Save className="w-4 h-4 mr-1" /> {t('common.save')}
            </Button>
          </div>
        }
      />

      <Card>
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.name')} *</label>
              <Input size="sm" value={name} onChange={e => setName(e.target.value)} placeholder="my-script" />
            </div>
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.type')}</label>
              <select
                value={type}
                onChange={e => setType(e.target.value)}
                className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] text-[var(--text-primary)] outline-none transition-all rounded-lg px-3.5 py-2 text-sm focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
              >
                {SCRIPT_TYPES.map(st => (
                  <option key={st.value} value={st.value}>{st.label}</option>
                ))}
              </select>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('scripts.content')} *</label>
            <Textarea
              size="sm"
              value={content}
              onChange={e => setContent(e.target.value)}
              rows={16}
              className="font-mono text-sm leading-relaxed"
              placeholder={type === 'shell' ? '#!/bin/bash\n\necho "Hello World"' : type === 'bat' ? '@echo off\necho Hello World' : '# PowerShell script\nWrite-Output "Hello World"'}
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.description')}</label>
            <Textarea size="sm" value={description} onChange={e => setDescription(e.target.value)} rows={2} />
          </div>

          {/* Preview */}
          {content && (
            <div>
              <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('common.preview')}</label>
              <pre className="bg-[var(--bg-hover)] border border-[var(--bg-border)] rounded-lg p-4 text-xs font-mono leading-relaxed text-[var(--text-primary)] overflow-x-auto max-h-60 overflow-y-auto whitespace-pre">{content}</pre>
            </div>
          )}
        </div>
      </Card>
    </div>
  )
}
