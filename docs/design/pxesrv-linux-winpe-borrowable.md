# pxesrv-linux WinPE 方案对比与可借鉴项

> 来源：2026-10 对 `D:\NewCB\pxesrv-linux`（freddan88/pxesrv-linux）WinPE 方案的逐文件分析，对照 PxeLab 现有 wimboot 实现。
> 本文是**对比 + 可借鉴清单**，与 `docs/design/quick-install-optimization-req.md` 的「后续待办 B1/B2」互为补充（B1 即本文借鉴项 1）。
> 结论先行：pxesrv-linux 唯一真正值得借鉴的是**离线驱动注入（dism /add-driver 打进 boot.wim）**；其引导方式（memdisk+ISO）、文件传输（SMB）、安装执行（交互式 setup.exe）均**不应借鉴**。

---

## 0. 双方现状速览

### pxesrv-linux（WinPE 相关文件）

| 文件 | 作用 |
|---|---|
| `winpe/winpe_x64.cmd` / `winpe_x86.cmd` | 离线构建 WinPE ISO：`dism /mount-wim` → `xcopy` 工具 → 替换 `startnet.cmd` → `dism /unmount-wim /commit` → `oscdimg` 出 ISO |
| `winpe/drivers/install_64.bat` / `install_32.bat` | 遍历 `winpe/drivers/{64,32}/*.inf`，逐个 `dism /add-driver /image:<mount> /driver:<inf>`，再 `dism /get-drivers` 验证 |
| `winpe/startnet/winpe_startnet_{x64,x86}.txt` | 注入为 PE 的 `startnet.cmd`：`wpeinit` → `net use P: \\<ip>\pxe /user:pxesrv pxesrv` → 执行 `P:\pxe-boot\share\autoexec_winpe_*.cmd` |
| `files/pxe/pxe-boot/share/autoexec_winpe_{x64,x86}.cmd` | 直接运行 `P:\...\windows\win7_64\setup.exe`（交互式安装） |
| `files/pxe/pxe-boot/tftp/pxelinux.cfg/{default,windows_menu.cfg}` | 静态菜单：`linux memdisk` + `append iso initrd=http://<ip>:70/winpe/winpe_x64.iso`（整 ISO 载入内存） |
| `files/pxe/configs/samba_share.txt` | SMB 共享 `[pxe]`，`net use` 挂载用 |

### PxeLab（现有 wimboot 实现，关键锚点）

| 能力 | 位置 |
|---|---|
| wimboot 引导行渲染 | `internal/netboot/script.go` → `generateWimbootLine()`（`kernel <wimboot>` + `initrd bootmgr/bootmgr.efi/boot/bcd/boot.sdi/sources/boot.wim`） |
| 内嵌 wimboot 二进制 | `internal/netboot/menus/wimboot`（66560B），服务路径 `/netboot/menu/wimboot` |
| 主机菜单 wds 分支 | `internal/boot/ipxe/templates.go`（`wds` 类型 → wimboot 脚本） |
| 本地 ISO 提取 | `internal/osimage/engine.go` → `ExtractISO()`（Windows 走 PowerShell `Mount-DiskImage`+`Copy-Item`；Linux 走 `7z`） |
| 一键 wds Profile | `internal/api/profiles.go` → `CreateFromOSImage()`；`internal/api/os_images.go` → `SetCatalogLocal()` |
| 驱动包存储约定 | `boot/drivers/<名称>/`（`internal/api/driver_packages.go` → `listDriverPackFiles()`） |
| 首登驱动注入 | `internal/api/driver_packages.go` → `buildDriverInstallCommand()`（`pnputil /add-driver ... /subdirs /install`） |
| 首登编排注入点 | `internal/api/baseline_pull_scripts.go` → `augmentAutoUnattendFirstLogon()`（autounattend `FirstLogonCommands`，Order 1..N 驱动 + N+1 基线） |
| winpeshl 应答 | `internal/api/install_tasks.go` → `GetAnswerFile()`（模板作 `install.bat`，`?file=winpeshl.ini` 返回 `[LaunchApps]` + `%SYSTEMROOT%\System32\install.bat` 的独立 INI 包装） |
| 应答文件传输 | iPXE `initrd` 拉取 `http://<server>/api/v1/netboot/answer/<task_id>` |

---

## 1. 对比表

