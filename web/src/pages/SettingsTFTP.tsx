import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type TFTPSettingsData } from '../api/client'
import Files from './Files'

type BootloaderKey = 'ipxe' | 'pxelinux' | 'grub2'

interface BootFileRow {
  arch: string
  code: string
  file: string
}

interface BootloaderGroup {
  label: string
  rows: BootFileRow[]
  note?: string
}

const bootloaderGroups: Record<BootloaderKey, BootloaderGroup> = {
  ipxe: {
    label: 'iPXE（默认）',
    rows: [
      { arch: 'BIOS x86', code: '00000', file: 'undionly.kpxe' },
      { arch: 'UEFI IA32', code: '00006', file: 'ipxe32.efi' },
      { arch: 'UEFI x64', code: '00007', file: 'ipxe.efi' },
      { arch: 'EFI BC (x64)', code: '00009', file: 'ipxe.efi' },
      { arch: 'UEFI ARM64', code: '00011', file: 'ipxe-arm64.efi' },
      { arch: 'UEFI RISC-V 64', code: '00027', file: 'ipxe-riscv64.efi' },
    ],
  },
  pxelinux: {
    label: 'PXELinux',
    rows: [
      { arch: 'BIOS x86', code: '00000', file: 'pxelinux.bios' },
      { arch: 'UEFI IA32', code: '00006', file: 'pxelinux.efi' },
      { arch: 'UEFI x64', code: '00007', file: 'pxelinux.efi' },
      { arch: 'EFI BC (x64)', code: '00009', file: 'pxelinux.efi' },
    ],
  },
  grub2: {
    label: 'GRUB2',
    note: 'BIOS 架构不支持 GRUB2',
    rows: [
      { arch: 'BIOS x86', code: '00000', file: '—' },
      { arch: 'UEFI x64', code: '00007', file: 'grubx64.efi' },
      { arch: 'EFI BC (x64)', code: '00009', file: 'grubx64.efi' },
      { arch: 'UEFI ARM64', code: '00011', file: 'grubaa64.efi' },
    ],
  },
}

const bootloaderTabs = [
  { key: 'ipxe' as BootloaderKey, label: 'iPXE（默认）', color: 'text-blue-400', activeBorder: 'border-blue-500/30', activeBg: 'bg-blue-500/10' },
  { key: 'pxelinux' as BootloaderKey, label: 'PXELinux', color: 'text-amber-400', activeBorder: 'border-amber-500/30', activeBg: 'bg-amber-500/10' },
  { key: 'grub2' as BootloaderKey, label: 'GRUB2', color: 'text-green-400', activeBorder: 'border-green-500/30', activeBg: 'bg-green-500/10' },
]

type PageTab = 'settings' | 'files'

export default function SettingsTFTP() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const pageTab = (searchParams.get('tab') as PageTab) || 'settings'
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [activeBootloader, setActiveBootloader] = useState<BootloaderKey>('ipxe')
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
        enabled: true,
        port: 69,
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
      await api.updateTFTPSettings({ ...config, enabled: true, port: 69 })
      success('设置已保存')
    } catch (err: any) {
      showError(err.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const group = bootloaderGroups[activeBootloader]

  return (
    <div>
      <h1 className="text-lg font-bold text-[var(--text-primary)] mb-6">TFTP 设置</h1>

      {/* 页面页签 + 操作按钮 */}
      <div className="flex items-center justify-between mb-4">
        <div className="flex gap-1">
          {([
            { key: 'settings' as PageTab, label: '基本设置' },
            { key: 'files' as PageTab, label: '文件管理' },
          ]).map(tab => (
            <button key={tab.key}
              onClick={() => setSearchParams({ tab: tab.key })}
              className={`px-4 py-2 rounded-lg text-sm font-medium border transition-all ${
                pageTab === tab.key
                  ? 'bg-blue-500/10 text-blue-400 border-blue-500/30'
                  : 'text-[var(--text-secondary)] border-transparent hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>
        {pageTab === 'settings' && (
          <div className="flex gap-2">
            <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
              <RefreshCw size={14} /> 刷新
            </Button>
            <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
              <Save size={14} /> {saving ? '保存中...' : t('settings.save')}
            </Button>
          </div>
        )}
      </div>

      {pageTab === 'settings' ? (
        <Card>
          {loading ? (
            <div className="flex items-center justify-center py-16">
              <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
              <span className="ml-3 text-sm text-[var(--text-muted)]">加载中...</span>
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid grid-cols-2 gap-4">
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

                <div className="flex gap-1 mb-4">
                  {bootloaderTabs.map(tab => {
                    const active = activeBootloader === tab.key
                    return (
                      <button key={tab.key}
                        onClick={() => setActiveBootloader(tab.key)}
                        className={`px-3 py-1.5 rounded-lg text-xs font-medium border transition-all ${
                          active
                            ? `${tab.color} ${tab.activeBg} ${tab.activeBorder}`
                            : 'text-[var(--text-secondary)] border-transparent hover:bg-[var(--bg-card)] hover:text-[var(--text-primary)]'
                        }`}
                      >
                        {tab.label}
                      </button>
                    )
                  })}
                </div>

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
                          <tr key={row.code + row.file} className="border-b border-[var(--bg-border)] last:border-0">
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

                {group.note && (
                  <p className="text-xs text-[var(--text-muted)] mt-2">{group.note}</p>
                )}
                <p className="text-xs text-[var(--text-muted)] mt-2">可在「文件管理」页签上传缺失的引导文件到 {config.root || '启动目录'}。</p>
              </div>
            </div>
          )}
        </Card>
      ) : (
        <div className="pt-4">
          <Files hideHeader rootPath={config.root} />
        </div>
      )}
    </div>
  )
}
