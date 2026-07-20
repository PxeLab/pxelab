# PxeLab 功能实现计划

## 功能一：审计日志（Audit Log）

### 1. 概述

在现有 `Event` 模型（用于网络服务事件 DHCP/TFTP/HTTP 等）基础上，新增独立的 `AuditLog` 模型，记录所有用户发起的变更操作（CREATE/UPDATE/DELETE），持久化至 SQLite，通过 eventbus 异步发布，提供 API 查询和前端页面展示。

---

### 2. 需要创建/修改的文件

| 文件路径 | 类型 | 说明 |
|---------|------|------|
| `internal/models/audit_log.go` | **新建** | AuditLog 数据模型 |
| `internal/store/audit_log.go` | **新建** | AuditLog 存储接口定义 |
| `internal/store/sqlite_audit.go` | **新建** | AuditLog SQLite 实现 |
| `internal/api/audit.go` | **新建** | AuditLog API Handler |
| `internal/middleware/audit.go` | **新建** | 审计日志中间件/拦截器 |
| `internal/api/handler.go` | **修改** | 注册 AuditHandler、路由 |
| `internal/store/store.go` | **修改** | Interface 添加 AuditLogStore |
| `internal/store/sqlite.go` | **修改** | Migrate 添加 AuditLog AutoMigrate |
| `web/src/api/client.ts` | **修改** | 添加审计日志 API 调用和类型 |
| `web/src/pages/AuditLogs.tsx` | **新建** | 审计日志页面 |
| `web/src/App.tsx` | **修改** | 添加路由 `/audit-logs` |
| `web/src/components/layout/AppShell.tsx` | **修改** | 侧边栏添加导航项 |
| `web/src/locales/zh-CN.json` | **修改** | 添加中文翻译 |
| `web/src/locales/en.json` | **修改** | 添加英文翻译 |

---

### 3. 数据结构

#### 3.1 Go Model — `internal/models/audit_log.go`

```go
package models

import "time"

type AuditAction string

const (
    AuditCreate AuditAction = "CREATE"
    AuditUpdate AuditAction = "UPDATE"
    AuditDelete AuditAction = "DELETE"
)

type AuditLog struct {
    ID        uint        `json:"id" gorm:"primaryKey;autoIncrement"`
    RemoteIP  string      `json:"remote_ip" gorm:"index"`
    Source    string      `json:"source" gorm:"index"`     // "api"
    Resource  string      `json:"resource" gorm:"index"`   // "host", "profile", "dns_record" 等
    Action    AuditAction `json:"action" gorm:"index"`     // CREATE/UPDATE/DELETE
    ResourceID string     `json:"resource_id" gorm:"index"` // 被操作资源的 ID
    Detail    string      `json:"detail"`                  // JSON: 变更摘要或请求体
    Method    string      `json:"method"`                  // HTTP 方法: POST/PUT/DELETE
    Path      string      `json:"path"`                    // 请求路径
    CreatedAt time.Time   `json:"created_at" gorm:"index"`
}
```

#### 3.2 Store 接口 — `internal/store/audit_log.go`

```go
package store

import (
    "context"
    "github.com/pxelab/pxelab/internal/models"
)

type AuditLogFilter struct {
    Resource string
    Action   string
    RemoteIP string
    Search   string
    From     *time.Time
    To       *time.Time
    Page     int
    Size     int
}

type AuditLogStore interface {
    ListAuditLogs(ctx context.Context, filter AuditLogFilter) ([]models.AuditLog, int64, error)
    CreateAuditLog(ctx context.Context, entry *models.AuditLog) error
    PruneAuditLogs(ctx context.Context, before time.Time) error
}
```

#### 3.3 前端 TypeScript 类型 — `web/src/api/client.ts` 新增

```typescript
export interface AuditLog {
  id: number
  remote_ip: string
  source: string
  resource: string
  action: string
  resource_id: string
  detail: string
  method: string
  path: string
  created_at: string
}

export interface AuditLogFilter {
  resource?: string
  action?: string
  remote_ip?: string
  search?: string
  page?: number
  size?: number
}
```

---

### 4. API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/audit-logs` | 分页查询审计日志，支持 resource/action/remote_ip/search/from/to 过滤 |

查询参数:
- `resource` — 资源类型过滤（host, profile, dns_record, dhcp_reservation 等）
- `action` — 操作类型过滤（CREATE/UPDATE/DELETE）
- `remote_ip` — 来源 IP 过滤
- `search` — 模糊搜索（resource_id + detail）
- `from` / `to` — 时间范围（RFC3339）
- `page` / `size` — 分页

响应格式:
```json
{
  "success": true,
  "data": {
    "audit_logs": [...],
    "meta": { "page": 1, "size": 50, "total": 123 }
  }
}
```

---

### 5. 详细实现步骤

#### 5.1 Step 1: 数据模型 — `internal/models/audit_log.go`

- 新建文件，定义 `AuditAction` 常量（CREATE/UPDATE/DELETE）
- 定义 `AuditLog` 结构体，使用 GORM tag 标注
- `Detail` 字段存储 JSON 字符串（变更摘要），避免 GORM 复杂 JSON 类型

