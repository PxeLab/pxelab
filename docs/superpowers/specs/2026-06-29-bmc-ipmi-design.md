# BMC/IPMI 带外管理设计

> PxeGo 独立带外管理系统：支持 BMC/IPMI 远程电源管理、引导设备控制、CSV 批量导入。
> 独立于主机管理，可管理不在 Host 列表中的设备。

---

## 1. 数据模型

### 1.1 `bmc_configs` 表

```sql
CREATE TABLE bmc_configs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL DEFAULT '',         -- 设备标识名
    host        TEXT NOT NULL,                     -- BMC IP/域名
    port        INTEGER DEFAULT 623,              -- 端口（IPMI 623, Redfish 443）
    username    TEXT NOT NULL DEFAULT '',
    password    TEXT NOT NULL DEFAULT '',          -- 敏感字段，API 返回时脱敏
    protocol    TEXT NOT NULL DEFAULT 'ipmi',      -- ipmi / redfish
    boot_mode   TEXT NOT NULL DEFAULT 'auto',      -- auto / uefi / legacy
    serial      TEXT NOT NULL DEFAULT '',          -- 设备SN
    model       TEXT NOT NULL DEFAULT '',          -- 设备型号
    vendor      TEXT NOT NULL DEFAULT '',          -- 设备品牌
    mac         TEXT NOT NULL DEFAULT '',          -- MAC 地址（可选，联动用）
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

- `protocol` 替代 `redfish bool`，更灵活（后续支持 `ipmitool` 等）
- `boot_mode`：auto = 根据 DHCP Option 93 自动，uefi = 强制 UEFI，legacy = 强制 Legacy
- `vendor`/`model`/`serial`/`mac`/`name` 由系统自动探测填充，用户不手动填写
- 不设 `device_id` FK，保持独立于主机管理

### 1.2 密码脱敏

API GET 列表/详情时，`password` 字段返回 `****` 或空串，仅在表单回填时标记 `has_password: true`。POST/PUT 时若 password 为空则保留原值。

### 1.3 设备信息探测

`vendor`/`model`/`serial`/`mac`/`name`/`boot_mode` 等字段**不由用户填写**，由系统通过 IPMI/Redfish 自动探测获取。探测通过单独 API 触发：

```
POST /api/v1/bmc/probe    → 探测设备信息
{
    "host": "10.0.0.1",
    "port": 623,
    "username": "admin",
    "password": "admin",
    "protocol": "ipmi"
}

响应：
{
    "name": "svr-01",
    "vendor": "Dell",
    "model": "PowerEdge R750",
    "serial": "SN123456",
    "mac": "00:11:22:33:44:55",
    "boot_mode": "auto"
}
```

探测失败时不阻塞，返回空字段，用户可跳过检测直接保存。

### 1.4 探测协议参考

**IPMI FRU 读取**：通过 `ipmitool fru print` 或 `go-ipmi` 的 FRU 命令读取设备信息：
- Board Product → model
- Board Serial → serial  
- Board Manufacturer → vendor
- Chassis Part Number / Product Name → 型号补充

通过 IPMI LAN 配置读取 MAC。

**Redfish（二期）**：`/redfish/v1/Systems/{id}`：
- Manufacturer → vendor
- Model → model
- SerialNumber → serial
- EthernetInterfaces → mac
- Boot/BootSourceOverrideMode → boot_mode

---

## 2. 后端架构

### 2.1 `internal/bmc/` 包 — Controller 接口和实现

新建独立包，替代旧的 `internal/ipmi/`。

```go
// types.go
type PowerState string
const (
    PowerOn     PowerState = "on"
    PowerOff    PowerState = "off"
    PowerUnknown PowerState = "unknown"
)

type BootDevice string
const (
    BootPXE   BootDevice = "pxe"
    BootDisk  BootDevice = "disk"
    BootCDROM BootDevice = "cdrom"
    BootBIOS  BootDevice = "bios"
)

type Config struct {
    Host     string
    Port     int
    Username string
    Password string
    Redfish  bool
}

