# PxeLab 用户手册

> **版本**: v0.4.0-dev · **最后更新**: 2026-07-24 · **许可证**: MIT

---

## 目录

- [1. 项目概述](#1-项目概述)
- [2. 功能特性](#2-功能特性)
- [3. 快速开始](#3-快速开始)
  - [3.1 系统要求](#31-系统要求)
  - [3.2 安装方式](#32-安装方式)
  - [3.3 首次启动](#33-首次启动)
  - [3.4 验证服务](#34-验证服务)
- [4. 架构与原理](#4-架构与原理)
  - [4.1 两阶段网络引导](#41-两阶段网络引导)
  - [4.2 服务架构](#42-服务架构)
  - [4.3 部署模式](#43-部署模式)
- [5. DHCP 配置](#5-dhcp-配置)
  - [5.1 四种 DHCP 模式](#51-四种-dhcp-模式)
  - [5.2 多接口部署](#52-多接口部署)
  - [5.3 IP 预留（DHCP Reservation）](#53-ip-预留dhcp-reservation)
  - [5.4 访问控制（黑白名单）](#54-访问控制黑白名单)
- [6. 引导配置](#6-引导配置)
  - [6.1 iPXE 引导脚本系统](#61-ipxe-引导脚本系统)
  - [6.2 引导菜单类型](#62-引导菜单类型)
  - [6.3 Profile（引导配置文件）](#63-profile引导配置文件)
  - [6.4 默认引导菜单](#64-默认引导菜单)
  - [6.5 自定义 iPXE 脚本](#65-自定义-ipxe-脚本)
- [7. 架构映射与 Secure Boot](#7-架构映射与-secure-boot)
  - [7.1 支持的客户端架构](#71-支持的客户端架构)
  - [7.2 Secure Boot 链](#72-secure-boot-链)
  - [7.3 chain_to_ipxe](#73-chain_to_ipxe)
- [8. 网络启动目录（Netboot）](#8-网络启动目录netboot)
  - [8.1 OS 目录菜单](#81-os-目录菜单)
  - [8.2 覆盖层（Overlay）](#82-覆盖层overlay)
  - [8.3 应答文件模板](#83-应答文件模板)
  - [8.4 安装任务](#84-安装任务)
  - [8.5 本地缓存](#85-本地缓存)
- [9. TFTP 服务](#9-tftp-服务)
- [10. DNS 服务](#10-dns-服务)
  - [10.1 本地 DNS](#101-本地-dns)
  - [10.2 DNS 记录管理](#102-dns-记录管理)
- [11. NFS 服务](#11-nfs-服务)
  - [11.1 多挂载点](#111-多挂载点)
  - [11.2 IP 访问控制](#112-ip-访问控制)
- [12. 主机管理](#12-主机管理)
  - [12.1 主机 CRUD](#121-主机-crud)
  - [12.2 WOL 网络唤醒](#122-wol-网络唤醒)
  - [12.3 BMC/IPMI 带外管理](#123-bmcipmi-带外管理)
- [13. OS 镜像管理](#13-os-镜像管理)
- [14. Web UI 指南](#14-web-ui-指南)
  - [14.1 导航结构](#141-导航结构)
  - [14.2 服务管理](#142-服务管理)
  - [14.3 实时日志](#143-实时日志)
  - [14.4 审计日志](#144-审计日志)
  - [14.5 命令面板（⌘K）](#145-命令面板k)
- [15. REST API 参考](#15-rest-api-参考)
  - [15.1 通用约定](#151-通用约定)
  - [15.2 认证](#152-认证)
  - [15.3 端点列表](#153-端点列表)
- [16. 配置文件参考](#16-配置文件参考)
  - [16.1 配置文件位置](#161-配置文件位置)
  - [16.2 完整配置结构](#162-完整配置结构)
  - [16.3 CLI 参数](#163-cli-参数)
- [17. iPXE 自定义编译](#17-ipxe-自定义编译)
- [18. 故障排查](#18-故障排查)
- [19. 常见问题（FAQ）](#19-常见问题faq)
- [20. 版本历史](#20-版本历史)

---

## 1. 项目概述

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

## 2. 功能特性

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

## 3. 快速开始

### 3.1 系统要求

| 项目 | 要求 |
|------|------|
| **操作系统** | Windows 10+ / Linux (kernel 3.10+) / macOS 12+ |
| **架构** | amd64 / arm64 |
| **内存** | ≥ 512 MB |
| **磁盘** | ≥ 1 GB 可用空间 |
| **网络** | 需要管理员/root 权限（DHCP 端口 67） |
| **依赖** | 无（单二进制，所有文件内嵌） |

### 3.2 安装方式

#### 方式一：下载 Release（推荐）

从 GitHub Releases 下载对应平台的二进制文件：

```bash
# Linux amd64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_linux_amd64.tar.gz
tar xzf pxelab_linux_amd64.tar.gz

# macOS arm64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_darwin_arm64.tar.gz
tar xzf pxelab_darwin_arm64.tar.gz

# Windows
# 下载 pxelab_windows_amd64.zip，解压得到 pxelab.exe
```

#### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/user/pxelab.git
cd pxelab

# 编译后端
make build          # 生成 bin/pxelab

# 编译前端（可选，已内嵌）
make frontend       # cd web && npm ci && npm run build

# 完整发布构建
make release        # goreleaser release --clean
```

#### 方式三：Docker（计划中）

> Docker 容器化部署在 Roadmap 中，尚未实现。

### 3.3 首次启动

#### Linux / macOS

```bash
# 需要 root 权限（DHCP 端口 67 需要特权）
sudo ./bin/pxelab

# 或指定数据目录
sudo ./bin/pxelab --data-dir /opt/pxelab-data

# 或以 App 模式启动（自动打开浏览器）
sudo ./bin/pxelab --mode app
```

#### Windows

```cmd
# 管理员权限运行
pxelab.exe

# 或 App 模式
pxelab.exe --mode app
```

> **Windows 系统托盘**: 在 Windows 上，PxeLab 默认以系统托盘模式运行，自动隐藏控制台窗口。托盘图标提供打开浏览器、查看数据目录、退出等功能。

#### 首次启动行为

1. 自动创建数据目录 `~/.pxelab/`
2. 初始化 SQLite 数据库 `~/.pxelab/pxelab.db`
3. 生成默认配置文件 `~/.pxelab/config.yaml`
4. 释放内嵌的引导文件到 `~/.pxelab/boot/`
5. 释放内嵌的 netboot 目录到 `~/.pxelab/netboot/`
6. 启动 HTTP 服务器（端口 8080）
7. 自动打开浏览器（`--mode app` 时）

### 3.4 验证服务

启动后，访问以下地址验证服务状态：

| 地址 | 用途 |
|------|------|
| `http://localhost:8080` | Web 管理界面 |
| `http://localhost:8080/api/v1/status` | 服务状态 JSON |
| `http://localhost:8080/api/v1/metrics` | Prometheus 指标 |
| `http://localhost:8080/boot/ipxe/script` | iPXE 引导脚本 |

```bash
# 检查服务状态
curl -s http://localhost:8080/api/v1/status | python -m json.tool
```

---

## 4. 架构与原理

### 4.1 两阶段网络引导

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

### 4.2 服务架构

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

### 4.3 部署模式

PxeLab 支持两种运行模式：

#### Server 模式（默认）

```bash
pxelab --mode server
# 或
pxelab   # 默认即 server 模式
```

- 前台运行，日志输出到 stderr
- 适合服务器部署、后台运行
- Linux 下可通过 systemd 管理

#### App 模式

```bash
pxelab --mode app
```

- 自动打开浏览器
- Windows 下以系统托盘运行（隐藏控制台窗口）
- 适合桌面环境、个人使用

#### Windows 系统托盘

在 Windows 上，PxeLab 检测到桌面环境时自动启用系统托盘模式：

- 右键托盘图标：打开浏览器 / 打开数据目录 / 退出
- 托盘图标显示服务运行状态
- 关闭浏览器不会停止服务，通过托盘退出才会

---

## 5. DHCP 配置

### 5.1 四种 DHCP 模式

每个网络接口（子网）可独立设置 DHCP 模式：

| 模式 | 分配 IP | PXE 选项 | 非 PXE 客户端 | 适用场景 |
|------|---------|---------|--------------|---------|
| **full** | ✅ | ✅ | ✅ 正常分配 | 唯一 DHCP 服务器 |
| **proxy** | ❌ yiaddr=0 | ✅ | ❌ 忽略 | 叠加到现有 DHCP |
| **hybrid** | ✅ | ✅（仅 PXE） | ✅ 仅分配 IP | 默认，兼顾两方 |
| **off** | ❌ | ❌ | ❌ 忽略 | 关闭 DHCP |

#### full 模式

PxeLab 作为网络中的**唯一 DHCP 服务器**，管理整个 DHCP 生命周期：

```
客户端 Discover → PxeLab Offer（IP + 网关 + DNS + PXE 选项）→ Request → Ack
```

适用：新建网络、实验环境、隔离网络。

#### proxy 模式

PxeLab **仅提供 PXE 相关选项**，IP 地址由现有 DHCP 服务器分配：

```
客户端 Discover → 现有 DHCP Offer（IP）+ PxeLab ProxyOffer（PXE 选项，yiaddr=0）
```

关键参数：
- `yiaddr=0.0.0.0` — iPXE 识别 ProxyDHCP 的关键判据
- `Option 60 = "PXEClient"` — UEFI PXE Base Code 要求
- `siaddr` — 指向 PxeLab（TFTP/HTTP 服务器地址）

适用：已有 DHCP 服务器的网络，叠加 PXE 服务。

#### hybrid 模式（默认）

**智能双模**：对 PXE 客户端用 proxy 模式，对其他客户端用 full 模式：

```
PXE 客户端 → proxy 模式（yiaddr=0 + PXE 选项）
普通客户端 → full 模式（分配 IP + 标准选项）
```

适用：大多数场景的推荐默认模式。

#### off 模式

完全关闭该接口的 DHCP 功能，不影响 HTTP/TFTP/DNS 等其他服务。

### 5.2 多接口部署

支持多网卡多子网配置，每个接口独立设置 DHCP 模式：

```yaml
# 示例：管理口 + 业务口
interfaces:
  - name: eth0          # 管理口
    ip: 10.0.0.1
    subnets:
      - cidr: 10.0.0.0/24
        dhcp: full       # 自建 DHCP
        pool: 10.0.0.100-10.0.0.200

  - name: eth1          # 业务口
    ip: 192.168.1.100
    subnets:
      - cidr: 192.168.1.0/24
        dhcp: proxy      # 叠加 PXE，不干扰公司 DHCP
```

### 5.3 IP 预留（DHCP Reservation）

将特定 IP 地址永久绑定到 MAC 地址，确保关键设备始终获得相同 IP：

- Web UI：**DHCP → 预留管理**
- API：`POST /api/v1/dhcp/reservations`
- 冲突检测：创建/编辑时自动检查 IP 是否已被其他预留或活跃租约占用
- 仅 full 模式子网支持预留

### 5.4 访问控制（黑白名单）

| 列表 | 行为 | 适用场景 |
|------|------|---------|
| **白名单** | 仅允许列表中的 MAC 地址获取 IP | 严格控制接入设备 |
| **黑名单** | 拒绝列表中的 MAC 地址 | 封禁特定设备 |
| **未授权设备** | 不在任何列表中的设备 | 监控和审计 |

Web UI：**设置 → 访问控制**

配置支持从 YAML 种子文件预导入：

```yaml
blacklist_seeds:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "已报废设备"

whitelist_seeds:
  - mac: "11:22:33:44:55:66"
    subnet: "10.0.0.0/24"
    reason: "服务器区"
```

---

## 6. 引导配置

### 6.1 iPXE 引导脚本系统

PxeLab 的 iPXE 引导采用**配置驱动决策树**设计，无需编写原始 iPXE 脚本，通过 Web 界面可视化配置：

```
客户端请求引导脚本
    │
    ├─ 1. 自定义脚本？ → 有 → 直接返回，忽略所有下方配置
    │
    ├─ 2. 主机有 Profile？ → 有 Profile 且含菜单 → 返回 Profile 菜单
    │
    ├─ 3. 安装目录跳转？ → 已启用 → chain 到 OS 安装目录
    │
    └─ 4. 默认引导菜单 → 返回配置的默认菜单
```

### 6.2 引导菜单类型

| BootType | 用途 | 示例 |
|----------|------|------|
| `local` | 从本地硬盘启动 | 跳过网络引导 |
| `direct` | 直接加载内核 + initrd | Linux 发行版安装 |
| `chain` | Chain-load 其他引导器 | GRUB2、Windows Boot Manager |
| `wds` | Windows WIM 引导 | Windows PE / 安装 |
| `sanboot` | iSCSI SAN 启动 | 无盘工作站 |

#### sanboot 适用场景

| 场景 | 是否适合 | 原因 |
|------|---------|------|
| DOS 启动盘 | ✅ | 启动即运行，不再访问外部介质 |
| Live Linux | ✅ | 内核 + initramfs 自包含 |
| WinPE 维护盘 | ✅ | 进 PE 后通过网络获取工具 |
| Memtest86+ | ✅ | 引导后不读盘 |
| iSCSI LUN 直启 | ✅ | 已安装系统盘 |
| CentOS/RHEL 安装 | ⚠️→❌ | Anaconda 需显式 inst.repo |
| Windows 安装 | ❌ | 需提取 wim + BCD，走 wimboot |

### 6.3 Profile（引导配置文件）

Profile 是绑定到特定主机的引导配置，包含一个引导菜单：

- **创建 Profile**：指定名称、架构、引导类型和参数
- **绑定主机**：将 Profile 绑定到主机 MAC 地址
- **脚本版本管理**：每次修改自动保存版本快照，支持差异对比和回滚
- **从 Netboot 创建**：可从 OS 安装目录一键创建 Profile

每个 Profile 为单引导项，简化配置：

```
Profile: "Install Ubuntu 22.04"
  ├─ 架构: x86_64
  ├─ 类型: direct
  ├─ Kernel: vmlinuz
  ├─ Initrd: initrd.img
  └─ Cmdline: net.ifnames=0 console=tty0
```

### 6.4 默认引导菜单

当客户端**无关联 Profile** 且**未启用 Netboot 安装目录跳转**时，显示默认菜单。

支持两种模式（Web UI：**设置 → Netboot**）：

1. **显示默认 Profile 的引导项** — 仅展示标记为 default 的 Profile
2. **列出所有 Profile** — 将所有 Profile 作为菜单项

配置项：
- **菜单标题** — 默认 `PxeLab Boot Menu`
- **超时时间** — 0 = 不自动选择，>0 = 超时后自动选择默认条目
- **菜单条目** — 可添加多个引导项

### 6.5 自定义 iPXE 脚本

在 **设置 → Netboot → 自定义 iPXE 脚本** 中填写后，**完全替代**所有可视化配置：

```ipxe
#!ipxe
dhcp || clear
echo Booting custom script for {{.MAC}}
chain {{.URL}}/boot/custom.ipxe || shell
```

可用模板变量：
- `{{.URL}}` — 服务器地址（如 `http://192.168.1.10:8080`）
- `{{.MAC}}` — 客户端 MAC 地址
- `{{.NextServer}}` — 服务器 IP（不含端口）

---

## 7. 架构映射与 Secure Boot

### 7.1 支持的客户端架构

PxeLab 支持 11 种 CPU 架构，通过 DHCP Option 93 自动检测：

| 架构 | AL 码 | 引导文件 | Secure Boot |
|------|-------|---------|-------------|
| Intel x86 (BIOS) | 0 | ipxe.pxe / undionly.kpxe | ❌ |
| EFI IA32 | 6 | ipxe32.efi | ❌ |
| EFI x86-64 | 7 | ipxe.efi | ✅ |
| EFI BC | 9 | snponly.efi | ❌ |
| EFI ARM32 | 10 | ipxe-arm32.efi | ❌ |
| EFI ARM64 | 11 | ipxe-arm64.efi | ✅ |
| EFI RISC-V 32 | 25 | ipxe-riscv32.efi | ❌ |
| EFI RISC-V 64 | 27 | ipxe-riscv64.efi | ❌ |
| EFI LoongArch32 | 37 | ipxe-loong64.efi | ❌ |
| EFI LoongArch64 | 39 | ipxe-loong64.efi | ❌ |

### 7.2 Secure Boot 链

x86-64 和 ARM64 架构支持 UEFI Secure Boot，启动链为：

```
UEFI 固件 → shim-x86_64.efi（微软签名）→ ipxe-x86_64-sb.efi（PxeLab 签名）→ 引导菜单
```

在 **设置 → Boot Settings** 中可查看各架构的 Secure Boot 支持状态。

### 7.3 chain_to_ipxe

当接口配置的引导加载器为 `pxelinux` 或 `grub2` 时，启用 `chain_to_ipxe` 可自动将客户端升级到 iPXE：

```
客户端 PXE → PXELinux/GRUB2 加载
  → 请求配置文件
  → 服务器拦截，返回 iPXE chainload 配置
  → 客户端下载 ipxe.efi 并执行
  → 进入 PxeLab 引导决策树
```

---

## 8. 网络启动目录（Netboot）

### 8.1 OS 目录菜单

PxeLab 内置 OS 安装目录，集成 netboot.xyz 的发行版索引，支持 10 个分组：

| 分组 | 内容 |
|------|------|
| Linux Distributions | x86_64 Linux 发行版 |
| Linux Distributions (32-bit) | 32 位 Linux |
| Linux Distributions (arm64) | ARM64 Linux |
| BSD Systems | FreeBSD / OpenBSD 等 |
| Live CDs | 图形化 Live 环境 |
| Live CDs (arm64) | ARM64 Live 环境 |
| System Tools | 系统工具 / 救援镜像 |
| Windows | Windows PE / 安装 |
| DOS | DOS 引导 |
| Unix | 其他 Unix 系统 |

Web UI：**设置 → Netboot → 安装目录菜单结构**

- 启用/禁用分组
- 自定义显示标题
- 拖拽排序分组顺序

### 8.2 覆盖层（Overlay）

为特定发行版定制引导参数，不影响默认配置：

- API：`PUT /api/v1/netboot/overlays/{distro}`
- 覆盖 kernel 参数、initrd 参数等
- 按发行版独立配置

### 8.3 应答文件模板

自动化安装的应答文件管理：

- **预设模板**：常见发行版的安装应答文件
- **自定义模板**：创建、编辑、验证
- **版本管理**：保存历史版本，支持回滚
- **预览**：生成最终应答文件预览
- **验证**：语法和格式验证

API：`/api/v1/netboot/answer-templates`

### 8.4 安装任务

跟踪和管理网络安装任务：

- 创建安装任务（指定发行版、目标、应答文件）
- 查询任务状态
- 按 MAC 地址查询任务：`GET /api/v1/netboot/task/by-mac/{mac}`
- 获取应答文件：`GET /api/v1/netboot/answer/{task_id}`

### 8.5 本地缓存

设置 → Netboot 中启用**本地缓存**（默认开启）：

- 缓存下载的引导文件到磁盘
- 加快重复引导速度
- 缓存统计：`GET /api/v1/netboot/cache-stats`
- Web UI 实时显示缓存路径和磁盘占用

---

## 9. TFTP 服务

TFTP 服务提供 NBP（Network Bootstrap Program）文件传输：

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 端口 | 69 | UDP |
| 超时 | 默认值 | 客户端连接超时 |

Web UI：**设置 → TFTP**

TFTP 服务根据架构映射自动选择正确的引导文件返回给客户端。引导文件内嵌在二进制中，首次运行时释放到数据目录。

---

## 10. DNS 服务

### 10.1 本地 DNS

PxeLab 内置 DNS 服务器，支持：

- **上游转发** — 未匹配的查询转发到上游 DNS
- **本地解析** — A / AAAA / CNAME 记录
- **子网感知** — 根据客户端来源子网返回对应网段的服务器 IP
- **自动记录** — 启动时自动创建 `@` A 记录和服务器名称 A 记录

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 端口 | 53 | UDP |
| 本地域名 | pxelab.local | 后缀域名 |
| 上游 DNS | 系统默认 | 转发目标 |

Web UI：**设置 → DNS**

### 10.2 DNS 记录管理

通过 Web UI 或 API 管理 DNS 记录：

```
GET    /api/v1/dns/records          # 列出所有记录
POST   /api/v1/dns/records          # 创建记录
GET    /api/v1/dns/records/{id}     # 获取单条记录
PUT    /api/v1/dns/records/{id}     # 更新记录
DELETE /api/v1/dns/records/{id}     # 删除记录
```

支持记录类型：A、AAAA、CNAME

---

## 11. NFS 服务

### 11.1 多挂载点

PxeLab 内置 NFSv3 服务器，支持多个独立挂载点：

```yaml
nfs:
  enabled: true
  port: 2049
  mount_points:
    - label: "ISOs"
      export_path: /isos          # 客户端挂载路径
      local_dir: /data/isos       # 本地目录
      read_only: true
      allow_ips:
        - "10.0.0.0/24"

    - label: "Installs"
      export_path: /installs
      local_dir: /data/installs
      read_only: false
      allow_ips:
        - "192.168.1.0/24"
```

每个挂载点独立配置：
- **标签** — 显示名称
- **导出路径** — 客户端挂载别名
- **本地目录** — 服务器本地目录
- **只读** — 权限控制
- **IP/CIDR 白名单** — 访问控制

Web UI：**设置 → NFS**

### 11.2 IP 访问控制

- `allow_ips` 为空 = 不限制，所有客户端可挂载
- 支持 IP 地址和 CIDR 网段
- 内嵌 rpcbind（端口 111/UDP+TCP），自动注册 NFSv3/MOUNT 端口映射
- 版本感知 TCP 监听器：拦截 NFSv4 连接并回复 PROG_MISMATCH，强制回退到 v3

---

## 12. 主机管理

### 12.1 主机 CRUD

管理网络中的设备：

```
GET    /api/v1/hosts                # 列出所有主机
POST   /api/v1/hosts                # 创建主机
GET    /api/v1/hosts/{id}           # 获取主机详情
PUT    /api/v1/hosts/{id}           # 更新主机
DELETE /api/v1/hosts/{id}           # 删除主机
```

每个主机可配置：
- 名称、描述
- MAC 地址
- 绑定 Profile（引导配置）
- IP 地址
- 分组

### 12.2 WOL 网络唤醒

通过 Wake-on-LAN 远程开机：

```
POST /api/v1/hosts/{id}/wake       # 唤醒单台主机
POST /api/v1/hosts/batch/wake      # 批量唤醒
GET  /api/v1/wol/history           # 唤醒历史
POST /api/v1/wol/schedule          # 创建定时唤醒
GET  /api/v1/wol/schedules         # 列出定时任务
```

定时调度功能：
- 指定 MAC 地址和唤醒时间
- 支持一次性或周期性调度
- 唤醒历史记录

### 12.3 BMC/IPMI 带外管理

通过 IPMI 协议远程管理服务器电源：

```
POST /api/v1/bmc/probe              # 探测 BMC
POST /api/v1/bmc/{id}/power-on      # 开机
POST /api/v1/bmc/{id}/power-off     # 关机
POST /api/v1/bmc/{id}/restart       # 重启
GET  /api/v1/bmc/{id}/status        # 查询状态
POST /api/v1/bmc/{id}/boot-device   # 设置启动设备
```

支持批量操作：
```
POST /api/v1/bmc/batch/power-on     # 批量开机
POST /api/v1/bmc/batch/power-off    # 批量关机
POST /api/v1/bmc/batch/restart      # 批量重启
POST /api/v1/bmc/batch/status       # 批量查询状态
```

CSV 批量导入 BMC 配置：`POST /api/v1/bmc/configs/import`

---

## 13. OS 镜像管理

管理 ISO 镜像文件：

```
GET    /api/v1/os-images                # 列出所有镜像
POST   /api/v1/os-images/upload         # 上传 ISO
POST   /api/v1/os-images/import         # 导入本地文件
GET    /api/v1/os-images/{id}           # 获取镜像详情
PUT    /api/v1/os-images/{id}           # 更新元数据
DELETE /api/v1/os-images/{id}           # 删除镜像
POST   /api/v1/os-images/{id}/extract   # 解压 ISO
POST   /api/v1/os-images/{id}/mount     # 挂载 ISO
POST   /api/v1/os-images/{id}/unmount   # 卸载 ISO
POST   /api/v1/os-images/{id}/reprocess # 重新处理
GET    /api/v1/os-images/{id}/file      # 下载文件
```

功能：
- 上传 ISO 文件
- 自动检测发行版类型
- ISO 挂载/卸载
- 挂载点追踪
- 文件浏览

---

## 14. Web UI 指南

### 14.1 导航结构

```
├── 主机          # 主机列表、CRUD、绑定 Profile
├── 引导配置      # Profile 管理、脚本版本
├── 服务
│   ├── DHCP      # DHCP 设置、预留管理
│   ├── TFTP      # TFTP 设置、文件管理
│   ├── DNS       # DNS 设置、记录管理
│   └── NFS       # NFS 设置、挂载点管理
├── 设置
│   ├── 通用      # 服务器名称、监听地址
│   ├── 接口      # 网卡配置、DHCP 模式
│   ├── Netboot   # 引导菜单、安装目录、缓存
│   └── 访问控制  # 黑白名单
├── 日志          # 实时日志流、日志文件
├── OS 镜像       # ISO 管理、挂载
├── BMC           # IPMI 带外管理
├── WOL           # 网络唤醒、定时调度
└── 指标          # Prometheus 指标
```

### 14.2 服务管理

**服务管理页面**提供所有服务的集中管理：

| 功能 | 说明 |
|------|------|
| 独立启停 | 每个服务单独启动/停止/重启 |
| 批量操作 | 选择多个服务批量启停 |
| 自动刷新 | 实时显示服务运行状态 |
| 端口显示 | 如 "67/UDP"、"8080/TCP" |
| 保护服务 | HTTP 标记为 Core，禁止停止 |
| 自动启动 | 每个服务独立配置 auto_start |

### 14.3 实时日志

Web UI 日志页面提供多面板实时日志查看：

- **SSE 流** — Server-Sent Events 实时推送
- **按服务过滤** — DHCP / TFTP / HTTP / DNS / NFS 等
- **颜色标签** — 不同服务不同颜色
- **日志文件** — 查看历史日志文件列表
- **磁盘占用** — 日志占用空间统计
- **日志清理** — 手动清理旧日志

### 14.4 审计日志

所有配置变更自动记录审计日志：

```
GET /api/v1/audit-logs
```

记录内容：
- 操作类型（创建/更新/删除）
- 资源类型和名称
- 操作者 IP
- 变更详情（如 "名称: A→B; 架构: x86_64→arm64"）
- 时间戳

### 14.5 命令面板（⌘K）

按 `⌘K`（macOS）或 `Ctrl+K`（Windows/Linux）打开命令面板：

- 快速导航到任意页面
- 搜索功能和设置
- 键盘快捷操作

---

## 15. REST API 参考

### 15.1 通用约定

**Base URL**: `http://<host>:8080/api/v1`

**请求格式**: JSON (`Content-Type: application/json`)

**响应格式**:

```json
{
  "success": true,
  "data": { ... }
}
```

错误响应：

```json
{
  "success": false,
  "error": "错误描述"
}
```

### 15.2 认证

```
POST /api/v1/auth/login     # 登录
POST /api/v1/auth/logout    # 登出
GET  /api/v1/auth/session   # 检查会话
```

会话通过 Cookie 维持。

### 15.3 端点列表

#### 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/auth/login` | 登录 |
| POST | `/auth/logout` | 登出 |
| GET | `/auth/session` | 检查会话 |

#### 主机

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/hosts` | 列出主机 |
| POST | `/hosts` | 创建主机 |
| GET | `/hosts/{id}` | 获取主机 |
| PUT | `/hosts/{id}` | 更新主机 |
| DELETE | `/hosts/{id}` | 删除主机 |
| POST | `/hosts/{id}/wake` | WOL 唤醒 |
| POST | `/hosts/{id}/power` | IPMI 电源控制 |
| GET | `/hosts/{id}/boot-config` | 预览引导配置 |
| POST | `/hosts/batch/wake` | 批量唤醒 |

#### 引导配置（Profile）

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/profiles` | 列出 Profile |
| POST | `/profiles` | 创建 Profile |
| GET | `/profiles/{id}` | 获取 Profile |
| PUT | `/profiles/{id}` | 更新 Profile |
| DELETE | `/profiles/{id}` | 删除 Profile |
| POST | `/profiles/from-netboot` | 从 Netboot 创建 |
| GET | `/profiles/{id}/script-versions` | 脚本版本列表 |
| GET | `/profiles/{id}/script-diff/{verId}` | 版本差异 |
| POST | `/profiles/{id}/script-rollback/{verId}` | 版本回滚 |

#### 文件管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/files` | 列出文件 |
| GET | `/files/root` | 获取根目录 |
| POST | `/files/upload` | 上传文件 |
| DELETE | `/files` | 删除文件 |

#### 租约

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/leases` | 列出租约 |
| GET | `/leases/stats` | 租约统计 |
| DELETE | `/leases/{mac}` | 删除租约 |
| POST | `/leases/batch-delete` | 批量删除 |
| POST | `/leases/prune` | 清理过期租约 |

#### 设置

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/settings/general` | 通用设置 |
| PUT | `/settings/general` | 更新通用设置 |
| GET | `/settings/interfaces` | 接口配置 |
| PUT | `/settings/interfaces` | 更新接口配置 |
| GET | `/settings/netboot` | Netboot 设置 |
| PUT | `/settings/netboot` | 更新 Netboot 设置 |
| GET | `/settings/logging` | 日志设置 |
| PUT | `/settings/logging` | 更新日志设置 |

#### 服务

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/services/tftp` | TFTP 设置 |
| PUT | `/services/tftp` | 更新 TFTP |
| GET | `/services/dhcp` | DHCP 设置 |
| PUT | `/services/dhcp` | 更新 DHCP |
| GET | `/services/dns` | DNS 设置 |
| PUT | `/services/dns` | 更新 DNS |
| GET | `/services/nfs` | NFS 设置 |
| PUT | `/services/nfs` | 更新 NFS |
| GET | `/services/archmap` | 架构映射 |
| PUT | `/services/archmap` | 更新架构映射 |
| GET | `/services/archmap/defaults` | 默认架构映射 |
| GET | `/services/ipxe-script` | iPXE 脚本 |
| PUT | `/services/ipxe-script` | 更新 iPXE 脚本 |

#### 服务生命周期

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/services` | 列出所有服务 |
| POST | `/services/{name}/start` | 启动服务 |
| POST | `/services/{name}/stop` | 停止服务 |
| POST | `/services/{name}/restart` | 重启服务 |
| POST | `/services/batch/{action}` | 批量操作 |
| PUT | `/services/{name}/auto-start` | 设置自动启动 |

#### Netboot

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/netboot/catalog` | OS 目录 |
| GET | `/netboot/catalog/{distro}` | 发行版详情 |
| GET | `/netboot/groups` | 分组列表 |
| GET | `/netboot/check-files` | 检查文件 |
| GET | `/netboot/cache-stats` | 缓存统计 |
| GET/PUT/DELETE | `/netboot/overlays/{distro}` | 覆盖层 |
| GET/POST/PUT/DELETE | `/netboot/answer-templates` | 应答文件模板 |
| GET/POST/PUT/DELETE | `/netboot/tasks` | 安装任务 |

#### 访问控制

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST/DELETE | `/access/blacklist` | 黑名单 |
| GET/POST/DELETE | `/access/whitelist` | 白名单 |
| GET | `/access/unauthorized` | 未授权设备 |

#### DNS

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/dns/records` | DNS 记录 |
| GET/PUT/DELETE | `/dns/records/{id}` | 单条记录 |

#### BMC

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/bmc/configs` | BMC 配置 |
| POST | `/bmc/configs/import` | CSV 导入 |
| POST | `/bmc/probe` | 探测 BMC |
| POST | `/bmc/{id}/power-on` | 开机 |
| POST | `/bmc/{id}/power-off` | 关机 |
| POST | `/bmc/{id}/restart` | 重启 |
| GET | `/bmc/{id}/status` | 查询状态 |
| POST | `/bmc/{id}/boot-device` | 设置启动设备 |
| POST | `/bmc/batch/power-on` | 批量开机 |
| POST | `/bmc/batch/power-off` | 批量关机 |
| POST | `/bmc/batch/restart` | 批量重启 |
| POST | `/bmc/batch/status` | 批量查询 |

#### DHCP 预留

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/dhcp/reservations` | 预留列表/创建 |
| GET/PUT/DELETE | `/dhcp/reservations/{id}` | 单条操作 |

#### 网络诊断

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/network/ping` | Ping |
| POST | `/network/ping/stream` | 流式 Ping |
| POST | `/network/traceroute` | Traceroute |
| POST | `/network/traceroute/stream` | 流式 Traceroute |
| GET | `/network/interfaces` | 网络接口列表 |

#### WOL

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST/DELETE | `/wol/history` | 唤醒历史 |
| GET/POST/DELETE | `/wol/schedules` | 定时任务 |
| GET | `/wol/interfaces` | WOL 接口 |

#### OS 镜像

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/os-images` | 镜像列表/上传 |
| GET/PUT/DELETE | `/os-images/{id}` | 单条操作 |
| POST | `/os-images/{id}/extract` | 解压 |
| POST | `/os-images/{id}/mount` | 挂载 |
| POST | `/os-images/{id}/unmount` | 卸载 |
| GET | `/fs/browse` | 文件浏览 |

#### 其他

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/status` | 服务状态 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/events` | 事件列表 |
| GET | `/events/stream` | 事件流（SSE） |
| GET | `/logs/stream` | 日志流（SSE） |
| GET | `/logs/files` | 日志文件列表 |
| GET | `/logs/disk-usage` | 日志磁盘占用 |
| POST | `/logs/cleanup` | 清理日志 |
| GET | `/interfaces` | 网络接口 |
| GET | `/bootloader/check` | 引导文件检查 |
| GET | `/bootloader/files` | 引导文件列表 |

---

## 16. 配置文件参考

### 16.1 配置文件位置

PxeLab 按以下顺序搜索配置文件：

1. `--config` 参数指定的路径
2. 当前目录 `./config.yaml`
3. 数据目录 `~/.pxelab/config.yaml`
4. `/etc/pxelab/config.yaml`

### 16.2 完整配置结构

```yaml
# ── 全局设置 ──
global:
  listen_addr: ":8080"           # HTTP 监听地址
  data_dir: ""                   # 数据目录（默认 ~/.pxelab/）
  server_name: "PxeLab"          # 服务器名称
  app_mode: false                # App 模式（自动打开浏览器）

# ── 接口配置 ──
interfaces:
  - name: eth0
    ip: 192.168.1.100
    auto_start: true
    subnets:
      - cidr: 192.168.1.0/24
        dhcp: hybrid             # full / proxy / hybrid / off
        pool: 192.168.1.100-192.168.1.200
        gateway: 192.168.1.1
        dns: 8.8.8.8
        lease_time: 86400

# ── TFTP ──
tftp:
  port: 69
  timeout: 30

# ── DNS ──
dns:
  port: 53
  upstream: "8.8.8.8"            # 上游 DNS
  local_domain: "pxelab.local"   # 本地域名

# ── NFS ──
nfs:
  enabled: false
  port: 2049
  mount_points:
    - label: "Default"
      export_path: "/"
      local_dir: ""              # 默认 ~/.pxelab/boot/isos
      read_only: true
      allow_ips: []

# ── 引导配置 ──
boot:
  root_dir: ""                   # 引导文件目录（默认 ~/.pxelab/boot/）
  arch_map:                      # 架构映射（通常自动管理）
    - arch_code: 0
      arch_name: "Intel x86PC"
      nbp: "ipxe"
      ipxe: "ipxe.pxe"
    # ... 更多架构

# ── Netboot ──
netboot:
  enabled: true
  failsafe_prompt: true
  script_template: ""            # 自定义 iPXE 脚本（留空用可视化配置）
  sync:
    url: "https://github.com/netbootxyz/netboot.xyz.git"
    repo: "contrib/netboot.xyz"
  boot:
    default_menu:
      title: "PxeLab Boot Menu"
      timeout: 5000              # 毫秒，0=不自动选择
      default: 0
      entries:
        - label: "Boot from local disk"
          type: local
    catalog_redirect:
      enabled: true
      target_url: "http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}"
      detect_arch: true
    catalog_display:
      title: "[OS] Netboot OS Install Catalog"
      groups:
        - name: linux
          title: "Linux Distributions"
          enabled: true
          order: 1
        # ... 更多分组

# ── 服务自动启动 ──
service_auto_start:
  http: true
  tftp: false
  dns: false
  nfs: false

# ── 存储 ──
store:
  dsn: ""                        # SQLite 路径（默认 ~/.pxelab/pxelab.db）

# ── 日志 ──
log:
  level: info                    # debug / info / warn / error
  format: text
  file: ""                       # 日志文件路径（空=默认 ~/.pxelab/logs/）
  max_size_mb: 0                 # 单文件最大体积 (MB)，0=不限制
  max_backups: 0                 # 保留轮转文件数
  max_age_days: 0                # 保留天数
  compress: false                # gzip 压缩旧日志
  cleanup_interval: 0            # 清理检查间隔（小时）

# ── 访问控制种子 ──
blacklist_seeds:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "已报废设备"

whitelist_seeds:
  - mac: "11:22:33:44:55:66"
    subnet: "10.0.0.0/24"
    reason: "服务器区"
```

### 16.3 CLI 参数

```
pxelab [flags]

Flags:
  --config string       配置文件路径
  --data-dir string     数据目录 (default "~/.pxelab/")
  --log-level string    日志级别: debug/info/warn/error (default "info")
  --mode string         运行模式: app/server ("app" 自动打开浏览器)
```

---

## 17. iPXE 自定义编译

PxeLab 使用自定义编译的 iPXE 二进制，所有二进制内嵌同一份引导脚本：

```ipxe
#!ipxe
dhcp || goto dhcp_failed
isset proxydhcp/next-server && goto use_proxy

:use_dhcp
set next-server ${dhcp-server}
goto chain

:use_proxy
set next-server ${proxydhcp/next-server}

:chain
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || goto tftp_fallback
exit

:tftp_fallback
chain tftp://${next-server}/boot/menu.ipxe || shell
exit

:dhcp_failed
shell
```

### 编译所有架构

```bash
# 使用 Docker 交叉编译（推荐）
make ipxe-build-all

# 手动编译 x86_64 EFI
make ipxe-build           # 非嵌入式
make ipxe-build-embed     # 嵌入式（failsafe）
```

### 编译产物

| 文件 | 架构 | 用途 |
|------|------|------|
| `ipxe.pxe` | x86 BIOS | iPXE 引导 |
| `ipxe32.efi` | EFI IA32 | iPXE 引导 |
| `ipxe.efi` | EFI x86-64 | iPXE 引导 |
| `snponly.efi` | EFI BC | iPXE 引导（SNP 驱动） |
| `ipxe-arm32.efi` | EFI ARM32 | iPXE 引导 |
| `ipxe-arm64.efi` | EFI ARM64 | iPXE 引导 |
| `ipxe-riscv32.efi` | RISC-V 32 | iPXE 引导 |
| `ipxe-riscv64.efi` | RISC-V 64 | iPXE 引导 |
| `ipxe-loong64.efi` | LoongArch64 | iPXE 引导 |
| `ipxe-x86_64-sb.efi` | EFI x86-64 | Secure Boot iPXE |
| `ipxe-arm64-sb.efi` | EFI ARM64 | Secure Boot iPXE |
| `shim-x86_64.efi` | EFI x86-64 | Secure Boot Shim |
| `shim-arm64.efi` | EFI ARM64 | Secure Boot Shim |

详见 [docs/ipxe-build.md](ipxe-build.md)。

---

## 18. 故障排查

### 常见问题

| 现象 | 可能原因 | 排查步骤 |
|------|---------|---------|
| 客户端无法 PXE 启动 | DHCP 未配置 / 网络不通 | 1. 检查 PxeLab 是否运行 2. 检查客户端和服务器是否在同一 VLAN 3. 检查防火墙是否放行 UDP 67 |
| 客户端启动后直接进本地磁盘 | 默认菜单只有 local 条目 | 检查默认菜单配置，确保有网络引导条目 |
| 客户端看不到安装目录 | Netboot 未启用 | 检查「启用 OS 安装目录菜单」和「Profile 菜单行为」 |
| DHCP Offer 被 UEFI 固件拒绝 | 缺少 Option 53 | 确保 PxeLab 版本包含 Option 53 修复 |
| iPXE 循环重启 | PXE_STACK 缓存问题 | PxeLab 已通过自定义编译 iPXE 解决（禁用 PXE_STACK） |
| vmxnet3 虚拟网卡启动失败 | TXE:1 兼容问题 | 使用 undionly.kpxe（UNDI 接口） |
| 修改配置不生效 | 浏览器缓存 | 硬刷新 (Ctrl+F5) |
| NFS 挂载失败（Windows） | 路径分隔符问题 | 确保使用 PxeLab 最新版（已修复 path.Clean） |
| DNS 不解析 | 上游 DNS 未配置 | 检查 DNS 设置中的 upstream 字段 |

### 日志排查

```bash
# 查看实时日志
# Web UI: 日志页面 → 选择服务过滤

# 或查看日志文件
tail -f ~/.pxelab/logs/pxelab.log

# 按服务过滤
grep "DHCP" ~/.pxelab/logs/pxelab.log
```

### 网络诊断

Web UI 内置网络诊断工具：
- **Ping** — 测试网络连通性
- **Traceroute** — 追踪路由路径
- 支持流式输出，实时显示结果

API：
```bash
# Ping
curl -X POST http://localhost:8080/api/v1/network/ping \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1", "count": 4}'

# Traceroute
curl -X POST http://localhost:8080/api/v1/network/traceroute \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1"}'
```

---

## 19. 常见问题（FAQ）

### Q: PxeLab 需要什么权限？

**A**: Linux/macOS 需要 root 权限（DHCP 端口 67 是特权端口）。Windows 需要管理员权限。

### Q: 可以同时运行多个 DHCP 服务器吗？

**A**: 可以。使用 `proxy` 模式叠加到现有 DHCP 环境，或使用 `hybrid` 模式智能分流。

### Q: 支持哪些操作系统安装？

**A**: 通过 iPXE 引导支持：
- Linux：Ubuntu、Debian、CentOS、Fedora、Arch、Gentoo 等 50+ 发行版
- Windows：通过 WDS 仿真或 wimboot
- BSD：FreeBSD、OpenBSD、NetBSD
- Live CD：Kali、GParted、SystemRescue 等

### Q: 如何自定义引导菜单？

**A**: 三种方式：
1. **默认菜单** — 设置 → Netboot → 默认引导菜单
2. **Profile** — 为主机创建专用引导配置
3. **自定义脚本** — 设置 → Netboot → 自定义 iPXE 脚本（高级用户）

### Q: 数据存储在哪里？

**A**: 默认在 `~/.pxelab/`：
- `config.yaml` — 配置文件
- `pxelab.db` — SQLite 数据库
- `boot/` — 引导文件
- `netboot/` — 网络启动目录
- `logs/` — 日志文件

### Q: 如何备份数据？

**A**: 备份 `~/.pxelab/` 整个目录即可。核心数据在 `pxelab.db` 和 `config.yaml`。

### Q: 支持 Docker 部署吗？

**A**: 暂不支持，在 Roadmap 中。当前推荐直接运行二进制。

### Q: 如何升级 PxeLab？

**A**:
1. 停止当前运行的 PxeLab
2. 下载新版本二进制替换旧文件
3. 数据目录 `~/.pxelab/` 无需变动，新版本自动迁移
4. 重新启动

### Q: 多网卡如何配置？

**A**: 在 `config.yaml` 的 `interfaces` 部分配置多个接口，每个接口独立设置 DHCP 模式和子网。

---

## 20. 版本历史

### v0.4.0-dev（当前开发版）

**新增**:
- 完整 iPXE 架构支持（ARM32、RISC-V 32/64、LoongArch32/64）
- Secure Boot 支持（x86_64 + ARM64）
- NFS 多挂载点支持
- NFS IP 访问控制
- DNS 子网感知解析
- DHCP 预留（IP+MAC 绑定）
- 访问控制（黑白名单）
- BMC/IPMI 带外管理
- 安装任务跟踪
- 应答文件模板版本管理
- 日志轮转和清理
- 审计日志
- 网络诊断（Ping/Traceroute）
- 本地缓存（加速重复引导）
- Windows 系统托盘

**变更**:
- NBP 架构重构：Option 93 作为主要依据
- 服务生命周期管理
- Profile 简化为单引导项
- 路由重构

### v0.3.0

- iPXE 引导脚本系统
- DHCP 四种模式
- Web UI 全功能
- CLI 管理工具
- 事件总线和实时日志

### v0.2.0

- TFTP/DNS 服务
- 主机管理
- Profile 管理
- REST API v1

### v0.1.0

- 初始版本
- DHCP + HTTP 基础服务
- iPXE 引导

---

> **文档版本**: v1.0 · **适用于**: PxeLab v0.4.0-dev · **维护者**: PxeLab Team