#### 5.2 Step 2: Store 接口与实现

**`internal/store/audit_log.go`（新建）：**
- 定义 `AuditLogFilter` 结构体
- 定义 `AuditLogStore` 接口

**`internal/store/store.go`（修改）：**
- `Interface` 添加 `AuditLogStore` 嵌入

**`internal/store/sqlite_audit.go`（新建）：**
- `ListAuditLogs`: 支持多条件过滤 + 分页
- `CreateAuditLog`: 插入记录
- `PruneAuditLogs`: 按时间清理旧记录

**`internal/store/sqlite.go`（修改）：**
- `Migrate()` 的 AutoMigrate 列表中添加 `&models.AuditLog{}`

#### 5.3 Step 3: 审计拦截中间件 — `internal/middleware/audit.go`（新建）

**核心设计思路：**

创建一个 `AuditInterceptor`，不是传统 HTTP 中间件，而是一个**函数拦截器**，用于包装已有的 API Handler 方法。原因是：
- 现有 Handler 方法直接绑定到 `chi.Router`，无法通过中间件区分具体操作
- 需要知道操作类型（CREATE/UPDATE/DELETE）和资源类型

**方案：使用 `eventbus` 异步记录 + Handler 层主动调用**

在每个变更操作的 Handler 方法中，在操作成功后调用一个统一的 `recordAudit` 函数，通过 eventbus 发布审计事件。

```go
// internal/middleware/audit.go
package middleware

import (
    "encoding/json"
    "net/http"
    "strings"

    "github.com/pxelab/pxelab/internal/eventbus"
    "github.com/pxelab/pxelab/internal/models"
)

type AuditEvent struct {
    RemoteIP   string
    Method     string
    Path       string
    Resource   string
    Action     models.AuditAction
    ResourceID string
    Detail     any // 会被序列化为 JSON
}

// RecordAudit 通过 eventbus 异步发布审计事件
func RecordAudit(bus *eventbus.Bus, r *http.Request, resource string, action models.AuditAction, resourceID string, detail any) {
    remoteIP := r.RemoteAddr
    if host, _, err := net.SplitHostPort(remoteIP); err == nil {
        remoteIP = host
    }

    // 也尝试从 X-Forwarded-For 获取
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        parts := strings.Split(xff, ",")
        remoteIP = strings.TrimSpace(parts[0])
    }

    detailJSON := ""
    if detail != nil {
        if b, err := json.Marshal(detail); err == nil {
            detailJSON = string(b)
        }
    }

    bus.PublishAsync("audit", models.AuditLog{
        RemoteIP:   remoteIP,
        Source:     "api",
        Method:     r.Method,
        Path:       r.URL.Path,
        Resource:   resource,
        Action:     action,
        ResourceID: resourceID,
        Detail:     detailJSON,
    })
}
```

#### 5.4 Step 4: API Handler — `internal/api/audit.go`（新建）

```go
package api

type AuditHandler struct {
    store store.Interface
    bus   *eventbus.Bus
}

func NewAuditHandler(st store.Interface, bus *eventbus.Bus) *AuditHandler

func (h *AuditHandler) List(w http.ResponseWriter, r *http.Request) {
    // 解析查询参数: resource, action, remote_ip, search, from, to, page, size
    // 调用 store.ListAuditLogs
    // 返回分页结果
}
```

#### 5.5 Step 5: 在各变更 Handler 中插入审计调用

需要在以下 Handler 的 Create/Update/Delete 方法中添加审计调用：

| Handler 文件 | 需要记录的方法 |
|-------------|-------------|
| `hosts.go` | Create, Update, Delete |
| `profiles.go` | Create, Update, Delete, CreateFromNetboot |
| `dns_records.go` | Create, Update, Delete |
| `dhcp_reservation.go` | Create, Update, Delete |
| `access.go` | CreateBlacklist, DeleteBlacklist, CreateWhitelist, DeleteWhitelist |
| `answer_templates.go` | Create, Update, Delete |
| `install_tasks.go` | Create, Update, Delete |
| `netboot_overlay.go` | Upsert, Delete |
| `bmc_handler.go` | Create, Update, Delete, ImportCSV, PowerOn, PowerOff, Restart |
| `leases.go` | Delete, BatchDelete, Prune |
| `os_images.go` | Upload, Delete |
| `settings.go` | UpdateGeneral, UpdateInterfaces, UpdateDHCP, UpdateDNS, UpdateNFS, UpdateTFTP, UpdateArchMap, UpdateNetboot |
| `wol_handler.go` | Wake, BatchWake, DeleteHistory, DeleteAllHistory, CreateSchedule, DeleteSchedule |
| `files.go` | Upload, Delete |
| `script_handler.go` | Save, Sync, Rollback |
| `auth.go` | Login（记录登录事件） |

**实现方式：** 在每个方法中，操作成功后调用：

```go
middleware.RecordAudit(h.bus, r, "host", models.AuditCreate, host.ID, host)
```

**注意：** 这需要给每个 Handler 添加 `eventbus.Bus` 引用。有两种方案：

