# 文档优化 阶段 0（样例先行）实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: 使用 superpowers:executing-plans 按任务执行。步骤用 `- [ ]` 勾选追踪。

**Goal:** 交付「三件套样例」供用户审阅文档新方向：重写的 getting-started、教程 1（给裸机装 Ubuntu）、术语表、侧边栏示例，以及 2 张真实 UI 截图。

**Architecture:** 基于已批准设计文档 `docs/superpowers/specs/2026-07-31-docs-optimization-design.md` 的阶段 0。只动 website/docs（git submodule）内：3 篇新内容文件（中英双语）、config.mts 演示版侧边栏、public/screenshots/ 截图；不涉及后端代码改动。教程/快速开始内容引用的事实（UI 路径、API、默认行为）已通过代码核实。

**Tech Stack:** VitePress 1.6（website/docs）、React Web UI（本地 `go run` 提供）、系统 Edge 无头截图、curl 预置数据。

**已核实的事实（探索代理 + 直接阅读确认）：**
- `cmd/pxelab/webdist/` 已提交，`go build` 直接可用；SPA fallback 由 `cmd/pxelab/embed.go` 的 `spaHandler()` 实现
- 本机（localhost/127.0.0.1）访问 API 与 UI 免认证（`internal/httpd/middleware.go` needsAuth/isLocalRequest）
- 默认配置仅 `service_auto_start.http: true`，DHCP/TFTP/DNS/NFS 不自动启动 → `go run` 无需管理员权限
- 服务启动失败仅置 error 状态，服务器不退出（`cmd/pxelab/main.go:283`、`internal/servicemanager/manager.go`）
- Hosts 创建 API：`POST /api/v1/hosts`，必填 `name` + `mac`（`^([0-9a-f]{2}[:-]){5}[0-9a-f]{2}$`，小写），可选 `ip`/`profile_id`
- 前端路由：`/` 仪表盘、`/services/dhcp` DHCP 配置、`/netboot-catalog`、`/hosts`、`/install-tasks`（`web/src/App.tsx:90-123`）
- 无 playwright/puppeteer；系统 Edge 位于 `C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`
- docs 构建命令：`npm run build`（vitepress build），`ignoreDeadLinks: false` → 死链会构建失败（这是文档的「测试」）
- GitHub 真实仓库地址：`github.com/PxeLab/pxelab`（config.mts socialLinks 已确认）
- 默认引导行为：无 Profile 的客户端 → catalog_redirect 启用 → 直接跳转 Netboot OS 安装目录（`reference/config-file.md` netboot.boot.catalog_redirect.enabled: true）
- 引导菜单标题默认 `[OS] Netboot OS Install Catalog`（config 示例）

---

### Task 1: 构建并运行后端，验证 UI 可访问

**Files:** 无改动（仅运行）

- [ ] **Step 1: 启动后端**

```bash
cd /d/NewCB/PxeGo && make build && ./bin/pxelab
```
用 `run_in_background: true` 运行（服务器常驻）。等待 3 秒。

- [ ] **Step 2: 验证 UI 与 API**

```bash
curl -s http://localhost:8080/api/v1/status | head -c 300
curl -s http://localhost:8080/ | head -c 200
```
预期：status JSON 返回；根路径返回 index.html（含 `<div id="root">` 之类标记）。
若失败（如 bootdist 缺失导致 build 失败），手动同步：`rm -rf cmd/pxelab/bootdist && mkdir -p cmd/pxelab/bootdist && cp -r boot/* cmd/pxelab/bootdist/ && rm -f cmd/pxelab/bootdist/README.md` 后重试 build。

- [ ] **Step 3: 验证 4 个目标 UI 页面 HTTP 200**

```bash
for p in / /services/dhcp /netboot-catalog /hosts /install-tasks; do echo "$p: $(curl -s -o /dev/null -w '%{http_code}' http://localhost:8080$p)"; done
```
预期全部 200。

---

### Task 2: 截图管线 —— 预置演示数据 + Edge 无头截图 ×2

