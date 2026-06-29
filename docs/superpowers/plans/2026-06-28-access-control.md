# Access Control — 黑白名单实现计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add MAC-based blacklist (global) and whitelist (subnet-level) filtering to PxeGo's DHCP handler, with full CRUD API and web UI.

**Architecture:** Config YAML holds toggle switches + seed MAC entries; SQLite stores runtime entries; DHCP handler queries DB on every request (real-time). Two new DB tables, one new API handler, one new frontend page.

**Tech Stack:** Go 1.22+, chi router, GORM SQLite, React with TypeScript

---

### Task 1: Config model — add whitelist toggle + seed entry fields

**Files:**
- Modify: `internal/config/config.go`

- [ ] **Step 1: Add WhitelistEnabled to GlobalConfig**

```go
type GlobalConfig struct {
    DataDir          string `yaml:"data_dir" mapstructure:"data_dir"`
    AppMode          bool   `yaml:"app_mode" mapstructure:"app_mode"`
    ServerName       string `yaml:"server_name" mapstructure:"server_name"`
    ListenAddr       string `yaml:"listen_addr" mapstructure:"listen_addr"`
    WhitelistEnabled bool   `yaml:"whitelist_enabled" mapstructure:"whitelist_enabled"`
}
```

- [ ] **Step 2: Add WhitelistEnabled to SubnetConfig**

```go
type SubnetConfig struct {
    CIDR             string   `yaml:"cidr" mapstructure:"cidr"`
    DHCP             string   `yaml:"dhcp" mapstructure:"dhcp"`
    Pool             string   `yaml:"pool" mapstructure:"pool"`
    Pools            []string `yaml:"pools" mapstructure:"pools"`
    Gateway          string   `yaml:"gateway" mapstructure:"gateway"`
    DNSServers       string   `yaml:"dns_servers" mapstructure:"dns_servers"`
    NextServer       string   `yaml:"next_server" mapstructure:"next_server"`
    LeaseTime        int      `yaml:"lease_time" mapstructure:"lease_time"`
    WhitelistEnabled bool     `yaml:"whitelist_enabled" mapstructure:"whitelist_enabled"`
}
```

- [ ] **Step 3: Add seed entry slices and MACEntry type at the end of the file**

```go
// MACEntry represents a MAC address with optional reason, used for seed entries in config.
type MACEntry struct {
    MAC    string `yaml:"mac"`
    Reason string `yaml:"reason,omitempty"`
}

// WhitelistSeedEntry is a seed entry for whitelists (includes subnet).
type WhitelistSeedEntry struct {
    MAC    string `yaml:"mac"`
    Reason string `yaml:"reason,omitempty"`
    Subnet string `yaml:"subnet"`
}
```

- [ ] **Step 4: Add seed fields to Config struct**

```go
type Config struct {
    ConfigPath       string                  `yaml:"-" json:"-" mapstructure:"-"`
    Global           GlobalConfig            `yaml:"global" mapstructure:"global"`
    DNS              DNSConfig               `yaml:"dns" mapstructure:"dns"`
    Interfaces       []InterfaceConfig       `yaml:"interfaces" mapstructure:"interfaces"`
    Auth             AuthConfig              `yaml:"auth" mapstructure:"auth"`
    Boot             BootConfig              `yaml:"boot" mapstructure:"boot"`
    Netboot          NetbootConfig           `yaml:"netboot" mapstructure:"netboot"`
    Store            StoreConfig             `yaml:"store" mapstructure:"store"`
    Log              LogConfig               `yaml:"log" mapstructure:"log"`
    ServiceAutoStart ServiceAutoStartConfig  `yaml:"service_auto_start" mapstructure:"service_auto_start"`
    BlacklistSeeds   []MACEntry              `yaml:"blacklist,omitempty"`
    WhitelistSeeds   []WhitelistSeedEntry    `yaml:"whitelist,omitempty"`
}
```

- [ ] **Step 5: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 6: Commit**

