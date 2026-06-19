import { useState, useEffect, useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Copy, Check, RotateCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { api, setAuthToken, type SettingsData, type InterfaceInfo, type InterfaceSettings, type MenuEntry } from '../api/client'

type Tab = 'general' | 'interfaces' | 'dhcp' | 'tftp' | 'dns' | 'http' | 'netboot'

interface InterfaceConfig {
  name: string; ip: string; dhcpMode: string; bootloader: string; chainToIPXE: boolean; subnet: string
  pools: string[]
  gateway: string; dnsServers: string; leaseTime: string; nextServer: string
  tftp: boolean; http: boolean; dns: boolean
}

const defaultIface: InterfaceConfig = {
  name: '', ip: '', dhcpMode: 'full', bootloader: 'ipxe', chainToIPXE: false, subnet: '',
  pools: [''],
  gateway: '', dnsServers: '8.8.8.8', leaseTime: '3600', nextServer: '',
  tftp: true, http: true, dns: false,
}

function validateIP(ip: string): boolean {
  if (!ip) return true
  const parts = ip.split('.')
  return parts.length === 4 && parts.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  })
}

function validateCIDR(cidr: string): boolean {
  if (!cidr) return true
  const parts = cidr.split('/')
  if (parts.length !== 2) return false
  const ips = parts[0].split('.')
  const mask = parseInt(parts[1])
  return ips.length === 4 && ips.every(p => {
    const n = parseInt(p)
    return n >= 0 && n <= 255 && String(n) === p
  }) && !isNaN(mask) && mask >= 0 && mask <= 32
}

function validatePort(n: number): boolean {
  return !isNaN(n) && n >= 1 && n <= 65535
}

function ipToInt(ip: string): number {
  const parts = ip.trim().split('.')
  if (parts.length !== 4) return NaN
  return parts.reduce((acc, oct) => {
    const n = parseInt(oct)
    if (isNaN(n) || n < 0 || n > 255) return NaN
    return (acc << 8) + n
  }, 0) >>> 0
}

function ipInCIDR(ip: string, cidr: string): boolean {
  const [netIP, maskStr] = cidr.split('/')
  const mask = parseInt(maskStr)
  if (isNaN(mask) || mask < 0 || mask > 32) return false
  const ipInt = ipToInt(ip)
  const netInt = ipToInt(netIP)
  if (isNaN(ipInt) || isNaN(netInt)) return false
  const maskInt = mask === 0 ? 0 : (~0 << (32 - mask)) >>> 0
  return (ipInt & maskInt) === (netInt & maskInt)
}

function poolsOverlap(a: string, b: string): boolean {
  const partsA = a.split('-').map(s => ipToInt(s.trim()))
  const partsB = b.split('-').map(s => ipToInt(s.trim()))
  if (partsA.length < 2 || partsB.length < 2) return false
  const [aStart, aEnd] = partsA
  const [bStart, bEnd] = partsB
  if (isNaN(aStart) || isNaN(aEnd) || isNaN(bStart) || isNaN(bEnd)) return false
  return aStart <= bEnd && bStart <= aEnd
}