**Files:**
- Create: `website/docs/public/screenshots/dashboard.png`
- Create: `website/docs/public/screenshots/services-dhcp.png`
- Modify: `website/docs/README.md`（记录截图生成方法，维护约定）

- [ ] **Step 1: 预置 4 台演示主机**

```bash
for i in 01 02 03 04; do
  curl -s -X POST http://localhost:8080/api/v1/hosts \
    -H "Content-Type: application/json" \
    -d "{\"name\":\"node-$i\",\"mac\":\"aa:bb:cc:dd:ee:$i\",\"ip\":\"192.168.50.10$i\"}"
done
```
预期：每条返回 201/200 且含 id。

- [ ] **Step 2: Edge 无头截仪表盘**

```bash
"/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" --headless --disable-gpu --hide-scrollbars --window-size=1920,1080 --virtual-time-budget=20000 --screenshot="D:/NewCB/PxeGo/website/docs/public/screenshots/dashboard.png" http://localhost:8080/
```
- [ ] **Step 3: Edge 无头截 DHCP 服务页**

```bash
"/c/Program Files (x86)/Microsoft/Edge/Application/msedge.exe" --headless --disable-gpu --hide-scrollbars --window-size=1920,1080 --virtual-time-budget=20000 --screenshot="D:/NewCB/PxeGo/website/docs/public/screenshots/services-dhcp.png" http://localhost:8080/services/dhcp
```
- [ ] **Step 4: 检查截图质量（Read 工具查看两张 png）**
  - 非空白/非全黑；能看出是仪表盘布局；若有「正在加载」残留 → 提高 `--virtual-time-budget` 到 30000 重截
  - 若整页空白 → 改用 `--headless=new` 标志重试
  - 截图内容需无真实敏感数据（本机演示数据，满足）

- [ ] **Step 5: 在 docs/README.md 追加「截图维护」小节**

内容（README 为构建排除文件，只给维护者看）：
```markdown
## 截图维护

文档截图位于 `public/screenshots/`，生成方式：
1. `make build && ./bin/pxelab`（本机 HTTP 8080，免认证）
2. 预置演示数据（见下方 curl 示例）：POST /api/v1/hosts 若干条
3. 系统 Edge 无头截图：
   `msedge --headless --disable-gpu --window-size=1920,1080 --virtual-time-budget=20000 --screenshot=<输出路径> http://localhost:8080/<路由>`

UI 改版后重跑以上三步刷新截图。
```

- [ ] **Step 6: Commit（website 子模块内）**

```bash
cd /d/NewCB/PxeGo/website && git add docs/public/screenshots/ docs/README.md && git commit -m "docs: add sample screenshots (dashboard, dhcp service page)"
```

---

### Task 3: 新增术语表（zh + en）

**Files:**
- Create: `website/docs/glossary.md`
- Create: `website/docs/en/glossary.md`

- [ ] **Step 1: 写入 `website/docs/glossary.md`（内容见下方）**
- [ ] **Step 2: 写入 `website/docs/en/glossary.md`（下方内容的英文翻译）**
- [ ] **Step 3: Commit**

```bash
cd /d/NewCB/PxeGo/website && git add docs/glossary.md docs/en/glossary.md && git commit -m "docs: add glossary (zh/en)"
```

**glossary.md 完整内容：**

```markdown
# 术语表

> 遇到不认识的缩写？这里用一句话解释。正文里术语首次出现时也会顺带解释。