**方案 A（推荐）：给 Handler 结构体添加 `bus` 字段**

修改 `Handler` 结构体，添加 `bus *eventbus.Bus` 字段。在 `NewHandler` 中赋值。各子 Handler 从父 Handler 获取 bus。

**方案 B：使用独立的 `AuditRecorder` 服务**

创建一个独立的 `AuditRecorder` 结构体，接收 eventbus 和 store，在初始化时订阅 `"audit"` topic 并持久化。

推荐 **方案 B**，因为：
- 不需要修改现有 Handler 的签名
- 与现有 Event 系统模式一致（EventHandler 订阅 "event" topic）
- 持久化逻辑集中在一处

#### 5.6 Step 6: 审计日志订阅器 — `internal/api/audit.go` 中

```go
func NewAuditHandler(st store.Interface, bus *eventbus.Bus) *AuditHandler {
    h := &AuditHandler{store: st, bus: bus}
    // 订阅 "audit" topic，异步持久化
    bus.Subscribe("audit", func(e eventbus.Event) {
        if log, ok := e.Payload.(models.AuditLog); ok {
            if err := st.CreateAuditLog(context.Background(), &log); err != nil {
                slog.Warn("审计日志写入失败", "error", err)
            }
        }
    })
    return h
}
```

#### 5.7 Step 7: 路由注册 — `internal/api/handler.go` 修改

```go
// Handler 结构体添加字段
type Handler struct {
    // ... 现有字段 ...
    Audit *AuditHandler
}

// NewHandler 中初始化
h.Audit = NewAuditHandler(st, bus)

// RegisterRoutes 中添加路由
r.Get("/audit-logs", h.Audit.List)
```

#### 5.8 Step 8: 前端 API 客户端 — `web/src/api/client.ts` 修改

```typescript
// 添加接口类型
export interface AuditLog { ... }

// 添加 API 函数
export function getAuditLogs(params?: Record<string, unknown>): Promise<
  ApiResponse<{ audit_logs: AuditLog[]; meta: { page: number; size: number; total: number } }>
> {
  return request('GET', '/audit-logs' + buildQuery(params))
}

// 在 export default 中添加
export { getAuditLogs }
```

#### 5.9 Step 9: 前端页面 — `web/src/pages/AuditLogs.tsx`（新建）

页面结构参考 `Events.tsx` 的模式：

```
┌─────────────────────────────────────────────────┐
│ 审计日志                         [过滤器 chips]  │
├─────────────────────────────────────────────────┤
│ 表头: 时间 | 来源IP | 资源类型 | 操作 | 资源ID  │
│        详情                                       │
├─────────────────────────────────────────────────┤
│ 数据行...                                        │
├─────────────────────────────────────────────────┤
│ [分页组件]                                        │
└─────────────────────────────────────────────────┘
```

**组件要点：**
- 使用 `Card` + `Pagination` + `Tag` 组件（与 Events.tsx 一致）
- 过滤 chips：全部 / CREATE / UPDATE / DELETE
- 资源类型下拉过滤
- 操作列用彩色 Tag 区分：CREATE=green, UPDATE=blue, DELETE=red
- 点击行可展开查看 Detail JSON
- 不需要 SSE 实时流（审计日志是低频操作）

#### 5.10 Step 10: 路由和导航 — `App.tsx` + `AppShell.tsx`

**`App.tsx`：**
```tsx
const AuditLogs = lazy(() => import('./pages/AuditLogs'))
// 在 Routes 中添加
<Route path="/audit-logs" element={<AuditLogs />} />
```

**`AppShell.tsx`：**
在 `navSections` 的 `nav.section.monitor` 分组中添加：
```tsx
{ path: '/audit-logs', label: 'nav.auditLogs', icon: Activity }
```

#### 5.11 Step 11: 国际化翻译

**`web/src/locales/zh-CN.json`：**
```json
{
  "nav.auditLogs": "审计日志",
  "audit.title": "审计日志",
  "audit.all": "全部",
  "audit.create": "创建",
  "audit.update": "更新",
  "audit.delete": "删除",
  "audit.remoteIP": "来源 IP",
  "audit.resource": "资源类型",
  "audit.action": "操作",
  "audit.resourceID": "资源 ID",
  "audit.detail": "详情",
  "audit.time": "时间",
  "audit.method": "方法",
  "audit.path": "路径",
  "audit.noLogs": "暂无审计日志",
  "audit.loadFailed": "加载审计日志失败",
  "audit.filterResource": "资源类型",
  "audit.filterAction": "操作类型"
}
```

**`web/src/locales/en.json`：**
```json
{
  "nav.auditLogs": "Audit Logs",
  "audit.title": "Audit Logs",
  "audit.all": "All",
  "audit.create": "Create",
  "audit.update": "Update",
  "audit.delete": "Delete",
  "audit.remoteIP": "Remote IP",
  "audit.resource": "Resource",
  "audit.action": "Action",
  "audit.resourceID": "Resource ID",
  "audit.detail": "Detail",
  "audit.time": "Time",
  "audit.method": "Method",
  "audit.path": "Path",
  "audit.noLogs": "No audit logs",
  "audit.loadFailed": "Failed to load audit logs",
  "audit.filterResource": "Resource Type",
  "audit.filterAction": "Action Type"
}
```

