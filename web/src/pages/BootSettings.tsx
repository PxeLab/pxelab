import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, CheckCircle, XCircle, Save, RotateCcw, ArrowRight } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { api, type BootloaderCheckResult, type ArchEntryData, type BootFileInfo } from '../api/client'

// NBP 类型选项
const NBP_OPTIONS = [
  { value: 'ipxe', label: 'iPXE' },
  { value: 'pxelinux', label: 'PXELinux' },
  { value: 'grub2', label: 'GRUB2' },
]

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

  useEffect(() => { runHealthCheck() }, [])
  useEffect(() => { loadArchMap() }, [])

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
      }
      return updated
    }))
  }

  async function saveArchMap() {
    setSaving(true)
    setSaveMsg(null)
    try {
      await api.updateArchMap({ entries: archEntries })
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

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('bootSettings.title')}</h1>
        <Button variant="secondary" size="sm" onClick={runHealthCheck} disabled={loading}>
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {t('common.refresh', '刷新')}
        </Button>
      </div>

      {/* ── 汇总统计 ── */}
      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">{t('settings.loading')}</span>
          </div>
        ) : !checkResult ? null : (
          <div className="space-y-4">
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <Card padding={false}>
                <div className="p-4 text-center">
                  <div className="text-[10px] text-[var(--text-muted)] mb-1">{t('bootloader.total')}</div>
                  <div className="text-lg font-semibold text-[var(--text-primary)]">{checkResult.total}</div>
                </div>
              </Card>
              <Card padding={false}>
                <div className="p-4 text-center">
                  <div className="text-[10px] text-[var(--text-muted)] mb-1">{t('bootloader.present')}</div>
                  <div className="text-lg font-semibold text-green-400">{checkResult.present}</div>
                </div>
              </Card>
              <Card padding={false}>
                <div className="p-4 text-center">
                  <div className="text-[10px] text-[var(--text-muted)] mb-1">{t('bootloader.missing')}</div>
                  <div className="text-lg font-semibold text-red-400">{checkResult.missing}</div>
                </div>
              </Card>
              <Card padding={false}>
                <div className="p-4 text-center">
                  <div className="text-[10px] text-[var(--text-muted)] mb-1">{t('bootloader.status')}</div>
                  <div className="flex items-center justify-center gap-1 mt-1">
                    {checkResult.all_ok
                      ? <><CheckCircle size={16} className="text-green-400" /><span className="text-sm font-semibold text-green-400">{t('bootloader.ok')}</span></>
                      : <><XCircle size={16} className="text-red-400" /><span className="text-sm font-semibold text-red-400">{t('bootloader.issues')}</span></>
                    }
                  </div>
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
              <span className={`text-xs ${saveMsg.ok ? 'text-green-400' : 'text-red-400'}`}>
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
        {archLoading ? (
          <div className="flex items-center justify-center py-12">
            <div className="animate-spin w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full" />
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-left text-xs font-semibold text-[var(--text-secondary)] border-b border-[var(--bg-border)]">
                  <th className="px-4 py-3">{t('bootSettings.arch')}</th>
                  <th className="px-4 py-3">{t('bootSettings.archCode')}</th>
                  <th className="px-4 py-3">{t('bootSettings.nbpType')}</th>
                  <th className="px-4 py-3">{t('bootSettings.chainLoad')}</th>
                  <th className="px-4 py-3">{t('bootSettings.secureBoot', 'Secure Boot')}</th>
                  <th className="px-4 py-3">{t('bootSettings.bootFile')}</th>
                  <th className="px-4 py-3">{t('bootSettings.fileStatus')}</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--bg-border)]">
                {archEntries.map(e => (
                  <ArchRow
                    key={e.arch_code}
                    entry={e}
                    files={checkResult?.files ?? []}
                    onUpdate={(field, value) => updateEntry(e.arch_code, field, value)}
                  />
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="p-4 bg-[var(--bg-card)] border-t border-[var(--bg-border)]">
          <p className="text-xs text-[var(--text-muted)]">{t('bootSettings.nbpHint')}</p>
        </div>
      </Card>
    </div>
  )
}

/** 架构行：NBP 选择 + 链式加载 + 引导文件 */
function ArchRow({ entry, files, onUpdate }: {
  entry: ArchEntryData
  files: BootFileInfo[]
  onUpdate: (field: keyof ArchEntryData, value: string | boolean) => void
}) {
  const { t } = useTranslation()
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

  return (
    <tr className="hover:bg-[var(--bg-hover)]/30">
      <td className="px-4 py-3 text-[var(--text-primary)] font-mono text-xs">{entry.arch_name}</td>
      <td className="px-4 py-3 text-[var(--text-secondary)] font-mono text-xs">{entry.arch_code}</td>

      {/* NBP 类型选择 */}
      <td className="px-4 py-2">
        <select
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
          value={nbp}
          onChange={v => onUpdate('nbp', v.target.value)}
        >
          {NBP_OPTIONS.map(opt => (
            <option key={opt.value} value={opt.value}>{opt.label}</option>
          ))}
        </select>
        {support === 'fallback' && (
          <div className="flex items-center gap-1 text-[10px] text-blue-400 mt-1">
            <ArrowRight size={10} />
            <span>{t('bootSettings.willFallback', '运行时自动回退')}</span>
          </div>
        )}
        {support === 'unsupported' && (
          <div className="flex items-center gap-1 text-[10px] text-yellow-400 mt-1">
            <span>{t('bootSettings.notNative', '非原生支持')}</span>
          </div>
        )}
      </td>

      {/* 链式加载开关 */}
      <td className="px-4 py-2">
        {nbp === 'ipxe' ? (
          <span className="text-xs text-[var(--text-muted)]">-</span>
        ) : (
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              className="w-4 h-4 rounded border-[var(--bg-border)] text-blue-500 focus:ring-blue-500"
              checked={chainLoad}
              onChange={v => onUpdate('chain_load', v.target.checked)}
            />
            <span className="text-xs text-[var(--text-secondary)]">{t('bootSettings.toIPXE', '到 iPXE')}</span>
          </label>
        )}
      </td>

      {/* Secure Boot 支持 */}
      <td className="px-4 py-2">
        {entry.secure_boot ? (
          <div className="space-y-1">
            <div className="flex items-center gap-1 text-[10px] text-green-400">
              <CheckCircle size={10} />
              <span>{t('bootSettings.supported', '支持')}</span>
            </div>
            {entry.ipxe_sb && (
              <div className="text-[10px] text-[var(--text-muted)] font-mono">{entry.ipxe_sb}</div>
            )}
            {entry.shim && (
              <div className="text-[10px] text-[var(--text-muted)] font-mono">{entry.shim}</div>
            )}
          </div>
        ) : (
          <span className="text-xs text-[var(--text-muted)]">-</span>
        )}
      </td>

      {/* 引导文件名（可编辑） */}
      <td className="px-4 py-2">
        <input
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded px-2 py-1.5 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
          value={bootFile}
          onChange={v => {
            if (nbp === 'pxelinux') onUpdate('pxelinux', v.target.value)
            else if (nbp === 'grub2') onUpdate('grub', v.target.value)
            else onUpdate('ipxe', v.target.value)
          }}
        />
        {chainLoad && chainTarget && (
          <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] mt-1">
            <ArrowRight size={10} />
            <span className="font-mono">{chainTarget}</span>
          </div>
        )}
      </td>

      {/* 文件状态 */}
      <td className="px-4 py-2">
        <FileStatus info={fileInfo} />
        {chainLoad && chainTarget && (
          <div className="mt-1">
            <FileStatus info={chainFileInfo} />
          </div>
        )}
      </td>
    </tr>
  )
}

/** 文件状态指示器 */
function FileStatus({ info }: { info?: BootFileInfo }) {
  if (!info) {
    return (
      <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)]">
        <XCircle size={10} className="text-yellow-400" />
        <span>-</span>
      </div>
    )
  }
  return (
    <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
      {info.present
        ? <CheckCircle size={10} className="text-green-400 shrink-0" />
        : <XCircle size={10} className={info.required ? 'text-red-400' : 'text-yellow-400'} />
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