- **PXE** — 让电脑开机时直接从网络加载操作系统的机制，不需要 U 盘或光驱
- **iPXE** — PXE 的增强版开源实现，支持 HTTP 引导、脚本编程等，功能远强于传统 PXE
- **NBP**（Network Bootstrap Program，网络引导程序）— 客户端通过网络加载的第一个引导程序（如 ipxe.pxe），负责拉起后续引导流程
- **DHCP** — 自动为网络设备分配 IP 地址与引导信息的协议
- **ProxyDHCP** — 叠加在现有 DHCP 之上的引导信息源：IP 仍由现有 DHCP 分配，PXE 引导信息由 PxeLab 提供
- **TFTP** — 简单的文件传输协议，传统 PXE 用它传输引导文件
- **HTTP** — 网页传输协议，iPXE 用它更快地加载引导文件与安装镜像
- **DNS** — 域名解析服务
- **NFS** — 网络文件系统，可把远程目录当作本地目录使用
- **UEFI / BIOS** — 两种电脑固件类型，决定引导流程的形态
- **Secure Boot** — UEFI 安全启动机制，只允许加载经过签名的引导程序
- **引导菜单（Boot Menu）** — 客户端引导后看到的选项列表（安装系统、从本地硬盘启动等）
- **Profile（引导配置文件）** — 绑定到特定主机的引导配置，决定该机器如何引导
- **Netboot 目录（OS 安装目录）** — PxeLab 内置的发行版安装菜单，列出可安装的操作系统
- **应答文件** — 预填安装问题的配置文件（preseed / kickstart / autounattend.xml），实现无人值守安装
- **架构码（DHCP Option 93）** — DHCP 选项里标识客户端 CPU 架构的字段，PxeLab 据此选择对应的引导文件
- **WOL（Wake-on-LAN，网络唤醒）** — 通过网络发送魔术包，远程唤醒关机状态的机器
- **BMC / IPMI** — 服务器的带外管理接口，可远程开机、关机、查看电源状态
- **sanboot** — 从 iSCSI 存储直接启动系统，客户端硬盘都不需要（无盘工作站）
- **WDS** — Windows 部署服务；PxeLab 支持 WDS 场景的 Windows 安装
```

**en/glossary.md 内容为上述条目的英文翻译**（阶段 5 再做精修，此处先保证结构完整与可读）。

---

### Task 4: 重写 getting-started.md（zh）

**Files:**
- Modify: `website/docs/getting-started.md`（全文替换）
- 注意：`en/getting-started.md` 本轮不动（阶段 5 翻译）

- [ ] **Step 1: 写入新内容（见下方）**
- [ ] **Step 2: 核对 Markdown 链接全部有效**（指向 glossary.md、tutorials/install-ubuntu.md、guides/dhcp.md、guides/netboot.md、guides/install-tasks.md、guides/host-management.md、troubleshooting.md、faq.md、reference/config-file.md —— 均为现有或本计划将创建的文件）
- [ ] **Step 3: Commit**

```bash
cd /d/NewCB/PxeGo/website && git add docs/getting-started.md && git commit -m "docs: rewrite getting-started for beginner-first onboarding"
```

**getting-started.md 完整内容：**

```markdown
# 快速开始

> 目标：15 分钟内，从下载 PxeLab 到完成第一次网络装机。

**相关文档**: [术语表](glossary.md) | [教程 1：给裸机装 Ubuntu](tutorials/install-ubuntu.md) | [故障排查](troubleshooting.md)

---

## PXE 是什么？

PXE（Preboot eXecution Environment，预启动执行环境）是一种让电脑**开机时直接从网络加载操作系统**的机制——不需要 U 盘，不需要光驱，甚至不需要硬盘里已经有系统。

一个典型场景：机房新到 20 台裸机，要在半天内全部装好系统。用 U 盘逐台安装不现实，PXE 让每台机器开机后自动从网络获取系统镜像，批量装机变成一件可管理的事。

PxeLab 把 PXE 所需的 **DHCP、TFTP、HTTP、DNS、NFS** 五个网络服务打包进一个零依赖的程序，用 Web 界面管理整个流程。想了解技术细节？见[术语表](glossary.md)（更深入的引导机制讲解在「进阶主题 · 引导架构」中，建设中）。

---

## 系统要求

| 项目 | 要求 |
|------|------|
| **操作系统** | Windows 10+ / Linux（kernel 3.10+）/ macOS 12+ |
| **硬件架构** | amd64 / arm64（Linux 另支持 armv7） |
| **内存 / 磁盘** | ≥ 512 MB / ≥ 1 GB |
| **网络** | 运行 DHCP 需要管理员/root 权限（端口 67） |

