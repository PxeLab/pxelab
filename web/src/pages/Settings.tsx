import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'

type Tab = 'general' | 'dhcp' | 'tftp' | 'dns' | 'http' | 'ipmi'

export default function Settings() {
  const { t } = useTranslation()
  const { success } = useToast()
  const [activeTab, setActiveTab] = useState<Tab>('general')
  const [config, setConfig] = useState({
    serverName: 'PxeGo-Server',
    logLevel: 'info',
    dataDir: '/var/lib/pxego',
    mode: 'server',
    listenAddr: ':8080',
    authToken: 'pxego-secret-token',
    autoOpen: true,
    persistEvents: true,
    dhcpEnabled: true,
    dhcpRange: '192.168.1.100-200',
    dhcpGateway: '192.168.1.1',
    dhcpSubnet: '255.255.255.0',
    dhcpDns: '8.8.8.8',
    dhcpLeaseTime: '86400',
    tftpEnabled: true,
    tftpPort: '69',
    tftpRoot: '/var/lib/pxego/boot',
    dnsEnabled: false,
    dnsPort: '53',
    dnsUpstream: '8.8.8.8:53',
    httpPort: '8080',
    httpBootDir: '/var/lib/pxego/boot',
    ipmiEnabled: false,
    ipmiTimeout: '5',
  })

  const tabs: { key: Tab; label: string }[] = [
    { key: 'general', label: t('settings.general') },
    { key: 'dhcp', label: t('settings.dhcp') },
    { key: 'tftp', label: t('settings.tftp') },
    { key: 'dns', label: t('settings.dns') },
    { key: 'http', label: 'HTTP' },
    { key: 'ipmi', label: 'IPMI' },
  ]

  function handleSave() {
    success(t('settings.saved'))
  }

  function renderField(label: string, value: string, onChange: (v: string) => void, opts?: { type?: string; placeholder?: string }) {
    return (
      <div className="form-group">
        <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">{label}</label>
        <input
          type={opts?.type || 'text'}
          value={value}
          onChange={e => onChange(e.target.value)}
          placeholder={opts?.placeholder}
          className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 focus:ring-3 focus:ring-blue-500/10 transition-all placeholder-[#6b7294]"
        />
      </div>
    )
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div></div>
        <div className="flex gap-2">
          <Button variant="secondary" size="sm" onClick={() => success('配置已重载')}>
            <RefreshCw size={14} /> {t('common.reload', '重载配置')}
          </Button>
          <Button variant="primary" size="sm" onClick={handleSave}>
            <Save size={14} /> {t('settings.save')}
          </Button>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-[#232738] mb-5">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setActiveTab(tab.key)}
            className={`px-5 py-2.5 text-sm font-semibold transition-colors border-b-2 -mb-px ${
              activeTab === tab.key
                ? 'text-blue-400 border-blue-500'
                : 'text-[#6b7294] border-transparent hover:text-[#9aa0ab]'
            }`}
          >
            {tab.label}
          </button>
        ))}
      </div>

      <Card>
        {activeTab === 'general' && (
          <div className="space-y-4">
            <div className="grid grid-cols-2 gap-4">
              {renderField(t('common.serverName', '服务器名称'), config.serverName, v => setConfig({...config, serverName: v}))}
              <div>
                <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">{t('common.logLevel', '日志级别')}</label>
                <select className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 appearance-none" value={config.logLevel} onChange={e => setConfig({...config, logLevel: e.target.value})}>
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
                <label className="block text-xs font-semibold text-[#9aa0ab] mb-1.5">{t('common.mode', '运行模式')}</label>
                <select className="w-full bg-[#1a1d2e] border border-[#232738] rounded-lg px-3.5 py-2 text-sm text-[#e8eaed] outline-none focus:border-blue-500 appearance-none" value={config.mode} onChange={e => setConfig({...config, mode: e.target.value})}>
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
      </Card>
    </div>
  )
}
