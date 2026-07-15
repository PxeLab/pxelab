# BMC/IPMI 带外控制 + PXE 引导模式设计

> **设计目标：** 在 MuQue 中支持 BMC/IPMI/Redfish 远程电源管理和引导设备控制，同时完善 PXE 引导模式（UEFI/Legacy）自动探测和配置。

---

## 1. 数据模型

### 1.1 `bmc_configs` 表（独立表）

```sql
CREATE TABLE IF NOT EXISTS bmc_configs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    device_id INTEGER UNIQUE NOT NULL,
    host TEXT NOT NULL,
    port INTEGER DEFAULT 623,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    redfish BOOLEAN DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (device_id) REFERENCES devices(id) ON DELETE CASCADE
);
```

- `device_id` UNIQUE — 每台设备最多一个 BMC 配置
- `redfish` — false = IPMI (RMCP/UDP 623)，true = Redfish (HTTPS)
- BMC 配置通过 DeviceForm 或 "带外控制"页面管理

### 1.2 `boot_mode` 字段（devices 表）

devices 表加字段：

```sql
ALTER TABLE devices ADD COLUMN boot_mode TEXT NOT NULL DEFAULT 'auto';
```

- `auto` — 根据 DHCP Option 93（Client Architecture Identifier）自动选择
- `uefi` — 强制 UEFI 引导（返回 `ipxe.efi`）
- `legacy` — 强制 Legacy BIOS 引导（返回 `undionly.kpxe`）

---

## 2. 后端模块

### 2.1 `internal/bmc/types.go` — 公共类型和接口

```go
package bmc

import "context"

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
}
```

### 2.2 `internal/bmc/ipmi.go` — IPMI 实现（bougo/go-ipmi）

使用 `github.com/bougou/go-ipmi` 库通过 RMCP/UDP 623 通信：

- `NewIPMIController(cfg Config) Controller`
- 支持：电源控制（chassis power on/off/restart/status）、引导设备设置（chassis bootdev）
- 实现所有 `Controller` 接口方法

### 2.3 `internal/bmc/redfish.go` — Redfish 实现

标准 REST API 实现，无需外部依赖（用 `net/http`）：

- `NewRedfishController(cfg Config) Controller`
- 支持：电源控制（`/redfish/v1/Systems/{id}/Actions/...`）、引导设备设置（`Boot/BootSourceOverrideTarget`）
- 自动发现 System ID（枚举 Systems 集合）
- 实现所有 `Controller` 接口方法

### 2.4 `internal/bmc/factory.go` — 工厂函数

```go
func NewController(cfg Config) (Controller, error) {
    if cfg.Redfish {
        return NewRedfishController(cfg), nil
    }
    return NewIPMIController(cfg)
}
```

### 2.5 PXE 引导模式逻辑（修改现有代码）

在 `internal/server/ipxe_routes.go` 中，选择 bootfile 时增加模式判断：

```
auto: 解析 DHCP Option 93 值 → 0/1/2/4/5/6/7/8/9/10/11/12/13/14/15/16 → UEFI: ipxe.efi, Legacy: undionly.kpxe
uefi: 强制 ipxe.efi
legacy: 强制 undionly.kpxe
```

Option 93 值对照：

| AAI | 架构 | 引导文件 |
|-----|------|----------|
| 0 | BIOS x86 | undionly.kpxe |
| 6 | UEFI x86_64 | ipxe.efi |
| 7 | UEFI x86 | bootx32.efi |
| 9 | UEFI ARM64 | ipxe-arm64.efi |

---

## 3. API 路由

### 3.1 BMC 配置 CRUD

```
GET    /api/bmc/configs              → 列表（支持 ?device_id= 筛选）
GET    /api/bmc/configs/:id          → 详情
POST   /api/bmc/configs              → 创建
PUT    /api/bmc/configs/:id          → 更新
DELETE /api/bmc/configs/:id          → 删除
POST   /api/bmc/configs/import       → CSV 批量导入
```

### 3.2 电源控制

```
POST   /api/bmc/:configId/poweron    → 开机
POST   /api/bmc/:configId/poweroff   → 关机  
POST   /api/bmc/:configId/restart    → 重启
GET    /api/bmc/:configId/status     → 电源状态
POST   /api/bmc/:configId/bootdev    → 设置引导设备 { "device": "pxe"|"disk"|"cdrom"|"bios" }
```

