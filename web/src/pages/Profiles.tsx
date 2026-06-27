import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { api, getNetbootCatalog, type Profile, type MenuEntry, type NetbootDistro } from '../api/client'

export default function Profiles() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<Profile | null>(null)
  const [form, setForm] = useState({ name: '', description: '', arch: 'x86_64', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' } })
  const [showOSPicker, setShowOSPicker] = useState(false)
  const [osCatalog, setOSCatalog] = useState<NetbootDistro[]>([])

  useEffect(() => { loadProfiles() }, [])
  useEffect(() => {
    getNetbootCatalog().then(res => setOSCatalog(res.data?.distros || [])).catch(() => {})
  }, [])

  async function loadProfiles() {
    setLoading(true)
    try {
      const res = await api.getProfiles()
      setProfiles(res.data)
    } catch { error('加载失败') }
    finally { setLoading(false) }
  }

  function openCreate() {
    setEditing(null)
    setForm({ name: '', description: '', arch: 'x86_64', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' } })
    setShowModal(true)
  }

  function openEdit(p: Profile) {
    setEditing(p)
    const e = p.menu?.entries?.[0] || { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' }
    setForm({
      name: p.name,
      description: p.description || '',
      arch: p.arch || 'x86_64',
      is_default: p.is_default,
      entry: { ...e, kernel: e.kernel || '', initrd: e.initrd || '', cmdline: e.cmdline || '', url: e.url || '', wim: e.wim || '' },
    })
    setShowModal(true)
  }

  async function handleSave() {
    try {
      const label = form.name || '未命名'
      const data = {
        name: form.name,
        description: form.description,
        arch: form.arch,
        is_default: form.is_default,
        menu: { entries: [{ ...form.entry, label }] },
      }
      if (editing) {
        await api.updateProfile(editing.id, data)
        success('配置已更新')
      } else {
        await api.createProfile(data)
        success('配置已创建')
      }
      setShowModal(false)
      loadProfiles()
    } catch (err: any) { error(err.message) }
  }

  async function handleDelete(id: string) {
    if (!confirm(t('profiles.deleteConfirm'))) return
    try { await api.deleteProfile(id); success('已删除'); loadProfiles() }
    catch (err: any) { error(err.message) }
  }

  function updateEntry(field: string, value: string) {
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
    { key: 'is_default', label: t('profiles.isDefault'), render: (p) => p.is_default ? <Tag color="green">默认</Tag> : null },
    { key: 'actions', label: '', render: (p) => (
      <div className="flex gap-1">
        <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>{t('common.edit')}</Button>
        <Button variant="ghost" size="sm" onClick={() => handleDelete(p.id)}>{t('common.delete')}</Button>
      </div>
    ), width: '100px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div></div>
        <Button variant="primary" onClick={openCreate}>
          <Plus size={14} /> {t('profiles.addProfile')}
        </Button>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-4 gap-4 mb-6">
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">{t('profiles.total', '总配置数')}</div>
          <div className="text-[28px] font-bold tracking-tight">{profiles.length}</div>
        </div>
        <div className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[var(--text-muted)] mb-2">{t('profiles.isDefault')}</div>
          <div className="text-base font-bold">{defaultProfile?.name || '无'}</div>
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
        <DataTable columns={columns} data={profiles} loading={loading} emptyText={t('profiles.empty', '暂无配置')} />
      </Card>

      <Modal
        open={showModal}
        onClose={() => setShowModal(false)}
        title={editing ? t('profiles.editProfile') : t('profiles.addProfile')}
        width="600px"
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowModal(false)}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleSave}>{t('common.save')}</Button>
          </>
        }
      >
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.name')}</label>
              <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.name} onChange={e => setForm({...form, name: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.arch')}</label>
              <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.arch} onChange={e => setForm({...form, arch: e.target.value})}>
                <option>x86_64</option>
                <option>arm64</option>
                <option>i386</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.description')}</label>
            <input className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.description} onChange={e => setForm({...form, description: e.target.value})} />
          </div>
          <label className="flex items-center gap-2.5 cursor-pointer">
            <button type="button" onClick={() => setForm({...form, is_default: !form.is_default})} className={`relative w-10 h-5.5 rounded-full transition-colors ${form.is_default ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`}>
              <span className={`absolute top-0.5 left-0.5 w-4.5 h-4.5 rounded-full bg-white transition-transform ${form.is_default ? 'translate-x-4.5' : ''}`} />
            </button>
            <span className="text-sm text-[var(--text-secondary)]">{t('profiles.isDefault')}</span>
          </label>

          <div className="border-t border-[var(--bg-border)] pt-4">
            <span className="text-sm font-semibold text-[var(--text-primary)] mb-3 block">引导项</span>
            <div className="bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg p-4">
              <div className="grid grid-cols-2 gap-3 mb-3">
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.entryType')}</label>
                  <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none" value={form.entry.type} onChange={e => updateEntry('type', e.target.value)}>
                    <option value="direct">direct（内核+initrd）</option>
                    <option value="local">local（本地硬盘）</option>
                    <option value="chain">chain（链式加载）</option>
                    <option value="sanboot">sanboot（SAN 启动）</option>
                    <option value="wds">wds（WIM 文件）</option>
                  </select>
                </div>
              </div>
              {form.entry.type === 'direct' && (
                <>
                  <div className="grid grid-cols-2 gap-3 mb-2">
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.kernel')}</label>
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.kernel || ''} onChange={e => updateEntry('kernel', e.target.value)} />
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.initrd')}</label>
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.initrd || ''} onChange={e => updateEntry('initrd', e.target.value)} />
                    </div>
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.cmdline')}</label>
                    <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.cmdline || ''} onChange={e => updateEntry('cmdline', e.target.value)} />
                  </div>
                  <div className="mt-2 flex justify-end">
                    <button
                      type="button"
                      onClick={() => setShowOSPicker(true)}
                      className="text-xs text-blue-500 hover:text-blue-400"
                    >
                      从 OS 目录选择
                    </button>
                  </div>
                </>
              )}
              {form.entry.type === 'chain' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.url')}</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} />
                </div>
              )}
              {form.entry.type === 'wds' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.wim')}</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.wim || ''} onChange={e => updateEntry('wim', e.target.value)} />
                </div>
              )}
            </div>
          </div>
        </div>{showOSPicker && (
          <Modal open={showOSPicker} onClose={() => setShowOSPicker(false)} title="选择操作系统" width="500px">
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
                        if (ver.remote) {
                          updateEntry('kernel', ver.remote.kernel)
                          updateEntry('initrd', ver.remote.initrd)
                        }
                        updateEntry('cmdline', ver.cmdline || '')
                        setShowOSPicker(false)
                      }}
                    >
                      <span className="font-medium">{ver.name}</span>
                      <span className="text-[var(--text-muted)] ml-2">({ver.arch})</span>
                      {ver.local && <span className="text-green-500 ml-2 text-xs">本地</span>}
                    </button>
                  ))}
                </div>
              ))}
              {osCatalog.length === 0 && (
                <div className="text-center py-8 text-[var(--text-muted)] text-sm">暂无可用操作系统</div>
              )}
            </div>
          </Modal>
        )}
      </Modal>
    </div>
  )
}