| 维度 | pxesrv-linux | PxeLab（PxeGo） |
|---|---|---|
| **WinPE 引导方式** | memdisk + 整 ISO 载入内存（`append iso initrd=.../winpe_x64.iso`） | wimboot 流式（`kernel <wimboot>` + `initrd bootmgr/bcd/boot.sdi/boot.wim`），不整 ISO 入内存 |
| **WinPE 镜像构建** | 离线脚本：Windows AIK + dism 挂载 WIM → xcopy 工具 → 替换 `startnet.cmd` → unmount /commit → oscdimg 出 ISO | 无构建；直接提取 Windows ISO 的 `bootmgr/bcd/boot.sdi/boot.wim`，wimboot 引导 |
| **驱动注入时机** | **离线**：`dism /add-driver` 打进挂载的 `boot.wim`（PE 启动前） | **首登**：autounattend `FirstLogonCommands` → `pnputil /add-driver`（系统装完后） |
| **驱动注入命令** | `dism /add-driver /image:<mount> /driver:<inf>` | `pnputil /add-driver ($dest + '\*.inf') /subdirs /install` |
| **驱动包目录约定** | `winpe/drivers/32`、`winpe/drivers/64`（**按架构分**） | `boot/drivers/<名称>/`（**架构无关**） |
| **PE 入口钩子** | `startnet.cmd`（`wpeinit` → `net use` → `autoexec_winpe_*.cmd`） | `winpeshl.ini`（`[LaunchApps]` → `%SYSTEMROOT%\System32\install.bat`）或 `autounattend.xml` |
| **文件传输** | SMB/CIFS（`net use P: \\server\pxe /user:pxesrv pxesrv`） | HTTP（iPXE `initrd` 拉应答；驱动走 `/boot/drivers/`） |
| **安装执行** | 交互式 `setup.exe`（autoexec 里直接跑） | 无人值守：`autounattend.xml` 或 winpeshl `install.bat` |
| **架构支持** | x86 + x64（各自独立 ISO/脚本） | 仅 x64（wimboot 限制） |
| **菜单** | 静态 `pxelinux.cfg`（硬编码 IP/LABEL） | 动态生成（catalog/DB → iPXE 菜单） |
| **驱动验证/清理** | `dism /get-drivers`、`dism /cleanup-wim` | 无 |
| **WIM 索引处理** | 固定 `index:1` | 无 WIM 概念 |

---

## 2. 可借鉴项

### 借鉴项 1【P1 / L】离线 boot-critical 驱动注入（dism /add-driver 打进 boot.wim）

- **Gap（现状）**：PxeLab 仅在**首登**注入驱动（`augmentAutoUnattendFirstLogon` → `buildDriverInstallCommand` → `pnputil`）。装系统阶段就缺的驱动（RAID/存储控制器、部分网卡）会让 `setup.exe` 认不到盘，**走不到首登**。`internal/osimage/engine.go` 无任何 WIM 挂载/注入能力（`ExtractISO` 仅复制文件）。
- **最小改动（文件 + 函数）**：
  - 新增 `internal/osimage/wim.go`（平台无关接口）+ `internal/osimage/wim_windows.go`（dism 实现）+ `internal/osimage/wim_other.go`（wimlib 或返回「不支持」）：
    - `MountWIM(wimPath, mountDir string, index int) error`
    - `AddDriverToWIM(mountDir, driverDir string, recurse bool) error`
    - `UnmountWIM(mountDir string, commit bool) error`
    - `ListWIMDrivers(mountDir string) ([]string, error)`
    - `CleanupWIM() error`
    - `GetWIMInfo(wimPath string) ([]WIMIndex, error)`
  - 新增 `internal/api/wim_inject.go`：`(*OSImageHandler) InjectDrivers(w, r)` → `POST /api/v1/os-images/{id}/inject-drivers`（body：`{driver_packs:[], target:"boot"|"install", index:N}`）；在 `internal/api/handler.go` 注册。
  - `internal/models/os_image.go` 增字段 `InjectedDrivers string`（JSON 列表）或新增 `models.WimInjection` 表记录注入历史。
  - 注入产出**新副本**（原 `sources/boot.wim` 备份为 `boot.wim.orig`），`SetCatalogLocal`/`CreateFromOSImage` 指向注入后的副本。
