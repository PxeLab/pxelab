import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { RefreshCw, CheckCircle, XCircle, Save, RotateCcw, AlertTriangle, ArrowRight } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { api, type BootloaderCheckResult, type ArchEntryData, type BootFileInfo } from '../api/client'

// 各引导加载器的原生架构支持情况
// native  = 该引导加载器有此架构的原生构建
// fallback = PXELinux 不支持该架构，运行时回退到 GRUB
// unsupported = 无原生构建，无回退
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

  function updateEntry(code: number, field: 'ipxe' | 'pxelinux' | 'grub', value: string) {
    setArchEntries(prev => prev.map(e => e.arch_code === code ? { ...e, [field]: value } : e))
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

      {/* ── 可编辑架构映射表（含文件状态） ── */}
      <Card padding={false}>
        <div className="p-4 border-b border-[var(--bg-border)] flex items-center justify-between">
          <div>
            <h2 className="text-sm font-semibold text-[var(--text-primary)]">{t('bootSettings.archMap')}</h2>
            <p className="text-xs text-[var(--text-muted)] mt-1">{t('bootSettings.archMapHint')}</p>
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
                  <th className="px-4 py-3">iPXE</th>
                  <th className="px-4 py-3">PXELinux</th>
                  <th className="px-4 py-3">GRUB2</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--bg-border)]">
                {archEntries.map(e => (
                  <tr key={e.arch_code} className="hover:bg-[var(--bg-hover)]/30">
                    <td className="px-4 py-3 text-[var(--text-primary)] font-mono text-xs">{e.arch_name}</td>
                    <td className="px-4 py-3 text-[var(--text-secondary)] font-mono text-xs">{e.arch_code}</td>
                    <td className="px-4 py-2">
                      <EditableCell
                        value={e.ipxe}
                        files={checkResult?.files ?? []}
                        onChange={v => updateEntry(e.arch_code, 'ipxe', v)}
                      />
                    </td>
                    <td className="px-4 py-2">
                      <EditableCell
                        value={e.pxelinux}
                        hint={BOOTLOADER_SUPPORT.pxelinux[e.arch_code]}
                        files={checkResult?.files ?? []}
                        onChange={v => updateEntry(e.arch_code, 'pxelinux', v)}
                      />
                    </td>
                    <td className="px-4 py-2">
                      <EditableCell
                        value={e.grub}
                        hint={BOOTLOADER_SUPPORT.grub[e.arch_code]}
                        files={checkResult?.files ?? []}
                        onChange={v => updateEntry(e.arch_code, 'grub', v)}
                      />
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
        <div className="p-4 bg-[var(--bg-card)] border-t border-[var(--bg-border)]">
          <p className="text-xs text-[var(--text-muted)]">{t('bootSettings.editHint')}</p>
        </div>
      </Card>
    </div>
  )
}

/** 单个文件名输入框 + 下方文件状态行 + 兼容性提示 */
function EditableCell({ value, hint, files, onChange }: {
  value: string
  hint?: 'native' | 'fallback' | 'unsupported'
  files: BootFileInfo[]
  onChange: (v: string) => void
}) {
  const { t } = useTranslation()
  const info = value ? findFileInfo(files, value) : undefined
  return (
    <div className="flex flex-col gap-1">
      <input
        className={`w-full bg-[var(--bg-input)] border rounded px-2 py-1.5 text-xs font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 ${hint && hint !== 'native' ? 'border-dashed border-[var(--bg-border)] opacity-60' : 'border-[var(--bg-border)]'}`}
        value={value}
        onChange={v => onChange(v.target.value)}
      />
      {value && info && (
        <div className="flex items-center gap-2 text-[10px] text-[var(--text-muted)]">
          {info.present
            ? <CheckCircle size={10} className="text-green-400 shrink-0" />
            : <XCircle size={10} className={info.required ? 'text-red-400' : 'text-yellow-400'} />
          }
          <span className="font-mono">{info.present ? formatSize(info.size ?? 0) : '-'}</span>
          <span className="font-mono max-w-[72px] truncate shrink-0" title={'SHA-256: ' + info.checksum}>
            {info.checksum ? info.checksum.substring(0, 8) : '-'}
          </span>
        </div>
      )}
      {value && !info && (
        <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)]">
          <XCircle size={10} className="text-yellow-400 shrink-0" />
          <span>-</span>
        </div>
      )}
      {!value && hint === 'fallback' && (
        <div className="flex items-center gap-1 text-[10px] text-blue-400">
          <ArrowRight size={10} className="shrink-0" />
          <span>{t('bootSettings.fallbackGrub', '回退到 GRUB')}</span>
        </div>
      )}
      {!value && hint === 'unsupported' && (
        <div className="flex items-center gap-1 text-[10px] text-[var(--text-muted)] italic">
          <AlertTriangle size={10} className="shrink-0" />
          <span>{t('bootSettings.notSupported', '不支持此引导加载器')}</span>
        </div>
      )}
      {!value && !hint && (
        <div className="text-[10px] text-[var(--text-muted)]">-</div>
      )}
    </div>
  )
}