没有其他依赖——单文件，下载即用。

---

## 下载与安装

### 方式一：下载 Release（推荐）

从 GitHub Releases 下载对应平台的二进制：

```bash
# Linux amd64
wget https://github.com/PxeLab/pxelab/releases/latest/download/pxelab_linux_amd64.tar.gz
tar xzf pxelab_linux_amd64.tar.gz

# macOS arm64
wget https://github.com/PxeLab/pxelab/releases/latest/download/pxelab_darwin_arm64.tar.gz
tar xzf pxelab_darwin_arm64.tar.gz
```

Windows：下载 `pxelab_windows_amd64.zip`，解压得到 `pxelab.exe`。

### 方式二：从源码编译

```bash
git clone https://github.com/PxeLab/pxelab.git
cd pxelab
make build          # 生成 bin/pxelab
```

### 方式三：Docker

容器镜像在 Roadmap 中，暂未提供。

---

## 首次启动

```bash
# Linux / macOS（DHCP 端口 67 需要 root）
sudo ./pxelab

# Windows（以管理员身份运行）
pxelab.exe
```

启动后，打开浏览器访问 **http://localhost:8080**，你会看到仪表盘：

![PxeLab 仪表盘](./screenshots/dashboard.png)

顶部的**服务状态栏**显示各服务的运行状态：默认只启动了 HTTP 服务，DHCP / TFTP / DNS / NFS 会在你配置并启动后变绿。

> **Windows**：PxeLab 默认以系统托盘模式运行，托盘图标提供打开浏览器、查看数据目录、退出等功能。

---

## 第一次网络装机（5 步）

准备好一台支持网络引导的电脑（裸机或已有系统的机器都行），跟着做：

**第 1 步：创建子网**
进入 **基础配置 → 服务配置 → DHCP**，点击「添加子网」，填写网段（如 `192.168.50.0/24`）、地址池（如 `192.168.50.100-200`）、网关、DNS，DHCP 模式保持 **server**。保存后在服务列表启动 DHCP 服务。

**第 2 步：确认 OS 安装目录已开启**
进入 **基础配置 → 服务配置 → Netboot 目录**，确认「启用」开关是开的（默认开启）。它决定客户端引导后能看到哪些操作系统。

**第 3 步：客户端开机，选择网络引导**
客户端开机后进入启动菜单（常见按键：F12 / F11 / Esc，各厂商不同），选择**网络引导（PXE Boot / Network Boot）**。

**第 4 步：在引导菜单选择系统**
客户端出现引导菜单后，选择 **Netboot OS Install Catalog** → Ubuntu → 选择版本 → 开始安装。

**第 5 步：验证**
回到仪表盘：新主机出现在「在线主机」列表；在**管理 → 安装任务**页能看到这台机器的安装记录。安装完成后，客户端从本地硬盘正常启动。

> 想每个细节都走一遍？看[教程 1：给一台裸机安装 Ubuntu](tutorials/install-ubuntu.md)。

---

## 下一步

