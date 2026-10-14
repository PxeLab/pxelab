# PxeLab Windows 网络安装 — Phase 0/1/2 路线图

> 定位：**阶段化路线图（排期与状态视角）**。承接操作手册 `docs/windows-deployment.md`、借鉴清单 `docs/design/pxesrv-linux-winpe-borrowable.md`、需求文档 `docs/design/quick-install-optimization-req.md`（R6 剩余 + 待办 B1/B2）。
> 结论先行：**Phase 0（端到端可用）已闭环**；Phase 1（boot-critical 驱动覆盖）中借鉴项 2 已随 Phase 0 落地、借鉴项 1 待做；Phase 2（生产级完善）待排期。

---

## 0. 阶段总览

| 阶段 | 目标 | 状态 | 关联借鉴项 / 需求 |
|---|---|---|---|
| **Phase 0** | Windows 网络安装端到端可用（离线可引导 + 无人值守 + 应答注入） | ✅ 已完成 | R6 A 档（`a2c6533` 基础上补全驱动/共享/流式） |
| **Phase 1** | boot-critical 驱动覆盖（PE 认盘 / 认网） | 借鉴项 2 已落地；借鉴项 1 待做 | 借鉴项 1（= B1）、借鉴项 2 |
| **Phase 2** | 生产级完善（架构 / 索引 / 验证 / 预设 / 继承） | 待排期 | 借鉴项 3/4/5/6、待办 B2 |

---

## 1. Phase 0 — 端到端闭环（已完成）

### 1.1 交付物（按组件）

| 组件 | 位置 |
|---|---|
| 介质交付 | `internal/smb/server.go` — 内置最小只读 SMB 服务：NTLMv2 账号认证、IP 白名单、只读 `osfs` 挂载、固定 445 端口 |
| 配置 / 生命周期 | `internal/config/{config,defaults,flags}.go`；`internal/api/service.go`；`cmd/pxelab/main.go`（注册 smb 服务） |
| REST API | `internal/api/settings.go` → `GET/PUT /api/v1/services/smb`、`POST /services/smb/validate-path`、`GET /services/smb/browse-path`（路径校验/浏览复用 NFS 实现）；路由注册于 `internal/api/handler.go` |
| 前端 | `web/src/pages/SettingsSMB.tsx`（Settings → SMB 页）+ `web/src/components/ui/StringListEditor.tsx`；`web/src/api/settings.ts`、`web/src/api/client.ts`；中英双语 i18n |
| 引导链 | DHCP → iPXE → `/boot/ipxe/script` → `/boot/ipxe/menu` → `generateBootMenu`（`internal/httpd/server.go`） |
| 应答注入 | `generateBootMenu` 按 MAC 解析活跃安装任务（`GetLatestInstallTaskByHostMAC` + `netboot.TaskBootDecision`）→ `answerFromTask` → 注入该主机所有 `wds` 条目（`MenuEntryData.AnswerInitrds` + `WIMIndex`，模板 `internal/boot/ipxe/templates.go`） |
| PE 内执行 | `winpeshl.ini` = `[LaunchApps]` + `%SYSTEMROOT%\System32\install.bat`（`internal/api/install_tasks.go`）；`install.bat` 预设 = **Recipe B**（`internal/api/answer_templates.go`） |
| 文件传输 | `/boot/*` 改 `bootFS.Serve`（支持 Range/HEAD），TFTP 改 `bootFS.Open` + `rf.ReadFrom` — 流式下发，修复大文件 OOM |

### 1.2 关键技术决策（已批准）

- **介质交付 = 平台内置只读 SMB，而非 HTTP**：`dism /Apply-Image /ImageFile:` 与 `setup.exe` 不接受 http URL；SMB 是 Windows 原生可达路径。
- **认证仅 NTLMv2 账号，不做 guest / 匿名**：`go-filesystems/smb` 不支持 guest；且 24H2 / Server 2025 强制 SMB 签名后 guest 被禁。
- **安装 = Recipe B**：`net use Z: \\<server>\<share>` → `dism /Apply-Image /ImageFile:%SRC%\install.wim /Index:%WIMINDEX%` → `bcdboot … /f UEFI` → 重启；不经交互式 `setup.exe`。
- **最低支持 Win7 / Server 2008 R2（WinPE 3.0）**；服务端仅 SMB 2.1 / 3.0 / 3.0.2（无 SMB1/2.0）。
- **WIM 索引贯通**：`models.InstallTask.WIMIndex` → `BootTaskInfo.WIMIndex` → wimboot kernel 行 `index=<n>`（多映像介质）。