### 3.3 批量操作

```
POST   /api/bmc/batch/poweron        → 批量开机 { "config_ids": [1,2,3] }
POST   /api/bmc/batch/poweroff       → 批量关机
POST   /api/bmc/batch/restart        → 批量重启
POST   /api/bmc/batch/status         → 批量查询状态
```

### 3.4 引导模式（已有路由扩展）

现有 `/ipxe?mac=XX` 路由，根据设备的 `boot_mode` 自动适配。

---

## 4. 前端

### 4.1 菜单结构

新增一级菜单「带外控制」，和「设备」「ISO」「部署」「设置」同级。

点击后在主内容区显示 BMC 管理页面。

### 4.2 BMC 管理页面

```
┌──────────────────────────────────────────────┐
│  带外控制                                      │
│  [导入 CSV] [批量开机] [批量关机] [批量重启]     │
├──────────────────────────────────────────────┤
│  ☐ │ 设备名 │ BMC IP   │ 协议  │ 状态  │ 操作  │
│  ☐ │ svr-01 │ 10.0.0.1 │ IPMI  │ ● 开机 │ [开机][关机][重启][引导] │
│  ☐ │ svr-02 │ 10.0.0.2 │ Redfish │ ○ 关机│ [开机][关机][重启][引导] │
│  ☐ │ svr-03 │ 10.0.0.3 │ IPMI  │ ● 开机 │ [开机][关机][重启][引导] │
└──────────────────────────────────────────────┘
```

- 每行显示电源状态指示灯（绿/红/灰）
- 操作按钮直接在当前行执行
- 批量操作：勾选后点顶部按钮
- 引导设备设置：弹出下拉选择（PXE/硬盘/光驱/BIOS）

### 4.3 引导模式选择器

在 DeviceForm 原有 OS Type 下面加：

```
引导模式: [auto (自动探测) ▼] [uefi] [legacy]
```

### 4.4 BMC 配置面板

DeviceForm 加可折叠「BMC/带外配置」面板：

```
▶ BMC/带外配置（点击展开）
  ┌─┬──────────┐
  │ 主机地址    │ [input]
  │ 端口        │ [623]
  │ 协议        │ [IPMI ▼] [Redfish]
  │ 用户名      │ [input]
  │ 密码        │ [password]
  └─┴──────────┘
```

---

## 5. 需要改动的文件

| 文件 | 说明 |
|------|------|
| `internal/bmc/types.go` | 公共接口和类型定义 |
| `internal/bmc/ipmi.go` | go-ipmi 实现 |
| `internal/bmc/redfish.go` | Redfish REST 实现 |
| `internal/bmc/factory.go` | 工厂函数 |
| `internal/database/bmc.go` | bmc_configs CRUD |
| `internal/database/db.go` | bmc_configs 表迁移 |
| `internal/database/models.go` | bmc_configs model |
| `internal/database/device.go` | boot_mode 字段 CRUD |
| `internal/database/migrations.go` | boot_mode ALTER TABLE |
| `internal/server/bmc_routes.go` | BMC 配置 + 电源控制 API |
| `internal/server/ipxe_routes.go` | 引导模式适配 |
| `frontend/src/router/index.js` | 加 "带外控制" 路由 |
| `frontend/src/views/BmcView.vue` | BMC 管理页面 |
| `frontend/src/components/DeviceForm.vue` | BMC 配置 + 引导模式 |
| `frontend/src/components/BmcConfigForm.vue` | BMC 配置表单组件 |
| `main.go` | 注册路由 |

---

## 6. 依赖

| 依赖 | 用途 | 类型 |
|------|------|------|
| `github.com/bougou/go-ipmi` | IPMI RMCP 协议 | Go 模块 |
| `net/http`（标准库） | Redfish REST API | 内置 |

---

## 7. 后续迭代（本期不做）

- **Redfish 事件订阅**：SSE 接收硬件告警（温度、风扇、磁盘）
- **虚拟介质挂载**：通过 Redfish 挂载 ISO 远程安装
- **BMC 固件更新**：自动化固件升级流程
- **SNMP 监控集成**：通过 SNMP 采集硬件指标
