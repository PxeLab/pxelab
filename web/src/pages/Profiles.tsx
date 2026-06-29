import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Plus, ChevronDown, ChevronRight, Info, Eye } from 'lucide-react'
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
  const [form, setForm] = useState({ name: '', description: '', arch: '', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' } })
  const [showOSPicker, setShowOSPicker] = useState(false)
  const [showVars, setShowVars] = useState(false)
  const [saving, setSaving] = useState(false)
  const [nameError, setNameError] = useState(false)
  const [previewProfile, setPreviewProfile] = useState<Profile | null>(null)
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

  function openPreview(p: Profile) {
    setPreviewProfile(p)
  }

  function openCreate() {
    setEditing(null)
    setForm({ name: '', description: '', arch: '', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' } })
    setShowModal(true)
  }

  function openEdit(p: Profile) {
    setEditing(p)
    const e = p.menu?.entries?.[0] || { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '' }
    setForm({
      name: p.name,
      description: p.description || '',
      arch: p.arch || '',
      is_default: p.is_default,
      entry: { ...e, kernel: e.kernel || '', initrd: e.initrd || '', cmdline: e.cmdline || '', url: e.url || '', wim: e.wim || '' },
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
    finally { setSaving(false) }
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
        <Button variant="ghost" size="sm" onClick={() => openPreview(p)}><Eye size={13} /></Button>
        <Button variant="ghost" size="sm" onClick={() => openEdit(p)}>{t('common.edit')}</Button>
        <Button variant="ghost" size="sm" onClick={() => handleDelete(p.id)}>{t('common.delete')}</Button>
      </div>
    ), width: '130px' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">启动配置</h1>
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
            <Button variant="secondary" onClick={() => { setNameError(false); setShowModal(false) }} disabled={saving}>{t('common.cancel')}</Button>
            <Button variant="primary" onClick={handleSave} disabled={saving}>{saving ? '保存中...' : t('common.save')}</Button>
          </>
        }
      >
        <div className="space-y-5">
          {/* 基本信息 */}
          <div>
            <h3 className="text-sm font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">基本信息</h3>
            <div className="grid grid-cols-2 gap-4">
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.name')}</label>
                <input className={`w-full bg-[var(--bg-input)] border rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none transition-colors ${nameError ? 'border-red-500 focus:border-red-500' : 'border-[var(--bg-border)] focus:border-blue-500'}`} value={form.name} onChange={e => { setNameError(false); setForm({...form, name: e.target.value}) }} placeholder="例: BootOS" />
                {nameError && <p className="text-xs text-red-400 mt-1">请输入配置名称</p>}
              </div>
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">{t('profiles.arch')} <span className="text-[var(--text-muted)] font-normal">（如使用变量可留空）</span></label>
                <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={form.arch} onChange={e => setForm({...form, arch: e.target.value})}>
                  <option value="">自动</option>
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
            <h3 className="text-sm font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">引导项</h3>
            <div className="bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
              <div>
                <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.entryType')}</label>
                <select className="w-56 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none" value={form.entry.type} onChange={e => updateEntry('type', e.target.value)}>
                  <option value="direct">direct — 内核+initrd 引导</option>
                  <option value="chain">chain — 链式加载另一个 NBP</option>
                  <option value="local">local — 本地硬盘启动</option>
                  <option value="sanboot">sanboot — SAN 存储启动</option>
                  <option value="wds">wds — WIM 文件启动</option>
                </select>
              </div>
              {form.entry.type === 'direct' && (
                <>
                  <div className="grid grid-cols-2 gap-3">
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.kernel')}</label>
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.kernel || ''} onChange={e => updateEntry('kernel', e.target.value)} placeholder="vmlinuz 或 bootos/${arch}/vmlinuz" />
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.initrd')}</label>
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.initrd || ''} onChange={e => updateEntry('initrd', e.target.value)} placeholder="initrd.img 或 bootos/${arch}/initrd" />
                    </div>
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.cmdline')}</label>
                    <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500" value={form.entry.cmdline || ''} onChange={e => updateEntry('cmdline', e.target.value)} placeholder="例: console=tty0 quiet" />
                  </div>
                  <div className="flex justify-end">
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
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} placeholder="http://server/ipxe.efi 或 ${next-server}/bootmgr.efi" />
                </div>
              )}
              {form.entry.type === 'wds' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.wim')}</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.wim || ''} onChange={e => updateEntry('wim', e.target.value)} />
                </div>
              )}
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
              iPXE 变量参考 — 路径和参数中可使用以下变量，运行时自动替换
            </button>
            {showVars && (
              <div className="mt-2 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-xs leading-relaxed">
                <div className="grid grid-cols-[1fr_2px_1.8fr] gap-x-3 gap-y-1.5 text-[var(--text-secondary)]">
                  <span className="font-mono text-[var(--text-primary)]">${'{arch}'}</span><span></span><span>CPU 架构 — x86_64 / x86 / arm64</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{buildarch}'}</span><span></span><span>iPXE 编译目标架构</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{platform}'}</span><span></span><span>平台类型 — efi / pc (BIOS)</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/mac}'}</span><span></span><span>客户端 MAC 地址</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/ip}'}</span><span></span><span>客户端 IP 地址（DHCP 分配）</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/gateway}'}</span><span></span><span>网关地址</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/dns}'}</span><span></span><span>DNS 服务器</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{net0/next-server}'}</span><span></span><span>DHCP next-server（TFTP 地址）</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{uuid}'}</span><span></span><span>主机 SMBIOS UUID</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{serial}'}</span><span></span><span>主机序列号</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{manufacturer}'}</span><span></span><span>硬件厂商</span>
                  <span className="font-mono text-[var(--text-primary)]">${'{product}'}</span><span></span><span>硬件型号</span>
                </div>
                <div className="mt-2 pt-2 border-t border-[var(--bg-border)] text-[var(--text-muted)]">
                  路径中的变量由客户端 iPXE 在运行时替换，示例：<span className="font-mono text-[var(--text-primary)]">bootos/${'{arch}'}/vmlinuz</span> 会根据客户端架构自动加载对应文件
                </div>
              </div>
            )}
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

      {/* 预览 */}
      <Modal open={!!previewProfile} onClose={() => setPreviewProfile(null)} title={t('profiles.preview', '查看配置')} width="560px">
        {previewProfile && (() => {
          const entry = previewProfile.menu?.entries?.[0]
          const entryLabels: Record<string, string> = { direct: 'direct — 内核+initrd 引导', chain: 'chain — 链式加载 NBP', local: 'local — 本地硬盘启动', sanboot: 'sanboot — SAN 存储启动', wds: 'wds — WIM 文件启动' }
          return (
            <div className="space-y-5">
              <div>
                <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">基本信息</h3>
                <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.name')}</span>
                    <p className="font-medium text-[var(--text-primary)] mt-0.5">{previewProfile.name}</p>
                  </div>
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.arch')}</span>
                    <p className="font-mono text-sm text-[var(--text-primary)] mt-0.5">{previewProfile.arch || '自动'}</p>
                  </div>
                  <div className="col-span-2">
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.description')}</span>
                    <p className="text-sm text-[var(--text-primary)] mt-0.5">{previewProfile.description || '-'}</p>
                  </div>
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.isDefault')}</span>
                    <p className="text-sm mt-0.5">{previewProfile.is_default ? <Tag color="green">默认</Tag> : '否'}</p>
                  </div>
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">创建时间</span>
                    <p className="text-sm text-[var(--text-primary)] mt-0.5 font-mono">{previewProfile.created_at?.replace('T', ' ').slice(0, 19)}</p>
                  </div>
                </div>
              </div>
              {entry && (
                <div>
                  <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-3">引导项</h3>
                  <div className="bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
                    <div className="grid grid-cols-2 gap-x-6 gap-y-3 text-sm">
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.entryType')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5">{entryLabels[entry.type] || entry.type}</p>
                      </div>
                      {entry.type === 'local' && (
                        <div>
                          <span className="text-[var(--text-muted)] text-xs">标签</span>
                          <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs">{entry.label}</p>
                        </div>
                      )}
                    </div>
                    {entry.type === 'direct' && (
                      <div className="space-y-2.5">
                        <div className="grid grid-cols-2 gap-x-6 gap-y-2.5">
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">{t('profiles.kernel')}</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all">{entry.kernel || '-'}</p>
                          </div>
                          <div>
                            <span className="text-[var(--text-muted)] text-xs">{t('profiles.initrd')}</span>
                            <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all">{entry.initrd || '-'}</p>
                          </div>
                        </div>
                        <div>
                          <span className="text-[var(--text-muted)] text-xs">{t('profiles.cmdline')}</span>
                          <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all">{entry.cmdline || '-'}</p>
                        </div>
                      </div>
                    )}
                    {entry.type === 'chain' && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.url')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all">{entry.url || '-'}</p>
                      </div>
                    )}
                    {entry.type === 'wds' && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">{t('profiles.wim')}</span>
                        <p className="text-[var(--text-primary)] mt-0.5 font-mono text-xs break-all">{entry.wim || '-'}</p>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          )
        })()}
      </Modal>
    </div>
  )
}