---

### 6. 迁移/兼容性

- **数据库迁移：** `AutoMigrate` 会自动创建 `audit_logs` 表，无需手动 SQL
- **零破坏性：** 不修改任何现有表结构，不影响现有 Event 系统
- **性能：** 审计日志通过 eventbus 异步写入，不阻塞 API 响应
- **存储清理：** 可在 `PruneAuditLogs` 中实现定期清理（如保留 90 天）

---

### 7. 测试要点

- 验证各 CRUD 操作后 audit_logs 表中有对应记录
- 验证 remote_ip 正确提取（含 X-Forwarded-For）
- 验证分页和过滤查询正确
- 验证 Detail 字段包含有意义的变更摘要
- 验证高频操作下性能（异步写入不阻塞）
- 验证 GET 操作不产生审计记录
- 前端页面渲染、分页、过滤功能正常


---

## 功能二：日志管理设置（Log Management Settings）

### 1. 概述

在现有 `LogConfig` 基础上增加日志轮转和清理配置，在 `logbus` 中实现基于大小的日志轮转，在前端 SettingsModal 中添加日志管理设置面板。

---

### 2. 需要创建/修改的文件

| 文件路径 | 类型 | 说明 |
|---------|------|------|
| `internal/config/config.go` | **修改** | LogConfig 添加轮转字段 |
| `internal/config/defaults.go` | **修改** | 添加轮转默认值 |
| `internal/logbus/rotating.go` | **新建** | 轮转文件写入器 |
| `internal/logbus/logbus.go` | **修改** | 使用轮转写入器替换 plain os.File |
| `internal/api/settings.go` | **修改** | Get/Update Logging 设置 |
| `internal/api/logs.go` | **修改** | 添加日志文件列表/下载/清理端点 |
| `internal/api/handler.go` | **修改** | 注册新路由 |
| `web/src/api/client.ts` | **修改** | 添加日志管理 API 调用和类型 |
| `web/src/components/layout/SettingsModal.tsx` | **修改** | 添加日志管理 Section |
| `web/src/locales/zh-CN.json` | **修改** | 添加中文翻译 |
| `web/src/locales/en.json` | **修改** | 添加英文翻译 |

---

### 3. 数据结构

#### 3.1 Config 扩展 — `internal/config/config.go` 修改

```go
type LogConfig struct {
    Level           string `yaml:"level" mapstructure:"level"`
    Format          string `yaml:"format" mapstructure:"format"`
    File            string `yaml:"file" mapstructure:"file"`
    // ── 新增字段 ──
    MaxSize         int    `yaml:"max_size" mapstructure:"max_size"`             // MB, 单文件最大尺寸
    MaxBackups      int    `yaml:"max_backups" mapstructure:"max_backups"`       // 保留的备份文件数
    MaxAge          int    `yaml:"max_age" mapstructure:"max_age"`               // 保留天数
    Compress        bool   `yaml:"compress" mapstructure:"compress"`             // 是否 gzip 压缩旧日志
    CleanupInterval int    `yaml:"cleanup_interval" mapstructure:"cleanup_interval"` // 清理周期（小时）
}
```

#### 3.2 前端 TypeScript 类型

```typescript
export interface LogSettings {
  level: string
  format: string
  file: string
  max_size: number       // MB
  max_backups: number
  max_age: number        // 天
  compress: boolean
  cleanup_interval: number // 小时
}

export interface LogFileInfo {
  name: string
  size: number
  modified: string
  compressed: boolean
}

export interface LogDirInfo {
  path: string
  total_size: number
  files: LogFileInfo[]
}
```

---

### 4. API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| GET | `/api/v1/settings/logging` | 获取日志管理设置 |
| PUT | `/api/v1/settings/logging` | 更新日志管理设置 |
| GET | `/api/v1/logs/files` | 列出日志目录文件（含大小、时间） |
| POST | `/api/v1/logs/cleanup` | 手动触发日志清理 |
| GET | `/api/v1/logs/files/{name}` | 下载/查看指定日志文件内容 |

---

### 5. 详细实现步骤

#### 5.1 Step 1: Config 扩展 — `internal/config/config.go`

在 `LogConfig` 结构体中添加四个新字段（如上述数据结构）。

#### 5.2 Step 2: 默认值 — `internal/config/defaults.go`

```go
Log: LogConfig{
    Level:           DefaultLogLevel,
    MaxSize:         100,  // 100MB
    MaxBackups:      5,
    MaxAge:          30,   // 30 天
    Compress:        true,
    CleanupInterval: 24,   // 24 小时
},
```

#### 5.3 Step 3: 轮转写入器 — `internal/logbus/rotating.go`（新建）

这是核心组件，实现基于大小的文件轮转：