type Controller interface {
    PowerOn(ctx context.Context) error
    PowerOff(ctx context.Context) error
    PowerRestart(ctx context.Context) error
    PowerStatus(ctx context.Context) (PowerState, error)
    SetBootDevice(ctx context.Context, device BootDevice) error
    Close() error
}
```

**IPMI 实现**：改造现有 `internal/ipmi/ipmi.go`，适配新 `Controller` 接口。现有 `github.com/bougou/go-ipmi v0.8.3` 已具备。

**Redfish 实现（二期）**：标准 REST API，`net/http`，无额外依赖。

```go
// factory.go
func NewController(cfg Config) (Controller, error) {
    if cfg.Redfish {
        return nil, fmt.Errorf("Redfish not yet implemented")
    }
    return NewIPMIController(cfg)
}
```

### 2.2 `internal/store/bmc_config.go` — CRUD

BMCConfigStore interface（加入 store.Interface）：

```go
type BMCConfigStore interface {
    ListBMCConfigs(ctx context.Context) ([]models.BMCConfig, error)
    GetBMCConfig(ctx context.Context, id int64) (*models.BMCConfig, error)
    CreateBMCConfig(ctx context.Context, cfg *models.BMCConfig) error
    UpdateBMCConfig(ctx context.Context, cfg *models.BMCConfig) error
    DeleteBMCConfig(ctx context.Context, id int64) error
}
```

`internal/store/sqlite_bmc.go` 实现，使用 GORM AutoMigrate 建表。

### 2.3 `internal/api/bmc_handler.go` — API 路由

在 `handler.go` 的 `RegisterRoutes` 中注册于 `/api/v1` 下：

```
配置 CRUD:
  GET    /bmc/configs           → 列表
  POST   /bmc/configs           → 创建（填 host/port/username/password/protocol）
  GET    /bmc/configs/{id}      → 详情
  PUT    /bmc/configs/{id}      → 更新
  DELETE /bmc/configs/{id}      → 删除
  POST   /bmc/configs/import    → CSV 导入
  POST   /bmc/probe             → 探测设备信息（不存库，仅探测返回）
  POST   /bmc/{id}/refresh      → 重新探测并更新设备信息字段

电源操作:
  POST   /bmc/{id}/power-on     → 开机
  POST   /bmc/{id}/power-off    → 关机
  POST   /bmc/{id}/restart      → 重启
  GET    /bmc/{id}/status       → 电源状态
  POST   /bmc/{id}/boot-device  → 设置引导设备 { "device": "pxe"|"disk"|"cdrom"|"bios" }

批量操作:
  POST   /bmc/batch/power-on    → 批量开机 { "ids": [1,2,3] }
  POST   /bmc/batch/power-off   → 批量关机
  POST   /bmc/batch/restart     → 批量重启
  POST   /bmc/batch/status      → 批量状态查询
```

Handler 结构：

```go
type BMCHandler struct {
    store store.Interface
}
```

每个电源操作 handler 模式：
1. 解析 config ID
2. 查数据库获取 BMC 配置
3. 通过工厂函数创建 Controller
4. 执行操作
5. 返回结果

批量操作使用 `sync.WaitGroup` + 超时控制，单个失败不影响其他。

---

## 3. 前端

### 3.1 导航

侧边栏新增「带外管理」一级菜单，SVG 图标（服务器+天线），路由 `/bmc`。

### 3.2 BMC 管理页面（BmcView.vue）

```
┌──────────────────────────────────────────────────────────────┐
│  带外管理                                      [导入CSV]      │
│  [批量开机] [批量关机] [批量重启]                             │
├──────────────────────────────────────────────────────────────┤
│ ☐ │ 设备名  │ BMC地址     │ 协议   │ 品牌/型号    │ 电源状态  │ 操作  │
│ ☐ │ svr-01  │ 10.0.0.1   │ IPMI  │ Dell R750    │ ● 开机    │ [开][关][重启][引导][🌐] │
│ ☐ │ svr-02  │ 10.0.0.2   │ IPMI  │ HPe DL380    │ ○ 关机    │ [开][关][重启][引导][🌐] │
│ ☐ │ svr-03  │ 10.0.0.3   │ Redfish│ Inspur NF5280│ ? 未知    │ [开][关][重启][引导][🌐] │
└──────────────────────────────────────────────────────────────┘
```

关键交互：
- 电源状态自动轮询（首次加载 + 操作后延迟 2s + 每 10s 刷新）
- 🌐 按钮在新标签页打开 `https://{host}`
- 状态圆点颜色：绿(on)/红(off)/灰(unknown)
- 引导设备设置弹窗：下拉选择 PXE/硬盘/光驱/BIOS
- 表格无数据时显示空状态提示
- 加载中显示骨架屏或 loading
- 操作失败弹 toast 提示

### 3.3 BMC 配置表单 — 5 核心字段 + 探测

新建/编辑共用表单，用户只需填写 5 个核心字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| BMC 地址 | input | host，必填 |
| 端口 | number | port，默认 623 |
| 协议 | select | IPMI / Redfish |
| 用户名 | input | username，必填 |
| 密码 | password | password，必填 |

**交互流程：**

