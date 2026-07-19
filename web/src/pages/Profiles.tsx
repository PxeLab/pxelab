import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, ChevronDown, ChevronRight, Info, Eye, History as HistoryIcon } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { api, getNetbootCatalog, type Profile, type MenuEntry, type NetbootDistro, type ProfileScriptVersion } from '../api/client'

export default function Profiles() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<Profile | null>(null)
  const [form, setForm] = useState({ name: '', description: '', arch: '', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '', script: '', san_action: '' as MenuEntry['san_action'], san_no_describe: false, san_drive: '', san_keep_san: false } })
  const [showOSPicker, setShowOSPicker] = useState(false)
  const [showVars, setShowVars] = useState(false)
  const [showScriptPreview, setShowScriptPreview] = useState(false)
  const [saving, setSaving] = useState(false)
  const [nameError, setNameError] = useState(false)
  const [previewProfile, setPreviewProfile] = useState<Profile | null>(null)
  const [osCatalog, setOSCatalog] = useState<NetbootDistro[]>([])
  const [confirmDeleteId, setConfirmDeleteId] = useState<string | null>(null)
  const [versions, setVersions] = useState<ProfileScriptVersion[]>([])
  const [showHistory, setShowHistory] = useState(false)
  const [diffContent, setDiffContent] = useState('')
  const [showDiff, setShowDiff] = useState(false)
  const [versionLoading, setVersionLoading] = useState(false)

  useEffect(() => { loadProfiles() }, [])
  useEffect(() => {
    getNetbootCatalog().then(res => setOSCatalog(res.data?.distros || [])).catch(() => {})
  }, [])

  async function loadProfiles() {
    setLoading(true)
    try {
      const res = await api.getProfiles()
      setProfiles(res.data)
    } catch { error(t('profiles.loadFailed')) }
    finally { setLoading(false) }
  }

  function openPreview(p: Profile) {
    setPreviewProfile(p)
  }

  function openCreate() {
    setEditing(null)
    setShowVars(false)
    setShowScriptPreview(false)
    setForm({ name: '', description: '', arch: '', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '', script: '', san_action: '' as MenuEntry['san_action'], san_no_describe: false, san_drive: '', san_keep_san: false } })
    setShowModal(true)
  }

  function openEdit(p: Profile) {
    setEditing(p)
    setShowVars(false)
    setShowScriptPreview(false)
    const e = p.menu?.entries?.[0] || { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' }
    setForm({
      name: p.name,
      description: p.description || '',
      arch: p.arch || '',
      is_default: p.is_default,
      entry: {
        type: e.type,
        kernel: e.kernel || '',
        initrd: e.initrd || '',
        cmdline: e.cmdline || '',
        url: e.url || '',
        wim: e.wim || '',
        script: e.script || '',
        san_action: e.san_action || '' as MenuEntry['san_action'],
        san_no_describe: e.san_no_describe || false,
        san_drive: e.san_drive || '',
        san_keep_san: e.san_keep_san || false,
      },
    })
    setShowModal(true)
  }

  async function handleSave() {
    if (!form.name.trim()) {
      setNameError(true)
      return
    }
    setNameError(false)
    setSaving(true)
    try {
      const label = form.name || t('common.unnamed')
      const entryData: Partial<MenuEntry> = { label, ...form.entry }
      const data = {
        name: form.name,
        description: form.description,
        arch: form.arch,
        is_default: form.is_default,
        menu: { entries: [entryData as MenuEntry] },
      }
      if (editing) {
        await api.updateProfile(editing.id, data)
        success(t('profiles.updated'))
      } else {
        await api.createProfile(data)
        success(t('profiles.created'))
      }
      setShowVars(false)
      setShowScriptPreview(false)
      setShowModal(false)
      loadProfiles()
    } catch (err: any) { error(err.message) }
    finally { setSaving(false) }
  }

  async function loadVersions(profileId: string) {
    setVersionLoading(true)
    try {
      const res = await api.getScriptVersions(profileId)
      setVersions(res.data || [])
    } catch { setVersions([]) }
    finally { setVersionLoading(false) }
  }

  async function handleDiff(profileId: string, verId: number) {
    try {
      const res = await api.getScriptDiff(profileId, verId)
      setDiffContent(res.data?.diff || '')
      setShowDiff(true)
    } catch {}
  }

  async function handleRollback(profileId: string, verId: number) {
    try {
      await api.rollbackScriptVersion(profileId, verId)
      success(t('profiles.scriptRolledBack'))
      loadProfiles()
      // Reload versions and re-open history
      await loadVersions(profileId)
    } catch (err: any) { error(err.message) }
  }

  function openHistory() {
    if (editing) {
      setShowHistory(true)
      loadVersions(editing.id)
    }
  }

  async function handleDelete(id: string) {
    setConfirmDeleteId(id)
  }

  async function doDelete() {
    if (!confirmDeleteId) return
    try { await api.deleteProfile(confirmDeleteId); success(t('common.deleted')); loadProfiles() }
    catch (err: any) { error(err.message) }
    setConfirmDeleteId(null)
  }


  function generatePreview(): string {
    const e = form.entry
    let script: string
    switch (e.type) {
      case 'direct':
        script = `#!ipxe
kernel ${e.kernel || '<kernel>'} ${e.cmdline || ''}
initrd ${e.initrd || '<initrd>'}
boot`
        break
      case 'chain':
        script = `#!ipxe
chain ${e.url || '<url>'}`
        break
      case 'sanboot':
        const sanUrl = e.url || '<url>'
        const sanParts: string[] = []
        if (e.san_keep_san) sanParts.push('set keep-san 1')
        if (e.san_action === 'hook') {
          sanParts.push(`sanhook ${sanUrl}`)
        } else if (e.san_action === 'zap') {
          sanParts.push(`sanzboot ${sanUrl}`)
        } else if (e.san_action === 'unhook') {
          sanParts.push('sanhook')
        } else {
          let cmd = 'sanboot'
          if (e.san_no_describe) cmd += ' --no-describe'
          if (e.san_drive) cmd += ` --drive ${e.san_drive}`
          if (sanUrl !== '<url>') cmd += ` ${sanUrl}`
          sanParts.push(cmd)
        }
        script = `#!ipxe\n${sanParts.join('\n')}`
        break
      case 'local':
        script = `#!ipxe
exit`
        break
      case 'wds':
        script = `#!ipxe
set wds-server {{.URL}}
kernel wdsmgfw.efi
initrd bootmgr.exe
initrd boot.sdi
initrd ${e.wim || '<wim>'}
boot`
        break
      case 'custom':
        script = `#!ipxe
${e.script || t('profiles.emptyScriptPlaceholder')}`
        break
      default:
        script = t('profiles.unknownTypeComment')
    }
    return replaceServerVars(script)
  }

  function replaceServerVars(script: string): string {
    const serverUrl = window.location.origin
    const ipxeVars: Record<string, string> = {
      'net0/mac': 'aa:bb:cc:dd:ee:ff',
      'net0/ip': '192.168.1.100',
      'net0/gateway': '192.168.1.1',
      'net0/dns': '192.168.1.1',
      'net0/next-server': serverUrl.replace(/^https?:\/\//, ''),
      arch: 'x86_64',
      buildarch: 'x86_64',
      platform: 'efi',
      uuid: '12345678-1234-1234-1234-123456789abc',
      serial: 'VM-12345',
      manufacturer: 'QEMU',
      product: 'Standard PC',
    }
    let result = script
      .replace(/{{\.URL}}/g, serverUrl)
      .replace(/{{\.NextServer}}/g, serverUrl)
      .replace(/{{\.MAC}}/g, 'aa:bb:cc:dd:ee:ff')
      .replace(/{{\.IP}}/g, '192.168.1.100')
      .replace(/{{\.Hostname}}/g, 'pxe-client')
    for (const [key, val] of Object.entries(ipxeVars)) {
      result = result.replace(new RegExp('\\$\\{' + key + '\\}', 'g'), val)
    }
    return result
  }
  function updateEntry(field: string, value: string | boolean | undefined) {
    setForm(prev => ({ ...prev, entry: { ...prev.entry, [field]: value } }))
  }

  const defaultProfile = profiles.find(p => p.is_default)
  const ipxeCount = profiles.filter(p => p.menu?.entries?.some(e => e.type === 'direct')).length

  const columns: Column<Profile>[] = [
    { key: 'name', label: t('profiles.name'), render: (p) => <span className="font-medium text-[var(--text-primary)]">{p.name}</span> },
    { key: 'type', label: t('profiles.entryType'), render: (p) => {
      const types = [...new Set(p.menu?.entries?.map(e => e.type) || [])]
      return <div className="flex gap-1">{types.map(t => <Tag key={t} children={t} />)}</div>
    }},
    { key: 'arch', label: t('profiles.arch'), render: (p) => <span className="font-mono text-xs">{p.arch || '*'}</span> },
    { key: 'is_default', label: t('profiles.isDefault'), render: (p) => p.is_default ? <Tag color="green">{t('profiles.default')}</Tag> : null },
    { key: 'actions', label: '', render: (p) => (
      <div className="flex gap-1 whitespace-nowrap">
        <Button variant="ghost" size="sm" onClick={() => openPreview(p)}><Eye size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>{t('common.edit')}</Button>
        <Button variant="ghost" size="sm" onClick={() => handleDelete(p.id)}>{t('common.delete')}</Button>
      </div>
    ), width: '160px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('profiles.title')}</h1>
        <Button variant="primary" size="sm" onClick={openCreate}>
          <Plus size={14} /> {t('profiles.addProfile')}
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">{t('profiles.total')}</div>
          <div className="text-[28px] font-bold tracking-tight">{profiles.length}</div>
        </div>
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">{t('profiles.isDefault')}</div>
          <div className="text-base font-bold">{defaultProfile?.name || t('profiles.noDefault')}</div>
        </div>
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">iPXE</div>
          <div className="text-[28px] font-bold tracking-tight">{ipxeCount}</div>
        </div>
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">{t('profiles.arch')}</div>
          <div className="text-base font-bold font-mono">x86_64, arm64</div>
        </div>
      </div>

      <Card padding={false}>
        <DataTable columns={columns} data={profiles} loading={loading} emptyText={t('profiles.empty')} />
      </Card>

      <Modal
        open={showModal}
        onClose={() => { setShowVars(false); setShowScriptPreview(false); setShowModal(false) }}
        title={editing ? t('profiles.editProfile') : t('profiles.addProfile')}
        width="600px"
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setShowScriptPreview(!showScriptPreview)} disabled={saving}>{showScriptPreview ? t('profiles.hidePreview') : t('profiles.previewBtn')}</Button>
            <Button variant="secondary" onClick={() => { setNameError(false); setShowVars(false); setShowScriptPreview(false); setShowModal(false) }} disabled={saving}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleSave} disabled={saving}>{saving ? t('profiles.saving') : t('common.save')}</Button>
          </>
        }
      >
        <div className="space-y-5">
          {/* 基本信息 */}
          <div>
            <h3 className="text-sm font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">{t('profiles.basicInfo')}</h3>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.name')}</label>
                <input autoFocus className={`w-full bg-[var(--bg-input)] border rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none transition-colors ${nameError ? 'border-red-500 focus:border-red-500' : 'border-[var(--bg-border)] focus:border-blue-500'}`} value={form.name} onChange={e => { setNameError(false); setForm({...form, name: e.target.value}) }} placeholder={t('profiles.namePlaceholder')} />
                {nameError && <p className="text-xs text-red-400 mt-1">{t('profiles.nameRequired')}</p>}
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.arch')} <span className="text-[var(--text-muted)] font-normal">{t('profiles.archHint')}</span></label>
                <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.arch} onChange={e => setForm({...form, arch: e.target.value})}>
                  <option value="">{t('profiles.auto')}</option>
                  <option>x86_64</option>
                  <option>arm64</option>
                  <option>i386</option>
                </select>
              </div>
            </div>
            <div className="mt-3">
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.description')}</label>
              <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.description} onChange={e => setForm({...form, description: e.target.value})} />
            </div>
            <label className="flex items-center gap-2.5 cursor-pointer mt-3">
              <button type="button" onClick={() => setForm({...form, is_default: !form.is_default})} className={`relative w-10 h-5.5 rounded-full transition-colors ${form.is_default ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`}>
                <span className={`absolute top-0.5 left-0.5 w-4.5 h-4.5 rounded-full bg-white transition-transform ${form.is_default ? 'translate-x-4.5' : ''}`} />
              </button>
              <span className="text-sm text-[var(--text-secondary)]">{t('profiles.isDefault')}</span>
            </label>
          </div>

          {/* 引导项 */}
          <div>
            <h3 className="text-sm font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">{t('profiles.bootEntry')}</h3>
            <div className="bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.entryType')}</label>
                <select className="w-56 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none" value={form.entry.type} onChange={e => updateEntry('type', e.target.value)}>
                  <option value="direct">{t('profiles.typeDirect')}</option>
                  <option value="chain">{t('profiles.typeChain')}</option>
                  <option value="local">{t('profiles.typeLocal')}</option>
                  <option value="sanboot">{t('profiles.typeSanboot')}</option>
                  <option value="wds">{t('profiles.typeWds')}</option>
                  <option value="custom">{t('profiles.typeCustom')}</option>
                </select>
              </div>
              {form.entry.type === 'direct' && (
                <>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.kernel')}</label>
                    <textarea className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.kernel || ''} onChange={e => updateEntry('kernel', e.target.value)} placeholder={t('profiles.kernelPlaceholder')} />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.initrd')}</label>
                    <textarea className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.initrd || ''} onChange={e => updateEntry('initrd', e.target.value)} placeholder={t('profiles.initrdPlaceholder')} />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.cmdline')}</label>
                    <textarea className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.cmdline || ''} onChange={e => updateEntry('cmdline', e.target.value)} placeholder={t('profiles.cmdlinePlaceholder')} />
                  </div>
                  <div className="bg-amber-500/10 border border-amber-500/30 rounded px-3 py-2 text-xs text-amber-300 leading-relaxed">
                    {t('profiles.uefiInitrdHint')}
                  </div>
                </>
              )}
              {form.entry.type === 'chain' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.url')}</label>
                  <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} placeholder={t('profiles.chainUrlPlaceholder')} />
                </div>
              )}
              {form.entry.type === 'sanboot' && (
                <div className="space-y-2">
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.sanAction')}</label>
                    <select className="w-56 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none" value={form.entry.san_action || 'boot'} onChange={e => updateEntry('san_action', e.target.value === 'boot' ? undefined : e.target.value)}>
                      <option value="boot">{t('profiles.sanBoot')}</option>
                      <option value="hook">{t('profiles.sanHook')}</option>
                      <option value="zap">{t('profiles.sanZap')}</option>
                      <option value="unhook">{t('profiles.sanUnhook')}</option>
                    </select>
                  </div>
                  {form.entry.san_action !== 'unhook' && (
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.targetUrl')}</label>
                      <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} placeholder={t('profiles.sanUrlPlaceholder')} />
                    </div>
                  )}
                  <div className="flex gap-4">
                    <label className="flex items-center gap-1.5 cursor-pointer">
                      <input type="checkbox" className="accent-blue-500" checked={!!form.entry.san_keep_san} onChange={e => updateEntry('san_keep_san', e.target.checked)} />
                      <span className="text-xs text-[var(--text-secondary)]">set keep-san 1</span>
                    </label>
                    {(!form.entry.san_action || form.entry.san_action === 'boot') && (
                      <label className="flex items-center gap-1.5 cursor-pointer">
                        <input type="checkbox" className="accent-blue-500" checked={!!form.entry.san_no_describe} onChange={e => updateEntry('san_no_describe', e.target.checked)} />
                        <span className="text-xs text-[var(--text-secondary)]">--no-describe</span>
                      </label>
                    )}
                  </div>
                  {(!form.entry.san_action || form.entry.san_action === 'boot') && (
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.driveOptional')}</label>
                      <input className="w-32 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.san_drive || ''} onChange={e => updateEntry('san_drive', e.target.value)} placeholder="0x80" />
                    </div>
                  )}
                </div>
              )}
              {form.entry.type === 'custom' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.ipxeScript')} <span className="text-[var(--text-muted)] font-normal">{t('profiles.scriptHint')}</span></label>
                  <textarea className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-y" rows={6} value={form.entry.script || ''} onChange={e => updateEntry('script', e.target.value)} placeholder={"set keep-san 1\\nsanboot --drive 0x80 http://${next-server}/winpe.iso"} />
                  {editing && (
                    <button type="button" onClick={openHistory} className="mt-1.5 flex items-center gap-1 text-xs text-[var(--text-muted)] hover:text-blue-400 transition-colors">
                      <HistoryIcon size={12} />
                      {t('profiles.scriptHistory')} ({versions.length})
                    </button>
                  )}
                  {!editing && (
                    <p className="mt-1 text-[10px] text-[var(--text-muted)] italic">{t('profiles.scriptHistory')} — {t('profiles.scriptHistoryEmpty')}</p>
                  )}
                  {showHistory && editing && (
                    <div className="mt-2 border border-[var(--bg-border)] rounded-lg bg-[var(--bg-input)] max-h-48 overflow-y-auto">
                      {versionLoading ? (
                        <p className="text-xs text-[var(--text-muted)] p-2">{t('common.loading')}</p>
                      ) : versions.length === 0 ? (
                        <p className="text-xs text-[var(--text-muted)] p-2">{t('profiles.scriptHistoryEmpty')}</p>
                      ) : (
                        versions.map(v => (
                          <div key={v.id} className="flex items-center justify-between px-2.5 py-1.5 border-b border-[var(--bg-border)] last:border-b-0">
                            <div className="min-w-0 flex-1">
                              <div className="text-[11px] font-mono text-[var(--text-muted)]">{new Date(v.created_at).toLocaleString()}</div>
                              <div className="text-[10px] text-[var(--text-muted)] truncate">{v.comment || '-'}</div>
                            </div>
                            <div className="flex items-center gap-1 ml-2 shrink-0">
                              <button type="button" onClick={() => handleDiff(editing.id, v.id)} className="px-1.5 py-0.5 text-[10px] rounded bg-blue-500/10 text-blue-400 hover:bg-blue-500/20 transition-colors">{t('profiles.scriptDiff')}</button>
                              <button type="button" onClick={() => handleRollback(editing.id, v.id)} className="px-1.5 py-0.5 text-[10px] rounded bg-amber-500/10 text-amber-400 hover:bg-amber-500/20 transition-colors">{t('profiles.scriptRollback')}</button>
                            </div>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              )}
              {form.entry.type === 'wds' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.wim')}</label>
                  <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.wim || ''} onChange={e => updateEntry('wim', e.target.value)} />
                </div>
              )}

              <div className="flex justify-end pt-1">
                <button
                  type="button"
                  onClick={() => setShowOSPicker(true)}
                  className="text-xs text-blue-500 hover:text-blue-400"
                >
                  {t('profiles.selectFromOs')}
                </button>
              </div>
            </div>
          </div>

          {/* iPXE 变量参考 */}
          <div>
            <button
              type="button"
              onClick={() => setShowVars(!showVars)}
              className="flex items-center gap-1.5 text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)] transition-colors"
            >
              <Info size={12} />
              {showVars ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
              {t('profiles.varsReference')}
            </button>
            {showVars && (
              <div className="mt-2 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs leading-relaxed">
                <div className="grid grid-cols-[1fr_2px_1.8fr] gap-x-3 gap-y-1.5 text-[var(--text-secondary)]">
                  <span className="font-mono text-[var(--text-primary)]">${'{arch}'}</span><span></span><span>{t('profiles.varArchDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{buildarch}'}</span><span></span><span>{t('profiles.varBuildarchDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{platform}'}</span><span></span><span>{t('profiles.varPlatformDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/mac}'}</span><span></span><span>{t('profiles.varMacDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/ip}'}</span><span></span><span>{t('profiles.varIpDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/gateway}'}</span><span></span><span>{t('profiles.varGatewayDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/dns}'}</span><span></span><span>{t('profiles.varDnsDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/next-server}'}</span><span></span><span>{t('profiles.varNextServerDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{uuid}'}</span><span></span><span>{t('profiles.varUuidDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{serial}'}</span><span></span><span>{t('profiles.varSerialDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{manufacturer}'}</span><span></span><span>{t('profiles.varManufacturerDesc')}</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{product}'}</span><span></span><span>{t('profiles.varProductDesc')}</span>
                </div>
                <div className="mt-3 pt-3 border-t border-[var(--bg-border)]">
                  <div className="text-[var(--text-muted)] mb-2">{t('profiles.serverVarsDesc')}</div>
                  <div className="grid grid-cols-[1fr_2px_1.8fr] gap-x-3 gap-y-1.5 text-[var(--text-secondary)]">
                    <span className="font-mono text-[var(--text-primary)]">{'{{.URL}}'}</span><span></span><span>{t('profiles.varServerUrlDesc')}</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.NextServer}}'}</span><span></span><span>{t('profiles.varServerNextServerDesc')}</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.MAC}}'}</span><span></span><span>{t('profiles.varServerMacDesc')}</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.IP}}'}</span><span></span><span>{t('profiles.varServerIpDesc')}</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.Hostname}}'}</span><span></span><span>{t('profiles.varServerHostnameDesc')}</span>
                  </div>
                </div>
                <div className="mt-3 pt-3 border-t border-[var(--bg-border)] text-[var(--text-muted)]">
                  {t('profiles.varsNote')}<span className="font-mono text-[var(--text-primary)]">bootos/${'{arch}'}/vmlinuz</span>{t('profiles.varsNoteSuffix')}
                </div>
              </div>
            )}
          </div>
          {showScriptPreview && (
            <div>
              <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-2">{t('profiles.generatedScript')}</h3>
              <pre className="p-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] font-mono text-xs whitespace-pre-wrap break-all max-h-64 overflow-y-auto">{generatePreview()}</pre>
            </div>
          )}
        </div>{showOSPicker && (
          <Modal open={showOSPicker} onClose={() => setShowOSPicker(false)} title={t('profiles.selectOs')} width="500px">
            <div className="max-h-80 overflow-y-auto space-y-1">
              {osCatalog.filter(d => d.enabled).map(distro => (
                <div key={distro.name}>
                  <div className="font-medium text-sm px-2 py-1 bg-[var(--bg-secondary)] rounded mb-1">
                    {distro.name}
                  </div>
                  {distro.versions.filter(v => v.enabled).map(ver => (
                    <button
                      key={ver.codename}
                      className="w-full text-left px-4 py-1.5 text-sm hover:bg-[var(--bg-hover)] rounded transition-colors"
                      onClick={() => {
                        const bt = ver.boot_type || 'kernel'
                        setForm(prev => {
                          const base = { ...prev.entry, cmdline: ver.cmdline || '' }
                          switch (bt) {
                            case 'kernel':
                            case 'memdisk':
                            case 'memtest':
                              return { ...prev, entry: { ...base, type: 'direct' as const, kernel: ver.remote?.kernel || ver.local?.kernel || '', initrd: ver.remote?.initrd || ver.local?.initrd || '' } }
                            case 'wimboot':
                              return { ...prev, entry: { ...base, type: 'wds' as const, wim: ver.remote?.initrd || ver.local?.initrd || '', url: ver.remote?.kernel || ver.local?.kernel || '' } }
                            case 'sanboot':
                              return { ...prev, entry: { ...base, type: 'sanboot' as const, url: ver.remote?.kernel || '', san_action: undefined } }
                            default:
                              return { ...prev, entry: { ...base, type: 'custom' as const, script: `# ${bt} — ${t('profiles.pleaseConfigure')}` } }
                          }
                        })
                        setShowOSPicker(false)
                      }}
                    >
                      <span className="font-medium">{ver.name}</span>
                      <span className="text-[var(--text-muted)] ml-2">({ver.arch})</span>
                      {ver.boot_type && (
                        <span className={`ml-2 inline-block text-[10px] px-1.5 py-0.5 rounded font-mono font-medium ${
                          ver.boot_type === 'kernel' ? 'bg-cyan-100 text-cyan-700 dark:bg-cyan-900/30 dark:text-cyan-300' :
                          ver.boot_type === 'wimboot' ? 'bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-300' :
                          ver.boot_type === 'sanboot' ? 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/30 dark:text-indigo-300' :
                          'bg-gray-100 text-gray-600 dark:bg-gray-800 dark:text-gray-400'
                        }`}>{ver.boot_type}</span>
                      )}
                      {ver.local && <span className="text-green-500 ml-2 text-xs">{t('profiles.local')}</span>}
                    </button>
                  ))}
                </div>
              ))}
              {osCatalog.length === 0 && (
                <div className="text-center py-8 text-[var(--text-muted)] text-sm">{t('profiles.noOsAvailable')}</div>
              )}
            </div>
          </Modal>
        )}
      </Modal>

      {/* 预览 */}
      <Modal open={!!previewProfile} onClose={() => setPreviewProfile(null)} title={t('profiles.preview')} width="620px">
        {previewProfile && (() => {
          const entry = previewProfile.menu?.entries?.[0]
          const entryLabels: Record<string, string> = { direct: t('profiles.typeDirect'), chain: t('profiles.typeChain'), local: t('profiles.typeLocal'), sanboot: t('profiles.typeSanboot'), wds: t('profiles.typeWds'), custom: t('profiles.typeCustom') }
          return (
            <div className="space-y-4">
              {/* 基本信息 — 紧凑一行 */}
              <div className="flex flex-wrap gap-x-6 gap-y-1 text-sm bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-4 py-3">
                <div>
                  <span className="text-[var(--text-muted)] text-xs">{t('profiles.name')}</span>
                  <p className="font-medium text-[var(--text-primary)]">{previewProfile.name}</p>
                </div>
                <div>
                  <span className="text-[var(--text-muted)] text-xs">{t('profiles.arch')}</span>
                  <p className="font-mono text-[var(--text-primary)]">{previewProfile.arch || t('profiles.auto')}</p>
                </div>
                {previewProfile.description && (
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.description')}</span>
                    <p className="text-[var(--text-primary)]">{previewProfile.description}</p>
                  </div>
                )}
                <div>
                  <span className="text-[var(--text-muted)] text-xs">{t('profiles.isDefault')}</span>
                  <p>{previewProfile.is_default ? <Tag color="green">{t('profiles.default')}</Tag> : t('profiles.no')}</p>
                </div>
                {previewProfile.created_at && !previewProfile.created_at.startsWith("0001-") && (
                  <p className="font-mono text-[var(--text-primary)]">{previewProfile.created_at.replace("T", " ").slice(0, 19)}</p>
                )}
              </div>

              {/* 引导项 */}
              {entry && (
                <div>
                  <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-2">{t('profiles.bootEntry')}</h3>
            <div className="bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
                    <div>
                      <span className="text-[var(--text-muted)] text-xs">{t('profiles.entryType')}</span>
                      <p className="text-[var(--text-primary)] mt-0.5 font-medium">{entryLabels[entry.type] || entry.type}</p>
                    </div>

                    {entry.label && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.entryLabel')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">{entry.label}</p>
                      </div>
                    )}

                    {entry.type === 'direct' && (
                      <div className="space-y-2.5">
                        <div className="grid grid-cols-2 gap-x-6 gap-y-2.5">
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">{t('profiles.kernel')}</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.kernel || '-'}</p>
                          </div>
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">{t('profiles.initrd')}</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.initrd || '-'}</p>
                          </div>
                        </div>
                        <div>
                          <span className="text-[var(--text-muted)] text-xs">{t('profiles.cmdline')}</span>
                          <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.cmdline || '-'}</p>
                        </div>
                      </div>
                    )}

                    {entry.type === 'chain' && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.url')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.url || '-'}</p>
                      </div>
                    )}
                    {entry.type === 'sanboot' && (
                      <div className="space-y-1.5">
                        <div>
                          <span className="text-[var(--text-muted)] text-xs">{t('profiles.operation')}</span>
                          <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">{entry.san_action || 'boot'}</p>
                        </div>
                        {entry.san_action !== 'unhook' && (
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">URL</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.url || '-'}</p>
                          </div>
                        )}
                        {entry.san_keep_san && (
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">keep-san</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">true</p>
                          </div>
                        )}
                        {entry.san_no_describe && (
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">--no-describe</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">true</p>
                          </div>
                        )}
                        {entry.san_drive && (
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">--drive</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">{entry.san_drive}</p>
                          </div>
                        )}
                      </div>
                    )}

                    {entry.type === 'wds' && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.wim')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all whitespace-pre-wrap">{entry.wim || '-'}</p>
                      </div>
                    )}

                    {entry.type === 'custom' && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.scriptContent')}</span>
                        <pre className="text-[var(--text-primary)] mt-1 font-mono text-xs whitespace-pre-wrap break-all bg-[var(--bg-card)] border border-[var(--bg-border)] rounded p-2 max-h-48 overflow-auto">{entry.script || '-'}</pre>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )
        })()}
      </Modal>
      {/* Diff modal */}
      <Modal open={showDiff} onClose={() => setShowDiff(false)} title={t('profiles.scriptDiff')} width="620px">
        <pre className="p-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-xs font-mono whitespace-pre-wrap break-all max-h-96 overflow-y-auto leading-relaxed">
          {diffContent.split('\n').map((line, i) => {
            let cls = 'text-[var(--text-primary)]'
            if (line.startsWith('+')) cls = 'text-green-400'
            else if (line.startsWith('-')) cls = 'text-red-400'
            return <div key={i} className={cls}>{line || ' '}</div>
          })}
        </pre>
      </Modal>
      <ConfirmDialog
        open={!!confirmDeleteId}
        onClose={() => setConfirmDeleteId(null)}
        onConfirm={doDelete}
        title={t('profiles.deleteTitle')}
        message={t('profiles.deleteConfirm')}
      />
    </div>
  )
}