```go
package logbus

import (
    "compress/gzip"
    "fmt"
    "io"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"
)

type RotatingConfig struct {
    MaxSize    int  // MB
    MaxBackups int
    MaxAge     int  // 天
    Compress   bool
}

type RotatingFile struct {
    dir      string
    baseName string  // e.g. "dhcp"
    ext      string  // ".log"
    config   RotatingConfig
    mu       sync.Mutex
    file     *os.File
    size     int64
}

func NewRotatingFile(dir, baseName string, cfg RotatingConfig) (*RotatingFile, error) {
    rf := &RotatingFile{dir: dir, baseName: baseName, ext: ".log", config: cfg}
    if err := rf.open(); err != nil {
        return nil, err
    }
    return rf, nil
}

func (rf *RotatingFile) Write(p []byte) (n int, err error) {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    // 检查是否需要轮转
    if rf.size+int64(len(p)) > int64(rf.config.MaxSize*1024*1024) {
        if err := rf.rotate(); err != nil {
            // 轮转失败仍尝试写入
        }
    }

    n, err = rf.file.Write(p)
    rf.size += int64(n)
    return
}

func (rf *RotatingFile) rotate() error {
    rf.file.Close()

    // 重命名: dhcp.log -> dhcp.log.1, dhcp.log.1 -> dhcp.log.2, ...
    for i := rf.config.MaxBackups; i >= 1; i-- {
        old := rf.fileName(i)
        new := rf.fileName(i + 1)
        if i == rf.config.MaxBackups {
            os.Remove(new) // 删除最旧的
        }
        os.Rename(old, new)
    }

    // 压缩旧文件
    if rf.config.Compress {
        go rf.compressOld()
    }

    return rf.open()
}

func (rf *RotatingFile) open() error {
    fpath := filepath.Join(rf.dir, rf.baseName+rf.ext)
    f, err := os.OpenFile(fpath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
    if err != nil {
        return err
    }
    stat, _ := f.Stat()
    rf.file = f
    rf.size = stat.Size()
    return nil
}

func (rf *RotatingFile) fileName(index int) string {
    return filepath.Join(rf.dir, fmt.Sprintf("%s.log.%d", rf.baseName, index))
}

func (rf *RotatingFile) compressOld() {
    // 遍历 .1, .2, ... 文件，跳过已压缩的
    // 使用 compress/gzip 压缩，然后删除原文件
}

// Cleanup 删除超过 MaxAge 天的轮转文件
func (rf *RotatingFile) Cleanup() int {
    rf.mu.Lock()
    defer rf.mu.Unlock()

    cutoff := time.Now().AddDate(0, 0, -rf.config.MaxAge)
    pattern := filepath.Join(rf.dir, rf.baseName+".log.*")
    matches, _ := filepath.Glob(pattern)

    removed := 0
    for _, m := range matches {
        info, err := os.Stat(m)
        if err != nil {
            continue
        }
        if info.ModTime().Before(cutoff) {
            os.Remove(m)
            removed++
        }
    }
    return removed
}

func (rf *RotatingFile) Close() {
    rf.mu.Lock()
    defer rf.mu.Unlock()
    if rf.file != nil {
        rf.file.Close()
    }
}
```

#### 5.4 Step 4: 修改 logbus — `internal/logbus/logbus.go`

**修改 `BusHandler` 结构体：**

```go
type BusHandler struct {
    next       slog.Handler
    bus        *eventbus.Bus
    logDir     string
    files      map[string]*RotatingFile  // 改为 RotatingFile
    fileMu     sync.Mutex
    closed     bool
    attrs      []slog.Attr
    config     RotatingConfig            // 新增
    cleanupCh  chan struct{}              // 新增
    cleanupWg  sync.WaitGroup            // 新增
}
```

**修改 `NewBusHandler`：**

```go
func NewBusHandler(next slog.Handler, bus *eventbus.Bus, logDir string, cfg RotatingConfig) *BusHandler {
    h := &BusHandler{
        next:      next,
        bus:       bus,
        logDir:    logDir,
        files:     make(map[string]*RotatingFile),
        config:    cfg,
        cleanupCh: make(chan struct{}),
    }
    if logDir != "" {
        os.MkdirAll(logDir, 0755)
    }
    // 启动定期清理
    if cfg.CleanupInterval > 0 {
        h.cleanupWg.Add(1)
        go h.periodicCleanup()
    }
    return h
}
```

**修改 `writeServiceLog`：**

```go
func (h *BusHandler) writeServiceLog(entry LogEntry) {
    fname := strings.ToLower(entry.Service)
    h.fileMu.Lock()
    defer h.fileMu.Unlock()

    if h.closed {
        return
    }

    rf, ok := h.files[fname]
    if !ok {
        var err error
        rf, err = NewRotatingFile(h.logDir, fname, h.config)
        if err != nil {
            return
        }
        h.files[fname] = rf
    }

    // 格式化日志行（与现有逻辑相同）
    var b strings.Builder
    // ... 现有格式化逻辑 ...
    b.WriteByte('\n')

    rf.Write([]byte(b.String()))
}
```

**添加定期清理：**