1. 用户填写 5 个字段
2. 点击「检测设备」按钮 → 调用 `POST /bmc/probe`
3. 成功后，弹窗或下方区域展示探测结果（设备名、品牌、型号、SN、MAC、引导模式）
4. 用户可编辑探测结果（部分信息可能不准），确认后保存
5. 探测失败时展示错误信息，用户可选择「跳过检测直接保存」

**编辑时**：填 5 个核心字段（密码留空 = 不修改），同样可以重新探测刷新。
**列表页**：每行有「刷新」按钮，调用 `POST /bmc/{id}/refresh` 重新探测并更新库中字段。

### 3.4 CSV 导入弹窗

文本域粘贴 CSV，只需 5 个核心字段：
```
host,port,username,password,protocol
10.0.0.1,623,admin,admin,ipmi
10.0.0.2,443,admin,admin,redfish
```

导入后异步探测各设备信息，列表页逐步显示品牌/型号/SN 等。导入结果显示成功/失败数。

### 3.5 状态管理

- 每个 BMC 配置的电源状态独立维护（`Map<id, Status>`）
- 页面切换回来时重新加载
- 操作后 2s 自动刷新对应行状态

---

## 4. 引导模式（Boot Mode）

### 4.1 存储

`bmc_configs.boot_mode` 字段存储，可选值：
- `auto` — 默认，DHCP 根据 Option 93 自动选择引导文件
- `uefi` — 强制 UEFI（ipxe.efi）
- `legacy` — 强制 Legacy（undionly.kpxe）

### 4.2 与 PXE/DHCP 联动（待定）

当前 PxeGo 的 DHCP 处理器通过 `boot.NBPFilename(arch, bootloader)` 根据客户端架构选择引导文件。后续迭代中将 BMC 的 `boot_mode` 注入 DHCP 响应逻辑。本期只做 CRUD 和页面设置，不做 DHCP 联动。

---

## 5. 密码安全

- 密码明文存储于 SQLite（IPMI 协议本身不含加密，且 BMC 密码通常在内部网络）
- API GET 返回时 password 字段为空，附加 `has_password` 标记
- POST/PUT 时 password 为空串则可选：新建时报错，编辑时保留原值
- 未来可加 AES 加密存储

---

## 6. 依赖

| 依赖 | 用途 | 已有 |
|------|------|------|
| `github.com/bougou/go-ipmi` | IPMI RMCP 协议 | ✓ |
| `net/http`（标准库） | Redfish（二期） | ✓ |

---

## 7. 文件改动

### 新增
```
internal/bmc/types.go          — Controller 接口 + 类型
internal/bmc/ipmi.go           — 适配新接口的 IPMI 实现
internal/bmc/factory.go         — 工厂函数
internal/models/bmc_config.go   — BMCConfig 模型
internal/store/sqlite_bmc.go    — SQLite CRUD 实现
internal/api/bmc_handler.go     — BMC 配置 + 电源操作 API
webdist/views/BmcView.vue       — 带外管理页面
webdist/components/BmcConfigForm.vue — BMC 配置表单组件
```

### 修改
```
internal/models/models.go       — (如有必要) 注册 BMCConfig
internal/store/store.go         — Interface 加 BMCConfigStore
internal/store/sqlite.go        — AutoMigrate 加 BMCConfig
internal/api/handler.go         — 注册 BMCHandler 和路由
webdist/router/index.js         — 加 /bmc 路由
webdist/App.vue                 — 侧边栏加导航
webdist/i18n/locales/zh.json    — 中文文案
webdist/i18n/locales/en.json    — 英文文案
```

---

## 8. IPMI FRU 探测技术方案

### 8.1 探测内容与对应 IPMI 命令

通过 `go-ipmi` 库读取 BMC FRU 数据：

| 字段 | IPMI FRU 路径 |
|------|--------------|
| vendor | FRU Board Info → Board Manufacturer |
| model | FRU Board Info → Board Product Name |
| serial | FRU Board Info → Board Serial Number |
| name | FRU Chassis Info → Chassis Part Number 或 Board Product Name |

如果 FRU 数据不可用（某些 BMC 实现不完整），对应字段返回空串。

### 8.2 MAC 地址获取

通过 IPMI LAN 配置获取：
```
Get LAN Configuration Parameters → MAC Address
```

### 8.3 `go-ipmi` 库接口

当前使用的 `github.com/bougou/go-ipmi v0.8.3` 支持：
- `client.GetFRU(ctx)` — 读取 FRU 数据
- `client.GetLANConfig(ctx, channel, param)` — 读取 LAN 配置

---

## 9. 不作一期范围

- Redfish 协议支持
- 与现有 Host 模型的关联（device_id FK，主机表单面板）
- DHCP 引导模式联动
- 密码加密存储
- 物理定位（Identify/闪灯）
- 事件订阅和告警
