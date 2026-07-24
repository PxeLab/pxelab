# 架构与概述

> PxeLab 的核心定位、功能特性与服务架构。

**相关文档**: [快速开始](getting-started.md) | [DHCP 配置](guides/dhcp.md) | [引导配置](guides/boot-config.md)

---

## 项目概述

**PxeLab** 是一款一体化 PXE 网络引导服务器，将 DHCP、TFTP、HTTP、DNS、NFS 等服务集成在单个二进制中，通过 Web UI 和 REST API 进行管理。

### 核心定位

| 维度 | 描述 |
|------|------|
| **产品形态** | 单二进制 + 嵌入式 Web UI，开箱即用 |
| **目标用户** | IT 运维工程师、IDC 工程师、系统管理员 |
| **核心场景** | 批量装机、无盘工作站、OS 安装、服务器维护 |
| **技术栈** | Go 1.23+ / React 19 / TypeScript / Tailwind CSS 4 / SQLite |
| **平台支持** | Windows / Linux / macOS |

### 与其他方案的对比

| 特性 | PxeLab | 传统 PXE（TFTP-only） | Foreman/Cobbler |
|------|--------|----------------------|-----------------|
| 安装复杂度 | 单二进制，零依赖 | 手动配置多个服务 | 需要大量依赖 |
| iPXE 支持 | 内置自定义编译 iPXE | 需自行编译 | 需自行集成 |
| 多架构 | 11 种 CPU 架构 + Secure Boot | 通常仅 x86 | 有限 |
| Web 管理 | 内置，全功能 | 无 | 有，但复杂 |
| DHCP 模式 | full / proxy / hybrid / off | 通常仅 one mode | 有限 |
| NFS | 内置 NFSv3 | 需外部 NFS | 需外部 |

---

## 功能特性

### 网络服务

- **DHCP 服务器** — 支持 full / proxy / hybrid / off 四种模式，每个接口独立配置
- **ProxyDHCP** — 端口 4011，叠加到现有 DHCP 环境
- **TFTP 服务器** — 可配置端口和超时，提供 NBP 文件
- **HTTP 服务器** — 提供引导脚本、Web UI、SPA、引导文件
- **DNS 服务器** — 本地 DNS 解析 + 上游转发，A/AAAA/CNAME 记录
- **NFS 服务器** — 内置 NFSv3，多挂载点，IP 访问控制

### 引导能力

- **iPXE** — 自定义编译，嵌入式引导脚本，11 种架构
- **Secure Boot** — x86_64 和 ARM64 架构支持 UEFI Secure Boot
- **引导类型** — direct（内核+initrd）、chain（链式加载）、wds（Windows WIM）、sanboot（iSCSI SAN）、local（本地硬盘）
- **PXELinux** — 配置解析器 + AST + iPXE 脚本生成器
- **架构自动检测** — 根据 DHCP Option 93 客户端架构自动选择引导文件

### 管理功能

- **Web UI** — React SPA，暗色主题，中英双语
- **REST API** — v1 版本，完整的 CRUD 操作
- **主机管理** — 增删改查、分组、绑定 Profile
- **Profile** — 引导配置文件，支持脚本版本管理、差异对比、回滚
- **OS 安装目录** — 10 个预置发行版分组，拖拽排序
- **应答文件模板** — 预设模板 + 自定义，支持预览和验证
- **安装任务** — 跟踪装机进度

### 硬件管理

- **WOL** — 网络唤醒，支持定时调度
- **BMC/IPMI** — 带外电源控制（开机/关机/重启/状态查询），CSV 批量导入
- **OS 镜像** — ISO 上传、挂载、发行版检测

### 运维工具

- **实时日志** — 多面板 SSE 日志流，按服务过滤
- **审计日志** — 跟踪所有配置变更
- **访问控制** — MAC 黑白名单
- **网络诊断** — Ping / Traceroute（支持流式输出）
- **Prometheus 指标** — `/api/v1/metrics` 端点
- **日志轮转** — 按大小/天数/备份数自动轮转

