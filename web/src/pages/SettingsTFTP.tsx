import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type TFTPSettingsData } from '../api/client'

export default function SettingsTFTP() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [existingBootFiles, setExistingBootFiles] = useState<string[]>([])
  const [config, setConfig] = useState<TFTPSettingsData>({
    enabled: true, port: 69, root: '',
    pxe_config_file: 'pxelinux.cfg/default',
    grub_config_file: 'grub2/grub.cfg',
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const [res, fileRes] = await Promise.all([
        api.getTFTPSettings(),
        api.getFiles('').catch(() => ({ data: [] as any[] })),
      ])
      const d = res.data
      setConfig({
        enabled: d.enabled ?? true,
        port: d.port > 0 ? d.port : 69,
        root: d.root || '',
        pxe_config_file: d.pxe_config_file || 'pxelinux.cfg/default',
        grub_config_file: d.grub_config_file || 'grub2/grub.cfg',
      })
      setExistingBootFiles(fileRes.data ? fileRes.data.map((f: any) => f.name) : [])
    } catch (err: any) {
      showError(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.updateTFTPSettings(config)
      success('设置已保存')
    } catch (err: any) {
      showError(err.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">TFTP 设置</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
            <RefreshCw size={14} /> {t('common.reload', '重载配置')}
          </Button>
          <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
            <Save size={14} /> {saving ? '保存中...' : t('settings.save')}
          </Button>
        </div>
      </div>

      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">加载中...</span>
          </div>
        ) : (
          <div className="space-y-4">
            <Toggle checked={config.enabled} onChange={v => setConfig({...config, enabled: v})} label={t('settings.enabled') + ' TFTP'} />
            <div className="grid grid-cols-2 gap-4">
              <SettingsField label={t('settings.port')}>
                <SettingsInput value={String(config.port)} onChange={v => setConfig({...config, port: parseInt(v) || 69})} />
              </SettingsField>
              <SettingsField label={t('settings.tftpRoot', '根目录')}>
                <SettingsInput value={config.root} onChange={v => setConfig({...config, root: v})} />
              </SettingsField>
              <SettingsField label="PXE 配置文件路径" help="客户端请求 PXELinux 配置的路径">
                <SettingsInput value={config.pxe_config_file} onChange={v => setConfig({...config, pxe_config_file: v})} placeholder="pxelinux.cfg/default" />
              </SettingsField>
              <SettingsField label="GRUB2 配置文件路径" help="客户端请求 GRUB2 配置的路径">
                <SettingsInput value={config.grub_config_file} onChange={v => setConfig({...config, grub_config_file: v})} placeholder="grub2/grub.cfg" />
              </SettingsField>
            </div>

            {/* 架构引导文件映射 */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-1">客户端架构 → 引导文件映射（只读）</h3>
              <p className="text-xs text-[var(--text-muted)] mb-4">DHCP 根据客户端架构（Option 93 / Option 60 VCI）自动分配对应的 NBP 引导文件。在「网络接口」页签可切换每个接口的引导加载器。</p>

              {[
                {
                  label: 'iPXE（默认）',
                  color: 'text-blue-400',
                  rows: [
                    { arch: 'BIOS x86', code: '00000', file: 'undionly.kpxe' },
                    { arch: 'UEFI IA32', code: '00006', file: 'ipxe32.efi' },
                    { arch: 'UEFI x64', code: '00007', file: 'ipxe.efi' },
                    { arch: 'EFI BC (x64)', code: '00009', file: 'ipxe.efi' },
                    { arch: 'UEFI ARM64', code: '00011', file: 'ipxe-arm64.efi' },
                    { arch: 'UEFI RISC-V 64', code: '00027', file: 'ipxe-riscv64.efi' },
                  ],
                },
                {
                  label: 'PXELinux',
                  color: 'text-amber-400',
                  rows: [
                    { arch: 'BIOS x86', code: '00000', file: 'pxelinux.bios' },
                    { arch: 'UEFI IA32', code: '00006', file: 'pxelinux.efi' },
                    { arch: 'UEFI x64', code: '00007', file: 'pxelinux.efi' },
                    { arch: 'EFI BC (x64)', code: '00009', file: 'pxelinux.efi' },
                  ],
                },
                {
                  label: 'GRUB2',
                  color: 'text-green-400',
                  note: 'BIOS 架构不支持 GRUB2',
                  rows: [
                    { arch: 'BIOS x86', code: '00000', file: '—' },
                    { arch: 'UEFI x64', code: '00007', file: 'grubx64.efi' },
                    { arch: 'EFI BC (x64)', code: '00009', file: 'grubx64.efi' },
                    { arch: 'UEFI ARM64', code: '00011', file: 'grubaa64.efi' },
                  ],
                },
              ].map(group => (
                <div key={group.label} className="mb-4 last:mb-0">
                  <h4 className="text-xs font-semibold mb-2">
                    <span className={group.color}>{group.label}</span>
                    {group.note && <span className="text-[var(--text-muted)] ml-1 font-normal">（{group.note}）</span>}
                  </h4>
                  <div className="overflow-hidden rounded-lg border border-[var(--bg-border)]">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="bg-[var(--bg-card)] border-b border-[var(--bg-border)]">
                          <th className="text-left px-3 py-2 text-xs font-semibold text-[var(--text-secondary)]">架构</th>
                          <th className="text-left px-3 py-2 text-xs font-semibold text-[var(--text-secondary)]">代码</th>
                          <th className="text-left px-3 py-2 text-xs font-semibold text-[var(--text-secondary)] hidden sm:table-cell">VCI 特征</th>
                          <th className="text-left px-3 py-2 text-xs font-semibold text-[var(--text-secondary)]">引导文件</th>
                          <th className="text-left px-3 py-2 text-xs font-semibold text-[var(--text-secondary)] w-14">状态</th>
                        </tr>
                      </thead>
                      <tbody>
                        {group.rows.map(row => {
                          const exists = row.file !== '—' && existingBootFiles.includes(row.file)
                          const vci = row.file !== '—' ? `PXEClient:Arch:${row.code}` : ''
                          return (
                            <tr key={group.label + row.code} className="border-b border-[var(--bg-border)] last:border-0">
                              <td className="px-3 py-2 text-[var(--text-primary)] font-medium text-xs">{row.arch}</td>
                              <td className="px-3 py-2 text-[var(--text-muted)] font-mono text-xs">{row.code}</td>
                              <td className="px-3 py-2 hidden sm:table-cell">
                                {vci ? <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded text-[var(--text-muted)] font-mono">{vci}</code> : null}
                              </td>
                              <td className="px-3 py-2">
                                {row.file === '—'
                                  ? <span className="text-xs text-[var(--text-muted)]">不支持</span>
                                  : <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded text-[var(--text-primary)] font-mono">{row.file}</code>
                                }
                              </td>
                              <td className="px-3 py-2">
                                {row.file === '—' ? null
                                  : exists ? <span className="text-xs text-green-500 font-medium">✓</span>
                                  : <span className="text-xs text-[var(--text-muted)]">—</span>
                                }
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>
                </div>
              ))}
              <p className="text-xs text-[var(--text-muted)] mt-2">可在「文件管理」页面上传缺失的引导文件到 {config.root || '启动目录'}。</p>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