- **实现要点**：
  - dism 路径：优先 `dism.exe`（PATH），回退 `%SystemRoot%\System32\Dism\dism.exe`；用 `exec.Command` + 已有 `hideWindow`（`engine_windows.go`）。
  - 挂载：`dism /mount-wim /wimfile:<copy> /index:1 /mountdir:<tmp>`；mountdir 必须为空目录，放 `os.TempDir()` 下。
  - 注入：对每个驱动包 `dism /image:<mount> /add-driver /driver:<pkgdir> /recurse`。
  - 卸载：`dism /unmount-wim /mountdir:<tmp> /commit`；失败时 `/discard` 并 `dism /cleanup-wim`。
  - **安全**：绝不改原文件（先 copy）；串行化注入（dism 并发挂载有限）；崩溃残留用 `/cleanup-wim` 恢复；mountdir 用 `defer` 清理。
  - **Linux 主机限制**：`wimlib-imagex` 只能文件级注入，**无法注册驱动到 driver store**，WinPE 不会加载 → 离线注入实际需要 Windows 主机（或 Windows 辅助容器）。Linux 上降级为「仅文件级 + 文档说明」或走借鉴项 2。
- **验收标准**：
  - 注入后 `dism /get-drivers /image:<mount>` 列出该驱动。
  - 目标机磁盘控制器需该驱动时，PE 内 `setup.exe` 能看到磁盘并完成安装。
  - 原 `boot.wim` 未被修改（有备份）。
- **工作量**：L
- **优先级**：P1（即 `quick-install-optimization-req.md` 的 B1）

### 借鉴项 2【P1 / M】PE 阶段运行时驱动加载（PxeGo 原生替代）

- **Gap（现状）**：同借鉴项 1，但**无需 WIM 操作**即可覆盖多数 boot-critical 场景，且跨平台。
- **最小改动（文件 + 函数）**：
  - 扩展 `internal/api/driver_packages.go` 的 `buildDriverInstallCommand()`，新增 PE 阶段变体（或复用同一命令）供 winpeshl `install.bat` 使用。
  - 在 `internal/api/answer_templates.go` 的 winpeshl 预设里，在 `setup.exe` 之前插入驱动下载 + `pnputil /add-driver ... /install`（或 `drvload`）。
  - 或在 `internal/netboot/script.go` 的 winpeshl 分支注入一个内置 `install.bat` 模板（下载驱动 → 装驱动 → 跑 `setup.exe`）。
- **实现要点**：
  - WinPE 3.0+ 自带 `pnputil`；`drvload <inf>` 是 WinPE 专用、更轻量。
  - 必须在 `setup.exe` **之前**执行。
  - HTTP 拉取已有（`/boot/drivers/`）。
  - 与首登注入互补：PE 阶段装 boot-critical，首登装其余。
- **验收标准**：绑定驱动包的 PE 内，`pnputil /enum-drivers` 在 `setup.exe` 前已列出该驱动；需该驱动的机器能装。
- **工作量**：M
- **优先级**：P1

### 借鉴项 3【P2 / S】驱动包按架构分离

- **Gap（现状）**：`boot/drivers/<名称>/` 架构无关，`listDriverPackFiles()` 不按架构过滤；x86/x64 `.inf` 混放会导致 `pnputil` 失败或装错。
- **最小改动（文件 + 函数）**：约定扩展为 `boot/drivers/<名称>/<arch>/`（arch ∈ `x64`/`x86`/`arm64`）；`listDriverPackFiles()`/`buildDriverCommands()` 增加 arch 参数，按目标架构过滤；`DriverPackageHandler.List()` 返回各包可用架构。
- **实现要点**：arch 来源 `models.OSImage.Arch` 或 catalog version arch；`pnputil /subdirs` 会递归但架构不匹配仍失败。
- **验收标准**：x64 主机只下发 x64 驱动。
- **工作量**：S
- **优先级**：P2

### 借鉴项 4【P2 / S】WIM 索引感知

- **Gap（现状）**：PxeLab 无 WIM 索引概念；`boot.wim` index 1=WinPE、2=Setup；`install.wim` 每版本一个 index。注入错 index 无效。
- **最小改动（文件 + 函数）**：注入 API 增 `index` 参数；`GetWIMInfo()` 用 `dism /get-wiminfo` 列出；默认 `boot.wim` index 1。
- **验收标准**：注入命中正确 index；`dism /get-wiminfo` 可见。
- **工作量**：S（并入借鉴项 1）
- **优先级**：P2