---

## 两阶段网络引导

PxeLab 采用两阶段网络引导架构，将 PXE ROM 的有限能力逐步升级到功能完整的 iPXE：

```
Stage 1                           Stage 2
┌─────────────┐   TFTP/HTTP    ┌──────────┐   HTTP     ┌──────────────┐
│  PXE ROM    ├───────────────►│  iPXE    ├───────────►│  引导菜单    │
│  (BIOS/UEFI)│  undionly.kpxe │  (自定义  │  /boot/    │  (内核+initrd│
│             │  /ipxe.efi     │   编译)   │  ipxe/     │   /WIM/Chain │
└─────────────┘                │          │  script    │   /Local)    │
                               └──────────┘           └──────────────┘
```

**Stage 1: PXE ROM → iPXE**

1. 客户端 PXE ROM 发送 DHCP Discover
2. PxeLab DHCP 服务器响应 Offer/Ack，包含：
   - IP 地址（full/hybrid 模式）
   - next-server（TFTP 服务器地址）
   - bootfile（NBP 文件名，如 `ipxe.efi`）
   - Option 175.178（iPXE 引导脚本 URL）
3. 客户端通过 TFTP 下载 NBP
4. PXE ROM 加载并执行 NBP → iPXE 启动

**Stage 2: iPXE → 引导菜单**

1. iPXE 内嵌脚本自动执行：`dhcp` → `chain http://server:8080/boot/ipxe/script?mac=xx`
2. PxeLab 根据 MAC 地址查询主机绑定的 Profile
3. 返回对应的引导菜单脚本
4. 客户端显示菜单，用户选择引导项

---

## 服务架构

```
┌─────────────────────────────────────────────────────┐
│                    PxeLab Binary                     │
├──────────┬──────────┬──────────┬──────────┬─────────┤
│  HTTP    │  DHCP    │  TFTP    │  DNS     │  NFS    │
│  :8080   │  :67     │  :69     │  :53     │  :2049  │
│  TCP     │  UDP     │  UDP     │  UDP     │  TCP    │
├──────────┴──────────┴──────────┴──────────┴─────────┤
│              Service Manager (生命周期管理)           │
├─────────────────────────────────────────────────────┤
│  SQLite (pxelab.db)  │  Event Bus  │  Log Bus      │
├─────────────────────────────────────────────────────┤
│              Config (config.yaml)                    │
└─────────────────────────────────────────────────────┘
```

| 服务 | 默认端口 | 协议 | 默认自启 | 说明 |
|------|---------|------|---------|------|
| HTTP | 8080 | TCP | ✅ | Web UI + API + 引导文件服务 |
| DHCP | 67 | UDP | 取决于配置 | IP 地址分配 + PXE 选项 |
| ProxyDHCP | 4011 | UDP | 取决于配置 | 仅 PXE 选项（叠加模式） |
| TFTP | 69 | UDP | ❌ | NBP 文件传输 |
| DNS | 53 | UDP | ❌ | 本地 DNS 解析 |
| NFS | 2049 | TCP | ❌ | NFSv3 文件共享 |

---

## 部署模式

PxeLab 支持两种运行模式：

### Server 模式（默认）

```bash
pxelab --mode server
# 或
pxelab   # 默认即 server 模式
```

- 前台运行，日志输出到 stderr
- 适合服务器部署、后台运行
- Linux 下可通过 systemd 管理

### App 模式

```bash
pxelab --mode app
```

- 自动打开浏览器
- Windows 下以系统托盘运行（隐藏控制台窗口）
- 适合桌面环境、个人使用

### Windows 系统托盘

在 Windows 上，PxeLab 检测到桌面环境时自动启用系统托盘模式：

- 右键托盘图标：打开浏览器 / 打开数据目录 / 退出
- 托盘图标显示服务运行状态
- 关闭浏览器不会停止服务，通过托盘退出才会