- **教程**：[给裸机装 Ubuntu](tutorials/install-ubuntu.md) · 在现有 DHCP 网络叠加 PXE · 搭建无盘工作站（教程 2/3 建设中）
- **使用指南**：按功能深度了解（[DHCP 配置](guides/dhcp.md)、[引导菜单配置](guides/boot-config.md)、[主机管理](guides/host-management.md)…）
- **API 自动化**：REST API 快速上手（开发指南，建设中）
- **遇到问题**：[故障排查](troubleshooting.md) · [常见问题](faq.md)
```

> 注：内容中「教程 2/3 建设中」与「建设中」链接为占位表述——执行时如链接目标不存在则**不加链接**，仅保留纯文本「建设中」，避免死链导致 build 失败。

---

### Task 5: 新增教程 1 —— 给一台裸机安装 Ubuntu（zh + en）

**Files:**
- Create: `website/docs/tutorials/install-ubuntu.md`
- Create: `website/docs/en/tutorials/install-ubuntu.md`

- [ ] **Step 1: 写入 `website/docs/tutorials/install-ubuntu.md`（内容见下方）**
- [ ] **Step 2: 在跑起来的 Web UI 上核实关键文案**（浏览器打开 http://localhost:8080/services/dhcp）：子网创建按钮的确切文案（预期「添加子网」或「新建子网」）、表单字段标签；如与内容不符，就地修正
- [ ] **Step 3: 写入 `website/docs/en/tutorials/install-ubuntu.md`（下方内容的英文翻译）**
- [ ] **Step 4: Commit**

```bash
cd /d/NewCB/PxeGo/website && git add docs/tutorials/ && git commit -m "docs: add tutorial 1 - install ubuntu on a bare metal machine (zh/en)"
```

**install-ubuntu.md 完整内容：**

```markdown
# 教程 1：给一台裸机安装 Ubuntu

> ⏱ 预计用时：15 分钟 ｜ 难度：零基础 ｜ 读者：第一次使用 PxeLab 的用户
> 前置条件：PxeLab 已启动（见[快速开始](../getting-started.md)）、一台支持网络引导的电脑、客户端与服务器网络互通

---

## 场景

机房里新到一台没有系统的裸机：没有 U 盘、没有光驱。要给它装 Ubuntu，并且以后其他机器也想用同样的方式安装。

本教程带你走完完整流程：**创建网络 → 客户端网络引导 → 选择 Ubuntu → 开始安装**。

---

## 准备

- PxeLab 服务器已运行，Web 界面可访问（http://localhost:8080）
- 客户端与 PxeLab 在同一网段，能互相访问
- 一个空闲网段（本教程示例用 `192.168.50.0/24`，请按你的网络替换）
- 客户端硬件支持 PXE 网络引导（几乎所有网卡和主板都支持，默认开启）

---

## 第 1 步：创建子网

进入 **基础配置 → 服务配置 → DHCP**，点击「添加子网」，填写：

| 字段 | 示例值 |
|------|--------|
| 网段（CIDR） | `192.168.50.0/24` |
| 地址池 | `192.168.50.100 - 192.168.50.200` |
| 网关 | `192.168.50.1` |
| DNS | `8.8.8.8` |
| DHCP 模式 | `server` |

保存后，在服务列表里把 DHCP 服务启动起来。

> **模式说明**：`server` 模式表示 PxeLab 充当这个网段的唯一 DHCP 服务器，同时分配 IP 和引导信息。如果你的网络已有 DHCP 服务器，改用 [proxy 模式（教程 2，建设中）]——只需叠加引导信息，不影响现有网络。

## 第 2 步：确认 OS 安装目录已开启

进入 **基础配置 → 服务配置 → Netboot 目录**，确认「启用」开关是开的（默认开启）。

PxeLab 内置了 64+ 主流发行版的安装目录（Ubuntu、Debian、CentOS、Windows 等）。客户端引导后会看到这个目录，选择要安装的系统即可。

## 第 3 步：客户端开机，进入网络引导

客户端开机，进入启动菜单（常见按键 F12 / F11 / Esc，各厂商不同），选择**网络引导（PXE Boot / Network Boot）**。

正常的话，几秒内会出现 PxeLab 引导菜单。

## 第 4 步：选择 Ubuntu，开始安装

1. 在引导菜单中选择 **Netboot OS Install Catalog**，进入 OS 安装目录
2. 进入 **Ubuntu** → 选择要安装的版本
3. 客户端自动下载引导文件，进入 Ubuntu 安装向导
4. 按向导完成安装

## 第 5 步：验证

- **仪表盘**：「在线主机」列表出现这台新主机
- **安装任务**：**管理 → 安装任务** 页能看到这台机器对应的安装记录
- 安装完成后重启客户端，从本地硬盘正常进入 Ubuntu

---

## 本教程常见问题