### 借鉴项 5【P2 / S】注入验证与清理

- **Gap（现状）**：无 `dism /get-drivers` 验证、无 `dism /cleanup-wim` 清理。
- **最小改动（文件 + 函数）**：`ListWIMDrivers()` + `CleanupWIM()`；UI 展示已注入驱动；每次挂载操作后 `defer` cleanup。
- **验收标准**：UI 显示注入驱动清单；失败后无残留挂载。
- **工作量**：S
- **优先级**：P2

### 借鉴项 6【P3 / S】内置 winpeshl「跑 setup.exe」预设

- **Gap（现状）**：PxeLab winpeshl 模板需用户自写；无内置「跑 `setup.exe` + 应答」预设。
- **最小改动（文件 + 函数）**：`internal/api/answer_templates.go` 的 `Presets` 增 winpeshl 预设：`setup.exe /unattend:X:\autounattend.xml`（或从 HTTP 拉）。
- **验收标准**：选该预设即可无人值守安装。
- **工作量**：S
- **优先级**：P3

---

## 3. 具体命令与参数（Windows ADK/dism 语境）

> 注意：pxesrv-linux 用的是 **Windows AIK**（Win7 时代，已停更）。现代应使用 **Windows ADK**（含 `dism`、`oscdimg`）。dism 命令本身兼容。

### 3.1 离线注入（Windows 主机）

```cmd
:: 查看 WIM 索引（boot.wim: 1=WinPE, 2=Setup；install.wim: 每版本一个 index）
dism /get-wiminfo /wimfile:C:\win\sources\boot.wim
dism /get-wiminfo /wimfile:C:\win\sources\install.wim

:: 挂载（读写）—— index 1 = WinPE
dism /mount-wim /wimfile:C:\win\sources\boot.wim /index:1 /mountdir:C:\mount

:: 注入驱动（递归一个驱动包目录）
dism /image:C:\mount /add-driver /driver:C:\drivers\raid /recurse
dism /image:C:\mount /add-driver /driver:C:\drivers\nic\*.inf /recurse

:: 验证
dism /image:C:\mount /get-drivers

:: 提交
dism /unmount-wim /mountdir:C:\mount /commit

:: 清理残留挂载
dism /cleanup-wim
```

**参数要点**：

| 参数 | 说明 |
|---|---|
| `/index:N` | WIM 索引。`boot.wim`：1=WinPE（wimboot 引导的就是它）、2=Windows Setup；`install.wim`：每版本一个 index |
| `/recurse` | 递归子目录找 `.inf` |
| `/forceunsigned` | 允许未签名驱动（**仅 WinPE**，有安全风险，需文档标注） |
| `/commit` / `/discard` | 提交或丢弃挂载改动 |
| `/mountdir` | 必须为空目录；dism 拒绝重复挂载 |
| `/readonly` | 只读挂载（仅查看） |
| `/optimize` | 卸载时优化 WIM |
| `/checkintegrity` | 校验 WIM 完整性 |
| `/compress:max` | 卸载时最大压缩（boot.wim 建议默认 fast，压缩越大 PE 解压越慢） |

### 3.2 Linux 主机（wimlib-imagex）

```bash
wimlib-imagex info boot.wim
wimlib-imagex mountrw boot.wim 1 /mnt/wim
# 手动复制驱动文件 —— 注意：不会注册到 driver store
wimlib-imagex unmount /mnt/wim --commit
```

**关键 caveat**：wimlib **无法执行 DISM 的 driver-store 注册**，仅文件级复制不足以让 WinPE 加载驱动。真正的离线驱动注入**实际需要 Windows 主机**（或 Windows 辅助容器/VM）。Linux 上应降级为借鉴项 2（PE 阶段运行时加载）。

### 3.3 PE 阶段运行时加载（PxeGo 原生，借鉴项 2）

```cmd
:: 在 WinPE 的 winpeshl install.bat 中，setup.exe 之前：
pnputil /add-driver C:\drivers\*.inf /subdirs /install
:: 或（WinPE 专用、更轻量）
drvload C:\drivers\raid\iaStorAC.inf
```

### 3.4 关键 caveat 汇总

