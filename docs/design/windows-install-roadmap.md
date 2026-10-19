# PxeLab Windows 网络安装 — Phase 0/1/2 路线图

> 定位：**阶段化路线图（排期与状态视角）**。承接操作手册 `docs/windows-deployment.md`、借鉴清单 `docs/design/pxesrv-linux-winpe-borrowable.md`、需求文档 `docs/design/quick-install-optimization-req.md`（R6 剩余 + 待办 B1/B2）。
> 结论先行：**Phase 0（端到端可用）已闭环**；Phase 1（boot-critical 驱动覆盖）借鉴项 1、2 均已实现；Phase 2（生产级完善）借鉴项 3/4/5/6 与待办 B2 均已实现。**离线 dism 注入（借鉴项 1）与相关引导链仍待真机验证**（见 1.3）。

---

## 0. 阶段总览

| 阶段 | 目标 | 状态 | 关联借鉴项 / 需求 |
|---|---|---|---|
| **Phase 0** | Windows 网络安装端到端可用（离线可引导 + 无人值守 + 应答注入） | ✅ 已完成 | R6 A 档（`a2c6533` 基础上补全驱动/共享/流式） |
| **Phase 1** | boot-critical 驱动覆盖（PE 认盘 / 认网） | ✅ 已实现（借鉴项 1 待真机验证） | 借鉴项 1（= B1）、借鉴项 2 |
| **Phase 2** | 生产级完善（架构 / 索引 / 验证 / 预设 / 继承） | ✅ 已实现 | 借鉴项 3/4/5/6、待办 B2 |

---

## 1. Phase 0 — 端到端闭环（已完成）

### 1.1 交付物（按组件）

| 组件 | 位置 |
|---|---|
| 介质交付 | `internal/smb/server.go` — 内置最小只读 SMB 服务：NTLMv2 账号认证、IP 白名单、只读 `osfs` 挂载、固定 445 端口 |
| 介质交付（Windows 备选） | `internal/smb/system*.go` — **系统 SMB 后端**：使用 Windows 自带 SMB（LanmanServer）创建共享（`New-SmbShare` + `icacls`），不占用端口；适用于 445 被系统占用、无法停 LanmanServer 的主机。配置 `smb.backend: builtin \\| system` + `smb.system_account` |
| 配置 / 生命周期 | `internal/config/{config,defaults,flags}.go`；`internal/api/service.go`；`cmd/pxelab/main.go`（注册 smb 服务） |
| REST API | `internal/api/settings.go` → `GET/PUT /api/v1/services/smb`、`POST /services/smb/validate-path`、`GET /services/smb/browse-path`（路径校验/浏览复用 NFS 实现）；路由注册于 `internal/api/handler.go` |
| 前端 | `web/src/pages/SettingsSMB.tsx`（Settings → SMB 页）+ `web/src/components/ui/StringListEditor.tsx`；`web/src/api/settings.ts`、`web/src/api/client.ts`；中英双语 i18n |
| 引导链 | DHCP → iPXE → `/boot/ipxe/script` → `/boot/ipxe/menu` → `generateBootMenu`（`internal/httpd/server.go`） |
| 应答注入 | `generateBootMenu` 按 MAC 解析活跃安装任务（`GetLatestInstallTaskByHostMAC` + `netboot.TaskBootDecision`）→ `answerFromTask` → 注入该主机所有 `wds` 条目（`MenuEntryData.AnswerInitrds` + `WIMIndex`，模板 `internal/boot/ipxe/templates.go`） |
| PE 内执行 | `winpeshl.ini` = `[LaunchApps]` + `%SYSTEMROOT%\System32\install.bat`（`internal/api/install_tasks.go`）；`install.bat` 预设 = **Recipe B**（`internal/api/answer_templates.go`） |
| 文件传输 | `/boot/*` 改 `bootFS.Serve`（支持 Range/HEAD），TFTP 改 `bootFS.Open` + `rf.ReadFrom` — 流式下发，修复大文件 OOM |

### 1.2 关键技术决策（已批准）

