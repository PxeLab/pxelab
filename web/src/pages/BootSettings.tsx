import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { RefreshCw, CheckCircle, XCircle, Save, RotateCcw, ArrowRight } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { DataTable, type Column } from '../components/ui/DataTable'
import { PageHeader } from '../components/ui/PageHeader'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { Input, Select } from '../components/ui/FormControls'
import { api, type BootloaderCheckResult, type ArchEntryData, type BootFileInfo, type IPXEScriptSettings } from '../api/client'

// NBP 类型选项
const NBP_OPTIONS = [
  { value: 'ipxe', label: 'iPXE' },
  { value: 'pxelinux', label: 'PXELinux' },
  { value: 'grub2', label: 'GRUB2' },
]

// 支持 Secure Boot 的架构（需要 shim + 签名 iPXE）
const SECURE_BOOT_SUPPORTED = new Set([7, 11]) // EFI_X86_64, EFI_ARM64

// 各引导加载器的原生架构支持情况
const BOOTLOADER_SUPPORT: Record<string, Record<number, 'native' | 'fallback' | 'unsupported'>> = {
  pxelinux: {
    0: 'native',   // BIOS → pxelinux.bios
    6: 'native',   // IA32 → pxelinux32.efi
    7: 'native',   // x64 → pxelinux.efi
    9: 'fallback', // BC → fallback 到 grubx64.efi
    11: 'fallback', // ARM64 → fallback 到 grubaa64.efi
    21: 'unsupported',
  },
  grub: {
    0: 'unsupported',  // BIOS → GRUB2 无 BIOS 版本
    6: 'unsupported',  // IA32 → GRUB2 无 32 位 EFI 版本
    7: 'native',       // x64 → grubx64.efi
    9: 'native',       // BC → grubx64.efi
    11: 'native',      // ARM64 → grubaa64.efi
    21: 'unsupported',
  },
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
}

function findFileInfo(files: BootFileInfo[], name: string): BootFileInfo | undefined {
  return files.find(f => f.name === name)
}