- **WIM 索引**：`boot.wim` index 1 = WinPE（wimboot 引导的正是它）；index 2 = Windows Setup。**boot-critical 驱动注入 index 1**；装完系统要用的驱动注入 `install.wim`（所有版本）或走首登 `pnputil`。
- **架构**：`boot.wim` 必须与 PE 架构一致（x64）。x86 驱动不会在 x64 PE 加载。**wimboot 仅 x64**。
- **wimboot 兼容性**：wimboot 流式读取 `boot.wim`（默认 index 1）。dism `/commit` 产出的 WIM 仍是合法 WIM，wimboot 可正常读取。注入会增大 `boot.wim`，增加 HTTP 传输时间（仍是流式，不整 ISO 入内存）。**若注入 index 2，wimboot 仍引导 index 1 → 对 PE 无效**。
- **驱动签名**：未签名驱动需 `/forceunsigned`（WinPE），有安全风险，需文档标注。
- **挂载目录**：必须为空；dism 拒绝重复挂载；崩溃残留用 `/cleanup-wim` 恢复。
- **并发挂载**：dism 并发挂载数有限，注入操作应**串行化**。
- **原文件保护**：始终先 copy 再挂载（pxesrv-linux 是 move 后原地改；PxeLab 应产出副本）。
- **install.wim 体积**：注入 `install.wim` 可能数 GB，建议 boot-critical 只打 `boot.wim`，其余走首登 `pnputil`。

---

## 4. 不要借鉴（Do NOT borrow）

| 项 | pxesrv-linux 做法 | 不借鉴理由 |
|---|---|---|
| **memdisk + 整 ISO 引导** | `linux memdisk` + `append iso initrd=http://.../winpe_x64.iso` | 整 ISO 载入内存：慢、吃 RAM、不随镜像增大而扩展。PxeLab 的 wimboot 流式拉取 `bootmgr/bcd/boot.sdi/boot.wim`，更优。 |
| **SMB/CIFS 共享 + 交互式 setup.exe** | `startnet.cmd` 里 `net use P: \\server\pxe /user:pxesrv pxesrv`，再跑 `setup.exe` | 需额外 SMB 服务与凭据；交互式安装破坏自动化。PxeLab 用 HTTP（无需 SMB）+ autounattend/winpeshl 无人值守。 |
| **静态菜单 / 硬编码 IP** | `pxelinux.cfg` 硬编码 `192.168.0.254` 与静态 LABEL | PxeLab 菜单由 catalog/DB 动态生成，支持多架构/多版本/主机绑定。 |
| **Windows AIK 依赖** | 脚本依赖 `%ProgramFiles%\Windows AIK\...` | AIK 已停更（Win7 时代）。应使用 Windows ADK（`dism`/`oscdimg`）。 |
| **x86（32 位）WinPE** | 独立 x86 ISO + `autoexec_winpe_x86.cmd` | wimboot 仅 x64；x86 需另配 NBP（如 memdisk/sbmboot）。除非引入非 wimboot NBP，否则不做。 |
| **`startnet.cmd` 作为 PE 入口** | 用 `startnet.cmd` 做入口钩子 | PxeLab 用 `winpeshl.ini`（`[LaunchApps]` → `%SYSTEMROOT%\System32\install.bat`），更适合自动化；`startnet.cmd` 是回退 shell。无需改。 |
| **无错误处理的脚本风格** | `pause` 驱动、fire-and-forget、无退出码检查 | 借鉴其 dism 命令，但**不借鉴**其无错误处理风格；PxeLab 需结构化错误 + 清理。 |

---

## 5. 落地建议顺序

```
借鉴项 2（PE 阶段运行时加载，M）  ← 低风险、跨平台、覆盖多数 boot-critical，先做
借鉴项 1（离线 dism 注入，L）      ← 生产级 Windows 部署的完整方案（= B1）
借鉴项 3（按架构分离，S）          ← 与 1/2 配套
借鉴项 4/5（索引 + 验证清理，S）   ← 并入借鉴项 1
借鉴项 6（winpeshl 预设，S）       ← 体验优化，随时可插
```

> 与 `quick-install-optimization-req.md` 的 B1/B2 关系：本文借鉴项 1 = B1；借鉴项 3 是 B2（按 profile/OS 版本绑定默认驱动包）的前置（架构过滤）。