- **介质交付 = 平台内置只读 SMB，而非 HTTP**：`dism /Apply-Image /ImageFile:` 与 `setup.exe` 不接受 http URL；SMB 是 Windows 原生可达路径。Windows 上另提供**系统 SMB 后端**（`smb.backend: system`），借用 LanmanServer 的 445 创建共享，避免与系统服务争端口。
- **认证仅 NTLMv2 账号，不做 guest / 匿名**：`go-filesystems/smb` 不支持 guest；且 24H2 / Server 2025 强制 SMB 签名后 guest 被禁。系统 SMB 后端改用**已存在的 Windows 账号**（PxeLab 只授权、不建账号），原因相同：Win10/11 默认禁止 guest 网络登录。
- **安装 = Recipe B**：`net use Z: \\<server>\<share>` → `dism /Apply-Image /ImageFile:%SRC%\install.wim /Index:%WIMINDEX%` → `bcdboot … /f UEFI` → 重启；不经交互式 `setup.exe`。
- **最低支持 Win7 / Server 2008 R2（WinPE 3.0）**；服务端仅 SMB 2.1 / 3.0 / 3.0.2（无 SMB1/2.0）。
- **WIM 索引贯通**：`models.InstallTask.WIMIndex` → `BootTaskInfo.WIMIndex` → wimboot kernel 行 `index=<n>`（多映像介质）。

### 1.3 已知边界与风险（需真机验证）

- **WinPE 24H2 `System error 1727`**（认证式 SMB 连接失败）：根因 KB5051987，已修于 KB5058411（26100.4061），仍建议真机实测。
- **Windows 主机 445 端口被 LanmanServer（「Server」服务）占用**：两种情况——
  - 可停服务：先停 LanmanServer 再用**内置 SMB**；
  - 无法停（部分系统/策略下 `Stop-Service` 被拒或自动重启，示例：`New-SmbShare : 拒绝访问` 说明当前 PowerShell 未提权，而 `System error 5` 也可能来自服务受保护）：改用**系统 SMB 后端**（`smb.backend: system` + `smb.system_account`），PxeLab 以管理员身份创建共享并授权，无需动 LanmanServer。
- **真机验证缺失**：iPXE `initrd -n <name>` 具名语义（含 `driver_delivery=initrd` 时**多驱动文件注入**的实际表现与 PE 内存占用）、wimboot `index=`、`winpeshl.ini [LaunchApps]`、Recipe B 的 `dism /Apply-Image`、离线 `dism /Add-Driver` 注入（借鉴项 1）均只有 build / vet / 单测级证据，**未在真实 Windows 硬件上跑通**。→ 逐项验证步骤与结果记录见 `docs/design/windows-install-hw-test-plan.md`。

---

## 2. Phase 1 — boot-critical 驱动覆盖

> 目标：让 PE 在 `setup.exe` / `dism` 扫描磁盘前就具备存储控制器、网卡等驱动，覆盖「认不到盘」场景。分两条互补路径。

### 2.1 借鉴项 2 — PE 阶段运行时驱动加载 ✅ 已落地

- **实现**：
  - `internal/drivers/` → 驱动包领域逻辑（列举 / 架构过滤 / 架构推导 / `EffectivePacks` 继承规则），以及两种交付方式的文本生成：`PeInstallBlockHTTP`（HTTP 下载）与 `PeInstallBlockInitrd`（initrd 直投落地）+ `PlanInjections` / `InjectName` / `InitrdLines`。`internal/api/driver_packages.go` 保留薄别名。
  - `internal/api/install_tasks.go` → `GetAnswerFile`：在 winpeshl 模式（模板 `type=winpeshl`，或 overlay 版本 `answer_type=winpeshl`，见 `resolveTaskAnswerType`）且主机绑定驱动包时，把驱动块注入 `install.bat` **顶部**（位于 Recipe B 的 `dism` 之前）；`?file=winpeshl.ini` 那次请求不注入。
  - **交付方式由 `netboot.driver_delivery` 决定**（Settings → Netboot）：
    - `http`（默认）：从 `/boot/drivers/<包>/` 用 `curl`/`certutil` 下载 → `pnputil /add-driver … /install`（缺失 `pnputil` 时回退 `drvload`）。
    - `initrd`：iPXE 把**每个驱动文件**作为具名 initrd 注入 WinPE 内存盘的 `%SYSTEMROOT%\System32`（`internal/httpd` 菜单生成，见 `generateBootMenu`），`install.bat` 再 `copy` 落地到 `X:\pxelab-drivers\<包>\…` 后 `pnputil` 安装。**完全离线**、无需 PE 内 HTTP；代价是预启动即传输全部驱动文件、占用更多 PE 内存。
  - 两者共享 `drivers.PlanInjections`：注入文件名由 `pack`+相对路径确定性派生（`drivers.InjectName`），因此**菜单（initrd 行）与 install.bat（落地 copy）引用的文件名必然一致**（有单测守护该不变量）。
  - 包内按 `x64/`、`x86/`、`arm64/` 子目录按目标架构过滤。