export default function Settings() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [activeTab, setActiveTab] = useState<Tab>('general')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [tokenCopied, setTokenCopied] = useState(false)
  const [availableIfaces, setAvailableIfaces] = useState<InterfaceInfo[]>([])
  const [existingBootFiles, setExistingBootFiles] = useState<string[]>([])
  const [config, setConfig] = useState({
    serverName: '', logLevel: 'info', dataDir: '', mode: 'server',
    listenAddr: ':8080', authToken: '',
    autoOpen: true, persistEvents: true,
    interfaces: [{ ...defaultIface }],
    dhcpEnabled: true, dhcpRange: '', dhcpGateway: '', dhcpSubnet: '',
    dhcpDns: '', dhcpLeaseTime: '3600',
    tftpEnabled: true, tftpPort: '69', tftpRoot: '',
    dnsEnabled: false, dnsPort: '53', dnsUpstream: '8.8.8.8:53',
    httpPort: '8080', httpBootDir: '',
    netbootEnabled: true,
    netbootScriptTemplate: "",
    boot: {
      default_menu: { title: 'PxeGo Boot Menu', timeout: 5000, default: 0, entries: [] as MenuEntry[] },
      profile_behavior: { append_local: true, append_netboot: true, append_position: 'last' as 'last' | 'first' },
      catalog_redirect: { enabled: true, target_url: 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}', detect_arch: true, preamble: '' },
      catalog_display: {
        title: '[OS] Netboot OS Install Catalog',
        groups: [
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
    },
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const [res, ifaceRes] = await Promise.all([api.getSettings(), api.getInterfaces()])
      setAvailableIfaces(ifaceRes.data)
      const d = res.data
      if (d.server.token) setAuthToken(d.server.token)
      setConfig(prev => ({
        ...prev,
        serverName: d.server.name || 'pxego',
        logLevel: d.log_level || prev.logLevel,
        dataDir: d.data_dir || prev.dataDir,
        authToken: d.server.token || prev.authToken,
        autoOpen: d.server.app_mode ?? prev.autoOpen,
        dhcpEnabled: d.dhcp.enabled ?? prev.dhcpEnabled,
        dhcpRange: d.dhcp.range || prev.dhcpRange,
        dhcpGateway: d.dhcp.gateway || prev.dhcpGateway,
        dhcpSubnet: d.dhcp.subnet || prev.dhcpSubnet,
        dhcpDns: d.dhcp.dns_servers || prev.dhcpDns,
        dhcpLeaseTime: String(d.dhcp.lease_time || 3600),
        tftpEnabled: d.tftp.enabled ?? prev.tftpEnabled,
        tftpPort: String(d.tftp.port > 0 ? d.tftp.port : 69),
        tftpRoot: d.tftp.root || prev.tftpRoot,
        dnsEnabled: d.dns.enabled ?? prev.dnsEnabled,
        dnsPort: String(d.dns.port > 0 ? d.dns.port : 53),
        dnsUpstream: d.dns.upstream || prev.dnsUpstream,
        httpPort: String(d.http.port > 0 ? d.http.port : 8080),
        httpBootDir: d.http.boot_dir || prev.httpBootDir,
        netbootEnabled: d.netboot?.enabled ?? prev.netbootEnabled,
                netbootScriptTemplate: d.netboot?.script_template ?? prev.netbootScriptTemplate,
        boot: d.netboot?.boot ? {
          default_menu: {
            title: d.netboot.boot.default_menu?.title || prev.boot.default_menu.title,
            timeout: d.netboot.boot.default_menu?.timeout ?? prev.boot.default_menu.timeout,
            default: d.netboot.boot.default_menu?.default ?? prev.boot.default_menu.default,
            entries: d.netboot.boot.default_menu?.entries || [],
          },
          profile_behavior: {
            append_local: d.netboot.boot.profile_behavior?.append_local ?? true,
            append_netboot: d.netboot.boot.profile_behavior?.append_netboot ?? true,
            append_position: d.netboot.boot.profile_behavior?.append_position || 'last',
          },
          catalog_redirect: {
            enabled: d.netboot.boot.catalog_redirect?.enabled ?? true,
            target_url: d.netboot.boot.catalog_redirect?.target_url || 'http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}',
            detect_arch: d.netboot.boot.catalog_redirect?.detect_arch ?? true,
            preamble: d.netboot.boot.catalog_redirect?.preamble || '',
          },
          catalog_display: {
            title: d.netboot.boot.catalog_display?.title || prev.boot.catalog_display.title,
            groups: d.netboot.boot.catalog_display?.groups || prev.boot.catalog_display.groups,
          },
        } : prev.boot,
      }))
      // 加载接口配置
      if (d.interfaces && d.interfaces.length > 0) {
        setConfig(prev => ({
          ...prev,
          interfaces: d.interfaces.map((ir: InterfaceSettings) => ({
            name: ir.name || '',
            ip: ir.ip || '',
            dhcpMode: ir.dhcp_mode || 'full',
            bootloader: ir.bootloader || 'ipxe',
            chainToIPXE: ir.chain_to_ipxe || false,
            subnet: ir.subnet || '',
            pools: ir.pools && ir.pools.length > 0 ? ir.pools : [''],
            gateway: ir.gateway || '',
            dnsServers: ir.dns_servers || '8.8.8.8',
            leaseTime: String(ir.lease_time || 3600),
            nextServer: ir.next_server || '',
            tftp: ir.tftp ?? true,
            http: ir.http ?? true,
            dns: ir.dns ?? false,
          })),
        }))
      }

      // 加载启动文件列表（用于 TFTP 架构映射显示）
      try {
        const fileRes = await api.getFiles('')
        if (fileRes.data) {
          setExistingBootFiles(fileRes.data.map((f: any) => f.name))
        }
      } catch {}

    } catch (err: any) {
      showError(err.message || '加载设置失败')
    } finally {
      setLoading(false)
    }
  }

  function validateForm(): string[] {
    const errs: string[] = []
    for (let i = 0; i < config.interfaces.length; i++) {
      const iface = config.interfaces[i]
      if (!iface.name) continue
      if (iface.dhcpMode === 'full' || iface.dhcpMode === 'hybrid') {
        if (iface.subnet && !validateCIDR(iface.subnet)) {
          errs.push(`接口 #${i + 1}: 子网格式无效（如 192.168.1.0/24）`)
        }
        for (let pi = 0; pi < iface.pools.length; pi++) {
          const pool = iface.pools[pi]
          if (!pool) continue
          const parts = pool.split('-')
          if (parts.length !== 2 || !validateIP(parts[0].trim()) || !validateIP(parts[1].trim())) {
            errs.push(`接口 #${i + 1}: 地址池 #${pi + 1} 格式无效（如 192.168.1.100-192.168.1.200）`)
            continue
          }
          // 校验地址池是否属于对应子网
          if (iface.subnet) {
            const startIP = parts[0].trim()
            const endIP = parts[1].trim()
            if (!ipInCIDR(startIP, iface.subnet)) {
              errs.push(`接口 #${i + 1}: 地址池 #${pi + 1} 起始地址 ${startIP} 不属于子网 ${iface.subnet}`)
            }
            if (!ipInCIDR(endIP, iface.subnet)) {
              errs.push(`接口 #${i + 1}: 地址池 #${pi + 1} 结束地址 ${endIP} 不属于子网 ${iface.subnet}`)
            }
          }
        }
        // 检测地址池之间是否冲突
        const validPools = iface.pools.filter(p => p && p.includes('-'))
        for (let pi = 0; pi < validPools.length; pi++) {
          for (let pj = pi + 1; pj < validPools.length; pj++) {
            if (poolsOverlap(validPools[pi], validPools[pj])) {
              errs.push(`接口 #${i + 1}: 地址池 #${pi + 1} 和 #${pj + 1} 范围冲突，请检查`)
            }
          }
        }
        if (iface.gateway && !validateIP(iface.gateway)) {
          errs.push(`接口 #${i + 1}: 网关地址格式无效`)
        }
      }
    }
    if (config.dhcpSubnet && !validateCIDR(config.dhcpSubnet)) {
      errs.push('DHCP 子网格式无效')
    }
    if (config.dhcpGateway && !validateIP(config.dhcpGateway)) {
      errs.push('DHCP 网关格式无效')
    }
    if (!validatePort(parseInt(config.tftpPort))) {
      errs.push('TFTP 端口号无效（1-65535）')
    }
    if (!validatePort(parseInt(config.dnsPort))) {
      errs.push('DNS 端口号无效（1-65535）')
    }
    if (!validatePort(parseInt(config.httpPort))) {
      errs.push('HTTP 端口号无效（1-65535）')
    }
    return errs
  }

  async function handleSave() {
    const errs = validateForm()
    if (errs.length > 0) {
      showError(errs.join('\n'))
      return
    }

    setSaving(true)
    try {
      const interfaces: InterfaceSettings[] = config.interfaces
        .filter(iface => iface.name)
        .map(iface => ({
          name: iface.name,
          ip: iface.ip,
          dhcp_mode: iface.dhcpMode,
          bootloader: iface.bootloader,
          chain_to_ipxe: iface.chainToIPXE,
          subnet: iface.subnet,
          pools: iface.pools.filter(p => p && p.includes('-')),
          gateway: iface.gateway,
          dns_servers: iface.dnsServers,
          lease_time: parseInt(iface.leaseTime) || 3600,
          next_server: iface.nextServer,
          tftp: iface.tftp,
          http: iface.http,
          dns: iface.dns,
        }))

      const data: SettingsData = {
        log_level: config.logLevel,
        data_dir: config.dataDir,
        server: {
          name: config.serverName,
          app_mode: config.autoOpen,
          token: config.authToken,
        },
        dhcp: {
          enabled: config.dhcpEnabled,
          range: config.dhcpRange,
          gateway: config.dhcpGateway,
          subnet: config.dhcpSubnet,
          lease_time: parseInt(config.dhcpLeaseTime) || 3600,
          dns_servers: config.dhcpDns,
        },
        tftp: {
          enabled: config.tftpEnabled,
          port: parseInt(config.tftpPort) || 69,
          root: config.tftpRoot,
        },
        dns: {
          enabled: config.dnsEnabled,
          port: parseInt(config.dnsPort) || 53,
          upstream: config.dnsUpstream,
        },
        http: {
          port: parseInt(config.httpPort) || 8080,
          boot_dir: config.httpBootDir,
        },
        netboot: {
          enabled: config.netbootEnabled,
          script_template: config.netbootScriptTemplate || undefined,
          boot: {
            default_menu: {
              title: config.boot.default_menu.title,
              timeout: config.boot.default_menu.timeout,
              default: config.boot.default_menu.default,
              entries: config.boot.default_menu.entries.filter(e => e.label),
            },
            profile_behavior: config.boot.profile_behavior,
            catalog_redirect: config.boot.catalog_redirect,
            catalog_display: {
              title: config.boot.catalog_display.title,
              groups: config.boot.catalog_display.groups,
            },
          },
        },
        interfaces,
      }
      await api.updateSettings(data)
      success(t('settings.saved'))
    } catch (err: any) {
      showError(err.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  function renderField(label: string, value: string, onChange: (v: string) => void, opts?: { type?: string; placeholder?: string; disabled?: boolean; error?: boolean }) {
    const errorBorder = opts?.error ? 'border-red-500/50 focus:border-red-500 focus:ring-red-500/10' : 'border-[var(--bg-border)] focus:border-blue-500 focus:ring-blue-500/10'
    const bgColor = opts?.disabled ? 'bg-[var(--bg-input)]' : 'bg-[var(--bg-elevated)]'
    const disabledStyle = opts?.disabled ? 'opacity-50 cursor-not-allowed' : ''
    return (
      <div className={`form-group ${disabledStyle}`}>
        <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{label}</label>
        <input
          type={opts?.type || 'text'}
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder={opts?.placeholder}
          disabled={opts?.disabled}
          className={`w-full ${bgColor} border rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none transition-all placeholder-[var(--text-muted)] ${errorBorder}`}
        />
      </div>
    )
  }

  const regenerateToken = useCallback(async () => {
    const newToken = Array.from({ length: 32 }, () => Math.random().toString(36)[2]).join('')
    setConfig(prev => ({ ...prev, authToken: newToken }))
  }, [])

  const copyToken = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(config.authToken)
      setTokenCopied(true)
      setTimeout(() => setTokenCopied(false), 2000)
    } catch {}
  }, [config.authToken])

  const tabs: { key: Tab; label: string }[] = [
    { key: 'general', label: t('settings.general') },
    { key: 'interfaces', label: '网络接口' },
    { key: 'dhcp', label: t('settings.dhcp') },
    { key: 'tftp', label: t('settings.tftp') },
    { key: 'dns', label: t('settings.dns') },
    { key: 'http', label: 'HTTP' },
    { key: 'netboot', label: 'Netboot' },
  ]

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div></div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
            <RefreshCw size={14} /> {t('common.reload', '重载配置')}
          </Button>
          <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
            <Save size={14} /> {saving ? '保存中...' : t('settings.save')}
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-[var(--bg-border)] mb-5">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-5 py-2.5 text-sm font-semibold transition-colors border-b-2 -mb-px ${
              activeTab === tab.key
                ? 'text-blue-400 border-blue-500'
                : 'text-[var(--text-muted)] border-transparent hover:text-[var(--text-secondary)]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <Card>
        {loading ? (
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-[var(--text-muted)]">加载中...</span>
          </div>
        ) : (
          <>
        {activeTab === 'general' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('common.serverName', '服务器名称'), config.serverName, v => setConfig({...config, serverName: v}))}
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('common.logLevel', '日志级别')}</label>
                <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={config.logLevel} onChange={e => setConfig({...config, logLevel: e.target.value})}>
                  <option>info</option>
                  <option>debug</option>
                  <option>warn</option>
                  <option>error</option>
                </select>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.dataDir'), config.dataDir, v => setConfig({...config, dataDir: v}))}
              <div>
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{t('common.mode', '运行模式')}</label>
                <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={config.mode} onChange={e => setConfig({...config, mode: e.target.value})}>
                  <option>server</option>
                  <option>app</option>
                </select>
              </div>
            </div>
            {renderField(t('settings.listenAddr', '管理接口监听地址'), config.listenAddr, v => setConfig({...config, listenAddr: v}))}

            {/* API 令牌 */}
            <div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">API 认证令牌</label>
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={config.authToken}
                  readOnly
                  className="flex-1 bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none select-all"
                />
                <button
                  onClick={copyToken}
                  className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
                  title="复制令牌"
                >
                  {tokenCopied ? <Check size={16} className="text-green-500" /> : <Copy size={16} />}
                </button>
                <button
                  onClick={regenerateToken}
                  className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] transition-colors text-[var(--text-secondary)]"
                  title="重新生成"
                >
                  <RotateCw size={16} />
                </button>
              </div>
              <p className="text-xs text-[var(--text-muted)] mt-1">用于 API 请求的身份验证。修改后需要在 HTTP 请求头中添加 Authorization: Bearer {config.authToken ? `<令牌>` : ''}</p>
            </div>

            <div className="flex flex-col gap-3 pt-2">
              <Toggle checked={config.autoOpen} onChange={v => setConfig({...config, autoOpen: v})} label={t('settings.autoOpen', '启动时自动打开浏览器（app 模式）')} />
              <Toggle checked={config.persistEvents} onChange={v => setConfig({...config, persistEvents: v})} label={t('settings.persistEvents', '启用事件日志持久化')} />
            </div>
          </div>
        )}

        {activeTab === 'interfaces' && (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <p className="text-sm text-[var(--text-secondary)]">配置 PxeGo 在哪些网络接口上提供服务。每个接口可独立启用 DHCP/TFTP/HTTP/DNS。</p>
            </div>
            {config.interfaces.map((iface, i) => {
              const isProxy = iface.dhcpMode === 'proxy'
              const isOff = iface.dhcpMode === 'off'
              const disableFields = isProxy || isOff
              return (
              <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold text-[var(--text-primary)]">接口 #{i + 1}</span>
                  {config.interfaces.length > 1 && (
                    <Button variant="ghost" size="sm" onClick={() => setConfig({...config, interfaces: config.interfaces.filter((_, j) => j !== i)})}>移除</Button>
                  )}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">接口名称</label>
                    <select
                      className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none"
                      value={iface.name}
                      onChange={e => {
                        const sel = availableIfaces.find(x => x.name === e.target.value)
                        const next = [...config.interfaces]
                        const pool = sel?.ipv4?.[0] || ''
                        next[i] = {...next[i], name: e.target.value, ip: pool, subnet: pool ? pool.replace(/\.\d+$/, '.0/24') : next[i].subnet}
                        setConfig({...config, interfaces: next})
                      }}
                    >
                      <option value="">-- 选择网卡 --</option>
                      {availableIfaces.map(ai => (
                        <option key={ai.name} value={ai.name}>
                          {ai.name} {ai.ipv4?.length ? `(${ai.ipv4[0]})` : ''} {!ai.up ? '[未连接]' : ''}
                        </option>
                      ))}
                    </select>
                  </div>
                  {renderField('IP 地址', iface.ip, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], ip: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.100' })}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">DHCP 模式</label>
                    <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={iface.dhcpMode} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], dhcpMode: e.target.value}; setConfig({...config, interfaces: next})
                    }}>
                      <option value="full">full（完整 DHCP）</option>
                      <option value="proxy">proxy（代理 DHCP）</option>
                      <option value="hybrid">hybrid（混合）</option>
                      <option value="off">off（关闭）</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">引导加载器</label>
                    <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={iface.bootloader} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], bootloader: e.target.value}; setConfig({...config, interfaces: next})
                    }}>
                      <option value="ipxe">iPXE（默认）</option>
                      <option value="pxelinux">PXELinux</option>
                      <option value="grub2">GRUB2</option>
                    </select>
                  </div>
                  {renderField('子网', iface.subnet, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], subnet: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.0/24', disabled: disableFields })}
                </div>
                <div className="flex items-center gap-3 pt-2">
                  <label className="flex items-center gap-2 text-sm text-[var(--text-secondary)] cursor-pointer">
                    <input type="checkbox" checked={iface.chainToIPXE} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], chainToIPXE: e.target.checked}; setConfig({...config, interfaces: next})
                    }} disabled={iface.bootloader !== 'pxelinux' && iface.bootloader !== 'grub2'} className="rounded border-[var(--bg-border)]" />
                    <span className={iface.bootloader !== 'pxelinux' && iface.bootloader !== 'grub2' ? 'opacity-40' : ''}>链式加载到 iPXE</span>
                  </label>
                  {(iface.bootloader === 'pxelinux' || iface.bootloader === 'grub2') && iface.chainToIPXE && (
                    <span className="text-xs text-blue-400">iPXE 将接管后续引导流程</span>
                  )}
                </div>
                {disableFields && (
                  <p className="text-xs text-[var(--text-muted)] italic">
                    {isOff ? 'DHCP 已关闭，无需配置子网、地址池等信息。' : '代理 DHCP 模式不负责 IP 地址分配，子网/地址池/网关等字段不需要配置。'}
                  </p>
                )}
                {!disableFields && (
                <div className="space-y-2">
                  <label className="block text-xs font-semibold text-[var(--text-secondary)]">地址池（多个范围用 + 添加）</label>
                  {iface.pools.map((pool, pi) => (
                    <div key={pi} className="flex items-center gap-2">
                      <input
                        type="text"
                        value={pool}
                        onChange={e => {
                          const next = [...config.interfaces]
                          const np = [...next[i].pools]
                          np[pi] = e.target.value
                          next[i] = {...next[i], pools: np}
                          setConfig({...config, interfaces: next})
                        }}
                        placeholder="192.168.1.100-192.168.1.200"
                        className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all placeholder-[var(--text-muted)]"
                      />
                      <button
                        onClick={() => {
                          const next = [...config.interfaces]
                          next[i] = {...next[i], pools: next[i].pools.filter((_, j) => j !== pi)}
                          setConfig({...config, interfaces: next})
                        }}
                        className="p-2 rounded-lg border border-[var(--bg-border)] hover:bg-[var(--bg-hover)] text-[var(--text-muted)] hover:text-red-400 transition-colors text-xs font-bold"
                        title="移除"
                      >✕</button>
                    </div>
                  ))}
                  <button
                    onClick={() => {
                      const next = [...config.interfaces]
                      next[i] = {...next[i], pools: [...next[i].pools, '']}
                      setConfig({...config, interfaces: next})
                    }}
                    className="text-xs text-blue-400 hover:text-blue-300 transition-colors"
                  >+ 添加地址范围</button>
                </div>
                )}
                {!disableFields && (
                <div className="grid grid-cols-2 gap-4">
                  {renderField('租约时间（秒）', iface.leaseTime, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], leaseTime: v}; setConfig({...config, interfaces: next})
                  })}
                  {renderField('网关', iface.gateway, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], gateway: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.1' })}
                </div>
                )}
                {!disableFields && (
                <div className="grid grid-cols-2 gap-4">
                  {renderField('DNS 服务器', iface.dnsServers, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], dnsServers: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '8.8.8.8' })}
                  {renderField('Next Server (TFTP 服务器)', iface.nextServer, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], nextServer: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '同 IP 地址时留空', disabled: disableFields })}
                </div>
                )}
                <div className="flex items-center gap-6 pt-2 border-t border-[var(--bg-border)]">
                  <label className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                    <input type="checkbox" checked={iface.tftp} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], tftp: e.target.checked}; setConfig({...config, interfaces: next})
                    }} className="rounded border-[var(--bg-border)]" /> TFTP
                  </label>
                  <label className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                    <input type="checkbox" checked={iface.http} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], http: e.target.checked}; setConfig({...config, interfaces: next})
                    }} className="rounded border-[var(--bg-border)]" /> HTTP
                  </label>
                  <label className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
                    <input type="checkbox" checked={iface.dns} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], dns: e.target.checked}; setConfig({...config, interfaces: next})
                    }} className="rounded border-[var(--bg-border)]" /> DNS
                  </label>
                </div>
              </div>
            )})}
            <Button variant="secondary" size="sm" onClick={() => setConfig({...config, interfaces: [...config.interfaces, { ...defaultIface }]})}>
              添加接口
            </Button>
          </div>
        )}

        {activeTab === 'dhcp' && (
          <div className="space-y-4">
            <p className="text-sm text-[var(--text-secondary)]">此页签为兼容旧版单接口配置保留。推荐在网络接口页签中为每个接口单独配置。</p>
            <Toggle checked={config.dhcpEnabled} onChange={v => setConfig({...config, dhcpEnabled: v})} label={t('settings.enabled') + ' DHCP'} />
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.dhcpRange', 'DHCP 地址池'), config.dhcpRange, v => setConfig({...config, dhcpRange: v}))}
              {renderField(t('settings.dhcpLeaseTime', '租约时间'), config.dhcpLeaseTime, v => setConfig({...config, dhcpLeaseTime: v}))}
            </div>
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.dhcpGateway', '网关'), config.dhcpGateway, v => setConfig({...config, dhcpGateway: v}))}
              {renderField(t('settings.dhcpSubnet', '子网掩码'), config.dhcpSubnet, v => setConfig({...config, dhcpSubnet: v}))}
            </div>
            {renderField('DNS', config.dhcpDns, v => setConfig({...config, dhcpDns: v}))}
          </div>
        )}

        {activeTab === 'tftp' && (
          <div className="space-y-4">
            <Toggle checked={config.tftpEnabled} onChange={v => setConfig({...config, tftpEnabled: v})} label={t('settings.enabled') + ' TFTP'} />
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.port'), config.tftpPort, v => setConfig({...config, tftpPort: v}))}
              {renderField(t('settings.tftpRoot', '根目录'), config.tftpRoot, v => setConfig({...config, tftpRoot: v}))}
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
              <p className="text-xs text-[var(--text-muted)] mt-2">可在「文件管理」页面上传缺失的引导文件到 {config.tftpRoot || '启动目录'}。</p>
            </div>
          </div>
        )}

        {activeTab === 'dns' && (
          <div className="space-y-4">
            <Toggle checked={config.dnsEnabled} onChange={v => setConfig({...config, dnsEnabled: v})} label={t('settings.enabled') + ' DNS'} />
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.port'), config.dnsPort, v => setConfig({...config, dnsPort: v}))}
              {renderField(t('settings.dnsUpstream', '上游 DNS'), config.dnsUpstream, v => setConfig({...config, dnsUpstream: v}))}
            </div>
          </div>
        )}

        {activeTab === 'http' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('settings.port'), config.httpPort, v => setConfig({...config, httpPort: v}))}
              {renderField(t('settings.httpBootDir', '启动文件目录'), config.httpBootDir, v => setConfig({...config, httpBootDir: v}))}
            </div>
          </div>
        )}

        {activeTab === 'netboot' && (
          <div className="space-y-4">
            <Toggle checked={config.netbootEnabled} onChange={v => setConfig({...config, netbootEnabled: v})} label="启用 OS 安装目录菜单" />
            <p className="text-xs text-[var(--text-muted)]">
              启用后，PXE 引导菜单将显示「[OS] 网络安装操作系统目录」选项，允许客户端从本地或远程引导文件安装操作系统。
              可在「OS 安装目录」页面浏览所有可用发行版。
            </p>

            {/* ── 自定义 iPXE 脚本（逃生口） ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">自定义 iPXE 脚本</label>
              <p className="text-xs text-[var(--text-muted)] mb-3">留空则使用下方可视化配置的引导逻辑。自定义脚本将完全替代默认菜单。可用模板变量：<code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.URL}}`}</code>（服务器地址）、<code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.MAC}}`}</code>（客户端 MAC）。</p>
              <textarea
                value={config.netbootScriptTemplate}
                onChange={e => setConfig({...config, netbootScriptTemplate: e.target.value})}
                rows={6}
                spellCheck={false}
                className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all placeholder-[var(--text-muted)]"
                placeholder={"#!ipxe\nmenu PxeGo Boot Menu\nitem shell iPXE Shell\nitem local Boot Local Disk\n\nchoose selected || shell\ngoto ${selected}"}
              />
            </div>

            {/* ── 默认引导菜单 ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">默认引导菜单</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">无关联 Profile 且 Netboot 未开启时，客户端看到此菜单。</p>
              <div className="grid grid-cols-2 gap-4 mb-4">
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">菜单标题</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
                    value={config.boot.default_menu.title}
                    onChange={e => { setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, title: e.target.value}}}) }} />
                </div>
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">超时时间（秒）</label>
                  <input type="number" className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
                    value={config.boot.default_menu.timeout}
                    onChange={e => { setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, timeout: parseInt(e.target.value) || 0}}}) }} />
                </div>
              </div>
              {/* 条目编辑器 */}
              <div className="space-y-2">
                <label className="block text-xs font-semibold text-[var(--text-secondary)]">菜单条目</label>
                {config.boot.default_menu.entries.map((entry: MenuEntry, i: number) => (
                  <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg p-3">
                    <div className="grid grid-cols-2 gap-3 mb-2">
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                        value={entry.label} placeholder="标签"
                        onChange={e => {
                          const entries = [...config.boot.default_menu.entries]
                          entries[i] = {...entries[i], label: e.target.value}
                          setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                        }} />
                      <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none"
                        value={entry.type}
                        onChange={e => {
                          const entries = [...config.boot.default_menu.entries]
                          entries[i] = {...entries[i], type: e.target.value as any}
                          setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                        }}>
                        <option value="local">local</option>
                        <option value="direct">direct</option>
                        <option value="chain">chain</option>
                        <option value="sanboot">sanboot</option>
                        <option value="wds">wds</option>
                      </select>
                    </div>
                    {entry.type === 'direct' && (
                      <div className="space-y-2">
                        <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                          value={entry.kernel || ''} placeholder="kernel 路径"
                          onChange={e => {
                            const entries = [...config.boot.default_menu.entries]
                            entries[i] = {...entries[i], kernel: e.target.value}
                            setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                          }} />
                        <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                          value={entry.initrd || ''} placeholder="initrd 路径"
                          onChange={e => {
                            const entries = [...config.boot.default_menu.entries]
                            entries[i] = {...entries[i], initrd: e.target.value}
                            setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                          }} />
                        <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                          value={entry.cmdline || ''} placeholder="cmdline 参数"
                          onChange={e => {
                            const entries = [...config.boot.default_menu.entries]
                            entries[i] = {...entries[i], cmdline: e.target.value}
                            setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                          }} />
                      </div>
                    )}
                    {entry.type === 'chain' && (
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                        value={entry.url || ''} placeholder="chain URL"
                        onChange={e => {
                          const entries = [...config.boot.default_menu.entries]
                          entries[i] = {...entries[i], url: e.target.value}
                          setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                        }} />
                    )}
                    {entry.type === 'wds' && (
                      <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2.5 py-1.5 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                        value={entry.wim || ''} placeholder="WIM 路径"
                        onChange={e => {
                          const entries = [...config.boot.default_menu.entries]
                          entries[i] = {...entries[i], wim: e.target.value}
                          setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries}}})
                        }} />
                    )}
                    <button className="mt-2 text-xs text-red-400 hover:text-red-300"
                      onClick={() => { setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries: config.boot.default_menu.entries.filter((_: any, j: number) => j !== i)}}}) }}>删除</button>
                  </div>
                ))}
                <button className="text-xs text-blue-400 hover:text-blue-300"
                  onClick={() => { setConfig({...config, boot: {...config.boot, default_menu: {...config.boot.default_menu, entries: [...config.boot.default_menu.entries, {label: '', type: 'local'}]}}}) }}>+ 添加条目</button>
              </div>
            </div>

            {/* ── Profile 菜单行为 ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">Profile 菜单行为</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">控制有 Profile 的主机在 iPXE 菜单中自动添加的条目。</p>
              <div className="flex flex-col gap-3">
                <Toggle checked={config.boot.profile_behavior.append_local}
                  onChange={v => { setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_local: v}}}) }}
                  label="追加「从本地硬盘启动」" />
                <Toggle checked={config.boot.profile_behavior.append_netboot}
                  onChange={v => { setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_netboot: v}}}) }}
                  label="追加「OS 安装目录」" />
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">追加位置</label>
                  <select className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none appearance-none"
                    value={config.boot.profile_behavior.append_position}
                    onChange={e => { setConfig({...config, boot: {...config.boot, profile_behavior: {...config.boot.profile_behavior, append_position: e.target.value as any}}}) }}>
                    <option value="last">菜单末尾</option>
                    <option value="first">菜单开头</option>
                  </select>
                </div>
              </div>
            </div>

            {/* ── 安装目录跳转 ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录跳转</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">无 Profile 且 Netboot 开启时跳转到系统安装目录的脚本行为。</p>
              <div className="flex flex-col gap-4">
                <Toggle checked={config.boot.catalog_redirect.enabled}
                  onChange={v => { setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, enabled: v}}}) }}
                  label="启用跳转" />
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">目标 URL</label>
                  <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] font-mono outline-none focus:border-blue-500"
                    value={config.boot.catalog_redirect.target_url}
                    onChange={e => { setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, target_url: e.target.value}}}) }} />
                  <p className="text-xs text-[var(--text-muted)] mt-1">支持 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">{`{{.URL}}`}</code> 变量替换为服务器地址。</p>
                </div>
                <Toggle checked={config.boot.catalog_redirect.detect_arch}
                  onChange={v => { setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, detect_arch: v}}}) }}
                  label="自动检测架构（arch/platform）" />
                <div>
                  <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">前置脚本（跳转前执行）</label>
                  <textarea rows={4} spellCheck={false}
                    className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-4 py-3 text-sm font-mono text-[var(--text-primary)] outline-none focus:border-blue-500"
                    value={config.boot.catalog_redirect.preamble}
                    onChange={e => { setConfig({...config, boot: {...config.boot, catalog_redirect: {...config.boot.catalog_redirect, preamble: e.target.value}}}) }} 
                    placeholder="# 可选：在跳转前执行 dhcp、设置变量等" />
                </div>
              </div>
            </div>

            {/* ── 安装目录菜单结构 ── */}
            <div className="pt-4 border-t border-[var(--bg-border)]">
              <h3 className="text-sm font-semibold text-[var(--text-primary)] mb-3">安装目录菜单结构</h3>
              <p className="text-xs text-[var(--text-muted)] mb-3">控制 <code className="text-[10px] bg-[var(--bg-card)] px-1 py-0.5 rounded font-mono">/netboot/menu.ipxe</code> 的标题和分组顺序。</p>
              <div className="mb-4">
                <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">菜单标题</label>
                <input className="w-full bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500"
                  value={config.boot.catalog_display.title}
                  onChange={e => { setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, title: e.target.value}}}) }} />
              </div>
              <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-2">分组列表（拖拽排序）</label>
              <div className="space-y-1">
                {[...config.boot.catalog_display.groups]
                  .sort((a: any, b: any) => a.order - b.order)
                  .map((g: any, i: number) => (
                  <div key={g.name}
                    draggable
                    onDragStart={e => e.dataTransfer.setData('text/plain', String(i))}
                    onDragOver={e => e.preventDefault()}
                    onDrop={e => {
                      e.preventDefault()
                      const fromIdx = parseInt(e.dataTransfer.getData('text/plain'))
                      const groups = [...config.boot.catalog_display.groups]
                      const sorted = groups.sort((a: any, b: any) => a.order - b.order)
                      const [moved] = sorted.splice(fromIdx, 1)
                      sorted.splice(i, 0, moved)
                      const reindexed = sorted.map((g: any, idx: number) => ({...g, order: idx + 1}))
                      setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups: reindexed}}})
                    }}
                    className="flex items-center gap-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2 cursor-grab active:cursor-grabbing">
                    <span className="text-[var(--text-muted)] cursor-grab">⠿</span>
                    <span className="text-xs font-mono text-[var(--text-muted)] w-16">{g.name}</span>
                    <input className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded px-2 py-1 text-xs text-[var(--text-primary)] outline-none focus:border-blue-500"
                      value={g.title}
                      onChange={e => {
                        const groups = config.boot.catalog_display.groups.map((g2: any) =>
                          g2.name === g.name ? {...g2, title: e.target.value} : g2)
                        setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups}}})
                      }} />
                    <input type="checkbox" checked={g.enabled}
                      onChange={e => {
                        const groups = config.boot.catalog_display.groups.map((g2: any) =>
                          g2.name === g.name ? {...g2, enabled: e.target.checked} : g2)
                        setConfig({...config, boot: {...config.boot, catalog_display: {...config.boot.catalog_display, groups}}})
                      }}
                      className="rounded border-[var(--bg-border)]" title="启用/禁用" />
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}
        </>
      )}
      </Card>
    </div>
  )
}
