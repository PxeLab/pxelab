import { useState, useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { api, type SettingsData } from '../api/client'

type Tab = 'general' | 'interfaces' | 'dhcp' | 'tftp' | 'dns' | 'http' | 'ipmi'

interface InterfaceConfig {
  name: string; ip: string; dhcpMode: string; subnet: string; pool: string
  gateway: string; dnsServers: string; leaseTime: string; nextServer: string
  tftp: boolean; http: boolean; dns: boolean
}

const defaultIface: InterfaceConfig = {
  name: '', ip: '', dhcpMode: 'full', subnet: '', pool: '',
  gateway: '', dnsServers: '8.8.8.8', leaseTime: '86400', nextServer: '',
  tftp: true, http: true, dns: false,
}

export default function Settings() {
  const { t } = useTranslation()
  const { success, error } = useToast()
  const [activeTab, setActiveTab] = useState<Tab>('general')
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState({
    serverName: '', logLevel: 'info', dataDir: '', mode: 'server',
    listenAddr: ':8080', authToken: '',
    autoOpen: true, persistEvents: true,
    interfaces: [{ ...defaultIface }],
    dhcpEnabled: true, dhcpRange: '', dhcpGateway: '', dhcpSubnet: '',
    dhcpDns: '', dhcpLeaseTime: '86400',
    tftpEnabled: true, tftpPort: '69', tftpRoot: '',
    dnsEnabled: false, dnsPort: '53', dnsUpstream: '8.8.8.8:53',
    httpPort: '80', httpBootDir: '',
    ipmiEnabled: false, ipmiTimeout: '5',
  })

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getSettings()
      const d = res.data
      setConfig(prev => ({
        ...prev,
        serverName: d.server.name || prev.serverName,
        logLevel: d.log_level || prev.logLevel,
        dataDir: d.data_dir || prev.dataDir,
        authToken: d.server.token || prev.authToken,
        autoOpen: d.server.app_mode ?? prev.autoOpen,
        dhcpEnabled: d.dhcp.enabled ?? prev.dhcpEnabled,
        dhcpRange: d.dhcp.range || prev.dhcpRange,
        dhcpGateway: d.dhcp.gateway || prev.dhcpGateway,
        dhcpSubnet: d.dhcp.subnet || prev.dhcpSubnet,
        dhcpDns: d.dhcp.dns_servers || prev.dhcpDns,
        dhcpLeaseTime: String(d.dhcp.lease_time) || prev.dhcpLeaseTime,
        tftpEnabled: d.tftp.enabled ?? prev.tftpEnabled,
        tftpPort: String(d.tftp.port) || prev.tftpPort,
        tftpRoot: d.tftp.root || prev.tftpRoot,
        dnsEnabled: d.dns.enabled ?? prev.dnsEnabled,
        dnsPort: String(d.dns.port) || prev.dnsPort,
        dnsUpstream: d.dns.upstream || prev.dnsUpstream,
        httpPort: String(d.http.port) || prev.httpPort,
        httpBootDir: d.http.boot_dir || prev.httpBootDir,
        ipmiEnabled: d.ipmi.enabled ?? prev.ipmiEnabled,
        ipmiTimeout: String(d.ipmi.timeout) || prev.ipmiTimeout,
      }))
    } catch (err: any) {
      error(err.message || '加载设置失败')
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
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
          lease_time: parseInt(config.dhcpLeaseTime) || 86400,
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
          port: parseInt(config.httpPort) || 80,
          boot_dir: config.httpBootDir,
        },
        ipmi: {
          enabled: config.ipmiEnabled,
          timeout: parseInt(config.ipmiTimeout) || 5,
        },
      }
      await api.updateSettings(data)
      success(t('settings.saved'))
    } catch (err: any) {
      error(err.message || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  function renderField(label: string, value: string, onChange: (v: string) => void, opts?: { type?: string; placeholder?: string }) {
    return (
      <div className="form-group">
        <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1.5">{label}</label>
        <input
          type={opts?.type || 'text'}
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder={opts?.placeholder}
          className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all placeholder-[var(--text-muted)]"
        />
      </div>
    )
  }

  const tabs: { key: Tab; label: string }[] = [
    { key: 'general', label: t('settings.general') },
    { key: 'interfaces', label: '网络接口' },
    { key: 'dhcp', label: t('settings.dhcp') },
    { key: 'tftp', label: t('settings.tftp') },
    { key: 'dns', label: t('settings.dns') },
    { key: 'http', label: 'HTTP' },
    { key: 'ipmi', label: 'IPMI' },
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
                <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={config.logLevel} onChange={e => setConfig({...config, logLevel: e.target.value})}>
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
                <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={config.mode} onChange={e => setConfig({...config, mode: e.target.value})}>
                  <option>server</option>
                  <option>app</option>
                </select>
              </div>
            </div>
            {renderField(t('settings.listenAddr', '管理接口监听地址'), config.listenAddr, v => setConfig({...config, listenAddr: v}))}
            {renderField(t('settings.authToken', 'API 认证令牌'), config.authToken, v => setConfig({...config, authToken: v}), { type: 'password' })}
            <div className="flex flex-col gap-3 pt-2">
              <Toggle checked={config.autoOpen} onChange={v => setConfig({...config, autoOpen: v})} label={t('settings.autoOpen', '启动时自动打开浏览器（app 模式）')} />
              <Toggle checked={config.persistEvents} onChange={v => setConfig({...config, persistEvents: v})} label={t('settings.persistEvents', '启用事件日志持久化')} />
            </div>
          </div>
        )}

        {activeTab === 'interfaces' && (
          <div className="space-y-6">
            <p className="text-sm text-[var(--text-secondary)]">配置 PxeGo 在哪些网络接口上提供服务。每个接口可独立启用 DHCP/TFTP/HTTP/DNS。</p>
            {config.interfaces.map((iface, i) => (
              <div key={i} className="bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-xl p-5 space-y-4">
                <div className="flex items-center justify-between">
                  <span className="text-sm font-semibold text-[var(--text-primary)]">接口 #{i + 1}</span>
                  {config.interfaces.length > 1 && (
                    <Button variant="ghost" size="sm" onClick={() => setConfig({...config, interfaces: config.interfaces.filter((_, j) => j !== i)})}>移除</Button>
                  )}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  {renderField('接口名称', iface.name, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], name: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: 'eth0' })}
                  {renderField('IP 地址', iface.ip, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], ip: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.100' })}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs font-semibold text-[var(--text-secondary)] mb-1">DHCP 模式</label>
                    <select className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3.5 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 appearance-none" value={iface.dhcpMode} onChange={e => {
                      const next = [...config.interfaces]; next[i] = {...next[i], dhcpMode: e.target.value}; setConfig({...config, interfaces: next})
                    }}>
                      <option value="full">full（完整 DHCP）</option>
                      <option value="proxy">proxy（代理 DHCP）</option>
                      <option value="hybrid">hybrid（混合）</option>
                      <option value="off">off（关闭）</option>
                    </select>
                  </div>
                  {renderField('子网', iface.subnet, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], subnet: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.0/24' })}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  {renderField('地址池', iface.pool, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], pool: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.100-200' })}
                  {renderField('租约时间（秒）', iface.leaseTime, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], leaseTime: v}; setConfig({...config, interfaces: next})
                  })}
                </div>
                <div className="grid grid-cols-2 gap-4">
                  {renderField('网关', iface.gateway, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], gateway: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '192.168.1.1' })}
                  {renderField('DNS 服务器', iface.dnsServers, v => {
                    const next = [...config.interfaces]; next[i] = {...next[i], dnsServers: v}; setConfig({...config, interfaces: next})
                  }, { placeholder: '8.8.8.8' })}
                </div>
                {renderField('Next Server (TFTP 服务器)', iface.nextServer, v => {
                  const next = [...config.interfaces]; next[i] = {...next[i], nextServer: v}; setConfig({...config, interfaces: next})
                }, { placeholder: '同 IP 地址时留空' })}
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
            ))}
            <Button variant="secondary" size="sm" onClick={() => setConfig({...config, interfaces: [...config.interfaces, { ...defaultIface }]})}>
              添加接口
            </Button>
          </div>
        )}

        {activeTab === 'dhcp' && (
          <div className="space-y-4">
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

        {activeTab === 'ipmi' && (
          <div className="space-y-4">
            <Toggle checked={config.ipmiEnabled} onChange={v => setConfig({...config, ipmiEnabled: v})} label={t('settings.enabled') + ' IPMI'} />
            {renderField(t('settings.ipmiTimeout', '超时时间'), config.ipmiTimeout, v => setConfig({...config, ipmiTimeout: v}))}
          </div>
        )}
        </>
      )}
      </Card>
    </div>
  )
}