| 现象 | 可能原因 | 解决 |
|------|---------|------|
| 客户端开机不进网络引导 | 启动顺序/固件设置 | 手动进启动菜单选择 PXE；检查网线连接 |
| 客户端拿不到 IP | DHCP 服务未启动/防火墙拦截 | 服务配置确认 DHCP 状态为运行中；放行 UDP 67/68 |
| 出现引导菜单但没有 OS 目录 | Netboot 目录未启用 | 回到第 2 步检查启用开关 |
| 配置改了不生效 | 未保存/服务未重启 | 确认保存后重新启动对应服务 |

---

## 更进一步

- **无人值守安装**：用[应答文件模板](../guides/answer-templates.md) + [安装任务](../guides/install-tasks.md)，安装过程无需人工点击向导
- **绑定主机引导**：为特定机器绑定 [Profile（引导配置）](../guides/profiles.md)，让不同机器装不同系统
- **更多场景**：在已有 DHCP 的网络叠加 PXE（教程 2，建设中）· 搭建无盘工作站（教程 3，建设中）
```

> 注：教程中「建设中」的不存在链接不写为链接，仅纯文本标注。

---

### Task 6: config.mts 侧边栏/导航示例（zh + en 同步）

**Files:**
- Modify: `website/docs/.vitepress/config.mts`

- [ ] **Step 1: 修改 `sidebarDef`**
  - `'/'` 分区：在 `产品` 组之后新增 `快速开始` 组：`getting-started`（快速开始）+ `glossary`（术语表）
  - 新增 `'/tutorials/'` 键：`教程` 组，1 项：`install-ubuntu`（教程 1：给裸机装 Ubuntu）
- [ ] **Step 2: 修改 nav（zh 与 en 两处）**
  - zh：`快速开始` 之后插入 `{ text: '教程', link: '/tutorials/install-ubuntu', activeMatch: '/tutorials/' }`
  - en：对应插入 `{ text: 'Tutorials', link: '/en/tutorials/install-ubuntu', activeMatch: '/en/tutorials/' }`
- [ ] **Step 3: 在 `srcExclude` 追加 `'tutorials/README.md'`**（如果创建了该文件；未创建则跳过）
- [ ] **Step 4: Commit**

```bash
cd /d/NewCB/PxeGo/website && git add docs/.vitepress/config.mts && git commit -m "docs: sidebar demo - add getting started & tutorials groups (phase 0)"
```

---

### Task 7: 验证与收尾

**Files:** 无改动

- [ ] **Step 1: VitePress 构建（死链即失败）**

```bash
cd /d/NewCB/PxeGo/website/docs && npm run build
```
预期：构建成功，无 dead link 报错。若报死链：逐一修复（多为教程内「建设中」的误加链接）。

- [ ] **Step 2: 本地预览检查渲染**

```bash
cd /d/NewCB/PxeGo/website/docs && npm run dev
```
打开 http://localhost:5173：检查新侧边栏分组、getting-started 排版与截图显示、教程页渲染。

- [ ] **Step 3: 停掉后台 pxelab 进程**
- [ ] **Step 4: 汇总交付清单**（改动的文件列表 + 截图预览 + 关键决策回顾）呈报用户审阅

---

## 自检记录（writing-plans 技能要求）

- **Spec 覆盖**：阶段 0 四件套（getting-started 重写 ✓ Task 4、教程 1 ✓ Task 5、侧边栏示例 ✓ Task 6、2 张样例截图 ✓ Task 2）；术语表 ✓ Task 3；占位符链接修复 ✓ Task 4 内容；截图维护约定 ✓ Task 2 Step 5
- **占位符扫描**：无 TBD/TODO；「建设中」条目明确不写链接避免死链；所有命令含预期输出
- **类型一致性**：文件路径/API 端点/路由全部来自核实结果；en 镜像结构一致
- **风险**：Edge 无头截图空白（Task 2 Step 4 有 fallback 步骤）；DHCP 页按钮文案（Task 5 Step 2 运行时核实）；`go run` 需 bootdist（Task 1 Step 2 有 fallback）
