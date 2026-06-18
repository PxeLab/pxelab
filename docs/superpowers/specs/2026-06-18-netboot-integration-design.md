# PxeGo netboot.xyz 集成设计文档

## 概述

将 netboot.xyz 的完整操作系统引导目录集成到 PxeGo 中，作为 PxeGo 的内置组件。
支持在线/离线双模式运行，用户可直接编辑所有配置文件，支持从上游 netboot.xyz 仓库同步更新。

## 架构

```
┌─────────────────────────────────────────────────────────┐
│                     PxeGo                                │
│                                                          │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐ │
│  │  HTTP Server  │  │  DHCP/TFTP  │  │  Web UI           │ │
│  │              │  │             │  │                   │ │
│  │  /boot/*     │  │  iPXE 固件  │  │  OS 目录浏览      │ │
│  │  /netboot/*  │  │  netboot    │  │  预设模板列表     │ │
│  │              │  │  fallback   │  │  引导文件管理     │ │
│  └──────┬───────┘  └──────┬──────┘  └────────┬──────────┘ │
│         │                │                    │            │
│         ▼                ▼                    ▼            │
│  ┌──────────────────────────────────────────────────────┐ │
│  │              internal/netboot/ 核心模块               │ │
│  │                                                       │ │
│  │  ┌──────────┐  ┌──────────┐  ┌───────────────────┐  │ │
│  │  │ Catalog   │  │ Script   │  │ Sync              │  │ │
│  │  │ (OS 目录) │  │ Generator│  │ (从上游同步)      │  │ │
│  │  │          │  │ (iPXE    │  │                   │  │ │
│  │  │ YAML解析  │  │  脚本)   │  │ git pull          │  │ │
│  │  │ 版本管理  │  │ 模板渲染 │  │ YAML 转换         │  │ │
│  │  └─────┬────┘  └────┬─────┘  └────────┬──────────┘  │ │
│  │        │            │                  │              │ │
│  │        ▼            ▼                  ▼              │ │
│  │  ~/.pxego/netboot/catalog/*.yaml                      │ │
│  │  ~/.pxego/netboot/scripts/*.ipxe                     │ │
│  │  ~/.pxego/boot/netboot/<distro>/<ver>/               │ │
│  └──────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────┘

                 ▲
                 │ pxego netboot sync
                 │
  ┌──────────────────────────────┐
  │  forked netboot.xyz repo     │ ← 可随时 git pull 上游
  │  contrib/netboot.xyz/        │
  │                              │
  │  roles/defaults/main.yml     │ ← 发行版定义（解析源）
  │  endpoints.yml               │ ← 文件 URL 定义（解析源）
  │  templates/menu/*.ipxe.j2    │ ← 参考模板格式
  └──────────────────────────────┘
```

## 三份数据源

| 数据源 | 位置 | 作用 | 可编辑 |
|--------|------|------|--------|
| A. forked netboot.xyz | `contrib/netboot.xyz/` | 权威发行版定义，同步上游 | 否（手动改 fork） |
| B. PxeGo catalog YAML | `~/.pxego/netboot/catalog/*.yaml` | PxeGo 内部目录，由 sync 生成 | 是（可删/改/增） |
| C. 预设 iPXE 脚本 | `~/.pxego/netboot/scripts/*.ipxe` | TFTP fallback + 参考 | 是 |

## 目录结构

```
~/.pxego/
├── boot/
│   └── netboot/                  ← 用户放置 OS 引导文件
│       ├── ubuntu/24.04/
│       │   ├── vmlinuz
│       │   └── initrd
│       └── debian/12/
│           ├── linux
│           └── initrd.gz
│
├── netboot/                      ← netboot 配置（全部可编辑）
│   ├── catalog/                  ← 发行版目录定义
│   │   ├── ubuntu.yaml
│   │   ├── debian.yaml
│   │   ├── almalinux.yaml
│   │   ├── arch.yaml
│   │   ├── fedora.yaml
│   │   ├── opensuse.yaml
│   │   ├── centos.yaml
│   │   ├── rockylinux.yaml
│   │   ├── freebsd.yaml
│   │   ├── clonezilla.yaml
│   │   ├── systemrescue.yaml
│   │   ├── gparted.yaml
│   │   ├── memtest.yaml
│   │   ├── netboot/              ← 子目录：按分类组织
│   │   │   ├── linux.yaml        ← 索引：汇总所有 Linux 发行版
│   │   │   ├── bsd.yaml
│   │   │   ├── tools.yaml
│   │   │   └── live.yaml
│   │   └── ... (50+ 文件)
│   │
│   ├── scripts/                  ← 预设 iPXE 脚本
│   │   ├── netboot.ipxe          ← 完整 OS 目录菜单
│   │   ├── ubuntu.ipxe
│   │   ├── debian.ipxe
│   │   └── ... (按需生成)
│   │
│   ├── endpoints.yaml            ← GitHub 镜像文件索引（自动生成）
│   └── user_overrides.yml        ← 用户自定义覆盖

└── contrib/
    └── netboot.xyz/              ← forked 仓库（仅同步源）
```