```bash
git add internal/config/config.go
git commit -m "feat: add whitelist toggle and MAC seed entry fields to config"
```

---

### Task 2: Data model — BlacklistEntry and WhitelistEntry GORM models

**Files:**
- Create: `internal/models/access.go`

- [ ] **Step 1: Create the model file**

```go
package models

import "time"

type BlacklistEntry struct {
    ID        uint      `gorm:"primaryKey" json:"id"`
    MAC       string    `gorm:"uniqueIndex;size:17" json:"mac"`
    Reason    string    `gorm:"size:255" json:"reason"`
    Source    string    `gorm:"size:32;default:db" json:"source"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
}

type WhitelistEntry struct {
    ID         uint      `gorm:"primaryKey" json:"id"`
    MAC        string    `gorm:"index:idx_mac_subnet,unique;size:17" json:"mac"`
    SubnetCIDR string    `gorm:"index:idx_mac_subnet,unique;size:43" json:"subnet_cidr"`
    Reason     string    `gorm:"size:255" json:"reason"`
    Source     string    `gorm:"size:32;default:db" json:"source"`
    CreatedAt  time.Time `json:"created_at"`
    UpdatedAt  time.Time `json:"updated_at"`
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/models/access.go
git commit -m "feat: add BlacklistEntry and WhitelistEntry models"
```

---

### Task 3: Store interface — add BlacklistStore and WhitelistStore

**Files:**
- Modify: `internal/store/store.go`

- [ ] **Step 1: Add interfaces to store.go**

Add these interface definitions after existing ones in `internal/store/store.go`:

```go
type BlacklistStore interface {
    ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error)
    CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error
    DeleteBlacklist(ctx context.Context, id uint) error
    IsBlacklisted(ctx context.Context, mac string) (bool, error)
}

type WhitelistStore interface {
    ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error)
    CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error
    DeleteWhitelist(ctx context.Context, id uint) error
    IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error)
}
```

Then update the main `Interface` to embed them:

```go
type Interface interface {
    HostStore
    ProfileStore
    EventStore
    LeaseStore
    NetbootOverlayStore
    AnswerTemplateStore
    InstallTaskStore
    BlacklistStore
    WhitelistStore
    Close() error
    Migrate() error
    Seed() error
}
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: compile error — `*sqliteStore` doesn't implement the new interfaces yet (Task 4 will fix this)

- [ ] **Step 3: Commit**

```bash
git add internal/store/store.go
git commit -m "feat: add BlacklistStore and WhitelistStore interfaces"
```

---

### Task 4: Store implementation — SQLite CRUD for blacklist/whitelist

**Files:**
- Create: `internal/store/sqlite_access.go`
- Modify: `internal/store/sqlite.go`

- [ ] **Step 1: Create sqlite_access.go**

```go
package store

import (
    "context"

    "github.com/pxego/pxego/internal/models"
)

// ── Blacklist ──

func (s *sqliteStore) ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error) {
    var entries []models.BlacklistEntry
    if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&entries).Error; err != nil {
        return nil, err
    }
    return entries, nil
}

func (s *sqliteStore) CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error {
    return s.db.WithContext(ctx).Create(entry).Error
}

func (s *sqliteStore) DeleteBlacklist(ctx context.Context, id uint) error {
    return s.db.WithContext(ctx).Delete(&models.BlacklistEntry{}, id).Error
}

func (s *sqliteStore) IsBlacklisted(ctx context.Context, mac string) (bool, error) {
    var count int64
    err := s.db.WithContext(ctx).Model(&models.BlacklistEntry{}).
        Where("mac = ?", mac).Count(&count).Error
    return count > 0, err
}

// ── Whitelist ──

func (s *sqliteStore) ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error) {
    var entries []models.WhitelistEntry
    if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&entries).Error; err != nil {
        return nil, err
    }
    return entries, nil
}

func (s *sqliteStore) CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error {
    return s.db.WithContext(ctx).Create(entry).Error
}

func (s *sqliteStore) DeleteWhitelist(ctx context.Context, id uint) error {
    return s.db.WithContext(ctx).Delete(&models.WhitelistEntry{}, id).Error
}

func (s *sqliteStore) IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error) {
    var count int64
    err := s.db.WithContext(ctx).Model(&models.WhitelistEntry{}).
        Where("mac = ? AND subnet_cidr = ?", mac, subnetCIDR).Count(&count).Error
    return count > 0, err
}
```

