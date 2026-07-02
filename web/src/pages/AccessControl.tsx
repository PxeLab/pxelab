import { useState, useEffect } from 'react'
import { useSearchParams } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { Plus, Trash2, ShieldPlus, ShieldX, AlertTriangle, RefreshCw } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { useToast } from '../components/ui/Toast'
import {
  api,
  type BlacklistEntry,
  type WhitelistEntry,
  type UnauthorizedDevice,
} from '../api/client'

type Tab = 'all' | 'whitelist' | 'blacklist' | 'unauthorized'

interface CombinedEntry {
  id: string
  mac: string
  subnet_cidr?: string
  reason?: string
  source?: string
  type: 'blacklist' | 'whitelist'
  created_at: string
}

const MAC_RE = /^([0-9A-Fa-f]{2}:){5}[0-9A-Fa-f]{2}$/

function parseMACs(input: string): string[] {
  return input
    .split(/[\n,;]+/)
    .map(s => s.trim())
    .filter(Boolean)
}

export default function AccessControl() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const activeTab = (searchParams.get('tab') as Tab) || 'all'
  const [blacklist, setBlacklist] = useState<BlacklistEntry[]>([])
  const [whitelist, setWhitelist] = useState<WhitelistEntry[]>([])
  const [unauthorized, setUnauthorized] = useState<UnauthorizedDevice[]>([])
  const [whitelistEnabled, setWhitelistEnabled] = useState(false)
  const [loading, setLoading] = useState(true)

  // Add modal state
  const [showAddModal, setShowAddModal] = useState(false)
  const [addType, setAddType] = useState<'blacklist' | 'whitelist'>('blacklist')
  const [addInput, setAddInput] = useState('')
  const [addCIDR, setAddCIDR] = useState('')
  const [addReason, setAddReason] = useState('')
  const [adding, setAdding] = useState(false)
  const [subnetOptions, setSubnetOptions] = useState<string[]>([])

  // Confirm modal state
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [confirmTitle, setConfirmTitle] = useState('')
  const [confirmMessage, setConfirmMessage] = useState('')
  const [confirmAction, setConfirmAction] = useState<() => Promise<void>>(async () => {})

  // Batch selection
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [batchProcessing, setBatchProcessing] = useState(false)

  useEffect(() => { loadAll() }, [])

  async function loadAll() {
    setLoading(true)
    try {
      const [blRes, wlRes, uaRes, settingsRes] = await Promise.all([
        api.getBlacklist(),
        api.getWhitelist(),
        api.getUnauthorizedDevices(),
        api.getSettings(),
      ])
      setBlacklist(blRes.data)
      setWhitelist(wlRes.data)
      setUnauthorized(uaRes.data)
      setWhitelistEnabled(settingsRes.data.whitelist_enabled)
      const subnets = new Set<string>()
      for (const iface of settingsRes.data.interfaces || []) {
        for (const sn of iface.subnets || []) {
          if (sn.cidr) subnets.add(sn.cidr)
        }
      }
      setSubnetOptions(Array.from(subnets).sort())
    } catch (err: any) {
      showError(err.message || t('accessControl.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  function openAddModal(type: 'blacklist' | 'whitelist') {
    setAddType(type)
    setAddInput('')
    setAddCIDR('')
    setAddReason('')
    setShowAddModal(true)
  }

  function confirm(title: string, message: string, action: () => Promise<void>) {
    setConfirmTitle(title)
    setConfirmMessage(message)
    setConfirmAction(() => action)
    setConfirmOpen(true)
  }

  async function handleAdd() {
    const macs = parseMACs(addInput)
    if (macs.length === 0) {
      showError(t('accessControl.macAtLeastOne'))
      return
    }

    const invalid = macs.filter(m => !MAC_RE.test(m))
    if (invalid.length > 0) {
      showError(t('accessControl.macInvalidFormat', { list: invalid.join('\n') }))
      return
    }

    setAdding(true)
    let added = 0
    try {
      for (const mac of macs) {
        if (addType === 'blacklist') {
          await api.createBlacklistEntry({ mac, reason: addReason.trim() || undefined })
        } else {
          await api.createWhitelistEntry({
            mac,
            subnet_cidr: addCIDR.trim(),
            reason: addReason.trim() || undefined,
          })
        }
        added++
      }
      success(t('accessControl.addCountSuccess', {
        count: added,
        type: addType === 'blacklist' ? t('accessControl.typeBlacklist') : t('accessControl.typeWhitelist'),
      }))
      setShowAddModal(false)
      loadAll()
    } catch (err: any) {
      showError(err.message || t('accessControl.addFailed'))
    } finally {
      setAdding(false)
    }
  }

  async function handleDeleteBlacklist(id: number) {
    confirm(t('accessControl.confirmDeleteBlacklist'), t('accessControl.confirmDeleteBlacklistMsg'), async () => {
      try {
        await api.deleteBlacklistEntry(id)
        success(t('accessControl.removedFromBlacklist'))
        loadAll()
      } catch (err: any) {
        showError(err.message || t('accessControl.deleteFailed'))
      }
    })
  }

  async function handleDeleteWhitelist(id: number) {
    confirm(t('accessControl.confirmDeleteBlacklist'), t('accessControl.confirmDeleteWhitelistMsg'), async () => {
      try {
        await api.deleteWhitelistEntry(id)
        success(t('accessControl.removedFromWhitelist'))
        loadAll()
      } catch (err: any) {
        showError(err.message || t('accessControl.deleteFailed'))
      }
    })
  }

  async function handleDeleteUnauthorized(id: number) {
    confirm(t('accessControl.confirmDeleteBlacklist'), t('accessControl.confirmDeleteUnauthorizedMsg'), async () => {
      try {
        await api.deleteUnauthorizedDevice(id)
        success(t('accessControl.ignored'))
        setSelected(prev => { const s = new Set(prev); s.delete(id); return s })
        loadAll()
      } catch (err: any) {
        showError(err.message || t('accessControl.deleteFailed'))
      }
    })
  }

  async function handleAddToWhitelist(mac: string, subnetCIDR: string) {
    confirm(t('accessControl.confirmAddToWhitelist'), t('accessControl.confirmAddToWhitelistMsg', { mac, subnet: subnetCIDR }), async () => {
      try {
        await api.addUnauthorizedToWhitelist(mac, subnetCIDR)
        success(t('accessControl.addedToWhitelist'))
        loadAll()
      } catch (err: any) {
        showError(err.message || t('accessControl.addFailed'))
      }
    })
  }

  async function handleAddToBlacklist(mac: string, subnetCIDR: string) {
    confirm(t('accessControl.confirmAddToBlacklist'), t('accessControl.confirmAddToBlacklistMsg', { mac }), async () => {
      try {
        await api.addUnauthorizedToBlacklist(mac, subnetCIDR)
        success(t('accessControl.addedToBlacklist'))
        loadAll()
      } catch (err: any) {
        showError(err.message || t('accessControl.addFailed'))
      }
    })
  }

  // ── batch actions ──

  const toggleSelect = (id: number) => {
    setSelected(prev => {
      const s = new Set(prev)
      if (s.has(id)) s.delete(id); else s.add(id)
      return s
    })
  }

  async function batchAddToWhitelist() {
    const items = unauthorized.filter(e => selected.has(e.id))
    confirm(t('accessControl.confirmBatchAddWhitelist'),
      t('accessControl.confirmBatchAddWhitelistMsg', { count: items.length }),
    async () => {
      setBatchProcessing(true)
      let ok = 0
      for (const item of items) {
        try {
          await api.addUnauthorizedToWhitelist(item.mac, item.subnet_cidr)
          ok++
        } catch { /* skip */ }
      }
      setBatchProcessing(false)
      setSelected(new Set())
      success(t('accessControl.batchWhitelistResult', { ok, total: items.length }))
      loadAll()
    })
  }

  async function batchAddToBlacklist() {
    const items = unauthorized.filter(e => selected.has(e.id))
    confirm(t('accessControl.confirmBatchAddBlacklist'),
      t('accessControl.confirmBatchAddBlacklistMsg', { count: items.length }),
    async () => {
      setBatchProcessing(true)
      let ok = 0
      for (const item of items) {
        try {
          await api.addUnauthorizedToBlacklist(item.mac, item.subnet_cidr)
          ok++
        } catch { /* skip */ }
      }
      setBatchProcessing(false)
      setSelected(new Set())
      success(t('accessControl.batchBlacklistResult', { ok, total: items.length }))
      loadAll()
    })
  }

  async function batchDeleteUnauthorized() {
    const items = unauthorized.filter(e => selected.has(e.id))
    confirm(t('accessControl.confirmBatchIgnore'),
      t('accessControl.confirmBatchIgnoreMsg', { count: items.length }),
    async () => {
      for (const item of items) {
        try { await api.deleteUnauthorizedDevice(item.id) } catch { /* skip */ }
      }
      setSelected(new Set())
      loadAll()
    })
  }

  function combinedList(): CombinedEntry[] {
    const combined: CombinedEntry[] = [
      ...blacklist.map(e => ({ ...e, id: `bl-${e.id}`, type: 'blacklist' as const })),
      ...whitelist.map(e => ({ ...e, id: `wl-${e.id}`, type: 'whitelist' as const })),
    ]
    combined.sort((a, b) => b.created_at.localeCompare(a.created_at))
    return combined
  }

  const tabs: { key: Tab; label: string; count?: number; badge?: string }[] = [
    { key: 'all', label: t('accessControl.tabAll'), count: blacklist.length + whitelist.length },
    { key: 'whitelist', label: t('accessControl.tabWhitelist'), count: whitelist.length },
    { key: 'blacklist', label: t('accessControl.tabBlacklist'), count: blacklist.length },
    { key: 'unauthorized', label: t('accessControl.tabUnauthorized'), count: unauthorized.length, badge: unauthorized.length > 0 ? undefined : whitelistEnabled ? undefined : t('accessControl.tabDisabled') },
  ]

  const parsedMACs = parseMACs(addInput)
  const macsValid = parsedMACs.length > 0 && parsedMACs.every(m => MAC_RE.test(m))

  if (loading) {
    return (
      <div className="flex items-center justify-center py-16">
        <div className="flex flex-col items-center gap-3">
          <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
          <span className="text-sm text-[var(--text-muted)]">{t('accessControl.loading')}</span>
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* ── Page Header ── */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-lg font-bold text-[var(--text-primary)]">{t('accessControl.title')}</h1>
        </div>
        <div className="flex items-center gap-2">
          <Button variant="ghost" size="sm" onClick={loadAll}>
            <RefreshCw size={14} />
          </Button>
          <Button variant="danger" size="sm" onClick={() => openAddModal('blacklist')}>
            <Plus size={14} />
            {t('accessControl.btnBlacklist')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => openAddModal('whitelist')}>
            <Plus size={14} />
            {t('accessControl.btnWhitelist')}
          </Button>
        </div>
      </div>

      {/* ── Tabs ── */}
      <div className="flex gap-1 border-b border-[var(--bg-border)]">
        {tabs.map(tab => (
          <button
            key={tab.key}
            onClick={() => setSearchParams({ tab: tab.key })}
            className={`flex items-center gap-2 px-4 py-2.5 text-sm font-medium border-b-2 transition-colors -mb-[1px] ${
              activeTab === tab.key
                ? 'border-blue-500 text-blue-400'
                : 'border-transparent text-[var(--text-muted)] hover:text-[var(--text-primary)]'
            }`}
          >
            {tab.key === 'unauthorized' && <AlertTriangle size={14} className="text-amber-400" />}
            {tab.label}
            {tab.badge ? (
              <span className="text-[10px] px-1.5 py-0.5 rounded-full bg-amber-500/10 text-amber-400">
                {tab.badge}
              </span>
            ) : tab.count !== undefined ? (
              <span className={`text-xs px-1.5 py-0.5 rounded-full ${
                activeTab === tab.key ? 'bg-blue-500/10 text-blue-400' : 'bg-[var(--bg-card)] text-[var(--text-muted)]'
              }`}>
                {tab.count}
              </span>
            ) : null}
          </button>
        ))}
      </div>

      {/* ── Table ── */}
      <Card padding={false}>
        {activeTab === 'all' && (
          <AccessTable
            columns={[t('accessControl.colMac'), t('accessControl.colType'), t('accessControl.colSubnet'), t('accessControl.colReason'), t('accessControl.colCreatedAt'), t('accessControl.colActions')]}
            rows={combinedList().map(entry => ({
              key: entry.id,
              cells: [
                <span key="mac" className="font-mono text-sm">{entry.mac}</span>,
                <span key="type" className={`text-xs font-medium px-2 py-0.5 rounded-full ${
                  entry.type === 'blacklist'
                    ? 'bg-red-500/10 text-red-400'
                    : 'bg-green-500/10 text-green-400'
                }`}>
                  {entry.type === 'blacklist' ? t('accessControl.typeBlacklist') : t('accessControl.typeWhitelist')}
                </span>,
                <span key="cidr" className="font-mono text-xs text-blue-400">{entry.subnet_cidr || '-'}</span>,
                <span key="reason" className="text-xs text-[var(--text-muted)]">{entry.reason || '-'}</span>,
                <span key="time" className="text-xs text-[var(--text-muted)]">{formatTime(entry.created_at)}</span>,
                <div key="actions" className="flex items-center gap-1">
                  <button
                    onClick={() => entry.type === 'blacklist'
                      ? handleDeleteBlacklist(Number(entry.id.replace('bl-', '')))
                      : handleDeleteWhitelist(Number(entry.id.replace('wl-', '')))
                    }
                    className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors"
                    title={t('accessControl.titleDelete')}
                  >
                    <Trash2 size={14} />
                  </button>
                </div>,
              ],
            }))}
            emptyMessage={t('accessControl.emptyAll')}
          />
        )}

        {activeTab === 'whitelist' && (
          <AccessTable
            columns={[t('accessControl.colMac'), t('accessControl.colSubnet'), t('accessControl.colReason'), t('accessControl.colCreatedAt'), t('accessControl.colActions')]}
            rows={whitelist.map(entry => ({
              key: `wl-${entry.id}`,
              cells: [
                <span key="mac" className="font-mono text-sm">{entry.mac}</span>,
                <span key="cidr" className="font-mono text-xs text-blue-400">{entry.subnet_cidr || t('accessControl.allSubnets')}</span>,
                <span key="reason" className="text-xs text-[var(--text-muted)]">{entry.reason || '-'}</span>,
                <span key="time" className="text-xs text-[var(--text-muted)]">{formatTime(entry.created_at)}</span>,
                <button
                  key="del"
                  onClick={() => handleDeleteWhitelist(entry.id)}
                  className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors"
                  title={t('accessControl.titleDelete')}
                >
                  <Trash2 size={14} />
                </button>,
              ],
            }))}
            emptyMessage={t('accessControl.emptyWhitelist')}
          />
        )}

        {activeTab === 'blacklist' && (
          <AccessTable
            columns={[t('accessControl.colMac'), t('accessControl.colReason'), t('accessControl.colCreatedAt'), t('accessControl.colActions')]}
            rows={blacklist.map(entry => ({
              key: `bl-${entry.id}`,
              cells: [
                <span key="mac" className="font-mono text-sm">{entry.mac}</span>,
                <span key="reason" className="text-xs text-[var(--text-muted)]">{entry.reason || '-'}</span>,
                <span key="time" className="text-xs text-[var(--text-muted)]">{formatTime(entry.created_at)}</span>,
                <button
                  key="del"
                  onClick={() => handleDeleteBlacklist(entry.id)}
                  className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors"
                  title={t('accessControl.titleDelete')}
                >
                  <Trash2 size={14} />
                </button>,
              ],
            }))}
            emptyMessage={t('accessControl.emptyBlacklist')}
          />
        )}

        {activeTab === 'unauthorized' && (
          <>
            {/* Batch action bar */}
            {selected.size > 0 && (
              <div className="flex items-center gap-2 px-4 py-2 border-b border-[var(--bg-border)] bg-amber-500/5">
                <span className="text-xs text-[var(--text-muted)]">
                  {t('accessControl.selectedCount', { count: selected.size })}
                </span>
                <div className="flex-1" />
                <Button
                  variant="primary"
                  size="sm"
                  onClick={batchAddToWhitelist}
                  disabled={batchProcessing}
                >
                  <ShieldPlus size={12} />
                  {t('accessControl.btnAddToWhitelist')}
                </Button>
                <Button
                  variant="danger"
                  size="sm"
                  onClick={batchAddToBlacklist}
                  disabled={batchProcessing}
                >
                  <ShieldX size={12} />
                  {t('accessControl.btnAddToBlacklist')}
                </Button>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={batchDeleteUnauthorized}
                  disabled={batchProcessing}
                >
                  <Trash2 size={12} />
                  {t('accessControl.btnIgnore')}
                </Button>
              </div>
            )}
            <AccessTable
              columns={['', t('accessControl.colMac'), t('accessControl.colSubnet'), t('accessControl.colRejectReason'), t('accessControl.colRequestCount'), t('accessControl.colLastRequest'), t('accessControl.colActions')]}
              rows={unauthorized.map(entry => ({
                key: `ua-${entry.id}`,
                cells: [
                  <input
                    key="cb"
                    type="checkbox"
                    checked={selected.has(entry.id)}
                    onChange={() => toggleSelect(entry.id)}
                    className="accent-blue-500 cursor-pointer"
                  />,
                  <span key="mac" className="font-mono text-sm">{entry.mac}</span>,
                  <span key="cidr" className="font-mono text-xs text-blue-400">{entry.subnet_cidr}</span>,
                  <span key="reason" className="text-xs text-[var(--text-muted)]">{entry.reason}</span>,
                  <span key="count" className="text-xs font-mono text-amber-400">{entry.count}</span>,
                  <span key="time" className="text-xs text-[var(--text-muted)]">{formatTime(entry.last_seen)}</span>,
                  <div key="actions" className="flex items-center gap-1">
                    <button
                      onClick={() => handleAddToWhitelist(entry.mac, entry.subnet_cidr)}
                      className="p-1 rounded hover:bg-green-500/10 text-[var(--text-muted)] hover:text-green-400 transition-colors"
                      title={t('accessControl.titleAddToWhitelist')}
                    >
                      <ShieldPlus size={14} />
                    </button>
                    <button
                      onClick={() => handleAddToBlacklist(entry.mac, entry.subnet_cidr)}
                      className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors"
                      title={t('accessControl.titleAddToBlacklist')}
                    >
                      <ShieldX size={14} />
                    </button>
                    <button
                      onClick={() => handleDeleteUnauthorized(entry.id)}
                      className="p-1 rounded hover:bg-red-500/10 text-[var(--text-muted)] hover:text-red-400 transition-colors"
                      title={t('accessControl.titleIgnore')}
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>,
                ],
              }))}
              emptyMessage={whitelistEnabled ? t('accessControl.emptyUnauthorizedEnabled') : t('accessControl.emptyUnauthorizedDisabled')}
            />
          </>
        )}
      </Card>

      {/* ── Confirm Modal ── */}
      <Modal
        open={confirmOpen}
        onClose={() => setConfirmOpen(false)}
        title={confirmTitle}
        width="420px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setConfirmOpen(false)}>
              {t('accessControl.btnCancel')}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={async () => {
                setConfirmOpen(false)
                await confirmAction()
              }}
            >
              {t('accessControl.btnConfirm')}
            </Button>
          </>
        }
      >
        <p className="text-sm text-[var(--text-muted)]">{confirmMessage}</p>
      </Modal>

      {/* ── Add Modal ── */}
      <Modal
        open={showAddModal}
        onClose={() => setShowAddModal(false)}
        title={t('accessControl.addModalTitle', { type: addType === 'blacklist' ? t('accessControl.typeBlacklist') : t('accessControl.typeWhitelist') })}
        width="560px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShowAddModal(false)}>
              {t('accessControl.btnCancel')}
            </Button>
            <Button
              variant={addType === 'blacklist' ? 'danger' : 'primary'}
              size="sm"
              onClick={handleAdd}
              disabled={adding || !macsValid}
            >
              {adding ? t('accessControl.btnAdding') : t('accessControl.btnAddCount', { count: parsedMACs.length > 0 ? parsedMACs.length : 0 })}
            </Button>
          </>
        }
      >
        <div className="space-y-5">
          {/* Type toggle */}
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">{t('accessControl.labelType')}</label>
            <div className="flex bg-[var(--bg-input)] rounded-lg border border-[var(--bg-border)] p-0.5 w-fit">
              <button
                onClick={() => setAddType('blacklist')}
                className={`px-4 py-1.5 text-xs font-medium rounded-md transition-colors ${
                  addType === 'blacklist'
                    ? 'bg-red-500/15 text-red-400 shadow-sm'
                    : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                {t('accessControl.typeBlacklist')}
              </button>
              <button
                onClick={() => setAddType('whitelist')}
                className={`px-4 py-1.5 text-xs font-medium rounded-md transition-colors ${
                  addType === 'whitelist'
                    ? 'bg-green-500/15 text-green-400 shadow-sm'
                    : 'text-[var(--text-muted)] hover:text-[var(--text-primary)]'
                }`}
              >
                {t('accessControl.typeWhitelist')}
              </button>
            </div>
          </div>

          {/* MAC input */}
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">
              {t('accessControl.labelMacAddress')}
              <span className="ml-2 font-normal text-[var(--text-muted)] opacity-60">
                {t('accessControl.macHint')}
              </span>
            </label>
            <textarea
              value={addInput}
              onChange={e => setAddInput(e.target.value)}
              placeholder={`00:11:22:33:44:55\nAA:BB:CC:DD:EE:FF`}
              rows={5}
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono placeholder-[var(--text-muted)] resize-none"
            />
            {addInput.trim() && (
              <div className="mt-2 flex items-center gap-2">
                <span className={`text-xs ${macsValid ? 'text-green-400' : 'text-red-400'}`}>
                  {macsValid
                    ? t('accessControl.macValid', { count: parsedMACs.length })
                    : t('accessControl.macInvalid', { count: parsedMACs.length })}
                </span>
                {macsValid && (
                  <div className="flex flex-wrap gap-1">
                    {parsedMACs.map((m, i) => (
                      <span key={i} className="text-[11px] font-mono px-1.5 py-0.5 rounded bg-[var(--bg-card)] text-[var(--text-muted)]">
                        {m}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            )}
          </div>

          {/* CIDR (whitelist only) */}
          {addType === 'whitelist' && (
            <div>
              <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">{t('accessControl.labelSubnetCidr')}</label>
              <select
                value={addCIDR}
                onChange={e => setAddCIDR(e.target.value)}
                className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 font-mono"
              >
                <option value="">{t('accessControl.subnetAllOption')}</option>
                {subnetOptions.map(s => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </div>
          )}

          {/* Reason */}
          <div>
            <label className="block text-xs font-semibold text-[var(--text-muted)] mb-2">
              {t('accessControl.labelReason')}
              <span className="ml-2 font-normal text-[var(--text-muted)] opacity-60">{t('accessControl.reasonOptional')}</span>
            </label>
            <input
              type="text"
              value={addReason}
              onChange={e => setAddReason(e.target.value)}
              placeholder={t('accessControl.reasonPlaceholder')}
              className="w-full bg-[var(--bg-input)] border border-[var(--bg-border)] rounded-lg px-3 py-2 text-sm text-[var(--text-primary)] outline-none focus:border-blue-500 placeholder-[var(--text-muted)]"
            />
          </div>
        </div>
      </Modal>
    </div>
  )
}

// ── Table Component ──

interface TableProps {
  columns: string[]
  rows: { key: string; cells: React.ReactNode[] }[]
  emptyMessage: string
}

function AccessTable({ columns, rows, emptyMessage }: TableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full">
        <thead>
          <tr className="border-b border-[var(--bg-border)]">
            {columns.map(col => (
              <th key={col} className="text-left px-4 py-3 text-xs font-semibold text-[var(--text-muted)] uppercase tracking-wider">
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={columns.length} className="text-center py-12 text-sm text-[var(--text-muted)]">
                {emptyMessage}
              </td>
            </tr>
          ) : (
            rows.map(row => (
              <tr key={row.key} className="border-b border-[var(--bg-border)] last:border-0 hover:bg-[var(--bg-card)]/50 transition-colors">
                {row.cells.map((cell, i) => (
                  <td key={i} className="px-4 py-3">
                    {cell}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}

// ── Helpers ──

function formatTime(t: string): string {
  if (!t) return '-'
  try {
    const d = new Date(t)
    const pad = (n: number) => String(n).padStart(2, '0')
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  } catch {
    return t
  }
}