## Catalog YAML 格式

每个发行版一个文件：

```yaml
# ~/.pxego/netboot/catalog/ubuntu.yaml
name: Ubuntu
enabled: true
website: https://ubuntu.com
menu_group: linux              # linux | bsd | live | tools | unix | dos | windows
logo: ubuntu

# 远程源配置
mirror: http://archive.ubuntu.com
archive_mirror: http://old-releases.ubuntu.com

# 本地文件存储路径（相对于 boot/ 目录）
local_base: netboot/ubuntu

# kernel 命令行参数模板
kernel_params: initrd=initrd.magic ${cmdline}

versions:
  - codename: noble
    name: 24.04 LTS Noble Numbat
    arch: amd64
    enabled: true
    # 本地文件路径（优先级最高）
    local:
      kernel: ubuntu/24.04/vmlinuz
      initrd: ubuntu/24.04/initrd
    # 远程 URL（本地文件不存在时使用）
    remote:
      kernel: http://archive.ubuntu.com/ubuntu/dists/noble-updates/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64/linux
      initrd: http://archive.ubuntu.com/ubuntu/dists/noble-updates/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64/initrd.gz
    cmdline: ""
    install_type: subiquity    # subiquity | legacy | live

  - codename: jammy
    name: 22.04 LTS Jammy Jellyfish
    arch: amd64
    enabled: true
    # 没有 local 定义 → 只用远程
    remote:
      kernel: http://archive.ubuntu.com/ubuntu/dists/jammy-updates/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64/linux
      initrd: http://archive.ubuntu.com/ubuntu/dists/jammy-updates/main/installer-amd64/current/images/netboot/ubuntu-installer/amd64/initrd.gz
    cmdline: ""
    install_type: subiquity

  - codename: focal
    name: 20.04 LTS Focal Fossa
    arch: amd64
    enabled: true
    remote:
      kernel: http://archive.ubuntu.com/ubuntu/dists/focal-updates/main/installer-amd64/current/legacy-images/netboot/ubuntu-installer/amd64/linux
      initrd: http://archive.ubuntu.com/ubuntu/dists/focal-updates/main/installer-amd64/current/legacy-images/netboot/ubuntu-installer/amd64/initrd.gz
    cmdline: ""
    install_type: legacy
```

## 引导流程决策

```
客户端 PXE 启动
    │
    ▼
DHCP 返回通用 iPXE 固件 (undionly.kpxe / ipxe.efi)
    │
    ▼
iPXE 启动 → HTTP 请求 http://<pxego>/boot/ipxe/script?mac=XX
    │
    ▼
PxeGo 生成 iPXE 脚本：
    │
    ├─ [有装机任务] → 直接 chain 装机引导脚本（跳过所有菜单）
    │
    ├─ [有自定义 Profile] → 展示 Profile 定义的菜单
    │   └─ 其中包含"📦 OS 安装目录"选项
    │
    └─ [默认] → 展示全局默认菜单
         ├─ ① 从本地磁盘启动
         ├─ ② 内存操作系统 (MemDisk)
         ├─ ③ 📦 网络安装操作系统目录
         │    └─ 展开多级子菜单（由 catalog/*.yaml 动态生成）
         │       ├─ Linux 发行版 (64-bit)
         │       │   ├─ Ubuntu
         │       │   │   ├─ 24.04 LTS Noble Numbat  → kernel+initrd boot
         │       │   │   └─ 22.04 LTS Jammy Jellyfish
         │       │   ├─ Debian
         │       │   ├─ Fedora
         │       │   └─ ... (40+ 发行版)
         │       ├─ Linux 发行版 (32-bit)
         │       ├─ BSD 安装
         │       │   ├─ FreeBSD
         │       │   └─ OpenBSD
         │       ├─ Live CDs
         │       │   ├─ Ubuntu Live
         │       │   ├─ Kali Live
         │       │   └─ ...
         │       └─ 工具
         │           ├─ Clonezilla
         │           ├─ GParted
         │           ├─ SystemRescue
         │           ├─ MemTest86
         │           └─ ...
         └─ ④ iPXE Shell / 网络信息
```