### 1.3 已知边界与风险（需真机验证）

- **WinPE 24H2 `System error 1727`**（认证式 SMB 连接失败）：根因 KB5051987，已修于 KB5058411（26100.4061），仍建议真机实测。
- **Windows 主机 445 端口被 LanmanServer（「Server」服务）占用**：须先停该服务，否则 PxeLab SMB 无法监听。
- **真机验证缺失**：iPXE `initrd -n <name>` 具名语义、wimboot `index=`、`winpeshl.ini [LaunchApps]`、Recipe B 的 `dism /Apply-Image` 均只有 build / vet / 单测级证据，**未在真实 Windows 硬件上跑通**。

---

## 2. Phase 1 — boot-critical 驱动覆盖

> 目标：让 PE 在 `setup.exe` / `dism` 扫描磁盘前就具备存储控制器、网卡等驱动，覆盖「认不到盘」场景。分两条互补路径。

### 2.1 借鉴项 2 — PE 阶段运行时驱动加载 ✅ 已落地

- **实现**：
  - `internal/api/driver_packages.go` → `buildPeDriverInstallBlock` / `injectWinpeShellDrivers`（另 `buildDriverCommands` 供首登）。
  - `internal/api/install_tasks.go` → `GetAnswerFile`：在 winpeshl 模式（模板 `type=winpeshl`，或 overlay 版本 `answer_type=winpeshl`，见 `resolveTaskAnswerType`）且主机绑定驱动包时，把驱动块注入 `install.bat` **顶部**（位于 Recipe B 的 `dism` 之前）；`?file=winpeshl.ini` 那次请求不注入。
  - 驱动块：从 `/boot/drivers/<包>/` HTTP 下载 → `pnputil /add-driver … /install`（缺失 `pnputil` 时回退 `drvload`）；包内按 `x64/`、`x86/`、`arm64/` 子目录按目标架构过滤。
- **验收**：winpeshl 主机绑定驱动包后，PE 内 `pnputil /enum-drivers` 在 `dism` 前已列出该驱动。
- **测试**：`internal/api/driver_packages_test.go` → `TestGetAnswerFileWinpeShellInjectsDrivers`、`TestGetAnswerFileWinpeShellViaOverlayAnswerType`。

### 2.2 借鉴项 1（= 待办 B1）— 离线 `dism /add-driver` 注入 `boot.wim` ⏳ 待做

- **Gap**：PE 阶段运行时加载已覆盖多数场景，但部分驱动须在 PE 启动前注册进 driver store 才生效；`internal/osimage` 现仅有 ISO 提取（`ExtractISO`），**无 WIM 挂载 / 注入能力**。
- **最小改动（文件 + 函数）**：
  - 新增 `internal/osimage/wim.go`（平台无关接口）+ `wim_windows.go`（dism 实现）+ `wim_other.go`（wimlib 或返回「不支持」）：
    `MountWIM(wimPath, mountDir string, index int) error`、`AddDriverToWIM(mountDir, driverDir string, recurse bool) error`、`UnmountWIM(mountDir string, commit bool) error`、`ListWIMDrivers(mountDir string) ([]string, error)`、`CleanupWIM() error`、`GetWIMInfo(wimPath string) ([]WIMIndex, error)`。
  - 新增 `internal/api/wim_inject.go`：`POST /api/v1/os-images/{id}/inject-drivers`（body `{driver_packs:[], target:"boot"|"install", index:N}`）；`internal/api/handler.go` 注册。
  - `internal/models/os_image.go` 增注入记录字段（或新增注入历史表）。
  - **注入产出新副本**（原 `boot.wim` 备份为 `boot.wim.orig`），`SetCatalogLocal` / `CreateFromOSImage` 指向注入后的副本。
