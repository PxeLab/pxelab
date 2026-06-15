# PxeGo 设计文档

日期：2026-06-15
状态：草稿

## 概述

PxeGo 是一个 Go 语言开发的一体化 PXE 服务器，完整复现并扩展 Tiny PXE Server 的全部功能。单二进制、跨平台（Windows/Linux/macOS），双模式运行（无桌面环境的命令行启动模式 + 浏览器管理界面）。

## 架构

```
┌──────────────────────────────────────────────────────────────┐
│                     App Orchestrator                          │
│   errgroup + 信号处理 + 优雅关闭（反向顺序 Stop）            │
└──────┬──────────┬──────────┬──────────┬─────────────────────┘
       │          │          │          │
  ┌────┴───┐ ┌───┴────┐ ┌──┴─────┐ ┌──┴──────────┐
  │  DHCP  │ │  TFTP  │ │  DNS   │ │   HTTP      │
  │  :67   │ │  :69   │ │  :53   │ │  :8080      │
  │ full/  │ │        │ │ 转发   │ │ /api/v1/*   │
  │ proxy/ │ │        │ │ 本地   │ │ /boot/*     │
  │ hybrid │ │        │ │ 记录   │ │ /* (SPA)    │
  └───┬────┘ └───┬────┘ └───┬────┘ └──────┬──────┘
      │          │          │              │
      └──────────┴──────────┴──────────────┘
                      │
             ┌────────┴────────┐
             │ BootFileServer  │
             │ 统一文件根目录   │
             └────────┬────────┘
                      │
    ┌─────────────────┼─────────────────┐
    │                 │                 │
┌───┴────┐   ┌───────┴───────┐  ┌──────┴─────┐
│ iPXE   │   │  PXELinux     │  │ WDS        │
│ 引擎   │   │  解析→AST→    │  │ 仿真       │
│ (模板) │   │  iPXE 生成    │  │            │
└────────┘   └───────────────┘  └────────────┘

┌──────────────────────────────────────────────────────────────┐
│  基础设施层                                                   │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌─────────────┐  │
│  │ EventBus │  │  Store   │  │  IPMI    │  │  Config     │  │
│  │ pub/sub  │  │ SQLite   │  │ go-ipmi  │  │  viper+YAML │  │
│  │ 非阻塞   │  │ GORM     │  │ 封装     │  │  热重载     │  │
│  └──────────┘  └──────────┘  └──────────┘  └─────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Server 接口

每个服务模块实现以下接口：

```go
type Server interface {
    Name() string
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
}
```

App 编排器流程：加载配置 → 初始化所有 Server → errgroup 并行 Start → 等待信号 → 反向顺序 Stop 级联。

## 多网卡架构

网卡（interface）是第一级配置单元，每个 interface 独立绑定 IP、子网和服务角色。

```yaml
interfaces:
  # 场景 A：单网卡，多网段
  - name: eth0
    ip: 0.0.0.0
    subnets:
      - cidr: 192.168.1.0/24
        dhcp: full
        pool: 192.168.1.100-200
        gateway: 192.168.1.1
        next_server: 192.168.1.10
      - cidr: 10.0.0.0/24
        dhcp: full
        pool: 10.0.0.100-200

  # 场景 B：多网卡，不同角色
  - name: eth0
    ip: 192.168.1.1/24
    dhcp: full
    tftp: true
    http: true
  - name: eth1
    ip: 10.0.0.1/24
    dhcp: proxy

  # 场景 C：单网卡，纯 proxy
  - name: eth0
    ip: 192.168.1.10/24
    dhcp: proxy
    tftp: true
    http: true
    dns: true