```go
func (h *BusHandler) periodicCleanup() {
    defer h.cleanupWg.Done()
    interval := time.Duration(h.config.CleanupInterval) * time.Hour
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-h.cleanupCh:
            return
        case <-ticker.C:
            h.runCleanup()
        }
    }
}

func (h *BusHandler) runCleanup() {
    h.fileMu.Lock()
    defer h.fileMu.Unlock()

    for _, rf := range h.files {
        rf.Cleanup()
    }

    // 也清理不在 files map 中的旧轮转文件
    pattern := filepath.Join(h.logDir, "*.log.*")
    matches, _ := filepath.Glob(pattern)
    cutoff := time.Now().AddDate(0, 0, -h.config.MaxAge)
    for _, m := range matches {
        info, err := os.Stat(m)
        if err != nil {
            continue
        }
        if info.ModTime().Before(cutoff) {
            os.Remove(m)
        }
    }
}

func (h *BusHandler) Close() {
    close(h.cleanupCh)
    h.fileMu.Lock()
    defer h.fileMu.Unlock()
    h.closed = true
    for name, f := range h.files {
        f.Close()
        delete(h.files, name)
    }
}
```

#### 5.5 Step 5: 修改调用点 — `cmd/pxelab/main.go`

在 `NewBusHandler` 调用处传入配置：

```go
// 从 config 中获取日志轮转配置
rotCfg := logbus.RotatingConfig{
    MaxSize:    cfg.Log.MaxSize,
    MaxBackups: cfg.Log.MaxBackups,
    MaxAge:     cfg.Log.MaxAge,
    Compress:   cfg.Log.Compress,
}
busHandler := logbus.NewBusHandler(next, bus, logDir, rotCfg)
```

需要确认 `main.go` 中 `logbus.NewBusHandler` 的调用位置并修改。

#### 5.6 Step 6: Settings API — `internal/api/settings.go`

添加日志管理设置的 Get/Put：

```go
// GET /api/v1/settings/logging
func (h *SettingsHandler) GetLogging(w http.ResponseWriter, r *http.Request) {
    OK(w, map[string]any{
        "level":            h.cfg.Log.Level,
        "format":           h.cfg.Log.Format,
        "file":             h.cfg.Log.File,
        "max_size":         h.cfg.Log.MaxSize,
        "max_backups":      h.cfg.Log.MaxBackups,
        "max_age":          h.cfg.Log.MaxAge,
        "compress":         h.cfg.Log.Compress,
        "cleanup_interval": h.cfg.Log.CleanupInterval,
    })
}

// PUT /api/v1/settings/logging
func (h *SettingsHandler) UpdateLogging(w http.ResponseWriter, r *http.Request) {
    var body struct {
        Level            *string `json:"level"`
        MaxSize          *int    `json:"max_size"`
        MaxBackups       *int    `json:"max_backups"`
        MaxAge           *int    `json:"max_age"`
        Compress         *bool   `json:"compress"`
        CleanupInterval  *int    `json:"cleanup_interval"`
    }
    if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
        Error(w, http.StatusBadRequest, "无效的请求体")
        return
    }
    // 更新 cfg.Log 字段
    // 保存配置文件
    // 注意: 部分设置需要重启生效（如 level），部分可热更新（如 maxSize）
}
```

#### 5.7 Step 7: 日志文件 API — `internal/api/logs.go`

添加文件列表和清理端点：

```go
// GET /api/v1/logs/files — 列出日志文件及大小
func (h *LogStreamHandler) ListFiles(w http.ResponseWriter, r *http.Request) {
    entries, err := os.ReadDir(h.logDir)
    if err != nil {
        Error(w, http.StatusInternalServerError, "无法读取日志目录")
        return
    }
    var files []LogFileInfo
    var totalSize int64
    for _, e := range entries {
        info, _ := e.Info()
        size := info.Size()
        totalSize += size
        files = append(files, LogFileInfo{
            Name:      e.Name(),
            Size:      size,
            Modified:  info.ModTime(),
            Compressed: strings.HasSuffix(e.Name(), ".gz"),
        })
    }
    OK(w, map[string]any{
        "path":       h.logDir,
        "total_size": totalSize,
        "files":      files,
    })
}

// POST /api/v1/logs/cleanup — 手动触发清理
func (h *LogStreamHandler) CleanupLogs(w http.ResponseWriter, r *http.Request) {
    // 调用 logbus 的 Cleanup 方法
}

// GET /api/v1/logs/files/{name} — 查看日志文件内容
func (h *LogStreamHandler) GetLogFile(w http.ResponseWriter, r *http.Request) {
    // 读取指定文件内容，支持 gzip 解压
    // 返回最后 N 行
}
```

#### 5.8 Step 8: 路由注册 — `internal/api/handler.go`

```go
// 在 RegisterRoutes 中添加
r.Get("/settings/logging", h.Settings.GetLogging)
r.Put("/settings/logging", h.Settings.UpdateLogging)
r.Get("/logs/files", h.Logs.ListFiles)
r.Post("/logs/cleanup", h.Logs.CleanupLogs)
r.Get("/logs/files/{name}", h.Logs.GetLogFile)
```

#### 5.9 Step 9: 前端 API — `web/src/api/client.ts`

