import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, RotateCcw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { useToast } from '../components/ui/Toast'
import { api, type BaselineHooksData } from '../api/client'

type HookKey = keyof BaselineHooksData

const HOOK_KEYS: HookKey[] = ['kickstart', 'preseed', 'subiquity', 'autounattend']

const EMPTY: BaselineHooksData = { kickstart: '', preseed: '', subiquity: '', autounattend: '' }

export default function SettingsBaselineHooks() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [hooks, setHooks] = useState<BaselineHooksData>(EMPTY)
  const [defaults, setDefaults] = useState<BaselineHooksData>(EMPTY)

  useEffect(() => { load() }, [])

  async function load() {
    setLoading(true)
    try {
      const res = await api.getBaselineHooks()
      setHooks(res.data?.hooks ?? EMPTY)
      setDefaults(res.data?.defaults ?? EMPTY)
    } catch (err: any) {
      error(err.message || t('settings.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  // 与默认一致的内容按“空”提交，后端回落到内置默认
  async function handleSave() {
    setSaving(true)
    try {
      const payload = { ...EMPTY }
      for (const k of HOOK_KEYS) {
        payload[k] = hooks[k].trim() === defaults[k].trim() ? '' : hooks[k]
      }
      await api.updateBaselineHooks(payload)
      success(t('settings.saved'))
      load()
    } catch (err: any) {
      error(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  function effective(key: HookKey): string {
    return hooks[key].trim() === '' ? defaults[key] : hooks[key]
  }

  return (
    <div>
      <PageHeader
        title={t('baselineHooks.title')}
        className="mb-4"
        actions={
          <>
            <Button variant="secondary" size="sm" disabled={loading} onClick={load}>
              <RefreshCw size={14} /> {t('common.reload', '重载配置')}
            </Button>
            <Button variant="secondary" size="sm" disabled={loading} onClick={() => setHooks(EMPTY)}>
              <RotateCcw size={14} /> {t('baselineHooks.resetDefaults')}
            </Button>
            <Button variant="primary" size="sm" disabled={saving || loading} onClick={handleSave}>
              <Save size={14} /> {saving ? t('settings.saving') : t('settings.save')}
            </Button>
          </>
        }
      />

      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : (
          <div className="space-y-5">
            <p className="text-xs text-[var(--text-muted)]">{t('baselineHooks.description')}</p>
            {HOOK_KEYS.map(key => (
              <div key={key}>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">
                  {t(`baselineHooks.${key}`)}
                  {hooks[key].trim() !== '' && (
                    <span className="ml-2 px-1.5 py-px rounded bg-blue-500/10 text-blue-400 text-[10px] font-medium">{t('baselineHooks.customized')}</span>
                  )}
                </label>
                <textarea
                  className="w-full h-28 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-2 focus:ring-blue-500/10"
                  value={effective(key)}
                  onChange={e => setHooks({ ...hooks, [key]: e.target.value })}
                  spellCheck={false}
                />
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  )
}
