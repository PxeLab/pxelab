# CLAUDE.md

本文档为 Claude Code (claude.ai/code) 在此仓库中工作时提供指导。

## 构建与测试命令

```bash
# 构建后端
make build              # go build -o bin/pxelab ./cmd/pxelab

# 运行后端（开发模式）
make run                # go run ./cmd/pxelab

# 运行所有测试
make test               # go test ./...
go test ./internal/boot/ipxe/...
go test -v -run TestName ./internal/boot/

# 前端开发
cd web && npm run dev   # Vite 开发服务器，/api 代理到 :8080

# 构建前端
make frontend           # cd web && npm ci && npm run build

# 完整发布
make release            # goreleaser release --clean
```

## 架构概览

**PxeLab** 是一款一体化 PXE 服务器，集成了 DHCP、TFTP、HTTP、DNS 和 NFS 服务，通过 Web UI 和 REST API 管理。

### 后端 (`cmd/pxelab/` + `internal/`)

- **入口点**: `cmd/pxelab/main.go` — Cobra CLI，在 `run()` 函数中组装所有服务
- **配置**: `internal/config/` — 基于 YAML，由 Viper 加载。配置结构体是所有设置的单一事实来源
- **API 层**: `internal/api/` — 使用 chi 路由的 REST 处理器，在 `Handler.RegisterRoutes()` 中注册。每个处理器结构体映射到一个资源（Host、Profile、Settings 等）
- **HTTP 服务器**: `internal/httpd/` — 包装 chi 路由，从嵌入的 `webdist/` 提供 SPA，并提供引导文件和 iPXE 脚本
- **引导链**: `internal/boot/` — `BootFileServer` 提供 TFTP/HTTP 引导文件（含路径穿越防护）。`archmap.go` 将客户端架构映射到引导加载器文件名。`nbp.go` 处理 NBP 重定向。`scriptmanager.go` 管理自定义引导脚本
- **iPXE**: `internal/boot/ipxe/` — 脚本生成引擎，基于配置的决策树。`templates.go` 包含嵌入的 `.ipxe` 模板
- **PXELinux**: `internal/boot/pxelinux/` — pxelinux.cfg 格式的解析器 + 生成器（基于 AST）
- **网络启动目录**: `internal/netboot/` — 发行版目录管理、菜单生成、应答文件模板、覆盖层支持
- **DHCP**: `internal/dhcp/` — 标准 DHCP（端口 67）+ ProxyDHCP（端口 4011），使用 insomniacslk/dhcp。支持子网地址池和白名单
- **NFS**: `internal/nfs/` — 内置 NFSv3 服务器，含挂载点管理、基于 IP 的访问控制、连接追踪
- **TFTP**: `internal/tftp/` — 可配置端口和超时，后端为 `BootFileServer`
- **DNS**: `internal/dns/` — 本地 DNS 服务器，支持上游转发、A/AAAA/CNAME 记录
- **存储层**: `internal/store/` — 基于接口的数据层。提供 SQLite（通过 GORM + go-sqlite）和内存两种实现
- **数据模型**: `internal/models/` — GORM 模型结构体，与 store 接口匹配
- **服务管理器**: `internal/servicemanager/` — 管理所有服务（DHCP、TFTP、HTTP、DNS、NFS）的启动/停止/重启生命周期
- **事件总线**: `internal/eventbus/` — 内部事件的发布/订阅（DHCP 租约、启动事件、WOL）
- **OS 镜像**: `internal/osimage/` — ISO 挂载、发行版检测、挂载点追踪
- **其他**: WOL (`internal/wol/`)、BMC/IPMI (`internal/bmc/`, `internal/ipmi/`)、WDS (`internal/wds/`)、指标 (`internal/metrics/`, Prometheus)、系统托盘 GUI (`internal/app/`)

### 前端 (`web/`)

- **技术栈**: React 19、TypeScript、Vite 6、Tailwind CSS 4、react-router-dom v7
- **API 客户端**: `web/src/api/client.ts` — 类型化的 fetch 封装，所有 API 调用集中在一处
- **UI 组件**: `web/src/components/ui/` — 共享基础组件（Button、Modal、DataTable、Toggle 等），全部使用 CSS 变量实现主题
- **布局**: `web/src/components/layout/AppShell.tsx` — 主导航栏应用外壳
- **页面**: `web/src/pages/` — 每个路由一个文件，通过 `React.lazy()` 懒加载
- **国际化**: `web/src/locales/en.json`、`zh-CN.json` — i18next + `react-i18next`
- **主题**: `index.css` 中的 CSS 变量，`ThemeSwitcher.tsx` 切换亮色/暗色模式

### 数据流

1. 客户端发起 PXE 启动 → DHCP 服务器提供 `next_server` + 引导文件名
2. 客户端通过 TFTP（或原生 HTTP）获取 NBP
3. NBP 通过 HTTP (`/boot/ipxe/script`) 链加载到 iPXE 脚本
4. iPXE 展示网络启动目录或基于配置文件的引导菜单
5. 所有管理操作通过 `/api/*` 的 REST API 完成 → 由 `internal/api/` 处理
6. 配置变更通过 API → 尽可能热重载服务，无需重启

### 关键模式

- **每资源一个处理器**: 每个 API 处理器是一个结构体，包含 CRUD 操作方法
- **存储接口分离**: `store.Interface` 组合多个小型子接口（HostStore、ProfileStore 等），便于切换实现
- **`run()` 中的依赖注入**: `cmd/pxelab/main.go` 显式构造并传递所有服务 — 无全局单例
- **CSS 变量主题**: 所有颜色引用 `var(--bg-base)`、`var(--text-primary)` 等 — `ThemeSwitcher` 切换 `<html>` 上的 class
- **页面懒加载**: 前端使用 `React.lazy()` + `Suspense` 按路由进行代码分割
