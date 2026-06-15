import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { Tag } from '../components/ui/Tag'
import { useToast } from '../components/ui/Toast'
import { api, type Profile, type MenuEntry } from '../api/client'

export default function Profiles() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [loading, setLoading] = useState(true)
  const [showModal, setShowModal] = useState(false)
  const [editing, setEditing] = useState<Profile | null>(null)
  const [form, setForm] = useState({ name: '', description: '', arch: 'x86_64', is_default: false, entries: [] as MenuEntry[] })

  useEffect(() => { loadProfiles() }, [])

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
    setForm({ name: '', description: '', arch: 'x86_64', is_default: false, entries: [] as MenuEntry[] })
    setShowModal(true)
  }

  function openEdit(p: Profile) {
    setEditing(p)
    setForm({
      name: p.name,
      description: p.description || '',
      arch: p.arch || 'x86_64',
      is_default: p.is_default,
      entries: p.menu?.entries?.length > 0 ? p.menu.entries : [],
    })
    setShowModal(true)
  }

  async function handleSave() {
    try {
      const data = {
        name: form.name,
        description: form.description,
        arch: form.arch,
        is_default: form.is_default,
        menu: { entries: form.entries.filter(e => e.label) },
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

  function updateEntry(i: number, field: keyof MenuEntry, value: string) {
    setForm(prev => {
      const entries = [...prev.entries]
      entries[i] = { ...entries[i], [field]: value }
      return { ...prev, entries }
    })
  }

  const defaultProfile = profiles.find(p => p.is_default)
  const ipxeCount = profiles.filter(p => p.menu?.entries?.some(e => e.type === 'direct')).length

  const columns: Column<Profile>[] = [
    { key: 'name', label: t('profiles.name'), render: (p) => <span className="font-medium text-[#e8eaed]">{p.name}</span> },
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
        <div className="bg-[#16181f] border border-[#232738] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[#6b7294] mb-2">{t('profiles.total', '总配置数')}</div>
          <div className="text-[28px] font-bold tracking-tight">{profiles.length}</div>
        </div>
        <div className="bg-[#16181f] border border-[#232738] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[#6b7294] mb-2">{t('profiles.isDefault')}</div>
          <div className="text-base font-bold">{defaultProfile?.name || '无'}</div>
        </div>
        <div className="bg-[#16181f] border border-[#232738] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[#6b7294] mb-2">iPXE</div>
          <div className="text-[28px] font-bold tracking-tight">{ipxeCount}</div>
        </div>
        <div className="bg-[#16181f] border border-[#232738] rounded-xl p-4">
          <div className="text-xs font-semibold uppercase tracking-wider text-[#6b7294] mb-2">{t('profiles.arch')}</div>
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
              <label className="block text-xs font-semibold text-[#9aa0ab] mb-1">{t('profiles.name')}</label>
              <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500" value={form.name} onChange={e => setForm({...form, name: e.target.value})} />
            </div>
            <div>
              <label className="block text-xs font-semibold text-[#9aa0ab] mb-1">{t('profiles.arch')}</label>
              <select className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 appearance-none" value={form.arch} onChange={e => setForm({...form, arch: e.target.value})}>
                <option>x86_64</option>
                <option>arm64</option>
                <option>i386</option>
              </select>
            </div>
          </div>
          <div>
            <label className="block text-xs font-semibold text-[#9aa0ab] mb-1">{t('profiles.description')}</label>
            <input className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500" value={form.description} onChange={e => setForm({...form, description: e.target.value})} />
          </div>
          <label className="flex items-center gap-2.5 cursor-pointer">
            <button type="button" onClick={() => setForm({...form, is_default: !form.is_default})} className={`relative w-10 h-5.5 rounded-full transition-colors ${form.is_default ? 'bg-blue-500' : 'bg-[#232738]'}`}>
              <span className={`absolute top-0.5 left-0.5 w-4.5 h-4.5 rounded-full bg-white transition-transform ${form.is_default ? 'translate-x-4.5' : ''}`} />
            </button>
            <span className="text-sm text-[#9aa0ab]">{t('profiles.isDefault')}</span>
          </label>

          <div className="border-t border-[#232738] pt-4">
            <div className="flex items-center justify-between mb-3">
              <span className="text-sm font-semibold text-[#e8eaed]">{t('profiles.menuEntries')}</span>
              <Button size="sm" onClick={() => setForm(prev => ({ ...prev, entries: [...prev.entries, { label: '', type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '' }] }))}>
                + {t('profiles.addEntry')}
              </Button>
            </div>
            {form.entries.map((entry, i) => (
              <div key={i} className="bg-[#1a1d2e] border border-[#232738] rounded-lg p-4 mb-3">
                <div className="grid grid-cols-2 gap-3 mb-3">
                  <div>
                    <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.entryLabel')}</label>
                    <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.label} onChange={e => updateEntry(i, 'label', e.target.value)} />
                  </div>
                  <div>
                    <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.entryType')}</label>
                    <select className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none" value={entry.type} onChange={e => updateEntry(i, 'type', e.target.value as any)}>
                      <option value="direct">direct</option>
                      <option value="local">local</option>
                      <option value="chain">chain</option>
                      <option value="sanboot">sanboot</option>
                      <option value="wds">wds</option>
                    </select>
                  </div>
                </div>
                {entry.type === 'direct' && (
                  <>
                    <div className="grid grid-cols-2 gap-3 mb-2">
                      <div>
                        <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.kernel')}</label>
                        <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.kernel || ''} onChange={e => updateEntry(i, 'kernel', e.target.value)} />
                      </div>
                      <div>
                        <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.initrd')}</label>
                        <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.initrd || ''} onChange={e => updateEntry(i, 'initrd', e.target.value)} />
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.cmdline')}</label>
                      <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.cmdline || ''} onChange={e => updateEntry(i, 'cmdline', e.target.value)} />
                    </div>
                  </>
                )}
                {entry.type === 'chain' && (
                  <div>
                    <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.url')}</label>
                    <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.url || ''} onChange={e => updateEntry(i, 'url', e.target.value)} />
                  </div>
                )}
                {entry.type === 'wds' && (
                  <div>
                    <label className="block text-xs text-[#6b7294] mb-0.5">{t('profiles.wim')}</label>
                    <input className="w-full bg-[#111318] border border-[#232738] rounded px-2.5 py-1.5 text-xs text-[#e8eaed] outline-none focus:border-blue-500" value={entry.wim || ''} onChange={e => updateEntry(i, 'wim', e.target.value)} />
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </Modal>
    </div>
  )
}