```typescript
// 类型
export interface LogSettings {
  level: string
  format: string
  file: string
  max_size: number
  max_backups: number
  max_age: number
  compress: boolean
  cleanup_interval: number
}

export interface LogFileInfo {
  name: string
  size: number
  modified: string
  compressed: boolean
}

export interface LogDirInfo {
  path: string
  total_size: number
  files: LogFileInfo[]
}

// API 函数
export function getLoggingSettings(): Promise<ApiResponse<LogSettings>> {
  return request<LogSettings>('GET', '/settings/logging')
}

export function updateLoggingSettings(data: Partial<LogSettings>): Promise<ApiResponse<unknown>> {
  return request<unknown>('PUT', '/settings/logging', data)
}

export function getLogFiles(): Promise<ApiResponse<LogDirInfo>> {
  return request<LogDirInfo>('GET', '/logs/files')
}

export function cleanupLogs(): Promise<ApiResponse<unknown>> {
  return request<unknown>('POST', '/logs/cleanup')
}

export function getLogFileContent(name: string): Promise<ApiResponse<string>> {
  return request<string>('GET', '/logs/files/' + encodeURIComponent(name))
}
```

#### 5.10 Step 10: 前端 SettingsModal — `web/src/components/layout/SettingsModal.tsx`

**修改 `Section` 类型：**
```typescript
type Section = 'general' | 'boot' | 'netboot' | 'services' | 'logging'
```

**添加导航项：**
```typescript
const navItems: NavItem[] = [
    // ... 现有项 ...
    { key: 'logging', label: t('settings.logManagement'), icon: Activity },
]
```

**添加 `LogManagementForm` 组件：**

```tsx
function LogManagementForm({ config, onChange }: { config: LogSettings; onChange: (c: LogSettings) => void }) {
    const { t } = useTranslation()
    const [dirInfo, setDirInfo] = useState<LogDirInfo | null>(null)
    const [cleaning, setCleaning] = useState(false)

    // 加载日志目录信息
    useEffect(() => {
        getLogFiles().then(res => setDirInfo(res.data)).catch(() => {})
    }, [])

    const formatSize = (bytes: number) => {
        if (bytes === 0) return '0 B'
        const units = ['B', 'KB', 'MB', 'GB']
        const i = Math.floor(Math.log(bytes) / Math.log(1024))
        return (bytes / Math.pow(1024, i)).toFixed(1) + ' ' + units[i]
    }

    const handleCleanup = async () => {
        setCleaning(true)
        try {
            await cleanupLogs()
            const res = await getLogFiles()
            setDirInfo(res.data)
        } catch {}
        setCleaning(false)
    }

    return (
        <div className="p-6 space-y-4">
            <h3 className="text-sm font-semibold">{t('settings.logManagement')}</h3>

            {/* 基本设置 */}
            <SettingsField label={t('settings.logMaxSize')}>
                <div className="flex items-center gap-2">
                    <input type="number" min={10} max={1000}
                        value={config.max_size}
                        onChange={e => onChange({...config, max_size: parseInt(e.target.value) || 100})}
                        className={inputCls + ' w-24'} />
                    <span className="text-xs text-[var(--text-muted)]">MB</span>
                </div>
                <p className="text-xs text-[var(--text-muted)] mt-1">{t('settings.logMaxSizeHelp')}</p>
            </SettingsField>

            <SettingsField label={t('settings.logMaxBackups')}>
                <input type="number" min={0} max={50}
                    value={config.max_backups}
                    onChange={e => onChange({...config, max_backups: parseInt(e.target.value) || 5})}
                    className={inputCls + ' w-24'} />
            </SettingsField>

            <SettingsField label={t('settings.logMaxAge')}>
                <div className="flex items-center gap-2">
                    <input type="number" min={1} max={365}
                        value={config.max_age}
                        onChange={e => onChange({...config, max_age: parseInt(e.target.value) || 30})}
                        className={inputCls + ' w-24'} />
                    <span className="text-xs text-[var(--text-muted)]">{t('common.days', '天')}</span>
                </div>
            </SettingsField>

            <SettingsField label={t('settings.logCompress')}>
                <Toggle checked={config.compress} onChange={v => onChange({...config, compress: v})} />
            </SettingsField>

            <SettingsField label={t('settings.logCleanupInterval')}>
                <div className="flex items-center gap-2">
                    <input type="number" min={1} max={168}
                        value={config.cleanup_interval}
                        onChange={e => onChange({...config, cleanup_interval: parseInt(e.target.value) || 24})}
                        className={inputCls + ' w-24'} />
                    <span className="text-xs text-[var(--text-muted)]">{t('common.hours', '小时')}</span>
                </div>
            </SettingsField>

            {/* 磁盘使用 */}
            {dirInfo && (
                <div className="pt-4 border-t border-[var(--bg-border)]">
                    <h4 className="text-xs font-semibold mb-2">{t('settings.logDiskUsage')}</h4>
                    <p className="text-xs text-[var(--text-muted)]">
                        {t('settings.logDir')}: <code className="text-[var(--text-secondary)]">{dirInfo.path}</code>
                    </p>
                    <p className="text-xs text-[var(--text-muted)]">
                        {t('settings.logTotalSize')}: {formatSize(dirInfo.total_size)} · {dirInfo.files.length} {t('settings.logFiles')}
                    </p>
                    <Button variant="secondary" size="sm" onClick={handleCleanup} disabled={cleaning} className="mt-2">
                        {cleaning ? t('settings.loading') : t('settings.logCleanup')}
                    </Button>
                </div>
            )}
        </div>
    )
}
```

