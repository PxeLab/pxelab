import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, RefreshCw, Send, Pencil, Trash2, Webhook as WebhookIcon } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { Tag } from '../components/ui/Tag'
import { DataTable } from '../components/ui/DataTable'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import { Toggle } from '../components/ui/Toggle'
import { ChecklistPicker } from '../components/ui/ChecklistPicker'
import { api, type Webhook, type WebhookFormat } from '../api/client'

// 首版支持的四类事件（与后端 notify 包一致）
const EVENT_TYPES = [
  'install.finished',
  'install.failed',
  'baseline.script_failed',
  'host.first_pxe_boot',
] as const

const FORMATS: WebhookFormat[] = ['generic', 'dingtalk', 'feishu']

interface FormState {
  name: string
  url: string
  events: string[]
  format: WebhookFormat
  secret: string
  enabled: boolean
}

const EMPTY_FORM: FormState = { name: '', url: '', events: [], format: 'generic', secret: '', enabled: true }

export default function SettingsWebhooks() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [loading, setLoading] = useState(true)
  const [webhooks, setWebhooks] = useState<Webhook[]>([])

  const [showEdit, setShowEdit] = useState(false)
  const [editing, setEditing] = useState<Webhook | null>(null)
  const [form, setForm] = useState<FormState>(EMPTY_FORM)
  const [saving, setSaving] = useState(false)
  const [formError, setFormError] = useState('')
  const [confirmDelete, setConfirmDelete] = useState<Webhook | null>(null)
  const [testingId, setTestingId] = useState<string | null>(null)

  useEffect(() => { load() }, [])

  async function load() {
    setLoading(true)
    try {
      const res = await api.getWebhooks()
      setWebhooks(res.data ?? [])
    } catch (err: any) {
      error(err.message || t('settings.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  function openCreate() {
    setEditing(null)
    setForm(EMPTY_FORM)
    setFormError('')
    setShowEdit(true)
  }

  function openEdit(wh: Webhook) {
    setEditing(wh)
    setForm({
      name: wh.name,
      url: wh.url,
      events: wh.events ?? [],
      format: wh.format || 'generic',
      secret: wh.secret ?? '',
      enabled: wh.enabled,
    })
    setFormError('')
    setShowEdit(true)
  }

  async function handleSave() {
    if (!form.name.trim() || !form.url.trim()) {
      setFormError(t('webhooks.nameUrlRequired'))
      return
    }
    if (form.events.length === 0) {
      setFormError(t('webhooks.eventsRequired'))
      return
    }
    setSaving(true)
    setFormError('')
    try {
      const payload = {
        name: form.name.trim(),
        url: form.url.trim(),
        events: form.events,
        format: form.format,
        secret: form.secret,
        enabled: form.enabled,
      }
      if (editing) {
        await api.updateWebhook(editing.id, payload)
      } else {
        await api.createWebhook(payload)
      }
      setShowEdit(false)
      success(t('settings.saved'))
      load()
    } catch (err: any) {
      setFormError(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  async function handleTest(wh: Webhook) {
    setTestingId(wh.id)
    try {
      await api.testWebhook(wh.id)
      success(t('webhooks.testSuccess', { name: wh.name }))
    } catch (err: any) {
      error(t('webhooks.testFailed', { name: wh.name, error: err.message }))
    } finally {
      setTestingId(null)
    }
  }

  function toggleEvent(id: string) {
    setForm(f => ({
      ...f,
      events: f.events.includes(id) ? f.events.filter(e => e !== id) : [...f.events, id],
    }))
  }

  const formatColor = (f: string) => (f === 'dingtalk' ? 'blue' : f === 'feishu' ? 'cyan' : 'purple') as 'blue' | 'cyan' | 'purple'

  return (
    <div>
      <PageHeader
        title={t('webhooks.title')}
        className="mb-4"
        actions={
          <>
            <Button variant="secondary" size="sm" disabled={loading} onClick={load}>
              <RefreshCw size={14} /> {t('common.reload', '重载')}
            </Button>
            <Button variant="primary" size="sm" onClick={openCreate}>
              <Plus size={14} /> {t('webhooks.add')}
            </Button>
          </>
        }
      />

      <Card title={
        <div className="flex items-center gap-2">
          <WebhookIcon size={16} />
          <span>{t('webhooks.listTitle')}</span>
        </div>
      }>
        <p className="text-xs text-[var(--text-muted)] mb-4">{t('webhooks.description')}</p>
        <div className="-mx-5">
          <DataTable
            loading={loading}
            data={webhooks}
            rowKey={(wh: Webhook) => wh.id}
            emptyText={t('webhooks.empty')}
            columns={[
              {
                key: 'name',
                label: t('webhooks.colName'),
                render: (wh: Webhook) => <span className="text-sm font-medium text-[var(--text-primary)]">{wh.name}</span>,
              },
              {
                key: 'url',
                label: t('webhooks.colUrl'),
                render: (wh: Webhook) => (
                  <span className="block max-w-[260px] truncate text-xs font-mono text-[var(--text-muted)]" title={wh.url}>{wh.url}</span>
                ),
              },
              {
                key: 'format',
                label: t('webhooks.colFormat'),
                render: (wh: Webhook) => <Tag color={formatColor(wh.format)}>{wh.format}</Tag>,
              },
              {
                key: 'events',
                label: t('webhooks.colEvents'),
                render: (wh: Webhook) => (
                  <div className="flex flex-wrap gap-1">
                    {(wh.events ?? []).map(e => (
                      <Tag key={e} color="blue">{t(`webhooks.eventNames.${e}`, e)}</Tag>
                    ))}
                  </div>
                ),
              },
              {
                key: 'enabled',
                label: t('webhooks.colEnabled'),
                render: (wh: Webhook) => (
                  <Tag color={wh.enabled ? 'green' : 'red'}>
                    {wh.enabled ? t('webhooks.enabled') : t('webhooks.disabled')}
                  </Tag>
                ),
              },
              {
                key: 'actions',
                label: t('common.actions', '操作'),
                className: 'text-right',
                render: (wh: Webhook) => (
                  <div className="flex items-center justify-end gap-1.5">
                    <Button variant="secondary" size="sm" disabled={testingId === wh.id} onClick={() => handleTest(wh)}>
                      <Send size={12} /> {testingId === wh.id ? t('webhooks.testing') : t('webhooks.test')}
                    </Button>
                    <Button variant="secondary" size="sm" onClick={() => openEdit(wh)}>
                      <Pencil size={12} /> {t('common.edit')}
                    </Button>
                    <Button variant="danger" size="sm" onClick={() => setConfirmDelete(wh)}>
                      <Trash2 size={12} /> {t('common.delete')}
                    </Button>
                  </div>
                ),
              },
            ]}
          />
        </div>
      </Card>

      {/* Create / Edit Modal */}
      <Modal
        open={showEdit}
        onClose={() => setShowEdit(false)}
        title={editing ? t('webhooks.editTitle') : t('webhooks.createTitle')}
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowEdit(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" size="sm" onClick={handleSave} disabled={saving}>
              {saving ? t('settings.saving') : t('common.save')}
            </Button>
          </>
        }
      >
        {formError && (
          <div className="mb-4 px-3 py-2 rounded-lg bg-accent-red/10 border border-accent-red/20 text-xs text-accent-red">{formError}</div>
        )}
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('webhooks.fieldName')}</label>
            <Input placeholder="ops-dingtalk" value={form.name} onChange={e => setForm({ ...form, name: e.target.value })} />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('webhooks.fieldUrl')}</label>
            <Input
              placeholder="https://oapi.dingtalk.com/robot/send?access_token=..."
              value={form.url}
              onChange={e => setForm({ ...form, url: e.target.value })}
            />
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('webhooks.fieldEvents')}</label>
            <ChecklistPicker
              items={EVENT_TYPES.map(e => ({ id: e, name: t(`webhooks.eventNames.${e}`) }))}
              selected={form.events}
              onToggle={toggleEvent}
              emptyText=""
            />
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('webhooks.fieldFormat')}</label>
              <Select size="sm" value={form.format} onChange={e => setForm({ ...form, format: e.target.value as WebhookFormat })}>
                {FORMATS.map(f => (
                  <option key={f} value={f}>{t(`webhooks.formatNames.${f}`)}</option>
                ))}
              </Select>
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('webhooks.fieldSecret')}</label>
              <Input
                placeholder={t('webhooks.secretPlaceholder')}
                value={form.secret}
                onChange={e => setForm({ ...form, secret: e.target.value })}
              />
            </div>
          </div>
          <Toggle
            checked={form.enabled}
            onChange={v => setForm({ ...form, enabled: v })}
            label={t('webhooks.fieldEnabled')}
          />
        </div>
      </Modal>

      <ConfirmDialog
        open={!!confirmDelete}
        onClose={() => setConfirmDelete(null)}
        onConfirm={async () => {
          if (!confirmDelete) return
          try {
            await api.deleteWebhook(confirmDelete.id)
            success(t('webhooks.deleted'))
            load()
          } catch (err: any) {
            error(err.message || t('webhooks.deleteFailed'))
          }
          setConfirmDelete(null)
        }}
        title={t('webhooks.deleteTitle')}
        message={t('webhooks.deleteMessage', { name: confirmDelete?.name })}
      />
    </div>
  )
}