export default function BootSettings() {
  const { t } = useTranslation()
  const [checkResult, setCheckResult] = useState<BootloaderCheckResult | null>(null)
  const [loading, setLoading] = useState(true)
  const [archEntries, setArchEntries] = useState<ArchEntryData[]>([])
  const [archLoading, setArchLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [saveMsg, setSaveMsg] = useState<{ ok: boolean; text: string } | null>(null)

  // iPXE Script (Option 175)
  const [ipxeScript, setIpxeScript] = useState<IPXEScriptSettings | null>(null)
  const [ipxeScriptLoading, setIpxeScriptLoading] = useState(true)


  useEffect(() => { runHealthCheck() }, [])
  useEffect(() => { loadArchMap() }, [])
  useEffect(() => { loadIPXEScript() }, [])

  async function runHealthCheck() {
    setLoading(true)
    try {
      const res = await api.getBootloaderCheck()
      setCheckResult(res.data)
    } catch { /* ignore */ }
    finally { setLoading(false) }
  }

  async function loadArchMap() {
    setArchLoading(true)
    try {
      const res = await api.getArchMap()
      setArchEntries(res.data.entries)
    } catch { /* ignore */ }
    finally { setArchLoading(false) }
  }

  async function loadIPXEScript() {
    setIpxeScriptLoading(true)
    try {
      const res = await api.getIPXEScript()
      setIpxeScript(res.data)
    } catch { /* ignore */ }
    finally { setIpxeScriptLoading(false) }
  }

  // ── 全局 chain_load 控制 ──
  const nonIPXEEntries = archEntries.filter(e => e.nbp !== 'ipxe')
  const globalChainLoad = nonIPXEEntries.length > 0 && nonIPXEEntries.every(e => e.chain_load)

  function toggleGlobalChainLoad() {
    const newVal = !globalChainLoad
    setArchEntries(prev => prev.map(e => {
      if (e.nbp === 'ipxe') return { ...e, chain_load: false }
      return { ...e, chain_load: newVal }
    }))
  }

  function updateEntry(code: number, field: keyof ArchEntryData, value: string | boolean) {
    setArchEntries(prev => prev.map(e => {
      if (e.arch_code !== code) return e
      const updated = { ...e, [field]: value }
      // NBP 变更时自动更新引导文件名
      if (field === 'nbp') {
        updated.ipxe = getDefaultIPXE(code)
        if (value === 'pxelinux') {
          updated.pxelinux = getDefaultPXELinux(code)
        } else if (value === 'grub2') {
          updated.grub = getDefaultGRUB(code)
        }
        // 切换到 iPXE 时关闭 chain_load
        if (value === 'ipxe') {
          updated.chain_load = false
        }
      }
      return updated
    }))
  }

  async function saveArchMap() {
    setSaving(true)
    setSaveMsg(null)
    try {
      await api.updateArchMap({ entries: archEntries })
      // 同时保存 iPXE 脚本配置（Option 175）
      if (ipxeScript) {
        await api.updateIPXEScript(ipxeScript)
      }
      setSaveMsg({ ok: true, text: t('common.saved', '已保存') })
    } catch (err: any) {
      setSaveMsg({ ok: false, text: err.message || t('common.saveFailed', '保存失败') })
    }
    finally { setSaving(false) }
  }

  async function resetToDefaults() {
    try {
      const res = await api.getArchMapDefaults()
      setArchEntries(res.data.entries)
      setSaveMsg({ ok: true, text: t('bootSettings.resetDefaultsHint', '已恢复默认值，点击保存生效') })
    } catch { /* ignore */ }
  }

  const archColumns = buildArchColumns(t, checkResult?.files ?? [], updateEntry)

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('bootSettings.title')}
        className="mb-0"
        actions={
          <Button variant="secondary" size="sm" onClick={runHealthCheck} disabled={loading}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            {t('common.refresh', '刷新')}
          </Button>
        }
      />

      {/* ── 汇总统计 ── */}
      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : !checkResult ? null : (
          <div className="space-y-3">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
              <Card padding={false}>
                <div className="px-3 py-2.5 flex items-center gap-3 border-l-2 border-l-blue-500/40">
                  <span className="text-base font-bold text-[var(--text-primary)]">{checkResult.total}</span>
                  <span className="text-[11px] text-[var(--text-muted)]">{t('bootloader.total')}</span>
                </div>
              </Card>
              <Card padding={false}>
                <div className="px-3 py-2.5 flex items-center gap-3 border-l-2 border-l-accent-green/40">
                  <span className="text-base font-bold text-accent-green">{checkResult.present}</span>
                  <span className="text-[11px] text-[var(--text-muted)]">{t('bootloader.present')}</span>
                </div>
              </Card>
              <Card padding={false}>
                <div className="px-3 py-2.5 flex items-center gap-3 border-l-2 border-l-accent-red/40">
                  <span className="text-base font-bold text-accent-red">{checkResult.missing}</span>
                  <span className="text-[11px] text-[var(--text-muted)]">{t('bootloader.missing')}</span>
                </div>
              </Card>
              <Card padding={false}>
                <div className="px-3 py-2.5 flex items-center gap-3 border-l-2 border-l-accent-yellow/40">
                  {checkResult.all_ok
                    ? <><CheckCircle size={16} className="text-accent-green" /><span className="text-[11px] font-semibold text-accent-green">{t('bootloader.ok')}</span></>
                    : <><XCircle size={16} className="text-accent-red" /><span className="text-[11px] font-semibold text-accent-red">{t('bootloader.issues')}</span></>
                  }
                  <span className="text-[11px] text-[var(--text-muted)]">{t('bootloader.status')}</span>
                </div>
              </Card>
            </div>
            <div className="text-[11px] text-[var(--text-muted)] font-mono">
              {t('bootloader.rootDir')}: {checkResult.root_dir}
            </div>
          </div>
        )}
      </Card>

      {/* ── 架构引导配置 ── */}
      <Card padding={false}>
        <div className="p-4 border-b border-[var(--bg-border)] flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold text-[var(--text-primary)]">{t('bootSettings.nbpConfig')}</h2>
            <p className="text-xs text-[var(--text-muted)] mt-1">{t('bootSettings.nbpConfigHint')}</p>
          </div>
          <div className="flex items-center gap-2">
            {saveMsg && (
              <span className={`text-xs ${saveMsg.ok ? 'text-accent-green' : 'text-accent-red'}`}>
                {saveMsg.text}
              </span>
            )}
            <Button variant="secondary" size="sm" onClick={resetToDefaults}>
              <RotateCcw size={14} />
              {t('bootSettings.resetDefaults', '恢复默认')}
            </Button>
            <Button variant="primary" size="sm" onClick={saveArchMap} disabled={saving}>
              <Save size={14} className={saving ? 'animate-spin' : ''} />
              {t('common.save', '保存')}
            </Button>
          </div>
        </div>

        <DataTable
          columns={archColumns}
          data={archEntries}
          loading={archLoading}
          rowKey={e => String(e.arch_code)}
        />
        <div className="p-4 bg-[var(--bg-card)] border-t border-[var(--bg-border)]">
          <p className="text-xs text-[var(--text-muted)]">{t('bootSettings.nbpHint')}</p>
        </div>

        {/* ── 全局链式加载 + iPXE 脚本 (Option 175) ── */}
        <div className="px-4 py-4 border-t border-[var(--bg-border)] space-y-4">
          {/* 全局 chain_load */}
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm text-[var(--text-primary)]">
                {t('bootSettings.globalChainLoad', 'NBP 引导后链式加载到 iPXE')}
              </div>
              <div className="text-[11px] text-[var(--text-muted)]">
                {t('bootSettings.globalChainLoadHint', '开启后，所有使用 PXELinux 或 GRUB2 的架构将在引导 NBP 后自动链式加载到 iPXE 脚本')}
              </div>
            </div>
            <Toggle
              checked={globalChainLoad}
              onChange={toggleGlobalChainLoad}
            />
          </div>

          <div className="border-t border-[var(--bg-border)]" />

          {/* Option 175 配置 */}
          <div>
            {ipxeScriptLoading ? (
              <div className="flex items-center justify-center py-4">
                <div className="animate-spin w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full" />
              </div>
            ) : ipxeScript ? (
              <div className="space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <div className="text-sm text-[var(--text-primary)]">
                      {t('bootSettings.ipxeScriptEnabled', '启用 Option 175')}
                    </div>
                    <div className="text-[11px] text-[var(--text-muted)]">
                      {t('bootSettings.ipxeScriptEnabledHint', 'DHCP 应答中包含 iPXE 脚本 URL，客户端可直接 chainload')}
                    </div>
                  </div>
                  <Toggle
                    checked={ipxeScript.enabled}
                    onChange={v => setIpxeScript(prev => prev ? { ...prev, enabled: v } : prev)}
                  />
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                  <div>
                    <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
                      {t('bootSettings.ipxeScriptPort', 'HTTP 端口')}
                    </label>
                    <Input size="xs"
                      type="number"
                      className="py-1.5"
                      value={ipxeScript.port}
                      onChange={e => setIpxeScript(prev => prev ? { ...prev, port: parseInt(e.target.value) || 8080 } : prev)}
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
                      {t('bootSettings.ipxeScriptPath', '脚本路径')}
                    </label>
                    <Input size="xs"
                      type="text"
                      className="py-1.5 font-mono"
                      value={ipxeScript.path}
                      onChange={e => setIpxeScript(prev => prev ? { ...prev, path: e.target.value } : prev)}
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-medium text-[var(--text-secondary)] mb-1">
                      {t('bootSettings.ipxeScriptFlags', '特征标志 (子选项 177)')}
                    </label>
                    <Input size="xs"
                      type="number"
                      className="py-1.5"
                      value={ipxeScript.feature_flags}
                      onChange={e => setIpxeScript(prev => prev ? { ...prev, feature_flags: parseInt(e.target.value) || 1 } : prev)}
                    />
                    <p className="text-[10px] text-[var(--text-muted)] mt-1">
                      {t('bootSettings.ipxeScriptFlagsHint', '默认 0x01 = HTTP 模式')}
                    </p>
                  </div>
                </div>


              </div>
            ) : null}
          </div>
        </div>
      </Card>
    </div>
  )
}