- [ ] **Step 2: Register models in sqlite.go Migrate()**

Add the two new models to AutoMigrate:

```go
func (s *sqliteStore) Migrate() error {
    return s.db.AutoMigrate(
        &models.Host{},
        &models.Profile{},
        &models.Event{},
        &models.Lease{},
        &models.NetbootOverlay{},
        &models.AnswerTemplate{},
        &models.AnswerTemplateVersion{},
        &models.InstallTask{},
        &models.DNSRecord{},
        &models.BlacklistEntry{},
        &models.WhitelistEntry{},
    )
}
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: no errors (sqliteStore now implements all interfaces)

- [ ] **Step 4: Commit**

```bash
git add internal/store/sqlite_access.go internal/store/sqlite.go
git commit -m "feat: implement SQLite CRUD for blacklist/whitelist entries"
```

---

### Task 5: DHCP handler — add blacklist/whitelist interception chain

**Files:**
- Modify: `internal/dhcp/handler.go`

- [ ] **Step 1: In `Handle()` — after line ~246 (`if subnetCfg == nil { ... return }`) and before `var nextServer net.IP`, insert the access control checks**

Find the block:
```go
    if subnetCfg == nil {
        slog.Warn("未找到匹配子网", "mac", mac)
        return
    }

    slog.Info("子网匹配成功",
        "mac", mac,
        ...
    )

    var nextServer net.IP
