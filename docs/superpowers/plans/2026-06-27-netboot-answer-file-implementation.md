# Netboot 自动化应答实现计划

## 总览

4 个阶段，依次推进：DB/Store → 后端逻辑 → API → 前端。

---

## 阶段一：数据库模型与 Store 层

### 1.1 新增 Model（`internal/models/`）

**`netboot_overlay.go`**：
```go
type NetbootOverlay struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    DistroName      string    `gorm:"uniqueIndex;not null" json:"distro_name"`
    Enabled         bool      `gorm:"default:false" json:"enabled"`
    Mirror          string    `json:"mirror"`
    LocalBase       string    `json:"local_base"`
    KernelParams    string    `json:"kernel_params"`
    VersionOverrides string   `gorm:"type:text" json:"version_overrides"` // JSON string
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

**`answer_template.go`**：
```go
type AnswerTemplate struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `gorm:"not null" json:"name"`
    Description string    `json:"description"`
    Type        string    `gorm:"not null" json:"type"` // kickstart / preseed / subiquity / autoyast / autounattend
    Content     string    `gorm:"type:text;not null" json:"content"`
    Variables   string    `gorm:"type:text" json:"variables"` // JSON string
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}
```

**`install_task.go`**：
```go
type InstallTask struct {
    ID              string    `gorm:"primaryKey" json:"id"` // task_<uuid>
    HostID          string    `gorm:"not null" json:"host_id"`
    DistroName      string    `gorm:"not null" json:"distro_name"`
    VersionCodename string    `gorm:"not null" json:"version_codename"`
    AnswerTemplateID *uint    `json:"answer_template_id,omitempty"`
    ExtraCmdline    string    `json:"extra_cmdline"`
    Status          string    `gorm:"default:pending" json:"status"` // pending / installing / done / failed
    ErrorMsg        string    `json:"error_msg"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
}
```

`VersionOverrides` 和 `Variables` 存 JSON 字符串，由序列化/反序列化处理。

### 1.2 Store Interface 扩展（`internal/store/store.go`）

新增三个 sub-interface：

```go
type NetbootOverlayStore interface {
    ListNetbootOverlays(ctx context.Context) ([]models.NetbootOverlay, error)
    GetNetbootOverlay(ctx context.Context, distroName string) (*models.NetbootOverlay, error)
    UpsertNetbootOverlay(ctx context.Context, o *models.NetbootOverlay) error
    DeleteNetbootOverlay(ctx context.Context, distroName string) error
}

type AnswerTemplateStore interface {
    ListAnswerTemplates(ctx context.Context) ([]models.AnswerTemplate, error)
    GetAnswerTemplate(ctx context.Context, id uint) (*models.AnswerTemplate, error)
    CreateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error
    UpdateAnswerTemplate(ctx context.Context, t *models.AnswerTemplate) error
    DeleteAnswerTemplate(ctx context.Context, id uint) error
}

type InstallTaskStore interface {
    ListInstallTasks(ctx context.Context) ([]models.InstallTask, error)
    GetInstallTask(ctx context.Context, id string) (*models.InstallTask, error)
    CreateInstallTask(ctx context.Context, t *models.InstallTask) error
    UpdateInstallTask(ctx context.Context, t *models.InstallTask) error
    DeleteInstallTask(ctx context.Context, id string) error
    GetInstallTaskByHostMAC(ctx context.Context, mac string) (*models.InstallTask, error)
}
```

组合到 `Interface`：`NetbootOverlayStore`、`AnswerTemplateStore`、`InstallTaskStore`。

### 1.3 SQLite 实现（`internal/store/sqlite.go`）

- 在 `Migrate()` 的 `AutoMigrate` 中追加三个新 model
- 在 `sqliteStore` struct 上实现所有接口方法
- 已有的 `db` 字段（`*gorm.DB`）直接复用，无需新增依赖

### 1.4 Memory 实现（`internal/store/memory.go`）

- 在 `memoryStore` 中追加三个 map + 一个 `sync.RWMutex`（或复用已有的锁）
- 实现所有接口方法

---

## 阶段二：后端核心逻辑

### 2.1 Catalog Overlay 合并（`internal/netboot/overlay.go` 新增）

核心合并函数：

```go
// MergeOverlay 将 DB overlay 合并到 catalog distro 上，返回新的 *Distro（不修改原始对象）
func MergeOverlay(distro *Distro, overlay *models.NetbootOverlay) *Distro
```

合并规则：
- `overlay.Mirror` 非空 → 覆盖 `distro.Mirror`
- `overlay.LocalBase` 非空 → 覆盖 `distro.LocalBase`
- `overlay.KernelParams` 非空 → 追加到 `distro.KernelParams`
- `overlay.VersionOverrides` 解析 JSON → 按 `codename+arch` 匹配 version，覆盖对应字段（remote_kernel, remote_initrd, cmdline, answer_param, answer_type）

需要定义 `VersionOverride` Go struct 来解析 JSON。

### 2.2 应答模板渲染（`internal/netboot/answer.go` 新增）

```go
// RenderAnswerTemplate 渲染应答模板，用主机信息替换变量
func RenderAnswerTemplate(content string, host *models.Host, task *models.InstallTask) (string, error)
```

使用 `text/template`，注入变量：
- `.HostName`, `.HostIP`, `.HostMAC`, `.HostCIDR`, `.Gateway`, `.DNSServers`, `.Disk`, `.KeyboardLayout`
- `.ProductKey`, `.ComputerName`, `.JoinDomain`, `.DomainOU`, `.AdminPassword`, `.TimeZone`（Windows 额外变量）
- `.AnswerURL` 占位符由调用方替换为实际 URL

模板内容存储在 answer_templates.content 中，渲染后通过 `/api/v1/netboot/answer/{task_id}` 提供。

### 2.3 脚本生成增强（`internal/netboot/script.go`）

增强 `GenerateBootLine` 签名：

```go
// 新增参数 task *InstallTask
func GenerateBootLine(v *Version, serverAddr, bootPrefix, kernelParams string, task *InstallTask) string
```

注入逻辑：
1. 如果 `task != nil`，从 version 获取 `answer_param`
2. 替换 `{{.AnswerURL}}` → `http://{serverAddr}/api/v1/netboot/answer/{task_id}`
3. 追加到 kernel cmdline
4. 如果 `answer_type` 包含 `wimboot` → 触发 Windows 特殊处理

新增 `GenerateWindowsBootLine` 处理 wimboot 流程：
- 无任务：标准 wimboot 引导
- 有任务 + autounattend：注入 `autounattend.xml`
- 有任务 + winpeshl：注入 `install.bat`

### 2.4 脚本生成入口修改

`GenerateNetbootScript` 需要获取任务数据。因为任务查询需要 store，需要在调用链中传递。两种方案：

**方案 A**：将 `store.Interface` 传递给 `GenerateNetbootScript`，按 MAC 查询任务。
**方案 B**：由调用方（HTTP handler）预先查询任务，传递给脚本生成函数。

方案 B 更干净。HTTP handler 中：
- 从 query 或 session 获取 MAC
- 查询 `store.GetInstallTaskByHostMAC(ctx, mac)`
- 传入脚本生成函数

---

## 阶段三：API 层

### 3.1 新增 Handler（`internal/api/overlays.go`, `answer_templates.go`, `install_tasks.go`）

遵循 `HostHandler` 模式，每个 handler struct 持有 `store.Interface`。

**`overlays.go`**：
- `ListOverlays` — 返回所有 overlay
- `GetOverlay` — 按 distro_name 获取
- `UpsertOverlay` — PUT 创建或更新
- `DeleteOverlay` — 删除

**`answer_templates.go`**：
- `ListTemplates` — 列表
- `CreateTemplate` — POST 创建
- `GetTemplate` — 按 ID 获取
- `UpdateTemplate` — PUT 更新
- `DeleteTemplate` — 删除

**`install_tasks.go`**：
- `ListTasks` — 列表
- `CreateTask` — POST 创建
- `GetTask` — 按 ID 获取
- `UpdateTask` — PUT 更新（状态流转）
- `DeleteTask` — 删除

### 3.2 PXE 运行时端点（`install_tasks.go` 中）

**`GetTaskByMAC`**：
- 路径：`GET /api/v1/netboot/task/by-mac/{mac}`
- 返回安装任务配置（answer_param, extra_cmdline）
- 无认证，供 iPXE 使用

**`GetAnswerFile`**：
- 路径：`GET /api/v1/netboot/answer/{task_id}`
- 渲染并返回应答模板内容（Content-Type: text/plain）
- 无认证，供安装程序请求

### 3.3 路由注册（`internal/api/handler.go`）

在 `RegisterRoutes` 中追加：

```go
// Netboot overlays
r.Get("/netboot/overlays", h.NetbootOverlay.ListOverlays)
r.Get("/netboot/overlays/{distro}", h.NetbootOverlay.GetOverlay)
r.Put("/netboot/overlays/{distro}", h.NetbootOverlay.UpsertOverlay)
r.Delete("/netboot/overlays/{distro}", h.NetbootOverlay.DeleteOverlay)

// Answer templates
r.Get("/netboot/answer-templates", h.AnswerTemplate.ListTemplates)
r.Post("/netboot/answer-templates", h.AnswerTemplate.CreateTemplate)
r.Get("/netboot/answer-templates/{id}", h.AnswerTemplate.GetTemplate)
r.Put("/netboot/answer-templates/{id}", h.AnswerTemplate.UpdateTemplate)
r.Delete("/netboot/answer-templates/{id}", h.AnswerTemplate.DeleteTemplate)

// Install tasks
r.Get("/netboot/tasks", h.InstallTask.ListTasks)
r.Post("/netboot/tasks", h.InstallTask.CreateTask)
r.Get("/netboot/tasks/{id}", h.InstallTask.GetTask)
r.Put("/netboot/tasks/{id}", h.InstallTask.UpdateTask)
r.Delete("/netboot/tasks/{id}", h.InstallTask.DeleteTask)

// PXE runtime (no auth)
r.Get("/netboot/task/by-mac/{mac}", h.InstallTask.GetTaskByMAC)
r.Get("/netboot/answer/{task_id}", h.InstallTask.GetAnswerFile)
```

在 `Handler` struct 中新增三个字段：
```go
NetbootOverlay  *NetbootOverlayHandler
AnswerTemplate  *AnswerTemplateHandler
InstallTask     *InstallTaskHandler
```

### 3.4 公开路径白名单（`internal/httpd/middleware.go`）

在 `isPublicPath` 中追加：
- `/api/v1/netboot/task/by-mac/`
- `/api/v1/netboot/answer/`

---

## 阶段四：前端

### 4.1 API 客户端扩展（`web/src/api/client.ts`）

新增接口定义和 API 函数：

```typescript
interface NetbootOverlay { /* ... */ }
interface VersionOverride { /* ... */ }
interface AnswerTemplate { /* ... */ }
interface InstallTask { /* ... */ }

// Overlay APIs
getNetbootOverlays()
getNetbootOverlay(distro: string)
upsertNetbootOverlay(distro: string, overlay: NetbootOverlay)
deleteNetbootOverlay(distro: string)

// Answer Template APIs
getAnswerTemplates()
createAnswerTemplate(tpl: AnswerTemplate)
getAnswerTemplate(id: number)
updateAnswerTemplate(id: number, tpl: AnswerTemplate)
deleteAnswerTemplate(id: number)

// Install Task APIs
getInstallTasks()
createInstallTask(task: InstallTask)
getInstallTask(id: string)
updateInstallTask(id: string, task: InstallTask)
deleteInstallTask(id: string)
```

### 4.2 Overlay 配置面板

在现有 `NetbootCatalog.tsx` 的每个 distro 详情展开区增加 "覆盖配置" 标签：
- 启用/禁用 overlay
- 编辑 mirror / kernel_params
- 版本级覆盖：每行显示一个 version，可覆盖 remote_kernel / remote_initrd / cmdline / answer_param
- 选择 answer 模板类型

### 4.3 应答模板管理页（`web/src/pages/AnswerTemplates.tsx` 新增）

独立页面，路由 `/netboot/answer-templates`：
- 列表页：表格显示名称、类型、更新时间
- 创建/编辑页：表单 + textarea 编辑器
- 变量提示面板：显示可用变量列表

### 4.4 主机安装任务面板

在主机详情页增加"安装任务"区域：
- 从 catalog + overlay 合并后的列表选择 distro + version
- 选择应答模板
- 填写额外 cmdline
- 查看状态

---

## 文件变更清单

| 文件 | 操作 | 说明 |
|------|------|------|
| `internal/models/netboot_overlay.go` | 新增 | Overlay 数据模型 |
| `internal/models/answer_template.go` | 新增 | 应答模板数据模型 |
| `internal/models/install_task.go` | 新增 | 安装任务数据模型 |
| `internal/store/store.go` | 修改 | 追加三个 sub-interface |
| `internal/store/sqlite.go` | 修改 | AutoMigrate + CRUD 实现 |
| `internal/store/memory.go` | 修改 | In-memory 实现 |
| `internal/netboot/types.go` | 修改 | 可能需要新增 answer_param/wimboot 相关字段 |
| `internal/netboot/answer.go` | 新增 | 模板渲染逻辑 |
| `internal/netboot/overlay.go` | 新增 | 合并逻辑 |
| `internal/netboot/script.go` | 修改 | GenerateBootLine 增强 + Windows 分支 |
| `internal/netboot/manager.go` | 修改 | 可能需要扩展 |
| `internal/api/handler.go` | 修改 | 注册新路由 + Handler 字段 |
| `internal/api/overlays.go` | 新增 | Overlay CRUD |
| `internal/api/answer_templates.go` | 新增 | 模板 CRUD |
| `internal/api/install_tasks.go` | 新增 | 任务 CRUD + PXE 运行时端点 |
| `internal/httpd/server.go` | 可能修改 | 可能需要传递 store 给 netboot 脚本生成 |
| `internal/httpd/middleware.go` | 修改 | isPublicPath 白名单 |
| `internal/netboot/sync.go` | 不改 | 上游同步不受影响 |
| `web/src/api/client.ts` | 修改 | 新增 API 函数 |
| `web/src/pages/NetbootCatalog.tsx` | 修改 | 叠加配置面板 |
| `web/src/pages/AnswerTemplates.tsx` | 新增 | 模板管理页 |
| `web/src/App.tsx` | 修改 | 添加路由 |

## 执行顺序

```
阶段一（DB/Store）→ 阶段二（后端逻辑）→ 阶段三（API）→ 阶段四（前端）
```

每个阶段内按文件依赖顺序执行。阶段二和阶段三可以部分重叠（先写好模型和 store，就可以并行写后端逻辑和 API handler）。