**在 Modal body 中添加条件渲染：**
```tsx
{section === 'logging' && <LogManagementForm config={loggingConfig} onChange={setLoggingConfig} />}
```

#### 5.11 Step 11: SettingsModal 数据加载

在 `SettingsModal` 的 `useEffect` 中添加日志配置加载：

```typescript
const [loggingConfig, setLoggingConfig] = useState<LogSettings | null>(null)

useEffect(() => {
    if (!open) return
    // 在 Promise.all 中添加
    Promise.all([..., getLoggingSettings()])
        .then(([..., log]) => {
            setLoggingConfig(log.data as unknown as LogSettings)
        })
}, [open])
```

在 `handleSave` 中添加：
```typescript
if (loggingConfig) promises.push(updateLoggingSettings(loggingConfig))
```

#### 5.12 Step 12: 国际化翻译

**`zh-CN.json`：**
```json
{
  "settings.logManagement": "日志管理",
  "settings.logMaxSize": "单文件最大大小",
  "settings.logMaxSizeHelp": "单个日志文件超过此大小后自动轮转",
  "settings.logMaxBackups": "保留备份数",
  "settings.logMaxAge": "保留天数",
  "settings.logCompress": "压缩旧日志",
  "settings.logCleanupInterval": "清理周期",
  "settings.logDiskUsage": "磁盘使用",
  "settings.logDir": "日志目录",
  "settings.logTotalSize": "总大小",
  "settings.logFiles": "个文件",
  "settings.logCleanup": "立即清理"
}
```

**`en.json`：**
```json
{
  "settings.logManagement": "Log Management",
  "settings.logMaxSize": "Max File Size",
  "settings.logMaxSizeHelp": "Rotate log file when it exceeds this size",
  "settings.logMaxBackups": "Max Backups",
  "settings.logMaxAge": "Max Age",
  "settings.logCompress": "Compress Old Logs",
  "settings.logCleanupInterval": "Cleanup Interval",
  "settings.logDiskUsage": "Disk Usage",
  "settings.logDir": "Log Directory",
  "settings.logTotalSize": "Total Size",
  "settings.logFiles": "files",
  "settings.logCleanup": "Cleanup Now"
}
```

---

### 6. 迁移/兼容性

- **配置兼容：** 新字段有零值默认行为，旧配置文件无需修改即可运行
- **文件格式兼容：** 轮转文件命名 `.log.1`, `.log.2.gz` 等是行业标准，不破坏现有日志查看
- **API 兼容：** 新增端点不影响现有端点
- **logbus 接口变化：** `NewBusHandler` 签名变化需要修改所有调用处（仅 `cmd/pxelab/main.go`）
- **重启生效：** `level` 和 `format` 修改需重启；`max_size` 等轮转参数理想情况下可热更新（通过 Settings API 触发 logbus 配置刷新）

---

### 7. 测试要点

- 验证日志文件达到 MaxSize 后自动轮转
- 验证轮转文件命名正确（.log.1, .log.2, ...）
- 验证 gzip 压缩正常工作
- 验证超过 MaxAge 的文件被清理
- 验证 MaxBackups 限制正确（旧文件被删除）
- 验证手动清理按钮工作
- 验证日志目录大小显示正确
- 验证 Settings API 的 Get/Put 正确读写配置
- 验证 SSE 日志流在轮转后仍正常工作
- 前端 Settings Modal 中日志管理面板渲染和交互正常


---

## 实施顺序建议

1. **功能二优先**（日志管理设置），因为它是基础设施改进，影响较小
   - Step 1-2: Config 扩展
   - Step 3-4: logbus 轮转实现
   - Step 5: 调用点修改
   - Step 6-7: API 端点
   - Step 8-12: 前端

2. **功能一其次**（审计日志），依赖较多模块
   - Step 1-2: 模型和存储
   - Step 3-4: 中间件和 Handler
   - Step 5-6: 在各 Handler 中插入审计调用
   - Step 7: 路由注册
   - Step 8-11: 前端

---

## 共同注意事项

1. **Go 版本：** 项目使用 Go 1.25.11，可使用最新语言特性
2. **GORM AutoMigrate：** 新模型自动建表，无需手动 migration 文件
3. **eventbus 模式：** 两个功能都使用 `PublishAsync` 避免阻塞
4. **前端模式：** 遵循现有 `api/client.ts` 中 `request<T>()` 模式
5. **翻译：** 所有 UI 文本通过 i18next，zh-CN 为主语言
6. **不引入新依赖：** 日志轮转使用标准库 `compress/gzip`、`os`、`path/filepath`
7. **分页：** 审计日志查询使用与现有 Event 查询相同的 `Meta` 分页结构
