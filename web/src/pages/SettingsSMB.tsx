import { useState, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Save, RefreshCw, Plus, Trash2, Edit2, FolderOpen, Server, Users, CheckCircle, XCircle, Loader2, Eye, EyeOff, Info, AlertCircle } from 'lucide-react'
import { Card } from '../components/ui/Card'
import { Button } from '../components/ui/Button'
import { Modal } from '../components/ui/Modal'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import { PageHeader } from '../components/ui/PageHeader'
import { Toggle } from '../components/ui/Toggle'
import { useToast } from '../components/ui/Toast'
import { SettingsField, SettingsInput } from '../components/settings/SettingsField'
import { Input, Select } from '../components/ui/FormControls'
import { StringListEditor } from '../components/ui/StringListEditor'
import { DataTable, type Column } from '../components/ui/DataTable'
import { api, type SMBSettingsData, type SMBShare, type SMBUser } from '../api/client'

// Empty share for new entries
function newShare(): SMBShare {
  return { name: '', path: '', read_only: true, allow_users: [], write_users: [] }
}

// Empty user for new entries
function newUser(): SMBUser {
  return { name: '', password: '' }
}

export default function SettingsSMB() {
  const { t } = useTranslation()
  const { success, error: showError } = useToast()
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [config, setConfig] = useState<SMBSettingsData>({
    enabled: false,
    running: false,
    backend: 'builtin',
    system_account: '',
    port: 445,
    server_name: '',
    allow_ips: [],
    shares: [],
    users: [],
  })

  // Share modal state
  const [shareModalOpen, setShareModalOpen] = useState(false)
  const [editingShareIndex, setEditingShareIndex] = useState<number | null>(null)
  const [shareModalData, setShareModalData] = useState<SMBShare>(newShare())
  const [deleteShareIndex, setDeleteShareIndex] = useState<number | null>(null)

  // User modal state
  const [userModalOpen, setUserModalOpen] = useState(false)
  const [editingUserIndex, setEditingUserIndex] = useState<number | null>(null)
  const [userModalData, setUserModalData] = useState<SMBUser>(newUser())
  const [deleteUserIndex, setDeleteUserIndex] = useState<number | null>(null)
  const [showPassword, setShowPassword] = useState(false)

  // Browse directory modal
  const [browseOpen, setBrowseOpen] = useState(false)
  const [browsePath, setBrowsePath] = useState('/')
  const [browseEntries, setBrowseEntries] = useState<Array<{ name: string; path: string; is_dir: boolean }>>([])
  const [browseLoading, setBrowseLoading] = useState(false)
  const [browseError, setBrowseError] = useState<string | null>(null)

  async function loadBrowseDir(path: string) {
    setBrowseLoading(true)
    setBrowseError(null)
    try {
      const res = await api.browseSMBPath(path)
      setBrowsePath(res.data.current)
      setBrowseEntries(res.data.entries || [])
    } catch (err: any) {
      setBrowseError(err.message || t('settings.smbBrowseFailed'))
      setBrowseEntries([])
    } finally {
      setBrowseLoading(false)
    }
  }

  function handleBrowseOpen() {
    // Start from current share path or root
    const sharePath = shareModalData.path
    if (sharePath) {
      // Navigate to parent of current path
      // Handle both Unix (/foo/bar) and Windows (C:\foo\bar) separators
      const sep = sharePath.includes('\\') ? '\\' : '/'
      const parts = sharePath.split(sep).filter(Boolean)
      if (parts.length <= 1) {
        // At drive root or Unix root — show drives / root
        loadBrowseDir('')
      } else {
        // Go up one level
        const parent = parts.length > 1
          ? (sharePath.startsWith('/') ? '/' : parts[0] + sep) + parts.slice(1, -1).join(sep)
          : ''
        loadBrowseDir(parent)
      }
    } else {
      loadBrowseDir('')
    }
    setBrowseOpen(true)
  }

  function handleBrowseSelect(dirPath: string) {
    setShareModalData({ ...shareModalData, path: dirPath })
    handleSharePathChange(dirPath)
    setBrowseOpen(false)
  }

  // Path validation state (for share modal path)
  const [pathValidation, setPathValidation] = useState<{ exists: boolean; writable: boolean } | null>(null)
  const [validatingPath, setValidatingPath] = useState(false)
  const pathValidationTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  // Debounced path validation
  function handleSharePathChange(val: string) {
    setShareModalData({ ...shareModalData, path: val })
    setPathValidation(null)
    if (pathValidationTimer.current) clearTimeout(pathValidationTimer.current)
    if (!val.trim()) return
    pathValidationTimer.current = setTimeout(async () => {
      setValidatingPath(true)
      try {
        const res = await api.validateSMBPath(val.trim())
        setPathValidation({ exists: res.data.exists, writable: res.data.writable })
      } catch {
        setPathValidation(null)
      } finally {
        setValidatingPath(false)
      }
    }, 600)
  }

  // Clean up timer on unmount
  useEffect(() => {
    return () => { if (pathValidationTimer.current) clearTimeout(pathValidationTimer.current) }
  }, [])

  useEffect(() => { loadSettings() }, [])

  async function loadSettings() {
    setLoading(true)
    try {
      const res = await api.getSMB()
      const d = res.data
      setConfig({
        enabled: d.enabled ?? false,
        running: d.running ?? false,
        backend: d.backend || 'builtin',
        system_account: d.system_account || '',
        port: d.port || 445,
        server_name: d.server_name || '',
        allow_ips: d.allow_ips || [],
        shares: d.shares || [],
        users: d.users || [],
      })
    } catch (err: any) {
      showError(err.message || t('settings.loadFailed'))
    } finally {
      setLoading(false)
    }
  }

  async function handleSave() {
    setSaving(true)
    try {
      await api.updateSMB(config)
      success(t('settings.saved'))
    } catch (err: any) {
      showError(err.message || t('settings.saveFailed'))
    } finally {
      setSaving(false)
    }
  }

  // ── Shares ──

  function handleAddShare() {
    setEditingShareIndex(null)
    setShareModalData(newShare())
    setPathValidation(null)
    setShareModalOpen(true)
  }

  function handleEditShare(index: number) {
    setEditingShareIndex(index)
    setShareModalData({ ...config.shares[index] })
    setPathValidation(null)
    setShareModalOpen(true)
  }

  function handleShareModalSave() {
    if (!shareModalData.name?.trim()) {
      showError(t('settings.smbShareNameRequired'))
      return
    }
    if (!shareModalData.path?.trim()) {
      showError(t('settings.smbSharePathRequired'))
      return
    }
    // Check for duplicate share name (exclude current editing index)
    const dupIndex = config.shares.findIndex(
      (s, i) => s.name === shareModalData.name && i !== editingShareIndex
    )
    if (dupIndex !== -1) {
      showError(t('settings.smbShareNameDuplicate', { name: config.shares[dupIndex].name }))
      return
    }

    const shares = [...config.shares]
    if (editingShareIndex !== null) {
      shares[editingShareIndex] = shareModalData
    } else {
      shares.push(shareModalData)
    }
    setConfig({ ...config, shares })
    setShareModalOpen(false)
  }

  function handleDeleteShareConfirm() {
    if (deleteShareIndex === null) return
    setConfig({ ...config, shares: config.shares.filter((_, i) => i !== deleteShareIndex) })
    setDeleteShareIndex(null)
  }

  // ── Users ──

  function handleAddUser() {
    setEditingUserIndex(null)
    setUserModalData(newUser())
    setShowPassword(false)
    setUserModalOpen(true)
  }

  function handleEditUser(index: number) {
    setEditingUserIndex(index)
    // 不回填口令（服务端不回传）；留空 = 保持原口令
    setUserModalData({ name: config.users[index].name, password: '', password_set: config.users[index].password_set })
    setShowPassword(false)
    setUserModalOpen(true)
  }

  function handleUserModalSave() {
    if (!userModalData.name?.trim()) {
      showError(t('settings.smbUserNameRequired'))
      return
    }
    // 新增账号必须给口令；编辑已有账号留空表示保持原口令
    const isNew = editingUserIndex === null
    const willKeep = !isNew && !userModalData.password && userModalData.password_set
    if (!userModalData.password && !willKeep) {
      showError(t('settings.smbUserPasswordRequired'))
      return
    }
    // Check for duplicate user name (exclude current editing index)
    const dupIndex = config.users.findIndex(
      (u, i) => u.name === userModalData.name && i !== editingUserIndex
    )
    if (dupIndex !== -1) {
      showError(t('settings.smbUserNameDuplicate', { name: config.users[dupIndex].name }))
      return
    }

    const users = [...config.users]
    if (editingUserIndex !== null) {
      users[editingUserIndex] = { ...userModalData, password_set: userModalData.password ? true : userModalData.password_set }
    } else {
      users.push({ ...userModalData, password_set: !!userModalData.password })
    }
    setConfig({ ...config, users })
    setUserModalOpen(false)
  }

  function handleDeleteUserConfirm() {
    if (deleteUserIndex === null) return
    setConfig({ ...config, users: config.users.filter((_, i) => i !== deleteUserIndex) })
    setDeleteUserIndex(null)
  }

  // Table columns for shares
  const shareColumns: Column<SMBShare & { _index: number }>[] = [
    {
      key: 'name',
      label: t('settings.smbShareName'),
      render: (s) => <span className="font-medium text-foreground">{s.name}</span>,
    },
    {
      key: 'path',
      label: t('settings.smbSharePath'),
      render: (s) => (
        <code className="font-mono text-xs bg-card px-1.5 py-0.5 rounded truncate max-w-[300px] inline-block">{s.path}</code>
      ),
    },
    {
      key: 'read_only',
      label: t('settings.smbReadOnly'),
      width: '100px',
      render: (s) => (
        <span className={`px-2 py-0.5 text-xs rounded-full ${
          s.read_only
            ? 'bg-blue-500/20 text-blue-400'
            : 'bg-accent-yellow/20 text-accent-yellow'
        }`}>
          {s.read_only ? t('settings.smbReadOnlyYes') : t('settings.smbReadOnlyNo')}
        </span>
      ),
    },
    {
      key: 'allow_users',
      label: t('settings.smbAllowUsers'),
      render: (s) => {
        if (!s.allow_users || s.allow_users.length === 0) {
          return <span className="text-xs text-foreground-muted">—</span>
        }
        return (
          <span className="font-mono text-xs bg-card px-1.5 py-0.5 rounded">
            {s.allow_users.length > 2
              ? `${s.allow_users[0]}, +${s.allow_users.length - 1}`
              : s.allow_users.join(', ')}
          </span>
        )
      },
    },
    {
      key: 'actions',
      label: '',
      width: '80px',
      render: (s) => (
        <div className="flex items-center gap-1 justify-end">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleEditShare(s._index)}
            className="text-foreground-muted hover:text-foreground"
          >
            <Edit2 size={14} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeleteShareIndex(s._index)}
            className="text-accent-red"
          >
            <Trash2 size={14} />
          </Button>
        </div>
      ),
    },
  ]

  // Table columns for users
  const userColumns: Column<SMBUser & { _index: number }>[] = [
    {
      key: 'name',
      label: t('settings.smbUserName'),
      render: (u) => <span className="font-medium text-foreground">{u.name}</span>,
    },
    {
      key: 'password',
      label: t('settings.smbUserPassword'),
      render: (u) => (
        <span className="font-mono text-xs text-foreground-muted">{u.password_set ? '••••••' : '—'}</span>
      ),
    },
    {
      key: 'actions',
      label: '',
      width: '80px',
      render: (u) => (
        <div className="flex items-center gap-1 justify-end">
          <Button
            variant="ghost"
            size="sm"
            onClick={() => handleEditUser(u._index)}
            className="text-foreground-muted hover:text-foreground"
          >
            <Edit2 size={14} />
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={() => setDeleteUserIndex(u._index)}
            className="text-accent-red"
          >
            <Trash2 size={14} />
          </Button>
        </div>
      ),
    },
  ]

  // DataTable data with original index preserved
  const shareTableData = config.shares.map((s) => ({ ...s, _index: config.shares.indexOf(s) }))
  const userTableData = config.users.map((u) => ({ ...u, _index: config.users.indexOf(u) }))

  return (
    <div>
      {/* Header */}
      <PageHeader
        title={t('settings.smbTitle')}
        className="mb-4"
        actions={
          <>
            <Button variant="secondary" size="sm" disabled={loading} onClick={loadSettings}>
              <RefreshCw size={14} /> {t('settings.refresh')}
            </Button>
            <Button variant="primary" size="sm" disabled={saving} onClick={handleSave}>
              <Save size={14} /> {saving ? t('settings.saving') : t('settings.save')}
            </Button>
          </>
        }
      />

      {loading ? (
        <Card>
          <div className="flex items-center justify-center py-16">
            <div className="animate-spin w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full" />
            <span className="ml-3 text-sm text-foreground-muted">{t('settings.loading')}</span>
          </div>
        </Card>
      ) : (
        <>
          {/* 端口冲突提示：未运行且端口不可绑定（Windows 445 常被 LanmanServer 占用） */}
          {!config.running && config.port_available === false && (
            <div className="mb-4 flex items-start gap-2 text-xs text-accent-yellow bg-accent-yellow/10 border border-accent-yellow/30 rounded-lg px-3 py-2 leading-relaxed">
              <AlertCircle size={14} className="shrink-0 mt-0.5" />
              <span>{t('settings.smbPortConflictHint', { port: config.port })}</span>
            </div>
          )}

          {/* Status Bar */}
          <Card className="mb-4">
            <div className="flex items-center gap-6 flex-wrap">
              <div className="flex items-center gap-2">
                <Server size={16} className="text-foreground-muted" />
                <span className="text-xs text-foreground-muted">{t('settings.smbPort')}:</span>
                <span className="text-sm font-semibold text-foreground">{config.port}/TCP</span>
              </div>
              <div className="flex items-center gap-2">
                <Server size={16} className="text-foreground-muted" />
                <span className="text-xs text-foreground-muted">{t('settings.smbServerName')}:</span>
                <span className="text-sm font-semibold text-foreground">{config.server_name || '—'}</span>
              </div>
              <div className="flex items-center gap-2">
                <FolderOpen size={16} className="text-foreground-muted" />
                <span className="text-xs text-foreground-muted">{t('settings.smbShares')}:</span>
                <span className="text-sm font-semibold text-foreground">{config.shares.length}</span>
              </div>
              <div className="flex items-center gap-2">
                <Users size={16} className="text-foreground-muted" />
                <span className="text-xs text-foreground-muted">{t('settings.smbUsers')}:</span>
                <span className="text-sm font-semibold text-foreground">{config.users.length}</span>
              </div>
              <div className="ml-auto flex items-center gap-2">
                <div className={`w-2 h-2 rounded-full ${config.running ? 'bg-accent-green' : 'bg-gray-400'}`} />
                <span className="text-xs text-foreground-muted">{t(`settings.${config.running ? 'smbRunning' : 'smbStopped'}`)}</span>
              </div>
            </div>
          </Card>

          {/* Info note */}
          <Card className="mb-4">
            <div className="flex items-start gap-3">
              <Info size={16} className="text-blue-400 shrink-0 mt-0.5" />
              <div className="space-y-1">
                <p className="text-xs text-foreground-secondary">{t('settings.smbGuestNote')}</p>
                <p className="text-xs text-foreground-secondary">{t('settings.smbRestartNote')}</p>
              </div>
            </div>
          </Card>

          {/* General settings */}
          <Card className="mb-4">
            <h2 className="text-sm font-semibold text-foreground mb-4">{t('settings.smbGeneral')}</h2>
            <div className="space-y-4">
              <div className="flex items-center justify-between py-1">
                <Toggle
                  checked={config.enabled}
                  onChange={v => setConfig({ ...config, enabled: v })}
                  label={t('settings.smbEnabled')}
                />
              </div>
              <SettingsField label={t('settings.smbBackend')} help={t('settings.smbBackendHelp')}>
                <Select
                  value={config.backend}
                  onChange={e => setConfig({ ...config, backend: e.target.value as 'builtin' | 'system' })}
                >
                  <option value="builtin">{t('settings.smbBackendBuiltin')}</option>
                  <option value="system">{t('settings.smbBackendSystem')}</option>
                </Select>
                <p className="mt-1 text-[11px] text-[var(--text-muted)]">{t('settings.smbBackendRestartHint')}</p>
              </SettingsField>

              {config.backend === 'system' ? (
                <>
                  <SettingsField label={t('settings.smbSystemAccount')} help={t('settings.smbSystemAccountHelp')}>
                    <SettingsInput
                      value={config.system_account}
                      onChange={v => setConfig({ ...config, system_account: v })}
                      placeholder={t('settings.smbSystemAccountPlaceholder')}
                    />
                  </SettingsField>
                  <div className="text-xs text-[var(--text-muted)] leading-relaxed bg-[var(--bg-base)] border border-[var(--bg-border)] rounded-lg px-3 py-2 space-y-1.5">
                    <p>{t('settings.smbSystemHint')}</p>
                    <p className="font-mono break-all text-[var(--text-secondary)]">
                      {`net use Z: \\\\${window.location.hostname}\\${config.shares[0]?.name || 'isos'} /user:${config.system_account || '<account>'} <password>`}
                    </p>
                  </div>
                </>
              ) : (
                <>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                    <SettingsField label={t('settings.smbPort')} help={t('settings.smbPortHelp')}>
                      <SettingsInput
                        type="number"
                        min={1}
                        max={65535}
                        value={String(config.port)}
                        onChange={v => setConfig({ ...config, port: Number(v) || 0 })}
                        placeholder="445"
                      />
                    </SettingsField>
                    <SettingsField label={t('settings.smbServerName')} help={t('settings.smbServerNameHelp')}>
                      <SettingsInput
                        value={config.server_name}
                        onChange={v => setConfig({ ...config, server_name: v })}
                        placeholder={t('settings.smbServerNamePlaceholder')}
                      />
                    </SettingsField>
                  </div>
                  <SettingsField label={t('settings.smbAllowIPs')} help={t('settings.smbAllowIPsHelp')}>
                    <StringListEditor
                      value={config.allow_ips}
                      onChange={v => setConfig({ ...config, allow_ips: v })}
                      placeholder={t('settings.smbAllowIPsPlaceholder')}
                    />
                  </SettingsField>
                </>
              )}
            </div>
          </Card>

          {/* Shares List */}
          <Card className="mb-4">
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-foreground">{t('settings.smbShares')}</h2>
              <Button variant="primary" size="sm" onClick={handleAddShare}>
                <Plus size={14} /> {t('settings.smbAddShare')}
              </Button>
            </div>

            <DataTable
              columns={shareColumns}
              data={shareTableData}
              emptyText={t('settings.smbNoShares')}
              rowKey={(s) => `${s._index}`}
            />
          </Card>

          {/* Users List */}
          <Card>
            <div className="flex items-center justify-between mb-4">
              <h2 className="text-sm font-semibold text-foreground">{t('settings.smbUsers')}</h2>
              <Button variant="primary" size="sm" onClick={handleAddUser}>
                <Plus size={14} /> {t('settings.smbAddUser')}
              </Button>
            </div>

            <DataTable
              columns={userColumns}
              data={userTableData}
              emptyText={t('settings.smbNoUsers')}
              rowKey={(u) => `${u._index}`}
            />
          </Card>
        </>
      )}

      {/* Add/Edit Share Modal */}
      <Modal
        open={shareModalOpen}
        onClose={() => setShareModalOpen(false)}
        title={editingShareIndex !== null ? t('settings.smbEditShare') : t('settings.smbAddShare')}
        width="560px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setShareModalOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleShareModalSave}>
              {t('common.save')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <SettingsField label={t('settings.smbShareName')} help={t('settings.smbShareNameHelp')}>
            <SettingsInput
              value={shareModalData.name}
              onChange={v => setShareModalData({ ...shareModalData, name: v })}
              placeholder={t('settings.smbShareNamePlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.smbSharePath')} help={t('settings.smbSharePathHelp')}>
            <div className="flex gap-2">
              <div className="relative flex-1">
                <SettingsInput
                  value={shareModalData.path}
                  onChange={handleSharePathChange}
                  placeholder={t('settings.smbSharePathPlaceholder')}
                />
                {(validatingPath || pathValidation) && (
                  <div className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-1.5 pointer-events-none">
                    {validatingPath ? (
                      <Loader2 size={14} className="animate-spin text-foreground-muted" />
                    ) : pathValidation?.exists ? (
                      <span className="flex items-center gap-1 text-xs text-accent-green">
                        <CheckCircle size={12} />
                        {t('settings.smbPathExists')}{pathValidation.writable ? ` · ${t('settings.smbPathWritable')}` : ` · ${t('settings.smbPathNotWritable')}`}
                      </span>
                    ) : (
                      <span className="flex items-center gap-1 text-xs text-accent-yellow">
                        <XCircle size={12} />
                        {t('settings.smbPathNotExist')}
                      </span>
                    )}
                  </div>
                )}
              </div>
              <Button variant="secondary" size="sm" onClick={handleBrowseOpen} className="shrink-0">
                <FolderOpen size={14} /> {t('settings.smbBrowse')}
              </Button>
            </div>
          </SettingsField>

          <div className="flex items-center justify-between py-2">
            <Toggle
              checked={shareModalData.read_only}
              onChange={v => setShareModalData({ ...shareModalData, read_only: v })}
              label={t('settings.smbReadOnly')}
            />
          </div>
          <p className="text-xs text-foreground-muted -mt-2">{t('settings.smbReadOnlyHelp')}</p>

          <SettingsField label={t('settings.smbAllowUsers')} help={t('settings.smbAllowUsersHelp')}>
            <StringListEditor
              value={shareModalData.allow_users}
              onChange={v => setShareModalData({ ...shareModalData, allow_users: v })}
              placeholder={t('settings.smbUserPlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.smbWriteUsers')} help={t('settings.smbWriteUsersHelp')}>
            <StringListEditor
              value={shareModalData.write_users}
              onChange={v => setShareModalData({ ...shareModalData, write_users: v })}
              placeholder={t('settings.smbUserPlaceholder')}
            />
          </SettingsField>
        </div>
      </Modal>

      {/* Add/Edit User Modal */}
      <Modal
        open={userModalOpen}
        onClose={() => setUserModalOpen(false)}
        title={editingUserIndex !== null ? t('settings.smbEditUser') : t('settings.smbAddUser')}
        width="480px"
        footer={
          <>
            <Button variant="secondary" size="sm" onClick={() => setUserModalOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button variant="primary" size="sm" onClick={handleUserModalSave}>
              {t('common.save')}
            </Button>
          </>
        }
      >
        <div className="space-y-4">
          <SettingsField label={t('settings.smbUserName')} help={t('settings.smbUserNameHelp')}>
            <SettingsInput
              value={userModalData.name}
              onChange={v => setUserModalData({ ...userModalData, name: v })}
              placeholder={t('settings.smbUserNamePlaceholder')}
            />
          </SettingsField>

          <SettingsField label={t('settings.smbUserPassword')} help={t('settings.smbUserPasswordHelp')}>
            <div className="relative">
              <Input
                type={showPassword ? 'text' : 'password'}
                value={userModalData.password}
                onChange={e => setUserModalData({ ...userModalData, password: e.target.value })}
                placeholder={editingUserIndex !== null
                  ? t('settings.smbUserPasswordKeepPlaceholder')
                  : t('settings.smbUserPasswordPlaceholder')}
                className="pr-10"
              />
              <button
                type="button"
                onClick={() => setShowPassword(v => !v)}
                aria-label={showPassword ? t('settings.smbHidePassword') : t('settings.smbShowPassword')}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-foreground-muted hover:text-foreground transition-colors"
              >
                {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
            {editingUserIndex !== null && (
              <p className="mt-1 text-xs text-foreground-muted">{t('settings.smbUserPasswordKeepHint')}</p>
            )}
          </SettingsField>
        </div>
      </Modal>

      {/* Delete Share Confirm */}
      <ConfirmDialog
        open={deleteShareIndex !== null}
        onClose={() => setDeleteShareIndex(null)}
        onConfirm={handleDeleteShareConfirm}
        title={t('settings.smbDeleteShare')}
        message={deleteShareIndex !== null
          ? t('settings.smbDeleteShareConfirm', { name: config.shares[deleteShareIndex]?.name || '' })
          : ''}
        danger
      />

      {/* Delete User Confirm */}
      <ConfirmDialog
        open={deleteUserIndex !== null}
        onClose={() => setDeleteUserIndex(null)}
        onConfirm={handleDeleteUserConfirm}
        title={t('settings.smbDeleteUser')}
        message={deleteUserIndex !== null
          ? t('settings.smbDeleteUserConfirm', { name: config.users[deleteUserIndex]?.name || '' })
          : ''}
        danger
      />

      {/* Browse Directory Modal */}
      <Modal
        open={browseOpen}
        onClose={() => setBrowseOpen(false)}
        title={t('settings.smbBrowse')}
        width="560px"
        footer={
          <Button variant="secondary" size="sm" onClick={() => setBrowseOpen(false)}>
            {t('common.cancel')}
          </Button>
        }
      >
        <div className="space-y-3">
          {/* Current path display + navigate up */}
          <div className="flex items-center gap-2">
            <button
              disabled={!browsePath || browsePath === '/'}
              onClick={() => {
                if (!browsePath) return
                const sep = browsePath.includes('\\') ? '\\' : '/'
                const parts = browsePath.split(sep).filter(Boolean)
                if (parts.length <= 1) {
                  // At drive root or Unix root — go back to drive list / root
                  loadBrowseDir('')
                } else {
                  const parent = parts.length > 1
                    ? (browsePath.startsWith('/') ? '/' : parts[0] + sep) + parts.slice(1, -1).join(sep)
                    : ''
                  loadBrowseDir(parent)
                }
              }}
              className="px-2 py-1 text-xs rounded bg-input text-foreground-secondary hover:bg-hover disabled:opacity-40 disabled:cursor-not-allowed"
            >
              ↑
            </button>
            <code className="flex-1 text-xs font-mono bg-input px-2 py-1 rounded text-foreground truncate">
              {browsePath || t('settings.smbBrowseDrives')}
            </code>
            <Button variant="primary" size="sm" onClick={() => handleBrowseSelect(browsePath)}>
              {t('settings.smbSelectThisDir')}
            </Button>
          </div>

          {/* Directory list */}
          <div className="border border-border rounded-lg max-h-[320px] overflow-y-auto">
            {browseLoading ? (
              <div className="flex items-center justify-center py-8">
                <div className="animate-spin w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full" />
              </div>
            ) : browseError ? (
              <div className="py-8 text-center text-sm text-accent-yellow">{browseError}</div>
            ) : browseEntries.length === 0 ? (
              <div className="py-8 text-center text-sm text-foreground-muted">{t('settings.smbBrowseEmpty')}</div>
            ) : (
              <div>
                {browseEntries.map((entry) => (
                  <button
                    key={entry.path}
                    onClick={() => loadBrowseDir(entry.path)}
                    className="w-full flex items-center gap-2 px-3 py-2 text-left text-sm hover:bg-hover transition-colors border-b border-border last:border-b-0"
                  >
                    <FolderOpen size={14} className="text-accent-yellow shrink-0" />
                    <span className="text-foreground">{entry.name}</span>
                    <span className="ml-auto text-xs text-foreground-muted font-mono">{entry.path}</span>
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>
      </Modal>
    </div>
  )
}
