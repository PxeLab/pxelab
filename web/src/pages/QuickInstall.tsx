import { useState, useEffect, useMemo } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ArrowLeft, ArrowRight, Check, ChevronDown, ChevronUp, Monitor, Plus, Rocket, AlertTriangle } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { PageHeader } from '../components/ui/PageHeader'
import { Tag } from '../components/ui/Tag'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { Input, Select } from '../components/ui/FormControls'
import {
  claimPxeBootRecord,
  createHost,
  createInstallTask,
  createInstallTaskBatch,
  getAnswerTemplates,
  getHosts,
  getNetbootCatalog,
  getNetbootFileStatus,
  getProfiles,
  getPxeBootRecords,
  updateHost,
  type AnswerTemplate,
  type FileStatus,
  type Host,
  type NetbootDistro,
  type Profile,
  type PxeBootRecord,
} from '../api/client'
import {
  MAC_RE,
  buildReadySystems,
  defaultHostname,
  normalizeMac,
  type MatchedSystem,
} from '../utils/readySystems'

interface SubmitResult {
  mac: string
  ok: boolean
  error?: string
}

export default function QuickInstall() {
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { success, error: showError } = useToast()

  const [loading, setLoading] = useState(true)
  const [records, setRecords] = useState<PxeBootRecord[]>([])
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [distros, setDistros] = useState<NetbootDistro[]>([])
  const [fileStatus, setFileStatus] = useState<FileStatus[]>([])
  const [templates, setTemplates] = useState<AnswerTemplate[]>([])
  const [hosts, setHosts] = useState<Host[]>([])

  const [step, setStep] = useState(0)
  const [selectedMacs, setSelectedMacs] = useState<string[]>([])
  const [manualMacs, setManualMacs] = useState<string[]>([])
  const [manualInput, setManualInput] = useState('')
  const [manualError, setManualError] = useState('')
  const [selectedKey, setSelectedKey] = useState('')
  const [templateOverride, setTemplateOverride] = useState<number | null>(null)
  const [showAdvanced2, setShowAdvanced2] = useState(false)
  const [showAdvanced3, setShowAdvanced3] = useState(false)
  const [hostnameInput, setHostnameInput] = useState('')
  const [extraCmdline, setExtraCmdline] = useState('')
  const [snInput, setSnInput] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [results, setResults] = useState<SubmitResult[] | null>(null)

  useEffect(() => {
    let cancelled = false
    async function load() {
      const [recRes, profRes, catRes, fsRes, tmplRes, hostRes] = await Promise.all([
        getPxeBootRecords().catch(() => null),
        getProfiles().catch(() => null),
        getNetbootCatalog().catch(() => null),
        getNetbootFileStatus().catch(() => null),
        getAnswerTemplates().catch(() => null),
        getHosts({ page: '1', size: '9999' }).catch(() => null),
      ])
      if (cancelled) return
      setRecords(recRes?.data?.records || [])
      setProfiles(profRes?.data || [])
      setDistros(catRes?.data?.distros || [])
      setFileStatus(fsRes?.data || [])
      setTemplates(tmplRes?.data?.templates || [])
      setHosts(hostRes?.data?.hosts || [])
      setLoading(false)
    }
    load()
    return () => { cancelled = true }
  }, [])

  const unclaimedRecords = useMemo(
    () => records.filter(r => !r.claimed_host_id),
    [records],
  )

  // Reverse-match every profile menu entry back to its catalog version, then
  // require the matching check-files entry to report has_local (= ready).
  const readySystems = useMemo<MatchedSystem[]>(
    () => buildReadySystems(profiles, distros, fileStatus, templates),
    [profiles, distros, fileStatus, templates],
  )

  const selected = readySystems.find(s => s.key === selectedKey)
  const effectiveTemplate = selected
    ? (templateOverride != null ? templates.find(tp => tp.id === templateOverride) : selected.defaultTemplate)
    : undefined

  function toggleMac(mac: string) {
    setSelectedMacs(prev => prev.includes(mac) ? prev.filter(m => m !== mac) : [...prev, mac])
  }

  function addManualMac() {
    const mac = normalizeMac(manualInput)
    if (!MAC_RE.test(mac)) {
      setManualError(t('quickInstall.invalidMac'))
      return
    }
    if (selectedMacs.includes(mac) || unclaimedRecords.some(r => normalizeMac(r.mac) === mac)) {
      setManualError(t('quickInstall.duplicateMac'))
      return
    }
    setManualMacs(prev => [...prev, mac])
    setSelectedMacs(prev => [...prev, mac])
    setManualInput('')
    setManualError('')
  }

  function selectSystem(key: string) {
    setSelectedKey(key)
    setTemplateOverride(null)
  }

  async function handleSubmit() {
    if (!selected || !effectiveTemplate) return
    setSubmitting(true)
    setResults(null)
    const single = selectedMacs.length === 1

    // R7：多选走批量接口（共享 batch_id，服务端逐台校验/建档/认领/绑 profile）
    if (!single) {
      try {
        const res = await createInstallTaskBatch({
          hosts: selectedMacs.map(mac => ({
            mac,
            name: defaultHostname(mac),
            ip: unclaimedRecords.find(r => normalizeMac(r.mac) === mac)?.ip || undefined,
          })),
          distro_name: selected.distroName,
          version_codename: selected.versionCodename,
          arch: selected.arch,
          answer_template_id: effectiveTemplate.id ?? null,
          extra_cmdline: extraCmdline.trim(),
          profile_id: selected.profile.id,
        })
        const taskIds = (res.data?.tasks || []).map(task => task.id!).filter(Boolean)
        const skipped = res.data?.skipped || []
        setSubmitting(false)
        if (skipped.length === 0) {
          success(t('quickInstall.submitSuccess'))
          navigate(`/install-tasks?highlight=${taskIds.join(',')}`)
        } else {
          const skippedMacs = new Set(skipped.map(s => normalizeMac(s.mac)))
          setResults([
            ...selectedMacs.filter(mac => !skippedMacs.has(mac)).map(mac => ({ mac, ok: true })),
            ...skipped.map(s => ({ mac: normalizeMac(s.mac), ok: false, error: s.reason })),
          ])
          showError(t('quickInstall.submitPartial'))
        }
      } catch (err: any) {
        setSubmitting(false)
        showError(err.message || t('quickInstall.submitPartial'))
      }
      return
    }

    // 单机：保持 R1 逐台编排行为
    const taskIds: string[] = []
    const rs: SubmitResult[] = []
    for (const mac of selectedMacs) {
      try {
        const hostname = single && hostnameInput.trim() ? hostnameInput.trim() : defaultHostname(mac)
        let host = hosts.find(h => normalizeMac(h.mac || '') === mac)
        if (host) {
          if (host.profile_id !== selected.profile.id) {
            const res = await updateHost(host.id, { profile_id: selected.profile.id })
            host = res.data ?? host
          }
        } else {
          const rec = unclaimedRecords.find(r => normalizeMac(r.mac) === mac)
          const res = await createHost({
            name: hostname,
            mac,
            ip: rec?.ip || '',
            sn: single && snInput.trim() ? snInput.trim() : undefined,
            profile_id: selected.profile.id,
          })
          host = res.data
          if (host) setHosts(prev => [...prev, host!])
        }
        if (!host) throw new Error(t('quickInstall.loadFailed'))
        try { await claimPxeBootRecord(mac, host.id) } catch { /* record may not exist for manual MAC */ }
        const taskRes = await createInstallTask({
          host_id: host.id,
          distro_name: selected.distroName,
          version_codename: selected.versionCodename,
          arch: selected.arch,
          answer_template_id: effectiveTemplate.id ?? null,
          extra_cmdline: extraCmdline.trim(),
        })
        if (taskRes.data?.id) taskIds.push(taskRes.data.id)
        rs.push({ mac, ok: true })
      } catch (err: any) {
        rs.push({ mac, ok: false, error: err.message || String(err) })
      }
    }
    setResults(rs)
    setSubmitting(false)
    if (rs.every(r => r.ok)) {
      success(t('quickInstall.submitSuccess'))
      navigate(`/install-tasks?highlight=${taskIds.join(',')}`)
    } else {
      showError(t('quickInstall.submitPartial'))
    }
  }

  const steps = [t('quickInstall.stepMachine'), t('quickInstall.stepSystem'), t('quickInstall.stepConfirm')]
  const canNext = step === 0 ? selectedMacs.length > 0 : step === 1 ? !!selected : false
  const single = selectedMacs.length === 1

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('quickInstall.title')}
        description={t('quickInstall.description')}
        className="mb-0"
      />

      {/* Step indicator */}
      <div className="flex items-center gap-2">
        {steps.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            {i > 0 && <div className={`h-px w-8 ${i <= step ? 'bg-blue-500' : 'bg-[var(--bg-border)]'}`} />}
            <div className={`flex items-center gap-2 px-3 py-1.5 rounded-lg border text-xs font-semibold transition-colors ${
              i === step
                ? 'border-blue-500/50 bg-blue-500/10 text-blue-400'
                : i < step
                  ? 'border-accent-green/30 bg-accent-green/10 text-accent-green'
                  : 'border-[var(--bg-border)] text-[var(--text-muted)]'
            }`}>
              {i < step ? <Check size={12} /> : <span className="font-mono">{i + 1}</span>}
              {label}
            </div>
          </div>
        ))}
      </div>

      {loading && (
        <Card padding={false}>
          <div className="p-6 space-y-3">
            {[1, 2, 3].map(i => (
              <div key={i} className="h-12 rounded bg-[var(--bg-hover)] animate-pulse" />
            ))}
          </div>
        </Card>
      )}

      {/* ── Step 1: machines ── */}
      {!loading && step === 0 && (
        <Card title={t('quickInstall.selectMachines')}>
          <div className="space-y-3">
            {unclaimedRecords.length === 0 && manualMacs.length === 0 && (
              <div className="py-8 text-center">
                <Monitor size={28} className="mx-auto mb-3 text-[var(--text-muted)] opacity-40" />
                <p className="text-sm text-[var(--text-muted)]">{t('quickInstall.noPxeRecords')}</p>
              </div>
            )}
            {unclaimedRecords.map(r => {
              const mac = normalizeMac(r.mac)
              const checked = selectedMacs.includes(mac)
              return (
                <label
                  key={mac}
                  className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors ${
                    checked ? 'border-blue-500/50 bg-blue-500/5' : 'border-[var(--bg-border)] hover:bg-[var(--bg-hover)]'
                  }`}
                >
                  <input type="checkbox" checked={checked} onChange={() => toggleMac(mac)} className="accent-blue-500" />
                  <span className="text-sm font-mono text-[var(--text-primary)]">{mac}</span>
                  <span className="text-xs font-mono text-[var(--text-muted)]">{r.ip || '-'}</span>
                  {r.loader && <Tag>{r.loader}</Tag>}
                  <span className="ml-auto text-xs text-[var(--text-muted)]">
                    {r.last_seen ? new Date(r.last_seen).toLocaleString() : '-'}
                  </span>
                </label>
              )
            })}
            {manualMacs.map(mac => {
              const checked = selectedMacs.includes(mac)
              return (
                <label
                  key={mac}
                  className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border cursor-pointer transition-colors ${
                    checked ? 'border-blue-500/50 bg-blue-500/5' : 'border-[var(--bg-border)] hover:bg-[var(--bg-hover)]'
                  }`}
                >
                  <input type="checkbox" checked={checked} onChange={() => toggleMac(mac)} className="accent-blue-500" />
                  <span className="text-sm font-mono text-[var(--text-primary)]">{mac}</span>
                  <Tag color="purple">{t('quickInstall.manualTag')}</Tag>
                </label>
              )
            })}

            {/* Manual MAC input */}
            <div className="flex items-center gap-2 pt-2 border-t border-[var(--bg-border)]">
              <Input
                size="sm"
                value={manualInput}
                onChange={e => { setManualInput(e.target.value); setManualError('') }}
                placeholder={t('quickInstall.manualPlaceholder')}
                className="max-w-[220px] font-mono"
                onKeyDown={e => { if (e.key === 'Enter') addManualMac() }}
              />
              <Button size="sm" onClick={addManualMac} disabled={!manualInput.trim()}>
                <Plus size={12} />
                {t('quickInstall.addManual')}
              </Button>
              {manualError && <span className="text-xs text-accent-red">{manualError}</span>}
              {selectedMacs.length > 0 && (
                <span className="ml-auto text-xs text-[var(--text-muted)]">
                  {t('quickInstall.selectedCount', { count: selectedMacs.length })}
                </span>
              )}
            </div>
          </div>
        </Card>
      )}

      {/* ── Step 2: system ── */}
      {!loading && step === 1 && (
        <Card title={t('quickInstall.selectSystem')}>
          {readySystems.length === 0 ? (
            <div className="py-8 text-center space-y-3">
              <AlertTriangle size={28} className="mx-auto text-accent-yellow opacity-60" />
              <p className="text-sm text-[var(--text-muted)]">{t('quickInstall.noReadySystems')}</p>
              <Button variant="primary" size="sm" onClick={() => navigate('/store')}>
                {t('quickInstall.goStore')}
              </Button>
            </div>
          ) : (
            <div className="space-y-3">
              {readySystems.map(s => {
                const active = selectedKey === s.key
                return (
                  <button
                    key={s.key}
                    onClick={() => selectSystem(s.key)}
                    className={`w-full flex items-center gap-3 px-4 py-3 rounded-lg border text-left transition-colors ${
                      active ? 'border-blue-500/50 bg-blue-500/5' : 'border-[var(--bg-border)] hover:bg-[var(--bg-hover)]'
                    }`}
                  >
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 flex-wrap">
                        <span className="text-sm font-semibold text-[var(--text-primary)]">{s.profile.name}</span>
                        <Tag>{s.distroName}</Tag>
                        <Tag color="cyan">{s.versionName}</Tag>
                        <Tag color="purple">{s.arch}</Tag>
                      </div>
                      <div className="mt-1.5 flex items-center gap-2 text-xs text-[var(--text-muted)]">
                        <span>{t('quickInstall.defaultTemplate')}:</span>
                        {s.defaultTemplate ? (
                          <Tag color="green">{s.defaultTemplate.name} ({s.defaultTemplate.type})</Tag>
                        ) : (
                          <Tag color="red">{t('quickInstall.noTemplate')}</Tag>
                        )}
                      </div>
                    </div>
                    {active && <Check size={16} className="text-blue-400 shrink-0" />}
                  </button>
                )
              })}

              {/* Advanced: template override */}
              <div className="pt-2 border-t border-[var(--bg-border)]">
                <button
                  onClick={() => setShowAdvanced2(v => !v)}
                  className="flex items-center gap-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                >
                  {showAdvanced2 ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                  {t('quickInstall.advanced')}
                </button>
                {showAdvanced2 && (
                  <div className="mt-3">
                    <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('quickInstall.overrideTemplate')}</label>
                    <Select
                      size="sm"
                      value={templateOverride ?? ''}
                      onChange={e => setTemplateOverride(e.target.value ? Number(e.target.value) : null)}
                      disabled={!selected}
                    >
                      <option value="">{t('quickInstall.useDefault')}</option>
                      {templates.map(tp => (
                        <option key={tp.id} value={tp.id}>{tp.name} ({tp.type})</option>
                      ))}
                    </Select>
                  </div>
                )}
              </div>
            </div>
          )}
        </Card>
      )}

      {/* ── Step 3: confirm ── */}
      {!loading && step === 2 && selected && (
        <div className="space-y-4">
          {!effectiveTemplate && (
            <Card>
              <div className="flex items-center gap-3">
                <AlertTriangle size={16} className="text-accent-red shrink-0" />
                <span className="text-sm text-accent-red flex-1">{t('quickInstall.missingTemplateWarn')}</span>
                <Button size="sm" onClick={() => navigate('/answer-templates')}>
                  {t('quickInstall.goAnswerTemplates')}
                </Button>
              </div>
            </Card>
          )}

          <Card title={t('quickInstall.confirmTitle')}>
            <div className="space-y-4">
              {/* Summary */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 text-sm">
                <div>
                  <div className="text-xs text-[var(--text-muted)] mb-1">{t('quickInstall.sumSystem')}</div>
                  <div className="flex items-center gap-2 flex-wrap">
                    <Tag>{selected.distroName}</Tag>
                    <Tag color="cyan">{selected.versionName}</Tag>
                    <Tag color="purple">{selected.arch}</Tag>
                  </div>
                </div>
                <div>
                  <div className="text-xs text-[var(--text-muted)] mb-1">{t('quickInstall.sumProfile')}</div>
                  <span className="text-[var(--text-primary)]">{selected.profile.name}</span>
                </div>
                <div>
                  <div className="text-xs text-[var(--text-muted)] mb-1">{t('quickInstall.sumTemplate')}</div>
                  {effectiveTemplate ? (
                    <Tag color="green">{effectiveTemplate.name} ({effectiveTemplate.type})</Tag>
                  ) : (
                    <Tag color="red">{t('quickInstall.noTemplate')}</Tag>
                  )}
                </div>
                <div>
                  <div className="text-xs text-[var(--text-muted)] mb-1">{t('quickInstall.baselinePull')}</div>
                  <Toggle
                    checked={!!effectiveTemplate?.enable_baseline_pull}
                    onChange={() => {}}
                    disabled
                    label={effectiveTemplate?.enable_baseline_pull ? t('quickInstall.baselineOn') : t('quickInstall.baselineOff')}
                  />
                </div>
              </div>

              {/* Machines */}
              <div>
                <div className="text-xs text-[var(--text-muted)] mb-1">{t('quickInstall.sumMachines')}</div>
                <div className="space-y-1.5">
                  {selectedMacs.map(mac => (
                    <div key={mac} className="flex items-center gap-3 px-3 py-2 rounded-lg bg-[var(--bg-base)]/50 border border-[var(--bg-border)]/50">
                      <span className="text-sm font-mono text-[var(--text-primary)]">{mac}</span>
                      <span className="text-xs text-[var(--text-muted)]">→</span>
                      <span className="text-xs font-mono text-[var(--text-muted)]">
                        {single && hostnameInput.trim() ? hostnameInput.trim() : defaultHostname(mac)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              {/* Hostname (single machine only) */}
              {single && (
                <div>
                  <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('quickInstall.hostname')}</label>
                  <Input
                    size="sm"
                    value={hostnameInput}
                    onChange={e => setHostnameInput(e.target.value)}
                    placeholder={defaultHostname(selectedMacs[0])}
                    className="max-w-[280px] font-mono"
                  />
                </div>
              )}

              {/* Advanced */}
              <div className="pt-2 border-t border-[var(--bg-border)]">
                <button
                  onClick={() => setShowAdvanced3(v => !v)}
                  className="flex items-center gap-1 text-xs text-[var(--text-muted)] hover:text-[var(--text-primary)] transition-colors"
                >
                  {showAdvanced3 ? <ChevronUp size={12} /> : <ChevronDown size={12} />}
                  {t('quickInstall.advanced')}
                </button>
                {showAdvanced3 && (
                  <div className="mt-3 space-y-3">
                    <div>
                      <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('quickInstall.extraCmdline')}</label>
                      <Input
                        size="sm"
                        value={extraCmdline}
                        onChange={e => setExtraCmdline(e.target.value)}
                        placeholder="net.ifnames=0 biosdevname=0"
                      />
                    </div>
                    <div>
                      <label className="block text-xs font-medium text-[var(--text-muted)] mb-1">{t('quickInstall.snLabel')}</label>
                      <Input
                        size="sm"
                        value={snInput}
                        onChange={e => setSnInput(e.target.value)}
                        disabled={!single}
                        placeholder={single ? '' : t('quickInstall.snMultiHint')}
                        className="max-w-[280px] font-mono"
                      />
                    </div>
                  </div>
                )}
              </div>
            </div>
          </Card>

          {/* Per-machine submit results (shown on partial failure) */}
          {results && (
            <Card>
              <div className="space-y-1.5">
                {results.map(r => (
                  <div key={r.mac} className="flex items-center gap-3 text-sm">
                    <span className="font-mono text-[var(--text-primary)]">{r.mac}</span>
                    {r.ok ? (
                      <Tag color="green">{t('quickInstall.resultOk')}</Tag>
                    ) : (
                      <>
                        <Tag color="red">{t('quickInstall.resultFail')}</Tag>
                        <span className="text-xs text-accent-red/80 truncate" title={r.error}>{r.error}</span>
                      </>
                    )}
                  </div>
                ))}
                <div className="pt-2">
                  <Button size="sm" onClick={() => navigate('/install-tasks')}>
                    {t('quickInstall.viewTasks')}
                  </Button>
                </div>
              </div>
            </Card>
          )}
        </div>
      )}

      {/* Footer nav */}
      {!loading && (
        <div className="flex items-center justify-between">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => (step === 0 ? navigate('/') : setStep(step - 1))}
          >
            <ArrowLeft size={14} />
            {step === 0 ? t('common.cancel') : t('quickInstall.back')}
          </Button>
          {step < 2 ? (
            <Button variant="primary" size="sm" onClick={() => setStep(step + 1)} disabled={!canNext}>
              {t('quickInstall.next')}
              <ArrowRight size={14} />
            </Button>
          ) : (
            <Button
              variant="primary"
              size="sm"
              onClick={handleSubmit}
              disabled={submitting || !effectiveTemplate}
            >
              <Rocket size={14} />
              {submitting ? t('quickInstall.submitting') : t('quickInstall.submit')}
            </Button>
          )}
        </div>
      )}
    </div>
  )
}