/** 每行派生数据：NBP、链式加载目标、文件信息 */
function getArchRowData(entry: ArchEntryData, files: BootFileInfo[]) {
  const nbp = entry.nbp || 'ipxe'
  const chainLoad = entry.chain_load || false
  const support = nbp === 'pxelinux' ? BOOTLOADER_SUPPORT.pxelinux[entry.arch_code]
    : nbp === 'grub2' ? BOOTLOADER_SUPPORT.grub[entry.arch_code]
    : 'native'

  // 获取当前 NBP 对应的引导文件名
  const bootFile = nbp === 'pxelinux' ? entry.pxelinux
    : nbp === 'grub2' ? entry.grub
    : entry.ipxe

  // 获取链式加载目标
  const chainTarget = chainLoad ? entry.ipxe : ''

  const fileInfo = bootFile ? findFileInfo(files, bootFile) : undefined
  const chainFileInfo = chainTarget ? findFileInfo(files, chainTarget) : undefined

  return { nbp, chainLoad, support, bootFile, chainTarget, fileInfo, chainFileInfo }
}

/** 架构引导配置表的列定义：NBP 选择 + Secure Boot + 引导文件 */
function buildArchColumns(
  t: TFunction,
  files: BootFileInfo[],
  onUpdate: (code: number, field: keyof ArchEntryData, value: string | boolean) => void,
): Column<ArchEntryData>[] {
  return [
    {
      key: 'arch_name',
      label: t('bootSettings.arch'),
      render: e => <span className="text-[var(--text-primary)] font-mono text-xs">{e.arch_name}</span>,
    },
    {
      key: 'arch_code',
      label: t('bootSettings.archCode'),
      render: e => <span className="font-mono text-xs">{e.arch_code}</span>,
    },
    {
      // NBP 类型选择
      key: 'nbp',
      label: t('bootSettings.nbpType'),
      render: e => {
        const { nbp, support } = getArchRowData(e, files)
        return (
          <>
            <Select
              size="xs"
              className="py-1.5"
              value={nbp}
              onChange={v => onUpdate(e.arch_code, 'nbp', v.target.value)}
            >
              {NBP_OPTIONS.map(opt => (
                <option key={opt.value} value={opt.value}>{opt.label}</option>
              ))}
            </Select>
            {support === 'fallback' && (
              <div className="flex items-center gap-1 text-[10px] text-blue-400 mt-1">
                <ArrowRight size={10} />
                <span>{t('bootSettings.willFallback', '运行时自动回退')}</span>
              </div>
            )}
            {support === 'unsupported' && (
              <div className="flex items-center gap-1 text-[10px] text-accent-yellow mt-1">
                <span>{t('bootSettings.notNative', '非原生支持')}</span>
              </div>
            )}
          </>
        )
      },
    },
    {
      // Secure Boot 支持（可编辑）
      key: 'secure_boot',
      label: t('bootSettings.secureBoot', 'Secure Boot'),
      render: e => {
        const { nbp } = getArchRowData(e, files)
        return (
          <>
            <Toggle
              checked={e.secure_boot}
              onChange={v => onUpdate(e.arch_code, 'secure_boot', v)}
              disabled={!SECURE_BOOT_SUPPORTED.has(e.arch_code)}
            />
            {!SECURE_BOOT_SUPPORTED.has(e.arch_code) ? (
              <div className="text-[10px] text-[var(--text-muted)] mt-1">
                {t('bootSettings.secureBootUnsupported', '该架构不支持 Secure Boot')}
              </div>
            ) : e.secure_boot ? (
              <div className="mt-1.5 space-y-1.5">
                {nbp === 'grub2' ? (
                  <>
                    <div>
                      <label className="text-[10px] text-[var(--text-muted)]">{t('bootSettings.shimGrub', 'Shim (GRUB2)')}</label>
                      <Input size="xs"
                        className="text-[10px] font-mono"
                        value={e.shim_grub}
                        onChange={v => onUpdate(e.arch_code, 'shim_grub', v.target.value)}
                      />
                    </div>
                    <div>
                      <label className="text-[10px] text-[var(--text-muted)]">{t('bootSettings.grubSb', 'GRUB2 SB')}</label>
                      <Input size="xs"
                        className="text-[10px] font-mono"
                        value={e.grub_sb}
                        onChange={v => onUpdate(e.arch_code, 'grub_sb', v.target.value)}
                      />
                    </div>
                  </>
                ) : (
                  <>
                    <div>
                      <label className="text-[10px] text-[var(--text-muted)]">{t('bootSettings.shimIpxe', 'Shim (iPXE)')}</label>
                      <Input size="xs"
                        className="text-[10px] font-mono"
                        value={e.shim_ipxe}
                        onChange={v => onUpdate(e.arch_code, 'shim_ipxe', v.target.value)}
                      />
                    </div>
                    <div>
                      <label className="text-[10px] text-[var(--text-muted)]">{t('bootSettings.ipxeSb', 'iPXE SB')}</label>
                      <Input size="xs"
                        className="text-[10px] font-mono"
                        value={e.ipxe_sb}
                        onChange={v => onUpdate(e.arch_code, 'ipxe_sb', v.target.value)}
                      />
                    </div>
                  </>
                )}
              </div>
            ) : null}
          </>
        )
      },
    },
    {
      // 引导文件名（可编辑）
      key: 'boot_file',
      label: t('bootSettings.bootFile'),
      render: e => {
        const { nbp, chainLoad, bootFile, chainTarget } = getArchRowData(e, files)
        return (
          <>
            <Input size="xs"
              className="py-1.5 font-mono"
              value={bootFile}
              onChange={v => {
                if (nbp === 'pxelinux') onUpdate(e.arch_code, 'pxelinux', v.target.value)
                else if (nbp === 'grub2') onUpdate(e.arch_code, 'grub', v.target.value)
                else onUpdate(e.arch_code, 'ipxe', v.target.value)
              }}
            />
            {chainLoad && chainTarget && (
              <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] mt-1">
                <ArrowRight size={10} />
                <span className="font-mono">{chainTarget}</span>
              </div>
            )}
          </>
        )
      },
    },
    {
      // 文件状态
      key: 'file_status',
      label: t('bootSettings.fileStatus'),
      render: e => {
        const { chainLoad, chainTarget, fileInfo, chainFileInfo } = getArchRowData(e, files)
        return (
          <>
            <FileStatus info={fileInfo} />
            {chainLoad && chainTarget && (
              <div className="mt-1">
                <FileStatus info={chainFileInfo} />
              </div>
            )}
          </>
        )
      },
    },
  ]
}

