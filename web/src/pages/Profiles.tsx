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
  const [form, setForm] = useState({ name: '', description: '', arch: '', is_default: false, entry: { type: 'direct' as MenuEntry['type'], kernel: '', initrd: '', cmdline: '', url: '', wim: '', script: '', san_action: '' as MenuEntry['san_action'], san_no_describe: false, san_drive: '', san_keep_san: false } })
  const [showOSPicker, setShowOSPicker] = useState(false)
  const [showVars, setShowVars] = useState(false)
  const [showScriptPreview, setShowScriptPreview] = useState(false)
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
      const label = form.name || '未命名'
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
        success('配置已更新')
      } else {
        await api.createProfile(data)
        success('配置已创建')
      }
      setShowVars(false)
      setShowScriptPreview(false)
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
${e.script || '<空脚本>'}`
        break
      default:
        script = '# 未知类型'
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
        onClose={() => { setShowVars(false); setShowScriptPreview(false); setShowModal(false) }}
        title={editing ? t('profiles.editProfile') : t('profiles.addProfile')}
        width="600px"
        footer={
          <>
            <Button variant="ghost" size="sm" onClick={() => setShowScriptPreview(!showScriptPreview)} disabled={saving}>{showScriptPreview ? '隐藏预览' : '预览'}</Button>
            <Button variant="secondary" onClick={() => { setNameError(false); setShowVars(false); setShowScriptPreview(false); setShowModal(false) }} disabled={saving}>{t('common.cancel')}</Button>
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
                  <option value="custom">custom — 原始 iPXE 脚本</option>
                </select>
              </div>
              {form.entry.type === 'direct' && (
                <>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.kernel')}</label>
                    <textarea className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.kernel || ''} onChange={e => updateEntry('kernel', e.target.value)} placeholder="vmlinuz 或 bootos/${arch}/vmlinuz" />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.initrd')}</label>
                    <textarea className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.initrd || ''} onChange={e => updateEntry('initrd', e.target.value)} placeholder="initrd.img 或 bootos/${arch}/initrd" />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.cmdline')}</label>
                    <textarea className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-none whitespace-pre-wrap break-all" rows={2} value={form.entry.cmdline || ''} onChange={e => updateEntry('cmdline', e.target.value)} placeholder="例: console=tty0 quiet" />
                  </div>
                </>
              )}
              {form.entry.type === 'chain' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.url')}</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} placeholder="http://server/ipxe.efi 或 ${next-server}/bootmgr.efi" />
                </div>
              )}
              {form.entry.type === 'sanboot' && (
                <div className="space-y-2">
                  <div>
                    <label className="block text-xs text-[var(--text-muted)] mb-0.5">操作类型</label>
                    <select className="w-56 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none" value={form.entry.san_action || 'boot'} onChange={e => updateEntry('san_action', e.target.value === 'boot' ? undefined : e.target.value)}>
                      <option value="boot">sanboot — 从 SAN 启动</option>
                      <option value="hook">sanhook — 注册 SAN 盘</option>
                      <option value="zap">sanzboot — 清除后启动</option>
                      <option value="unhook">sanhook — 断开 SAN 连接</option>
                    </select>
                  </div>
                  {form.entry.san_action !== 'unhook' && (
                    <div>
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">目标 URL</label>
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.url || ''} onChange={e => updateEntry('url', e.target.value)} placeholder="iscsi://server/iqn 或 http://server/centos7.iso" />
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
                      <label className="block text-xs text-[var(--text-muted)] mb-0.5">--drive（可选）</label>
                      <input className="w-32 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.san_drive || ''} onChange={e => updateEntry('san_drive', e.target.value)} placeholder="0x80" />
                    </div>
                  )}
                </div>
              )}
              {form.entry.type === 'custom' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">iPXE 脚本 <span className="text-[var(--text-muted)] font-normal">{'（支持所有 iPXE 变量：${net0/mac}、${net0/next-server}、${arch} 等；服务端变量：{{.URL}}、{{.MAC}}）'}</span></label>
                  <textarea className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono resize-y" rows={6} value={form.entry.script || ''} onChange={e => updateEntry('script', e.target.value)} placeholder={"set keep-san 1\\nsanboot --drive 0x80 http://${next-server}/winpe.iso"} />
                </div>
              )}
              {form.entry.type === 'wds' && (
                <div>
                  <label className="block text-xs text-[var(--text-muted)] mb-0.5">{t('profiles.wim')}</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono" value={form.entry.wim || ''} onChange={e => updateEntry('wim', e.target.value)} />
                </div>
              )}

              <div className="flex justify-end pt-1">
                <button
                  type="button"
                  onClick={() => setShowOSPicker(true)}
                  className="text-xs text-blue-500 hover:text-blue-400"
                >
                  从 OS 目录选择
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
                <div className="mt-3 pt-3 border-t border-[var(--bg-border)]">
                  <div className="text-[var(--text-muted)] mb-2">服务端变量 — 由服务端在生成 iPXE 脚本时自动替换（预览时显示模拟值）</div>
                  <div className="grid grid-cols-[1fr_2px_1.8fr] gap-x-3 gap-y-1.5 text-[var(--text-secondary)]">
                    <span className="font-mono text-[var(--text-primary)]">{'{{.URL}}'}</span><span></span><span>服务器基地址 — http://server:port</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.NextServer}}'}</span><span></span><span>DHCP next-server 地址</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.MAC}}'}</span><span></span><span>客户端 MAC 地址</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.IP}}'}</span><span></span><span>客户端 IP 地址</span>
                    <span className="font-mono text-[var(--text-primary)]">{'{{.Hostname}}'}</span><span></span><span>客户端主机名</span>
                  </div>
                </div>
                <div className="mt-3 pt-3 border-t border-[var(--bg-border)] text-[var(--text-muted)]">
                  路径中的变量由客户端 iPXE 在运行时替换，示例：<span className="font-mono text-[var(--text-primary)]">bootos/${'{arch}'}/vmlinuz</span> 会根据客户端架构自动加载对应文件
                </div>
              </div>
            )}
          </div>
          {showScriptPreview && (
            <div>
              <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-2">生成的 iPXE 脚本</h3>
              <pre className="p-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg text-[var(--text-primary)] font-mono text-xs whitespace-pre-wrap break-all max-h-64 overflow-y-auto">{generatePreview()}</pre>
            </div>
          )}
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
                              return { ...prev, entry: { ...base, type: 'custom' as const, script: `# ${bt} — 请手动配置引导参数` } }
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
      <Modal open={!!previewProfile} onClose={() => setPreviewProfile(null)} title={t('profiles.preview', '查看配置')} width="620px">
        {previewProfile && (() => {
          const entry = previewProfile.menu?.entries?.[0]
          const entryLabels: Record<string, string> = { direct: 'direct — 内核+initrd 引导', chain: 'chain — 链式加载 NBP', local: 'local — 本地硬盘启动', sanboot: 'sanboot — SAN 存储启动', wds: 'wds — WIM 文件启动', custom: 'custom — 自定义脚本' }
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
                  <p className="font-mono text-[var(--text-primary)]">{previewProfile.arch || '自动'}</p>
                </div>
                {previewProfile.description && (
                  <div>
                    <span className="text-[var(--text-muted)] text-xs">{t('profiles.description')}</span>
                    <p className="text-[var(--text-primary)]">{previewProfile.description}</p>
                  </div>
                )}
                <div>
                  <span className="text-[var(--text-muted)] text-xs">{t('profiles.isDefault')}</span>
                  <p>{previewProfile.is_default ? <Tag color="green">默认</Tag> : '否'}</p>
                </div>
                {previewProfile.created_at && !previewProfile.created_at.startsWith("0001-") && (
                  <p className="font-mono text-[var(--text-primary)]">{previewProfile.created_at.replace("T", " ").slice(0, 19)}</p>
                )}
              </div>

              {/* 引导项 */}
              {entry && (
                <div>
                  <h3 className="text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider mb-2">引导项</h3>
                  <div className="bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg p-4 space-y-3">
                    <div>
                      <span className="text-[var(--text-muted)] text-xs">{t('profiles.entryType')}</span>
                      <p className="text-[var(--text-primary)] mt-0.5 font-medium">{entryLabels[entry.type] || entry.type}</p>
                    </div>

                    {entry.label && (
                      <div>
                        <span className="text-[var(--text-muted)] text-xs">标签</span>
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
                          <span className="text-[var(--text-muted)] text-xs">操作</span>
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
                        <span className="text-[var(--text-muted)] text-xs">脚本内容</span>
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
    </div>
  )
}