## 引导文件查找优先级

为每个发行版生成 iPXE boot 行时：

```
# PxeGo 生成的 iPXE 脚本逻辑
:boot_<distro>_<version>
imgfree

# 优先级 1: 本地文件存在 → PxeGo HTTP 本地服务
# （用户在 ~/.pxego/boot/netboot/ 放置了文件）
kernel http://<pxego>/boot/netboot/ubuntu/24.04/vmlinuz root=/dev/ram0 ...
initrd http://<pxego>/boot/netboot/ubuntu/24.04/initrd
boot

# 优先级 2: 在线 fallback（本地文件不存在）
# kernel/initrd URL 来自 catalog YAML 的 remote 字段
kernel http://archive.ubuntu.com/ubuntu/dists/noble/.../linux ...
initrd http://archive.ubuntu.com/ubuntu/dists/noble/.../initrd.gz
boot

# 优先级 3: 两者都不可达 → shell
# iPXE 显示 network error，用户可进入 shell 排查
```

## HTTP 端点

新增路由：

| 端点 | 说明 |
|------|------|
| `GET /netboot/catalog` | 返回完整 OS 目录 JSON（给 Web UI 用） |
| `GET /netboot/catalog/<distro>` | 返回单个发行版的版本详情 |
| `GET /netboot/check-files` | 检查本地文件状态：哪些发行版文件已存在 |
| `PUT /netboot/catalog/<distro>` | 更新某个发行版的配置 |
| `POST /netboot/sync` | 触发同步 |
| `GET /boot/netboot/*` | 通过现有 `/boot/*` handler 服务 netboot 文件 |

## Web UI 新增功能

1. **OS 目录页面** (`NetbootCatalog.tsx`)
   - 树形浏览所有发行版
   - 按分类筛选（Linux/BSD/Tools/Live）
   - 展开查看版本、架构、文件状态
   - 指示本地文件是否存在（绿色/红色标记）

2. **预设模板页面**
   - 列表显示所有可用的发行版+版本组合
   - 勾选 → 一键创建为 PxeGo Profile
   - 支持批量勾选

3. **引导条目选择器** (嵌入 Profile 创建/编辑页)
   - 点击"从 OS 目录选择"弹出选择器
   - 选中后自动填充 kernel/initrd/URL/cmdline

4. **引导文件管理**
   - 显示本地已有/缺失的文件
   - 给出下载命令指引（如 `wget <url> -O ~/.pxego/boot/netboot/...`）

## CLI 新增命令

```bash
# 从 forked 仓库同步发行版定义
pxego netboot sync

# 同步并重新生成预设 iPXE 脚本
pxego netboot sync --generate-scripts

# 列出所有可用发行版
pxego netboot list

# 查看某个发行版的详细信息
pxego netboot info <distro>

# 检查本地引导文件状态
pxego netboot check-files

# 生成 TFTP fallback 脚本
pxego netboot generate-scripts
```

## 同步机制 (`pxego netboot sync`)

步骤：
1. 定位 `contrib/netboot.xyz/` 目录
2. `git pull` 获取上游最新代码
3. 解析 `roles/netbootxyz/defaults/main.yml` → 提取 `releases:` 段
4. 解析 `endpoints.yml` → 提取文件 URL 映射
5. 将 Ansible YAML 格式转换为 PxeGo catalog YAML 格式
6. 写入 `~/.pxego/netboot/catalog/<distro>.yaml`（每个发行版一个文件）
7. 如果 `--generate-scripts`，生成预设 iPXE 脚本到 `netboot/scripts/`
8. 记录本次同步时间戳

## iPXE 脚本生成逻辑

PxeGo 的 `internal/boot/ipxe/templates.go` 会新增一个模板类型 `netboot-menu`。