- **关键 caveat**：注入 **`boot.wim` index 1**（wimboot 引导的正是它；注入 index 2 对 PE 无效）；**Linux 主机 `wimlib-imagex` 无法注册 driver store** → 真离线注入**实际需 Windows 主机**（Linux 降级为借鉴项 2）；注入后 WIM 仍为合法 WIM、wimboot 可继续流式读取（体积增大 → HTTP 传输变长）。
- **验收**：`dism /get-drivers /image:<mount>` 列出该驱动；需该驱动的机器 PE 内 `setup.exe` 可见磁盘并完成安装；原 `boot.wim` 未改（有备份）。
- **工作量**：**L**；**优先级**：**P1**。

---

## 3. Phase 2 — 生产级完善

### 3.1 借鉴项 3 — 驱动包按架构分离（S / P2）

- **Gap**：`boot/drivers/<名称>/` 架构无关，x86 / x64 `.inf` 混放会导致 `pnputil` 失败或装错。
- **改动**：约定扩展为 `boot/drivers/<名称>/<arch>/`（arch ∈ `x64`/`x86`/`arm64`）；`listDriverPackFiles` / `buildDriverCommands` 增 arch 参数按目标架构过滤；`DriverPackageHandler.List()` 返回各包可用架构。
- **验收**：x64 主机只下发 x64 驱动。

### 3.2 借鉴项 4 — WIM 索引感知（S / P2）

- **现状**：Phase 0 已把 **install 侧**索引贯通（`InstallTask.WIMIndex` → wimboot `index=`）。
- **待做**：离线注入 API 增 `index` 参数；`GetWIMInfo()`（`dism /get-wiminfo`）列出可选索引；默认 `boot.wim` index 1。
- **验收**：注入命中正确 index；`dism /get-wiminfo` 可见。

### 3.3 借鉴项 5 — 注入验证与清理（S / P2）

- **Gap**：无 `dism /get-drivers` 验证、无 `dism /cleanup-wim` 清理。
- **改动**：`ListWIMDrivers()` + `CleanupWIM()`；UI 展示已注入驱动；每次挂载操作后 `defer` 清理。
- **验收**：UI 显示注入清单；失败后无残留挂载。

### 3.4 借鉴项 6 — 内置 winpeshl「跑安装」预设（S / P3）

- **现状**：Phase 0 已内置 **Recipe B** winpeshl 预设（SMB 只读共享 + `dism /Apply-Image` + `bcdboot` + PE 阶段驱动块）。
- **待做（可选）**：预设参数化（共享名 / 盘符 / 索引 / 镜像路径）与多预设。
- **验收**：选预设即可无人值守安装（已基本达成）。

### 3.5 待办 B2 — 按 profile / OS 版本绑定默认驱动包（M）

- **Gap**：驱动包按主机逐台绑定，同型号同系统的多台机器需重复手绑。
- **改动**：profile（或 OS 镜像条目）增默认驱动包列表；主机未显式绑定时继承 profile 的；主机级绑定优先。R7 批量装机时同批机器自动获得正确驱动。
- **验收**：批量装机时同批机器自动获得正确驱动，无需逐台设置。

---

## 4. 建议排期

```
Phase 1: 借鉴项 2（✅ 已落地）→ 借鉴项 1（离线 dism 注入 boot.wim，L，需 Windows 主机）
Phase 2: 借鉴项 3（架构分离）→ 借鉴项 4/5（索引 + 验证清理，并入借鉴项 1）
         → 借鉴项 6 参数化 → B2（按 profile 继承驱动包）
```

> 借鉴项 1 的实现要点与命令细节见 `docs/design/pxesrv-linux-winpe-borrowable.md` §2 / §3。

---

## 5. 与既有文档的关系

| 文档 | 视角 |
|---|---|
| `docs/windows-deployment.md` | **操作手册** — 用户怎么装（两条路线步骤、常见问题） |
| `docs/design/pxesrv-linux-winpe-borrowable.md` | **借鉴清单** — 与 pxesrv-linux 的对照、借鉴项 1–6 的 gap / 改动 / 命令 |
| `docs/design/quick-install-optimization-req.md` | **需求** — R6（Windows 链路深化）剩余范围 + 待办 B1/B2 |
| 本文 | **阶段化路线图** — Phase 0/1/2 的状态、映射与排期 |