/** 文件状态指示器 */
function FileStatus({ info }: { info?: BootFileInfo }) {
  if (!info) {
    return (
      <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)]">
        <XCircle size={10} className="text-accent-yellow" />
        <span>-</span>
      </div>
    )
  }
  return (
    <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
      {info.present
        ? <CheckCircle size={10} className="text-accent-green shrink-0" />
        : <XCircle size={10} className={info.required ? 'text-accent-red' : 'text-accent-yellow'} />
      }
      <span className="font-mono">{info.present ? formatSize(info.size ?? 0) : '-'}</span>
    </div>
  )
}

// ── 默认引导文件名 ──

function getDefaultIPXE(archCode: number): string {
  switch (archCode) {
    case 0: return 'ipxe.pxe'         // BIOS x86
    case 6: return 'ipxe32.efi'       // EFI IA32
    case 7: case 9: return 'ipxe.efi' // EFI x86_64 / EFI BC
    case 10: return 'ipxe-arm32.efi'  // EFI ARM32
    case 11: return 'ipxe-arm64.efi'  // EFI ARM64
    case 25: return 'ipxe-riscv32.efi' // EFI RISC-V 32
    case 27: return 'ipxe-riscv64.efi' // EFI RISC-V 64
    case 37: case 39: return 'ipxe-loong64.efi' // LoongArch
    default: return 'ipxe.pxe'
  }
}

function getDefaultPXELinux(archCode: number): string {
  switch (archCode) {
    case 0: return 'pxelinux.bios'
    case 6: return 'pxelinux32.efi'
    case 7: return 'pxelinux.efi'
    default: return ''
  }
}

function getDefaultGRUB(archCode: number): string {
  switch (archCode) {
    case 7: case 9: return 'grubx64.efi'
    case 11: return 'grubaa64.efi'
    default: return ''
  }
}