生成的 iPXE 菜单脚本结构：

```
#!ipxe

:netboot_menu
menu 📦 网络安装操作系统目录
item --gap Linux 发行版
item linux-x86_64    Linux Network Installs (64-bit)
item linux-i386      Linux Network Installs (32-bit)
item linux-arm64     Linux Network Installs (arm64)
item --gap BSD 系统
item bsd             BSD Installs
item --gap Live CDs
item live            Live CDs
item --gap 工具
item tools           系统工具
item --gap
item back            ← 返回上级菜单
choose selected || goto exit

:linux-x86_64
menu Linux 发行版 (64-bit)
item ubuntu          Ubuntu
item debian          Debian
item fedora          Fedora
item centos          CentOS Stream
item ...             (动态生成)
item back            ← 返回
choose selected || goto netboot_menu

:ubuntu
menu Ubuntu - amd64
item noble           24.04 LTS Noble Numbat
item jammy           22.04 LTS Jammy Jellyfish
item focal           20.04 LTS Focal Fossa
item back            ← 返回
choose selected || goto linux-x86_64

:noble
kernel http://<pxego>/boot/netboot/ubuntu/24.04/vmlinuz ...
initrd http://<pxego>/boot/netboot/ubuntu/24.04/initrd
boot
```

## 配置变更

PxeGo 的 `config.yaml` 新增 `netboot` 段：

```yaml
netboot:
  enabled: true                   # 是否启用 netboot 菜单
  default_boot: menu             # menu | local | memdisk
  fallback_online: true          # 本地文件不存在时尝试在线源
  menu_title: "📦 网络安装操作系统目录"
  sync:
    auto: false                  # 是否启动时自动同步
    repo: contrib/netboot.xyz   # forked 仓库路径
  paths:
    catalog: ~/.pxego/netboot/catalog
    scripts: ~/.pxego/netboot/scripts
    boot_files: boot/netboot
```

## 新增 Go 包结构

```
internal/netboot/
├── types.go                    # 数据类型定义
│   ├── Distro struct           # 发行版定义
│   ├── Version struct          # 版本定义
│   ├── Catalog struct          # 完整目录
│   └── Endpoint struct         # 文件端点
│
├── catalog.go                  # 目录加载/存储
│   ├── LoadCatalog(dir)        # 从目录加载所有 YAML
│   ├── LoadDistro(file)        # 加载单个发行版
│   ├── SaveDistro(file, d)     # 保存发行版
│   └── GetCatalog()            # 获取完整目录（用于 API）
│
├── convert.go                  # 从 netboot.xyz Ansible YAML 转换
│   ├── ConvertFromAnsible(r, e)  # 转换主逻辑
│   └── ConvertEndpoint(e)      # 转换端点定义
│
├── script.go                   # iPXE 脚本生成
│   ├── GenerateNetbootScript(c)  # 生成完整菜单
│   ├── GenerateDistroScript(d)   # 生成单个发行版菜单
│   └── GenerateAllScripts(c)     # 生成所有预设脚本
│
├── sync.go                     # 同步逻辑
│   ├── SyncFromUpstream(dir)   # git pull + 转换
│   └── CheckUpstream(dir)      # 检查是否有更新
│
├── files.go                    # 文件状态管理
│   ├── CheckLocalFiles(c)      # 检查本地文件状态
│   └── FileStatus struct       # 文件状态
│
└── embed.go                    # 内建默认 catalog（编译时嵌入）
    ├── DefaultCatalog          # 默认目录（第一次运行时使用）
    └── DefaultEndpoints        # 默认端点配置
```

## 实现阶段

| 阶段 | 内容 | 文件数 |
|------|------|--------|
| 1 | Go 数据结构 + catalog YAML 解析/加载 | 3-4 |
| 2 | iPXE 菜单模板 + 脚本生成 | 2-3 |
| 3 | HTTP API 端点 + 集成到现有菜单引擎 | 3-4 |
| 4 | CLI 命令 (`pxego netboot sync/list/info`) | 2-3 |
| 5 | Web UI 页面（目录浏览 + 预设模板 + 文件管理） | 4-5 |
| 6 | forked netboot.xyz 仓库接入 + 同步机制 | 1-2 |
| 7 | 引导文件管理 + 状态检查 | 1-2 |