- **验收**：winpeshl 主机绑定驱动包后，PE 内 `pnputil /enum-drivers` 在 `dism` 前已列出该驱动（`http` 与 `initrd` 两种方式均适用；`initrd` 方式待真机验证多文件注入的实际表现）。
- **测试**：`internal/drivers/drivers_test.go`；`internal/api/driver_packages_test.go` → `TestGetAnswerFileWinpeShellInjectsDrivers`、`TestGetAnswerFileWinpeShellViaOverlayAnswerType`、`TestGetAnswerFileWinpeShellInitrdDelivery`；`internal/httpd/bootmenu_test.go` → `TestGenerateBootMenuInitrdDriverDelivery`。

### 2.2 借鉴项 1（= 待办 B1）— 离线 `dism /add-driver` 注入 `boot.wim` ✅ 已实现（待真机验证）

- **实现**：
  - `internal/osimage/wim.go`（平台无关解析层 + `ErrWIMUnsupported`）+ `wim_windows.go`（Dism PowerShell cmdlet 实现）+ `wim_other.go`（非 Windows 返回不支持）：
    `MountWIM`、`AddDriverToWIM`、`UnmountWIM`、`ListWIMDrivers`、`CleanupWIM`、`GetWIMInfo`。
    Windows 实现走 `Mount-WindowsImage` / `Add-WindowsDriver` / `Dismount-WindowsImage` / `Get-WindowsDriver` / `Get-WindowsImage` + `ConvertTo-Json`（结构化字段，不受系统显示语言本地化影响）；`CleanupWIM` 用 `dism.exe /Cleanup-Wim`（Dism 模块无对应 cmdlet）。
  - `internal/api/wim_inject.go`：`POST /api/v1/os-images/{id}/inject-drivers`（body `{driver_packs:[], target:"boot"|"install", index:N, recurse?:bool}`）+ `GET /api/v1/os-images/{id}/wim-info`（列出镜像索引）；路由注册于 `internal/api/handler.go`。
  - `internal/models/os_image.go`：`injected_drivers`（JSON 数组）+ `driver_injected_at` 注入记录字段。
  - 流程：驱动包按镜像架构过滤（复用借鉴项 3）→ 暂存本地目录 → 原 WIM 备份为 `<name>.orig`（已存在则保留）→ 挂载 → 注入 → 卸载前 `Get-WindowsDriver` 验证 → 提交卸载 → `dism /Cleanup-Wim` 清理。
  - **健壮性**：同一镜像的注入/还原按镜像 ID 加锁互斥（并发请求返回 **409**，避免同时 mount/commit 同一 WIM）；`OSImage.DriverInjectCount` 记录累计注入次数（重复注入会叠加）；`POST /os-images/{id}/restore-wim` 从 `<name>.orig` 还原原始 WIM 并清空注入记录。
  - **客户端断开安全**：dism 提交完成后，记账（DB 清单/次数 + 审计）改用 `context.WithoutCancel(r.Context())`，与请求生命周期解耦 —— 避免「WIM 已提交但界面/DB 显示未注入」的不一致。
  - **启动清理**：进程被强杀/断电留下的 dism 挂载点，在 PxeLab 启动时自动 `dism /Cleanup-Wim` 清理（非 Windows 静默跳过，异步不阻塞启动）。
  - **未做（有意延后）**：*异步任务 + 阶段进度 + 取消*。当前仍是请求内同步执行（详见「计划中的异步化」issue）。
- **关键 caveat**：注入 **`boot.wim` index 1**（默认）；**非 Windows 主机返回 501**（Linux `wimlib` 无法注册 driver store）→ 改为提示使用借鉴项 2 的 PE 阶段运行时加载。
- **验收（待真机）**：`dism /get-drivers /image:<mount>` 列出该驱动；需该驱动的机器 PE 内 `setup.exe` 可见磁盘并完成安装；原 `boot.wim` 未改（有 `.orig` 备份）。
- **测试**：`internal/osimage/wim_test.go`（解析层）、`internal/api/wim_inject_test.go`（校验/DTO 路径，无 dism）。

---

## 3. Phase 2 — 生产级完善

### 3.1 借鉴项 3 — 驱动包按架构分离 ✅ 已实现

