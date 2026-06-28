import { useState, useEffect } from 'react'
import { Plus, Trash2, Shield, ShieldOff } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { useToast } from '../components/ui/Toast'
import { api, type BlacklistEntry, type WhitelistEntry } from '../api/client'

export default function AccessControl() {
  const { success, error: showError } = useToast()
  const [blacklist, setBlacklist] = useState<BlacklistEntry[]>([])
  const [whitelist, setWhitelist] = useState<WhitelistEntry[]>([])
  const [loading, setLoading] = useState(true)

  // Blacklist add form
  const [blMac, setBlMac] = useState('')
  const [blReason, setBlReason] = useState('')

  // Whitelist add form
  const [wlMac, setWlMac] = useState('')
  const [wlCIDR, setWlCIDR] = useState('')
  const [wlReason, setWlReason] = useState('')

  useEffect(() => { loadAll() }, [])

  async function loadAll() {
    setLoading(true)
    try {
      const [blRes, wlRes] = await Promise.all([api.getBlacklist(), api.getWhitelist()])
      setBlacklist(blRes.data)
      setWhitelist(wlRes.data)
    } catch (err: any) {
      showError(err.message || '加载失败')
    } finally {
      setLoading(false)
    }
  }

  async function addBlacklist() {
    if (!blMac.trim()) return
    try {
      await api.createBlacklistEntry({ mac: blMac.trim(), reason: blReason.trim() || undefined })
      setBlMac(''); setBlReason('')
      success('已添加到黑名单')
      loadAll()
    } catch (err: any) {
      showError(err.message || '添加失败')
    }
  }

  async function deleteBlacklist(id: number) {
    try {
      await api.deleteBlacklistEntry(id)
      success('已从黑名单移除')
      loadAll()
    } catch (err: any) {
      showError(err.message || '删除失败')
    }
  }

  async function addWhitelist() {
    if (!wlMac.trim() || !wlCIDR.trim()) return
    try {
      await api.createWhitelistEntry({ mac: wlMac.trim(), subnet_cidr: wlCIDR.trim(), reason: wlReason.trim() || undefined })
      setWlMac(''); setWlCIDR(''); setWlReason('')
      success('已添加到白名单')
      loadAll()
    } catch (err: any) {
      showError(err.message || '添加失败')
    }
  }

  async function deleteWhitelist(id: number) {
    try {
      await api.deleteWhitelistEntry(id)
      success('已从白名单移除')
      loadAll()
    } catch (err: any) {
      showError(err.message || '删除失败')
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
        <span className="ml-3 text-sm text-[var(--text-muted)]">加载中...</span>
      </div>
    )
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
      {/* ── 黑名单 ── */}
      <Card>
        <div className="flex items-center gap-2 mb-4">
          <ShieldOff size={18} className="text-red-400" />
          <h2 className="text-base font-semibold text-[var(--text-primary)]">黑名单（全局）</h2>
        </div>
        <p className="text-xs text-[var(--text-muted)] mb-4">加入黑名单的 MAC 地址将不响应所有 DHCP 请求。</p>

        {/* 添加表单 */}
        <div className="flex gap-2 mb-4">
          <input
            type="text"
            value={blMac}
            onChange={e => setBlMac(e.target.value)}
            placeholder="MAC 地址 (00:11:22:33:44:55)"
            className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono placeholder-[var(--text-muted)]"
          />
          <input
            type="text"
            value={blReason}
            onChange={e => setBlReason(e.target.value)}
            placeholder="备注（可选）"
            className="w-32 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 placeholder-[var(--text-muted)]"
          />
          <Button variant="danger" size="sm" onClick={addBlacklist}>
            <Plus size={14} />
          </Button>
        </div>

        {/* 列表 */}
        <div className="space-y-1">
          {blacklist.length === 0 && (
            <p className="text-xs text-[var(--text-muted)] text-center py-6">暂无黑名单条目</p>
          )}
          {blacklist.map(entry => (
            <div key={entry.id} className="flex items-center gap-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2">
              <span className="font-mono text-sm text-[var(--text-primary)] flex-1">{entry.mac}</span>
              {entry.reason && <span className="text-xs text-[var(--text-muted)]">{entry.reason}</span>}
              <button onClick={() => deleteBlacklist(entry.id)} className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors">
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      </Card>

      {/* ── 白名单 ── */}
      <Card>
        <div className="flex items-center gap-2 mb-4">
          <Shield size={18} className="text-green-400" />
          <h2 className="text-base font-semibold text-[var(--text-primary)]">白名单（子网级）</h2>
        </div>
        <p className="text-xs text-[var(--text-muted)] mb-4">白名单中的 MAC 才允许在指定子网中引导。需在 Settings 或子网配置中开启白名单开关。</p>

        {/* 添加表单 */}
        <div className="flex gap-2 mb-4">
          <input
            type="text"
            value={wlMac}
            onChange={e => setWlMac(e.target.value)}
            placeholder="MAC"
            className="flex-[2] bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono placeholder-[var(--text-muted)]"
          />
          <input
            type="text"
            value={wlCIDR}
            onChange={e => setWlCIDR(e.target.value)}
            placeholder="子网 (192.168.1.0/24)"
            className="flex-[2] bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono placeholder-[var(--text-muted)]"
          />
          <input
            type="text"
            value={wlReason}
            onChange={e => setWlReason(e.target.value)}
            placeholder="备注"
            className="flex-1 bg-[var(--bg-elevated)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 placeholder-[var(--text-muted)]"
          />
          <Button variant="primary" size="sm" onClick={addWhitelist}>
            <Plus size={14} />
          </Button>
        </div>

        {/* 列表 */}
        <div className="space-y-1">
          {whitelist.length === 0 && (
            <p className="text-xs text-[var(--text-muted)] text-center py-6">暂无白名单条目</p>
          )}
          {whitelist.map(entry => (
            <div key={entry.id} className="flex items-center gap-3 bg-[var(--bg-card)] border border-[var(--bg-border)] rounded-lg px-3 py-2">
              <span className="font-mono text-sm text-[var(--text-primary)] flex-[2]">{entry.mac}</span>
              <span className="text-xs text-blue-400 font-mono flex-[2]">{entry.subnet_cidr}</span>
              {entry.reason && <span className="text-xs text-[var(--text-muted)] flex-1">{entry.reason}</span>}
              <button onClick={() => deleteWhitelist(entry.id)} className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors">
                <Trash2 size={14} />
              </button>
            </div>
          ))}
        </div>
      </Card>
    </div>
  )
}