```

Replace with (keeping the slog.Info block as-is):

```go
    if subnetCfg == nil {
        slog.Warn("未找到匹配子网", "mac", mac)
        return
    }

    // ── 黑白名单检查 ──

    // 1. 全局黑名单
    blacklisted, err := h.store.IsBlacklisted(ctx, mac)
    if err != nil {
        slog.Error("黑名单查询失败", "mac", mac, "error", err)
    } else if blacklisted {
        slog.Info("黑名单 MAC 已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
        h.eventBus.Publish("event", models.Event{
            Type:    models.EventDHCP,
            Level:   models.EventWarn,
            Message: fmt.Sprintf("黑名单 MAC 已拒绝: %s", mac),
            MAC:     &mac,
        })
        return
    }

    // 2. 全局白名单
    if h.config.Global.WhitelistEnabled {
        whitelisted, err := h.store.IsWhitelisted(ctx, mac, subnetCfg.CIDR)
        if err != nil {
            slog.Error("白名单查询失败", "mac", mac, "error", err)
        } else if !whitelisted {
            slog.Info("全局白名单未命中，已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
            h.eventBus.Publish("event", models.Event{
                Type:    models.EventDHCP,
                Level:   models.EventWarn,
                Message: fmt.Sprintf("全局白名单拒绝: %s (subnet %s)", mac, subnetCfg.CIDR),
                MAC:     &mac,
            })
            return
        }
    }

    // 3. 子网级白名单
    if subnetCfg.WhitelistEnabled {
        whitelisted, err := h.store.IsWhitelisted(ctx, mac, subnetCfg.CIDR)
        if err != nil {
            slog.Error("白名单查询失败", "mac", mac, "error", err)
        } else if !whitelisted {
            slog.Info("子网白名单未命中，已拒绝", "mac", mac, "cidr", subnetCfg.CIDR)
            h.eventBus.Publish("event", models.Event{
                Type:    models.EventDHCP,
                Level:   models.EventWarn,
                Message: fmt.Sprintf("子网白名单拒绝: %s (subnet %s)", mac, subnetCfg.CIDR),
                MAC:     &mac,
            })
            return
        }
    }

    slog.Info("子网匹配成功",
        "mac", mac,
        ...
    )

    var nextServer net.IP
```

- [ ] **Step 2: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add internal/dhcp/handler.go
git commit -m "feat: add blacklist/whitelist interception chain in DHCP handler"
```

---

### Task 6: API handler — CRUD endpoints for blacklist/whitelist

**Files:**
- Create: `internal/api/access.go`
- Modify: `internal/api/handler.go`

- [ ] **Step 1: Create internal/api/access.go**

```go
package api

import (
    "encoding/json"
    "net/http"
    "strconv"

    "github.com/go-chi/chi/v5"
    "github.com/pxego/pxego/internal/models"
    "github.com/pxego/pxego/internal/store"
)

type AccessHandler struct {
    store store.Interface
}

func NewAccessHandler(st store.Interface) *AccessHandler {
    return &AccessHandler{store: st}
}

// ── Blacklist ──

func (h *AccessHandler) ListBlacklist(w http.ResponseWriter, r *http.Request) {
    entries, err := h.store.ListBlacklist(r.Context())
    if err != nil {
        Error(w, http.StatusInternalServerError, "查询黑名单失败")
        return
    }
    OK(w, entries)
}

type createBlacklistRequest struct {
    MAC    string `json:"mac"`
    Reason string `json:"reason,omitempty"`
}

func (h *AccessHandler) CreateBlacklist(w http.ResponseWriter, r *http.Request) {
    var req createBlacklistRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        Error(w, http.StatusBadRequest, "请求格式错误")
        return
    }
    if req.MAC == "" {
        Error(w, http.StatusBadRequest, "MAC 地址不能为空")
        return
    }
    entry := &models.BlacklistEntry{
        MAC:    req.MAC,
        Reason: req.Reason,
        Source: "db",
    }
    if err := h.store.CreateBlacklist(r.Context(), entry); err != nil {
        Error(w, http.StatusConflict, "MAC 已存在于黑名单")
        return
    }
    OK(w, entry)
}

func (h *AccessHandler) DeleteBlacklist(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        Error(w, http.StatusBadRequest, "无效的 ID")
        return
    }
    if err := h.store.DeleteBlacklist(r.Context(), uint(id)); err != nil {
        Error(w, http.StatusNotFound, "条目未找到")
        return
    }
    OK(w, map[string]string{"status": "deleted"})
}

// ── Whitelist ──

func (h *AccessHandler) ListWhitelist(w http.ResponseWriter, r *http.Request) {
    entries, err := h.store.ListWhitelist(r.Context())
    if err != nil {
        Error(w, http.StatusInternalServerError, "查询白名单失败")
        return
    }
    OK(w, entries)
}

type createWhitelistRequest struct {
    MAC        string `json:"mac"`
    SubnetCIDR string `json:"subnet_cidr"`
    Reason     string `json:"reason,omitempty"`
}

func (h *AccessHandler) CreateWhitelist(w http.ResponseWriter, r *http.Request) {
    var req createWhitelistRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        Error(w, http.StatusBadRequest, "请求格式错误")
        return
    }
    if req.MAC == "" || req.SubnetCIDR == "" {
        Error(w, http.StatusBadRequest, "MAC 和子网 CIDR 不能为空")
        return
    }
    entry := &models.WhitelistEntry{
        MAC:        req.MAC,
        SubnetCIDR: req.SubnetCIDR,
        Reason:     req.Reason,
        Source:     "db",
    }
    if err := h.store.CreateWhitelist(r.Context(), entry); err != nil {
        Error(w, http.StatusConflict, "该 MAC 已在此子网的白名单中")
        return
    }
    OK(w, entry)
}

func (h *AccessHandler) DeleteWhitelist(w http.ResponseWriter, r *http.Request) {
    idStr := chi.URLParam(r, "id")
    id, err := strconv.ParseUint(idStr, 10, 64)
    if err != nil {
        Error(w, http.StatusBadRequest, "无效的 ID")
        return
    }
    if err := h.store.DeleteWhitelist(r.Context(), uint(id)); err != nil {
        Error(w, http.StatusNotFound, "条目未找到")
        return
    }
    OK(w, map[string]string{"status": "deleted"})
}
```

- [ ] **Step 2: Wire routes in handler.go**

In `internal/api/handler.go`, add the AccessHandler field:

```go
type Handler struct {
    Host           *HostHandler
    Profile        *ProfileHandler
    Event          *EventHandler
    File           *FileHandler
    WOL            *WOLHandler
    IPMI           *IPMIHandler
    Lease          *LeaseHandler
    Settings       *SettingsHandler
    Logs           *LogStreamHandler
    Netboot        *NetbootHandler
    NetbootOverlay *NetbootOverlayHandler
    AnswerTemplate *AnswerTemplateHandler
    InstallTask    *InstallTaskHandler
    Service        *ServiceHandler
    Auth           *AuthHandler
    Access         *AccessHandler
    svcController  ServiceController
    sessions       *session.Store
}
```

In `NewHandler()`, initialize it:

```go
h := &Handler{
    // ... existing ...
    Access:         NewAccessHandler(st),
    // ...
}
```

In `RegisterRoutes()`, add routes:

```go
// Access control
r.Get("/access/blacklist", h.Access.ListBlacklist)
r.Post("/access/blacklist", h.Access.CreateBlacklist)
r.Delete("/access/blacklist/{id}", h.Access.DeleteBlacklist)
r.Get("/access/whitelist", h.Access.ListWhitelist)
r.Post("/access/whitelist", h.Access.CreateWhitelist)
r.Delete("/access/whitelist/{id}", h.Access.DeleteWhitelist)
```

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add internal/api/access.go internal/api/handler.go
git commit -m "feat: add CRUD API for blacklist/whitelist entries"
```

---

### Task 7: Settings API — pass whitelist_enabled toggle

**Files:**
- Modify: `internal/api/settings.go`

- [ ] **Step 1: Extend SettingsResponse with whitelist_enabled**

```go
type SettingsResponse struct {
    Server           ServerSettings      `json:"server"`
    DHCP             DHCPSettings        `json:"dhcp"`
    TFTP             TFTPSettings        `json:"tftp"`
    DNS              DNSSettings         `json:"dns"`
    IPMI             IPMISettings        `json:"ipmi"`
    Netboot          NetbootSettings     `json:"netboot"`
    WhitelistEnabled bool                `json:"whitelist_enabled"`
    LogLevel         string              `json:"log_level"`
    DataDir          string              `json:"data_dir"`
    Interfaces       []InterfaceResponse `json:"interfaces"`
}
```

- [ ] **Step 2: Set the value in Get() handler**

After `resp := SettingsResponse{`, add:
```go
WhitelistEnabled: cfg.Global.WhitelistEnabled,
```

- [ ] **Step 3: Read the value in Update() handler**

After `h.cfg.Netboot.Boot = ...`, add:
```go
h.cfg.Global.WhitelistEnabled = req.WhitelistEnabled
```

- [ ] **Step 4: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 5: Commit**

```bash
git add internal/api/settings.go
git commit -m "feat: expose whitelist_enabled toggle through settings API"
```

---

### Task 8: Seed import at startup

**Files:**
- Modify: `cmd/pxego/main.go`

- [ ] **Step 1: Add seed import after st.Seed() in run()**

Find the block:
```go
    if err := st.Seed(); err != nil {
        return fmt.Errorf("初始化默认数据失败: %w", err)
    }
    defer st.Close()
```

Insert after `st.Seed()` but before `defer st.Close()`:

```go
    // 导入黑白名单种子（幂等——MAC 已存在则跳过）
    for _, entry := range cfg.BlacklistSeeds {
        exists, err := st.IsBlacklisted(context.Background(), entry.MAC)
        if err != nil {
            slog.Warn("查询黑名单种子失败", "mac", entry.MAC, "error", err)
            continue
        }
        if !exists {
            if err := st.CreateBlacklist(context.Background(), &models.BlacklistEntry{
                MAC:    entry.MAC,
                Reason: entry.Reason,
                Source: "seed",
            }); err != nil {
                slog.Warn("导入黑名单种子失败", "mac", entry.MAC, "error", err)
            } else {
                slog.Info("已导入黑名单种子", "mac", entry.MAC)
            }
        }
    }
    for _, entry := range cfg.WhitelistSeeds {
        exists, err := st.IsWhitelisted(context.Background(), entry.MAC, entry.Subnet)
        if err != nil {
            slog.Warn("查询白名单种子失败", "mac", entry.MAC, "error", err)
            continue
        }
        if !exists {
            if err := st.CreateWhitelist(context.Background(), &models.WhitelistEntry{
                MAC:        entry.MAC,
                SubnetCIDR: entry.Subnet,
                Reason:     entry.Reason,
                Source:     "seed",
            }); err != nil {
                slog.Warn("导入白名单种子失败", "mac", entry.MAC, "error", err)
            } else {
                slog.Info("已导入白名单种子", "mac", entry.MAC, "subnet", entry.Subnet)
            }
        }
    }
```

- [ ] **Step 2: Add models import if not already present**

Check the imports at top of `cmd/pxego/main.go` — `models` should already be imported via other uses. If not, add it.

- [ ] **Step 3: Build and verify**

Run: `go build ./...`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add cmd/pxego/main.go
git commit -m "feat: import blacklist/whitelist seed entries from config at startup"
```

---

### Task 9: Frontend — add API methods for access control

**Files:**
- Modify: `web/src/api/client.ts`

- [ ] **Step 1: Add blacklist API types and functions**

```typescript
// ── Access Control: Blacklist / Whitelist ──
export interface BlacklistEntry {
  id: number
  mac: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export interface WhitelistEntry {
  id: number
  mac: string
  subnet_cidr: string
  reason: string
  source: string
  created_at: string
  updated_at: string
}

export function getBlacklist(): Promise<ApiResponse<BlacklistEntry[]>> {
  return request<BlacklistEntry[]>('GET', '/access/blacklist')
}

export function createBlacklistEntry(data: { mac: string; reason?: string }): Promise<ApiResponse<BlacklistEntry>> {
  return request<BlacklistEntry>('POST', '/access/blacklist', data)
}

export function deleteBlacklistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/blacklist/${id}`)
}

export function getWhitelist(): Promise<ApiResponse<WhitelistEntry[]>> {
  return request<WhitelistEntry[]>('GET', '/access/whitelist')
}

export function createWhitelistEntry(data: { mac: string; subnet_cidr: string; reason?: string }): Promise<ApiResponse<WhitelistEntry>> {
  return request<WhitelistEntry>('POST', '/access/whitelist', data)
}

export function deleteWhitelistEntry(id: number): Promise<ApiResponse<unknown>> {
  return request<unknown>('DELETE', `/access/whitelist/${id}`)
}
```

- [ ] **Step 2: Add to api convenience namespace**

```typescript
export const api = {
  // ... existing ...
  getBlacklist,
  createBlacklistEntry,
  deleteBlacklistEntry,
  getWhitelist,
  createWhitelistEntry,
  deleteWhitelistEntry,
}
```

- [ ] **Step 3: Build check**

Run: `npx tsc --noEmit` (or just verify no syntax errors)

- [ ] **Step 4: Commit**

```bash
git add web/src/api/client.ts
git commit -m "feat: add access control API methods to frontend client"
```

---

### Task 10: Frontend — AccessControl page

**Files:**
- Create: `web/src/pages/AccessControl.tsx`

- [ ] **Step 1: Create the page component**

```tsx
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
```

- [ ] **Step 2: Commit**

```bash
git add web/src/pages/AccessControl.tsx
git commit -m "feat: add AccessControl page with blacklist/whitelist management"
```

---

### Task 11: Frontend — navigation and routing

**Files:**
- Modify: `web/src/App.tsx`
- Modify: `web/src/components/layout/AppShell.tsx`

- [ ] **Step 1: Add lazy import and route in App.tsx**

Add import:
```tsx
const AccessControl = lazy(() => import('./pages/AccessControl'))
```

Add route (after the answer-templates route):
```tsx
<Route path="/netboot/access-control" element={<AccessControl />} />
```

- [ ] **Step 2: Add nav item in AppShell.tsx**

In the `nav.section.manage` section, add after the answer templates item:
```tsx
{ path: '/netboot/access-control', label: '访问控制', icon: Shield },
```

And import `Shield` from lucide-react:
```tsx
import {
  LayoutDashboard, Server, FileCode, FolderOpen, Activity, Settings,
  Monitor, Menu, Shield,
} from 'lucide-react'
```

- [ ] **Step 3: Commit**

```bash
git add web/src/App.tsx web/src/components/layout/AppShell.tsx
git commit -m "feat: add access control route and navigation item"
```

---

### Task 12: Frontend — add global whitelist toggle to Settings page

**Files:**
- Modify: `web/src/pages/Settings.tsx`
- Modify: `web/src/api/client.ts` (SettingsData type)

- [ ] **Step 1: Add whitelist_enabled to SettingsData interface in client.ts**

```typescript
export interface SettingsData {
  whitelist_enabled?: boolean
  // ... rest
}
```

- [ ] **Step 2: Add toggle to Settings.tsx General tab**

Near the other toggles (autoOpen, persistEvents), add:
```tsx
<Toggle checked={config.whitelistEnabled ?? false} onChange={v => setConfig({...config, whitelistEnabled: v})} label="全局白名单开关" />
<p className="text-xs text-[var(--text-muted)] mt-1">开启后所有子网默认需在白名单中的 MAC 才能引导（子网级独立开关可覆盖此设置）。</p>
```

Add `whitelistEnabled` to the config state initialization:
```tsx
const [config, setConfig] = useState({
    // ... existing ...
    whitelistEnabled: false,
    // ...
})
```

Pass it in the API data on save:
```tsx
const data: SettingsData = {
    whitelist_enabled: config.whitelistEnabled,
    // ... rest
}
```

Read it back from API response:
```tsx
whitelistEnabled: d.whitelist_enabled ?? false,
```

- [ ] **Step 3: Commit**

```bash
git add web/src/pages/Settings.tsx web/src/api/client.ts
git commit -m "feat: add global whitelist toggle to settings page"
```

---

### Self-Review

**Spec coverage:**
- Config model: Task 1 covers WhitelistEnabled in GlobalConfig + SubnetConfig + seed fields ✓
- Data model: Task 2 covers BlacklistEntry + WhitelistEntry ✓
- Store interface: Task 3 covers BlacklistStore + WhitelistStore ✓
- Store implementation: Task 4 covers SQLite CRUD + Migrate ✓
- DHCP interception: Task 5 covers the 3-step chain (blacklist → global whitelist → subnet whitelist) ✓
- API endpoints: Task 6 covers all 6 CRUD endpoints ✓
- Settings API: Task 7 covers whitelist_enabled pass-through ✓
- Seed import: Task 8 covers startup merge ✓
- Frontend API: Task 9 covers all access control API methods ✓
- Frontend page: Task 10 covers dual-column blacklist/whitelist management ✓
- Navigation/routing: Task 11 covers AppShell menu + App.tsx route ✓
- Settings toggle: Task 12 covers the global whitelist toggle in Settings General tab ✓

**Placeholder check:** No TBD, TODO, or incomplete sections. All code blocks contain complete implementations.

**Type consistency:** Method signatures match across tasks — `IsBlacklisted(mac)` / `IsWhitelisted(mac, subnetCIDR)`, `CreateBlacklist(entry)` / `CreateWhitelist(entry)`. API paths `/access/blacklist` and `/access/whitelist` consistent across frontend and backend.