- **实现**：约定扩展为 `boot/drivers/<名称>/<arch>/`（arch ∈ `x64`/`x86`/`arm64`）；`filterDriverFilesByArch` 按目标架构过滤（`driverPackFiles` / `buildDriverCommands` / 离线注入均复用）；`DriverPackageHandler.List()` 返回各包可用架构（`driverPackageDTO.Archs`，扁平布局为 `[]`＝全架构通用）。
- **验收**：x64 主机只下发 x64 驱动。
- **测试**：`internal/api/driver_packages_test.go` → `TestDriverPackArchs`、`TestFilterDriverFilesByArch`、扩展 `TestDriverPackageListEndpoint`。

### 3.2 借鉴项 4 — WIM 索引感知 ✅ 已实现

- **实现**：离线注入 API 增 `index` 参数；`GetWIMInfo()`（`Get-WindowsImage`）列出可选索引（`GET /os-images/{id}/wim-info`）；默认 `boot.wim` index 1；install 侧索引仍由 `InstallTask.WIMIndex` → wimboot `index=` 贯通。
- **验收**：注入命中正确 index；`dism /get-wiminfo` 可见。

### 3.3 借鉴项 5 — 注入验证与清理 ✅ 已实现

- **实现**：`ListWIMDrivers()`（卸载前 `Get-WindowsDriver` 验证，结果随注入响应返回）；`CleanupWIM()`（`dism /Cleanup-Wim`）；出错路径 `defer` 丢弃式卸载，避免残留挂载；`OSImage.injected_drivers` 持久化已注入清单。
- **验收**：UI/响应可见注入清单；失败后无残留挂载。

### 3.4 借鉴项 6 — 内置 winpeshl「跑安装」预设 ✅ 已实现（参数化）

- **实现**：`netboot.AnswerData` 增 `SmbServer/SmbShare/WimIndex/WindowsDrive/EfiDrive`；内置 winpeshl 预设改用 `{{.SmbServer}}`/`{{.SmbShare}}`/`{{.WimIndex}}` 等变量；渲染侧（`GetAnswerFile` 与 `Preview`）由 `smb.shares` + 渲染基址 + `task.WIMIndex` 填充；`Preview` 支持可选覆盖。
- **验收**：选预设即可无人值守安装（已基本达成）。
- **测试**：`internal/api/answer_templates_test.go`。

### 3.5 待办 B2 — 按 profile / OS 版本绑定默认驱动包 ✅ 已实现

- **实现**：`models.Profile` 增 `driver_packs`（JSON 数组，与 `Host.DriverPacks` 同构）；`InstallTaskHandler.resolveDriverPacks` — 主机显式绑定优先，否则继承所属 Profile 的默认驱动包；`GetAnswerFile` 的 winpeshl / autounattend 两条注入路径均使用该解析；前端 Profiles 表单增驱动包编辑器。
- **验收**：批量装机时同批机器自动获得正确驱动，无需逐台设置。
- **测试**：`internal/api/driver_packages_test.go` → `TestProfileDriverPacksJSONRoundTrip`、`TestResolveDriverPacksInheritsProfile`。

---

## 4. 实现顺序（均已完成）

```
Phase 1: 借鉴项 2（✅ 已落地）→ 借鉴项 1（✅ 已实现，离线 dism 注入 boot.wim，需 Windows 主机，待真机验证）
Phase 2: 借鉴项 3（✅ 架构分离）→ 借鉴项 4/5（✅ 索引 + 验证清理，并入借鉴项 1）
         → 借鉴项 6（✅ 参数化）→ B2（✅ 按 profile 继承驱动包）
```

> 借鉴项 1 的实现要点与命令细节见 `docs/design/pxesrv-linux-winpe-borrowable.md` §2 / §3。
> **剩余工作**：见 1.3 的真机验证清单（离线注入、wimboot `index=`、`winpeshl.ini [LaunchApps]`、Recipe B `dism /Apply-Image`、SMB 认证 `System error 1727`）。

---

## 5. 与既有文档的关系

| 文档 | 视角 |
|---|---|
| `docs/windows-deployment.md` | **操作手册** — 用户怎么装（两条路线步骤、常见问题） |
| `docs/design/pxesrv-linux-winpe-borrowable.md` | **借鉴清单** — 与 pxesrv-linux 的对照、借鉴项 1–6 的 gap / 改动 / 命令 |
| `docs/design/quick-install-optimization-req.md` | **需求** — R6（Windows 链路深化）剩余范围 + 待办 B1/B2 |
| `docs/design/windows-install-hw-test-plan.md` | **真机测试清单** — 事项/步骤/预期/结果记录（验证 1.3 的待验证项） |
| 本文 | **阶段化路线图** — Phase 0/1/2 的状态、映射与排期 |