```

### ProxyDHCP 方案

**端口 67（统一处理）**：单 UDP socket 监听 `0.0.0.0:67`，通过 `ReadMsgUDP` 获取数据包的目标 IP（`DstIP`），以此匹配对应 interface 配置。检查 option 60 决定 full/proxy：

- 值 = `PXEClient` → proxy 逻辑（仅回 PXE option，不分配 IP）
- 值 ≠ `PXEClient` → full 逻辑（分配 IP + 网关 + PXE option）

**端口 4011（标准 ProxyDHCP）**：独立 socket 监听 `:4011`，接收 PXE 客户端直接发送到 4011 端口的请求。`net.ListenUDP("udp", ":4011")`，不需要 gopacket/npcap。用于需要严格 PXE 规范兼容的环境。

### 跨平台多网卡差异

| 能力 | Linux | Windows |
|------|-------|---------|
| `SO_BINDTODEVICE` | ✅ 支持 | ❌ 不支持 |
| 绑定到特定 IP | ✅ | ✅ |
| `ReadMsgUDP` 获取 `DstIP` | ✅ | ✅ |

**方案：统一用 `0.0.0.0:67` + `ReadMsgUDP` 获取目标 IP**，Linux 和 Windows 同一套代码。

## DHCP / PXE Options

### 已支持的 PXE 选项

| 代码 | 名称 | 支持状态 |
|------|------|---------|
| 43 | Vendor Specific Information | 库提供容器；我们实现子选项 6-11 |
| 60 | Vendor Class Identifier (PXEClient) | 库内置 |
| 66 | TFTP Server Name | 库内置 |
| 67 | Bootfile Name | 库内置 |
| 82 | Relay Agent Information | 库内置 |
| 93 | Client System Architecture | 库内置（37 种架构类型） |
| 94 | Client Network Interface ID | 库内置 |
| 97 | Client Machine Identifier (GUID) | 库内置 |
| 128-135 | Site-specific（由 NBP 使用）| 库提供常量支持 |
| 150 | TFTP Server Address | 库内置 |
| 175 | Etherboot/iPXE 选项 | 库提供常量；我们实现子选项 |
| 208-211 | PXELinux 选项 | 库内置 |

### PXE Option 43 子选项（需自定义实现）

| 子选项 | 名称 | 用途 |
|-------|------|------|
| 6 | Discovery Control | 控制客户端广播/组播/发现行为 |
| 7 | Boot Server List | 按类型分配特定引导服务器 |
| 8 | Boot Menu | 显示给用户的引导菜单项 |
| 9 | Boot Item | 选中的引导条目类型 + 层级 |
| 10 | Link Layer Discovery | 链路层信息 |
| 11 | Credential Types | 凭证类型信息 |

### DHCP Handler 处理流程

```
收到 DHCPDISCOVER
  → 读取 Option 60 (VCI)
    → 非 "PXEClient" → Full DHCP（分配 IP + 网关 + DNS）
    → "PXEClient" → 进入 PXE 流程
      → 读取 Option 93 (Arch) → 按架构选择引导文件
      → 读取 Option 97 (GUID) → 匹配主机策略
      → 匹配来源子网 → 确定 interface 角色
        → full：分配 IP + siaddr + bootfile + option 43 子选项
        → proxy：不分配 IP，仅回复 option 43 + 66/67 + siaddr
