import { useState, useEffect } from 'react'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { api, type NetbootSettingsData } from '../api/client'

export default function SettingsNetboot() {
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<NetbootSettingsData>({
    enabled: false,
    catalog_redirect: { enabled: true, target_url: '', detect_arch: true, preamble: '' },
    catalog_display: { title: '', groups: [] },
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getNetbootSettings()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        catalog_redirect: {
          enabled: d.catalog_redirect?.enabled ?? true,
          target_url: d.catalog_redirect?.target_url || 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}',
          detect_arch: d.catalog_redirect?.detect_arch ?? true,
          preamble: d.catalog_redirect?.preamble || '',
        },
        catalog_display: {
          title: d.catalog_display?.title || '[OS] Netboot OS Install Catalog',
          groups: d.catalog_display?.groups || [
            { name: 'linux', title: 'Linux Distributions', enabled: true, order: 1 },
            { name: 'linux-i386', title: 'Linux Distributions (32-bit)', enabled: true, order: 2 },
            { name: 'linux-arm64', title: 'Linux Distributions (arm64)', enabled: true, order: 3 },
            { name: 'bsd', title: 'BSD Systems', enabled: true, order: 4 },
            { name: 'live', title: 'Live CDs', enabled: true, order: 5 },
            { name: 'live-arm', title: 'Live CDs (arm64)', enabled: true, order: 6 },
            { name: 'tools', title: 'System Tools', enabled: true, order: 7 },
            { name: 'windows', title: 'Windows', enabled: true, order: 8 },
            { name: 'dos', title: 'DOS', enabled: true, order: 9 },
            { name: 'unix', title: 'Unix', enabled: true, order: 10 },
          ],
        },
      })
    } catch (err: any) {
      showError(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.updateNetbootSettings(config)
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
        <h1 className="text-lg font-bold text-[var(--text-primary)]">Netboot 设置</h1>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
            <RefreshCw size={14} /> 重载配置
          </Button>
          <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
            <Save size={14} /> {saving ? '保存中...' : '保存设置'}
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
            <Toggle checked={config.enabled} onChange={v => setConfig({...config, enabled: v})} label="启用 OS 安装目录菜单" />
            <p className="text-xs text-[var(--text-muted)]">
              启用后，PXE 引导菜单将显示「[OS] 网络安装操作系统目录」选项，允许客户端从本地或远程引导文件安装操作系统。
              可在「OS 安装目录」页面浏览所有可用发行版。
            </p>

            {/* 安装目录跳转 */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录跳转</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">无 Profile 且 Netboot 开启时跳转到系统安装目录的脚本行为。</p>
              <div className="flex flex-col gap-4">
                <Toggle checked={config.catalog_redirect.enabled}
                  onChange={v => setConfig({...config, catalog_redirect: {...config.catalog_redirect, enabled: v}})}
                  label="启用跳转" />
                <SettingsField label="目标 URL">
                  <SettingsInput value={config.catalog_redirect.target_url}
                    onChange={v => setConfig({...config, catalog_redirect: {...config.catalog_redirect, target_url: v}})}
                    className="font-mono" />
                </SettingsField>
                <p className="text-xs text-[var(--text-muted)] -mt-3">支持 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.URL}}`}</code> 变量替换为服务器地址。</p>
                <Toggle checked={config.catalog_redirect.detect_arch}
                  onChange={v => setConfig({...config, catalog_redirect: {...config.catalog_redirect, detect_arch: v}})}
                  label="自动检测架构（arch/platform）" />
                <SettingsField label="前置脚本（跳转前执行）">
                  <textarea rows={4} spellCheck={false}
                    className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all"
                    value={config.catalog_redirect.preamble}
                    onChange={e => setConfig({...config, catalog_redirect: {...config.catalog_redirect, preamble: e.target.value}})}
                    placeholder="# 可选：在跳转前执行 dhcp、设置变量等" />
                </SettingsField>
              </div>
            </div>

            {/* 安装目录菜单结构 */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录菜单结构</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">控制 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">/netboot/menu.ipxe</code> 的标题和分组顺序。</p>
              <SettingsField label="菜单标题">
                <SettingsInput value={config.catalog_display.title}
                  onChange={v => setConfig({...config, catalog_display: {...config.catalog_display, title: v}})} />
              </SettingsField>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-2 mt-4">分组列表（拖拽排序）</label>
              <div className="space-y-1">
                {[...config.catalog_display.groups]
                  .sort((a: any, b: any) => a.order - b.order)
                  .map((g: any, i: number) => (
                  <div key={g.name}
                    draggable
                    onDragStart={e => e.dataTransfer.setData('text/plain', String(i))}
                    onDragOver={e => e.preventDefault()}
                    onDrop={e => {
                      e.preventDefault()
                      const fromIdx = parseInt(e.dataTransfer.getData('text/plain'))
                      const groups = [...config.catalog_display.groups]
                      const sorted = groups.sort((a: any, b: any) => a.order - b.order)
                      const [moved] = sorted.splice(fromIdx, 1)
                      sorted.splice(i, 0, moved)
                      const reindexed = sorted.map((g: any, idx: number) => ({...g, order: idx + 1}))
                      setConfig({...config, catalog_display: {...config.catalog_display, groups: reindexed}})
                    }}
                    className="flex items-center gap-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2 cursor-grab active:cursor-grabbing">
                    <span className="text-[var(--text-muted)] cursor-grab">⠿</span>
                    <span className="text-xs font-mono text-[var(--text-muted)] w-16">{g.name}</span>
                    <input className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                      value={g.title}
                      onChange={e => {
                        const groups = config.catalog_display.groups.map((g2: any) =>
                          g2.name === g.name ? {...g2, title: e.target.value} : g2)
                        setConfig({...config, catalog_display: {...config.catalog_display, groups}})
                      }} />
                    <input type="checkbox" checked={g.enabled}
                      onChange={e => {
                        const groups = config.catalog_display.groups.map((g2: any) =>
                          g2.name === g.name ? {...g2, enabled: e.target.checked} : g2)
                        setConfig({...config, catalog_display: {...config.catalog_display, groups}})
                      }}
                      className="rounded border-[var(--bg-border)]" title="启用/禁用" />
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
      </Card>
    </div>
  )
}