```

### 架构类型 → 引导文件映射

> ⚠️ **注意**：`insomniacslk/dhcp` 库的 `iana.Arch` 常量命名与 RFC 4578 可能不一致（库中 EFI_X86_64=7、EFI_BC=9，而 RFC 4578 规定 7=BC、9=x86-64）。Phase 2 测试阶段需要抓包确认实际客户端 option 93 的 wire value，据此调整映射。

| 架构 | 标准值 | 建议引导文件 |
|------|--------|------------|
| BIOS (0) | 0 | undionly.kpxe |
| EFI IA32 (6) | 6 | ipxe32.efi |
| EFI x86-64 | 9 | ipxe.efi |
| EFI BC | 7 | ipxe.efi |
| EFI ARM64 (11) | 11 | ipxe-arm64.efi |

## 数据模型

### Host（主机）

```go
type Host struct {
    ID           string     // UUID
    Name         string     // 主机名
    MAC          string     // MAC 地址
    IP           string     // 固定/期望 IP
    ProfileID    *string    // 关联的引导配置（nil 表示用默认）
    BMCAddr      string     // IPMI BMC IP:port
    BMCUser      string
    BMCPass      string
    MenuOverride *string    // 主机级菜单覆盖（可选）
    CreatedAt    time.Time
    UpdatedAt    time.Time
    LastOnline   *time.Time
    BootCount    int
}
```

### Profile（引导配置）

```go
type Profile struct {
    ID          string
    Name        string
    Description string
    Menu        BootMenu   // 结构化菜单条目
    IsDefault   bool       // 全局默认标记
    Arch        *iana.Arch // 可选的架构绑定
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type BootMenu struct {
    Entries []MenuEntry
}

type MenuEntry struct {
    Label   string   // 显示名称
    Type    MenuType // local | direct | chain | sanboot | wds
    Kernel  *string  // direct 类型时需要
    Initrd  *string
    Cmdline *string
    URL     *string  // chain 类型时需要
    WIM     *string  // WDS 类型时需要
}
```

### 菜单解析顺序

```
主机 → Host.MenuOverride（精确匹配）
  │
  └→ 配置 → Profile.Menu（配置级）
       │
       └→ 默认 → Profile.IsDefault=true（全局默认）
```

### Event（事件日志）

```go
type Event struct {
    ID        string
    Type      EventType  // DHCP | TFTP | HTTP | BOOT | IPMI | DNS
    Level     EventLevel // INFO | WARN | ERROR
    Message   string
    MAC       *string
    IP        *string
    Detail    map[string]any
    Timestamp time.Time
}
```

### Lease（DHCP 租约）

```go
type Lease struct {
    MAC       string
    IP        string
    SubnetID  string
    Hostname  *string
    ExpiresAt time.Time
    CreatedAt time.Time
}
```

## REST API

```
GET    /api/v1/status                    → 服务健康 + 各组件状态

GET    /api/v1/events                    → 事件日志（分页、类型过滤）
GET    /api/v1/events/stream             → SSE 实时流

GET    /api/v1/hosts                     → 列表（搜索、分页）
POST   /api/v1/hosts                     → 创建
GET    /api/v1/hosts/{id}                → 详情
PUT    /api/v1/hosts/{id}                → 更新
DELETE /api/v1/hosts/{id}                → 删除
POST   /api/v1/hosts/{id}/wake           → WoL 唤醒
POST   /api/v1/hosts/{id}/power          → IPMI 电源操作

GET    /api/v1/profiles                  → 列表
POST   /api/v1/profiles                  → 创建
GET    /api/v1/profiles/{id}             → 详情
PUT    /api/v1/profiles/{id}             → 更新
DELETE /api/v1/profiles/{id}             → 删除

GET    /api/v1/files                     → 浏览目录 (?path=...)
POST   /api/v1/files/upload              → 上传文件
DELETE /api/v1/files                     → 删除文件 (?path=...)

GET    /api/v1/leases                    → 活跃 DHCP 租约

GET    /api/v1/settings                  → 当前配置
PUT    /api/v1/settings                  → 更新配置（热重载）
POST   /api/v1/settings/reload           → 重新加载配置文件
```

统一返回格式：
```json
{
    "success": true,
    "data": { ... },
    "error": null,
    "meta": { "page": 1, "size": 20, "total": 150 }
}
```

鉴权：Bearer token（可选）。未配置 token 时仅允许 localhost 访问。

## 目录结构

```
PxeGo/
├── cmd/pxego/
│   ├── main.go                    # cobra 根命令、信号处理
│   ├── embed.go                   # //go:embed web/dist
│   └── service_*.go               # OS 服务适配器
├── internal/
│   ├── app/                       # App 编排器 + Server 接口
│   ├── config/                    # YAML 配置结构体 + 多网卡
│   ├── models/                    # Host, Profile, BootMenu, Event, Lease
│   ├── store/                     # Store 接口 → SQLite (GORM)
│   ├── eventbus/                  # Channel pub/sub
│   ├── dhcp/
│   │   ├── server.go              # UDP 监听 :67 + :4011
│   │   ├── handler.go             # 按子网分发
│   │   ├── lease.go               # 多子网 IP 池
│   │   └── options.go             # PXE option 43 子选项构建器
│   ├── tftp/                      # TFTP :69
│   ├── dns/                       # DNS :53（转发 + 本地记录）
│   ├── boot/
│   │   ├── serve.go               # BootFileServer 统一根目录
│   │   ├── ipxe/                  # text/template 引擎 + 内置模板
│   │   ├── pxelinux/              # 解析器 → AST → iPXE 生成器
│   │   └── archmap.go             # 架构 → 引导文件映射
│   ├── httpd/                     # Chi 路由: API + boot + SPA
│   ├── api/                       # REST 处理器 + SSE
│   ├── ipmi/                      # go-ipmi 封装
│   └── wds/                       # WDS 仿真
├── web/                           # React SPA
│   ├── src/
│   │   ├── api/client.ts
│   │   ├── hooks/useSSE.ts
│   │   ├── i18n/                  # react-i18next 配置 + 翻译文件
│   │   ├── pages/                 # 7 个页面
│   │   └── components/            # ui/ + layout/ + domain/
│   ├── package.json
│   └── vite.config.ts
├── boot/                          # 默认引导文件目录
├── contrib/                       # systemd、launchd、示例配置
└── .goreleaser.yaml
```

## 前端设计

### 技术栈
- React 18 + TypeScript + Vite + Tailwind CSS 3 + React Router 6
- react-i18next（中英文）
- CSS Variables 实现主题切换（深色/浅色）
- Lucide React 图标

### 页面
1. **仪表板 Dashboard** — 服务状态指示灯 + 统计卡片 + 近期事件
2. **主机管理 Hosts** — 可搜索排序表格 + 分页
3. **主机详情 HostDetail** — 主机信息 + IPMI 电源控制面板 + 引导历史
4. **引导配置 Profiles** — 配置列表 + 结构化菜单编辑器
5. **文件管理 Files** — 目录树浏览 + 上传/删除
6. **事件日志 Events** — SSE 实时流 + 类型过滤 + 暂停/继续
7. **设置 Settings** — 配置表单（Tab 分页）

### 主题系统

```css
[data-theme="dark"] {
  --color-bg-primary: #0a0c10;
  --color-bg-elevated: #111318;
  --color-bg-card: #16181f;
  --color-text-primary: #e8eaed;
  --color-accent: #3b82f6;
  --color-success: #22c55e;
  --color-warning: #eab308;
  --color-danger: #ef4444;
}

[data-theme="light"] {
  --color-bg-primary: #f4f5f7;
  --color-bg-card: #ffffff;
  --color-text-primary: #1a1a2e;
  --color-accent: #4361ee;
  --color-success: #10b981;
  --color-warning: #f59e0b;
  --color-danger: #ef4444;
}
```

## 实施阶段

### Phase 1：基础设施（第 1-2 天）
- Go module + 目录树
- `internal/config/` — YAML 配置、多网卡结构、校验
- `internal/models/` — 域模型
- `internal/eventbus/` — 通道 pub/sub
- `internal/store/` — 接口 + GORM SQLite
- `internal/app/` — 编排器 + Server 接口
- `cmd/pxego/main.go` — cobra 根命令、配置加载、信号处理
- **交付物**：`pxego --help` 可用，配置文件解析完成，SQLite 数据库创建

### Phase 2：DHCP 核心（第 3-5 天）
- `internal/dhcp/lease.go` — 多子网 IP 池
- `internal/dhcp/options.go` — PXE option 43 子选项构建器
- `internal/dhcp/handler.go` — 按子网分发、full/proxy/hybrid
- `internal/dhcp/server.go` — UDP 监听 :67 + ProxyDHCP :4011
- 多网卡：统一用 `0.0.0.0:67` + `ReadMsgUDP` 获取 DstIP
- **交付物**：客户端通过 DHCP 获取 IP，按架构返回 PXE 选项

### Phase 3：TFTP + BootFileServer（第 6 天）
- `internal/boot/serve.go` — BootFileServer 抽象
- `internal/boot/archmap.go` — 架构 → 引导文件映射
- `internal/tftp/server.go` + `handler.go` — TFTP :69
- **交付物**：TFTP 文件传输可用，按架构自动选择

### Phase 4：HTTP + iPXE 引擎（第 7-9 天）
- `internal/httpd/` — Chi 路由器
- `/boot/*` 文件服务
- `internal/boot/ipxe/engine.go` — text/template
- `internal/boot/ipxe/templates.go` — menu/direct/local/chain/sanboot
- 三层菜单解析（默认 → 配置 → 主机）
- **交付物**：通过 HTTP 提供 PXE 菜单，动态 iPXE 脚本

### Phase 5：DNS + PXELinux（第 10-12 天）
- `internal/dns/server.go` + `handler.go` — 转发 + 本地 A/PTR
- `internal/boot/pxelinux/parser.go` — 递归下降 PXELinux 解析器
- `internal/boot/pxelinux/ast.go` — AST 节点类型
- `internal/boot/pxelinux/generator.go` — AST → iPXE，HTTP 路径重写
- **交付物**：PXELinux 配置无需修改即可工作，DNS 转发可用

### Phase 6：REST API + IPMI + WDS（第 13-15 天）
- `internal/api/` — 完整 REST 处理器 + SSE 流
- `internal/ipmi/` — 电源开关/重启/状态查询，基于 go-ipmi
- `internal/wds/` — WDS 仿真（bootmgr.exe、BCD、boot.sdi、WIM）
- Bearer token 鉴权中间件
- **交付物**：API 全功能可用，IPMI 电源控制可用，WDS 启动可用

### Phase 7：前端 SPA（第 16-20 天）
- React + Vite + TS + Tailwind 脚手架
- 国际化（中文/英文）+ 主题切换（深色/浅色）
- 7 个页面，完整状态覆盖（加载/空/错误/数据）
- SSE 实时事件流，支持暂停/继续
- 结构化菜单编辑器
- `//go:embed web/dist/*`
- **交付物**：单二进制在 http://localhost:8080/ 提供 SPA

### Phase 8：加固 + 服务集成 + 发布（第 21-23 天）
- 启动配置校验
- 租约/事件清理 goroutine
- 优雅关闭：逐服务超时控制、连接排空
- 错误处理审计
- Windows：`golang.org/x/sys/windows/svc`
- Linux：systemd notify + watchdog
- macOS：launchd 兼容
- `pxego service install|remove|start|stop`
- `.goreleaser.yaml`：linux/windows/darwin × amd64/arm64
- **交付物**：跨平台发布二进制，可作为 OS 服务安装

## 依赖清单

| 依赖 | 用途 |
|------|------|
| cobra + viper | CLI + YAML 配置 |
| go-chi/chi/v5 | HTTP 路由 |
| insomniacslk/dhcp | DHCP 包处理库 |
| pin/tftp | TFTP 服务器 |
| miekg/dns | DNS 库 |
| bougou/go-ipmi | IPMI 客户端 |
| GORM + glebarez/sqlite | 数据库（无 CGo） |
| google/gopacket | ARP/网络扫描 |
| google/uuid | UUID 生成 |
| go-wol | Wake-on-LAN |

## 风险清单

| 风险 | 等级 | 缓解措施 |
|------|------|---------|
| DHCP :67 需要 root（Linux）| 致命 | 文档说明 `CAP_NET_BIND_SERVICE`，systemd unit 添加 AmbientCapabilities |
| DHCP :67 需要管理员（Windows）| 致命 | 文档说明管理员要求，安装时添加防火墙规则 |
| SQLite 写入并发锁 | 高 | WAL 模式，连接池设为 1 |
| TFTP 并发性能不足 | 高 | 可配置最大并发传输数 |
| iPXE 模板注入 | 高 | 模板数据沙箱化，不允许用户自定义模板语法 |
| macOS :53 端口冲突 | 中 | 检测端口冲突，macOS 默认关闭 DNS |
| 跨平台 UDP 行为差异 | 中 | 所有平台测试广播行为 |
| PXELinux 配置匹配规则 | 中 | 完整实现 pxelinux.cfg 层级查找 |
| 前端构建依赖 | 低 | goreleaser before hook 执行 npm build |

## 跨平台服务集成

```
cmd/pxego/
├── main.go                        # 通用入口
├── service_windows.go             //go:build windows
├── service_linux.go               //go:build linux
├── service_darwin.go              //go:build darwin
├── path_windows.go                //go:build windows
└── path_unix.go                   //go:build unix
```

### 各平台注意事项

- **Linux**：`contrib/` 中提供 systemd service 文件，`CAP_NET_BIND_SERVICE` + `CAP_NET_RAW`
- **Windows**：需要管理员权限运行，Windows Defender 防火墙放行规则，`golang.org/x/sys/windows/svc`
- **macOS**：`contrib/` 中提供 launchd plist，SIP 对端口绑定的影响，端口 53 冲突检测

## PxeGo vs Tiny PXE Server 功能对照

| 功能 | Tiny PXE | PxeGo |
|------|----------|-------|
| DHCP full | ✅ | ✅ |
| DHCP proxy | ✅ | ✅ |
| ProxyDHCP :4011 | ✅ | ✅ |
| TFTP | ✅ | ✅ |
| HTTP 启动 | ✅ | ✅ |
| DNS | ✅ | ✅（可选转发） |
| PXELinux | ✅ | ✅（AST 翻译） |
| gPXE/iPXE | ✅ | ✅（模板引擎） |
| BIOS/UEFI 检测 | ✅ | ✅（option 93） |
| WinPE/WDS | ✅ | ✅（WDS 仿真） |
| IPMI | ❌ | ✅ |
| REST API | ❌ | ✅ |
| Web 管理界面 | ❌ | ✅ |
| 多网卡 | 部分 | ✅（一等配置支持） |
| 多子网 | ❌ | ✅ |
| 配置热重载 | ❌ | ✅ |
| 国际化 | ❌ | ✅（中/英） |
