# PxeLab Windows 网络安装 — 真机测试验证清单

> 用途：把本仓库 Windows 网络安装相关功能（Phase 0 闭环 + Phase 1/2 完善）在**真实硬件**上逐项验证，并记录结果。
> 关联：`docs/windows-deployment.md`（操作手册）、`docs/design/windows-install-roadmap.md`（路线图/状态）。
> 填法：每个事项在 §4 结果汇总表记 **P**（通过）/ **F**（失败）/ **N/A**（不适用），失败请填证据与现象。

---

## 0. 测试环境与前置

### 0.1 环境记录（开始前填写）

| 项 | 值 |
|---|---|
| PxeLab 服务端 OS / 版本 | |
| PxeLab 版本（界面右下角或 `/api/v1/version`） | |
| 服务端是否管理员/服务身份运行 | |
| 目标机型号 / CPU / 磁盘控制器（RAID/存储型号，关键） | |
| 网卡型号（是否有有线驱动） | |
| Windows 介质版本（10 / 11 / Server / 具体 build） | |
| 引导方式（UEFI / Legacy BIOS） | |
| SMB 共享名与账号 | |

### 0.2 必做前置

- [ ] **P0-1**：`Settings → SMB` 已启用只读共享（如 `isos` → `<数据目录>/boot/isos`）并配置 NTLMv2 账号。
- [ ] **P0-2**：Windows 服务端上，若 445 被 `LanmanServer`（「Server」服务）占用，先停止该服务；确认 `netstat -ano | findstr :445` 监听者为 PxeLab。
- [ ] **P0-3**：Windows 镜像已上传并**提取**（OS Images 页状态 `ready` + `Extracted`）。
- [ ] **P0-4**：驱动包已放到 `<boot 根目录>/drivers/<包名>/`（按需含 `x64/`、`x86/`、`arm64/` 子目录）。
- [ ] **P0-5**（可选）：用规模估算工具先看体量：

  ```powershell
  # 二进制：
  .\pxelab.exe drivers stats --root <boot 根目录> --arch amd64
  # 源码：
  go run ./cmd/pxelab drivers stats --root <boot 根目录> --arch amd64
  ```

  输出示例字段：`文件` / `总大小` / `过滤后文件`（= 该架构下 **initrd 直投行数**）/ `过滤后大小`（≈ PE 预启动内存增量）。

---

## 1. 事项总览

| 编号 | 功能 | 阶段 | 需要真机 |
|---|---|---|---|
| A1 | SMB 认证连接（`net use`）可用 | Phase 0 | ✅ |
| B1 | 离线引导进 PE（本机 wimboot + 本机 Windows 文件） | Phase 0 | ✅ |
| B2 | `wimboot index=` 选择 install.wim 映像 | Phase 0 / 借鉴项 4 | ✅ |
| B3 | `winpeshl.ini [LaunchApps]` 拉起 install.bat | Phase 0 | ✅ |
| B4 | Recipe B：`dism /Apply-Image` + `bcdboot` 装完进系统 | Phase 0 | ✅ |
| C1 | 驱动交付 = `http`（PE 内下载 + pnputil） | 借鉴项 2 | ✅ |
| C2 | 驱动交付 = `initrd`（iPXE 直投 + 落地 + pnputil） | 借鉴项 2 | ✅ |
| C3 | initrd 直投的规模/内存压力 | 借鉴项 2 | ✅ |
| C4 | 平台能力探测（非 Windows 置灰） | 借鉴项 1 | 否（本地即可） |
| D1 | 离线注入：列出 WIM 索引（`wim-info`） | 借鉴项 4 | ✅（Windows 服务端） |
| D2 | 离线注入：注入驱动 + 备份 + 验证结果 | 借鉴项 1/5 | ✅ |
| D3 | 离线注入后 WIM 仍可引导、PE 认盘 | 借鉴项 1 | ✅ |
| D4 | 离线注入幂等（`.orig` 不被覆盖） | 借鉴项 5 | ✅ |
| D5 | 非 Windows 服务端返回 501 | 借鉴项 1 | 否（本地即可） |
| D6 | 还原原始 WIM（`restore-wim`） | 借鉴项 5 | ✅ |
| D7 | 并发注入互斥（返回 409） | 借鉴项 1 | 否（可用脚本并发触发） |
| E1 | 驱动包架构上报 + 按架构过滤 | 借鉴项 3 | 部分（可用 x64 真机） |
| E2 | winpeshl 预设参数化（服务器/共享/索引） | 借鉴项 6 | ✅ |
| E3 | Profile 默认驱动包继承（B2） | 待办 B2 | ✅ |
| F1 | Linux 装机路径未受影响（回归） | Phase 0 回归 | ✅（可用 VM） |

---

## 2. 逐项验证步骤

### A1 — SMB 认证连接可用

- **目的**：确认 WinPE（或 Windows 客户端）能通过 `\\<server>\<share>` 以账号认证读取 install.wim。
- **前置**：P0-1、P0-2。
- **步骤**：
  1. 进入 PE（可先做 B1）；
  2. 执行 `net use Z: \\<server>\isos /user:<账号> <密码> /persistent:no`；
  3. `dir Z:\sources\install.wim`。
- **预期**：连接成功、可见 install.wim；无 `System error 1727` / `53` / `67`。
- **记录**：结果 = 　　；报错码/现象 = 　　；证据（截图/日志）= 　　。

### B1 — 离线引导进 PE

- **目的**：本机提供全部 Windows 引导文件，离线也能进 PE。
- **前置**：P0-3。
- **步骤**：
  1. OS Images → 该 Windows 镜像 → **设为本地源**；
  2. 目标机 PXE 引导 → Netboot Catalog 菜单 → 选 Windows PE 条目；
  3. 观察 iPXE 是否从本机拉取 `wimboot / bootmgr / bcd / boot.sdi / boot.wim`。
- **预期**：进入 WinPE（出现命令行/脚本执行），NetbootCatalog「缓存」列不显示「需外网」。
- **记录**：结果 = 　　；现象 = 　　。

### B2 — `wimboot index=` 生效

- **目的**：多映像介质（如 install.wim 含 Pro/Server 两个映像）能选到指定索引。
- **前置**：B1；安装任务 `wim_index` = 2（或非 1）。
- **步骤**：
  1. 抓取 `GET /boot/ipxe/menu?mac=<目标机 MAC>`，确认 `kernel <wimboot> index=2`；
  2. PE 内（或安装过程）确认使用的是第 2 个映像（版本/名称不符即为未生效）。
- **预期**：菜单行带 `index=2`；安装的是第 2 个映像。
- **记录**：结果 = 　　；实际映像 = 　　；菜单片段 = 　　。

### B3 — `winpeshl.ini [LaunchApps]` 拉起 install.bat

- **目的**：确认 PE 启动后自动执行 `%SYSTEMROOT%\System32\install.bat`。
- **前置**：任务应答模板 `type=winpeshl`（或 overlay 版本 `answer_type=winpeshl`）。
- **步骤**：
  1. 抓取 `GET /api/v1/netboot/answer/<task_id>?file=winpeshl.ini` → 应返回 `[LaunchApps]` + `%SYSTEMROOT%\System32\install.bat`；
  2. 抓取 `GET /api/v1/netboot/answer/<task_id>` → 应返回 install.bat 内容；
  3. 菜单 wds 块应含 `initrd -n install.bat …` 与 `initrd -n winpeshl.ini …?file=winpeshl.ini`；
  4. 真机引导，PE 内确认脚本被执行（例如脚本开头加一条 `echo` 或创建标记文件）。
- **预期**：install.bat 被 `winpeshl.exe` 自动执行。
- **记录**：结果 = 　　；证据 = 　　。

### B4 — Recipe B 安装完成

- **目的**：`dism /Apply-Image` + `bcdboot` 后能重启进入系统。
- **前置**：A1、B1（目标盘已分区格式化；ESPART 存在）。
- **步骤**：
  1. 引导进入 PE 并让 install.bat 执行；
  2. 观察 `net use` → `dism /Apply-Image` → `bcdboot` → `wpeutil reboot`；
  3. 重启后确认进入已安装系统。
- **预期**：安装完成并进入系统。
- **记录**：结果 = 　　；失败步骤与输出 = 　　。

### C1 — 驱动交付 `http`（默认，回归）

- **目的**：确认默认方式未被本次改动破坏。
- **前置**：`Settings → Netboot → 驱动交付方式 = PE 内 HTTP 下载（http）`；主机/Profile 已绑定驱动包。
- **步骤**：
  1. 抓取 install.bat，确认含 `rem PxeLab PE driver load (pxelab-driver-pull)` 且含 `curl.exe`/`certutil` 下载行；
  2. 真机进 PE，在 `dism` 之前执行 `pnputil /enum-drivers`。
- **预期**：install.bat 顶部含 HTTP 驱动块；`pnputil /enum-drivers` 在 `dism` 前已列出目标驱动。
- **记录**：结果 = 　　；`pnputil` 输出片段 = 　　。

### C2 — 驱动交付 `initrd`（新）

- **目的**：确认 iPXE 直投方式端到端可用。
- **前置**：`驱动交付方式 = iPXE initrd 直投（initrd）`；主机/Profile 绑定驱动包；任务为 winpeshl 路线。
- **步骤**：
  1. 抓取 `GET /boot/ipxe/menu?mac=<MAC>`：
     - 应出现 `initrd -n install.bat …`（应答）与**每个驱动文件一行** `initrd -n pxdrv_<hash>.<ext> http://<srv>/boot/drivers/<包>/<相对路径>`；
  2. 抓取 `GET /api/v1/netboot/answer/<task_id>`（install.bat）：
     - 应含 `rem PxeLab PE driver load via iPXE initrd (pxelab-driver-pull): <包>`；
     - 应含 `copy /y "%SYSTEMROOT%\System32\pxdrv_<hash>.<ext>" "%_pxelab_drv%\<相对路径>"`；
     - **不应**含 `curl.exe` / `certutil`；
  3. **逐文件核对**：菜单里的每个 `initrd -n pxdrv_X`，在 install.bat 里都应有一条同名 `copy ...\System32\pxdrv_X`（这是关键不变量）；
  4. 真机进 PE：
     - `dir X:\Windows\System32\pxdrv_*` → 注入文件存在；
     - `dir X:\pxelab-drivers\<包>\` → 子目录结构已重建、文件已落地；
     - `pnputil /enum-drivers` → 驱动已安装（在 `dism` 之前）。
- **预期**：注入→落地→安装三步都成立；菜单与 install.bat 文件名一一对应。
- **记录**：结果 = 　　；菜单行数 = 　　；PE 内证据 = 　　。

### C3 — initrd 直投的规模/内存压力

- **目的**：摸清「文件数大」时的实际表现（**最可能出问题处**）。
- **前置**：C2；准备一个**文件较多**的真实驱动包。
- **步骤**：
  1. `pxelab drivers stats --root <boot> --arch amd64` → 记录 `initrd行` 与 `过滤后大小`；
  2. 记录菜单中该 wds 块的 `initrd` 行数与总长度；
  3. 记录 iPXE 预下载耗时（引导到 PE 出现的时间差）；
  4. 在 PE 内记录可用内存（`wmic OS get FreePhysicalMemory` 或任务管理器）。
- **预期**：能成功引导；记录实际行数/体积/耗时/内存，作为后续取舍依据（过大则改回 `http`）。
- **记录**：结果 = 　　；行数 = 　　；体积 = 　　；引导耗时 = 　　；PE 可用内存 = 　　；是否出现失败/截断 = 　　。

### C4 — 平台能力探测（无需真机）

- **目的**：非 Windows 服务端不显示可用的「注入驱动」入口。
- **步骤**：
  1. `GET /api/v1/os-images/capabilities` → 检查 `offline_driver_injection` / `platform` / `reason`；
  2. 在非 Windows 服务端打开 OS Images 页 → 「注入驱动」按钮应为**置灰**并有提示。
- **预期**：Windows → `true`；Linux/macOS → `false` 且 `reason` 指向 PE 阶段运行时加载。
- **记录**：结果 = 　　；响应 = 　　。

### D1 — 离线注入：列出 WIM 索引

- **前置**：Windows 服务端、已提取的 Windows 镜像。
- **步骤**：`GET /api/v1/os-images/{id}/wim-info?target=boot`（或 `install`）。
- **预期**：返回 `indexes` 列表（index/name/description/size）；`boot.wim` 通常为 index 1。
- **记录**：结果 = 　　；indexes = 　　。

### D2 — 离线注入：注入 + 备份 + 验证

- **前置**：D1 通过；服务端以**管理员**权限运行；目标驱动包存在。
- **步骤**：
  1. 记录注入前 `sources/boot.wim` 的大小/哈希；
  2. `POST /api/v1/os-images/{id}/inject-drivers`，body：
     `{"driver_packs":["<包名>"],"target":"boot","index":1,"recurse":true}`；
  3. 检查响应：`injected`、`target`、`index`、`backup`、`drivers`（应为 Get-WindowsDriver 结果）；
  4. 检查磁盘：`sources/boot.wim` 变大、`sources/boot.wim.orig` 生成（= 注入前内容）。
- **预期**：注入成功；响应含已验证驱动清单；原文件有 `.orig` 备份。
- **记录**：结果 = 　　；响应 = 　　；大小变化 = 　　。

### D3 — 注入后 WIM 仍可引导、PE 认盘

- **目的**：注入后 WIM 合法、wimboot 可继续读取，且目标机 PE 能看到需该驱动的磁盘。
- **步骤**：
  1. 用注入后的 `boot.wim` 引导（B1/B4 流程）；
  2. 在 PE 内确认盘符/磁盘可见（或 `dism /Apply-Image` 成功）。
- **预期**：引导正常；PE 可见磁盘并完成安装。
- **记录**：结果 = 　　；现象 = 　　。

### D4 — 离线注入幂等

- **步骤**：
  1. 记录 `boot.wim.orig` 的哈希；
  2. 再次调用 `inject-drivers`（可换 `index` 或换驱动包）；
  3. 再次检查 `boot.wim.orig` 哈希。
- **预期**：可重复注入；`.orig` **不被覆盖**（始终是最初的原始文件）。
- **记录**：结果 = 　　；orig 哈希前后 = 　　。

### D5 — 非 Windows 服务端返回 501

- **步骤**：在 Linux/macOS 服务端调用 `POST /api/v1/os-images/{id}/inject-drivers`。
- **预期**：HTTP **501**，错误信息提示改用 PE 阶段运行时驱动加载（Linux 下 `wimlib` 无法注册 driver store）。
- **记录**：结果 = 　　；状态码/信息 = 　　。

### D6 — 还原原始 WIM

- **目的**：注入可回滚（恢复到注入前的 WIM）。
- **前置**：已完成过至少一次 D2（存在 `sources/boot.wim.orig`）。
- **步骤**：
  1. 记录注入后 `boot.wim` 与 `boot.wim.orig` 的哈希；
  2. `POST /api/v1/os-images/{id}/restore-wim`（body `{"target":"boot"}`），或在 OS Images 页注入弹窗点「还原原始 WIM」；
  3. 对比 `boot.wim` 是否等于 `.orig`；检查列表中的「已注入」标记与注入次数是否清零。
- **预期**：`boot.wim` 恢复为原始内容；`.orig` 保留（可重复还原）；注入记录清零。
- **记录**：结果 = 　　；哈希对比 = 　　。

### D7 — 并发注入互斥（409）

- **目的**：同一镜像并发注入不会同时 mount/commit 而损坏 WIM。
- **步骤**：对同一镜像**几乎同时**发起两次 `inject-drivers`（例如用两个终端/循环脚本）。
- **预期**：其中一个返回 **409**（“该镜像正有离线注入/还原在进行”）；WIM 不被并发破坏；无幽灵挂载点残留（`dism /Cleanup-Wim` 后 `dism /Get-MountedWimInfo` 干净）。
- **记录**：结果 = 　　；两次响应码 = 　　。

### E1 — 驱动包架构上报 + 按架构过滤

- **步骤**：
  1. `GET /api/v1/driver-packages` → 每个包应有 `files` / `inf_files` / `size_bytes` / `archs`；
  2. 用一个含 `x64/` 与 `x86/` 双架构的包，绑定到 x64 主机，抓取 install.bat；
  3. 确认只出现 `x64/` 下的文件（以及顶层通用文件），无 `x86/`。
- **预期**：`archs` 正确；x64 主机不下发 x86 驱动。
- **记录**：结果 = 　　；archs/下载清单 = 　　。

### E2 — winpeshl 预设参数化

- **步骤**：
  1. 在 Answer Templates 用内置预设 `Windows PE install.bat (SMB media + dism apply)` 新建模板并绑定到任务；
  2. 配置 `smb.shares` 首个共享名（如 `isos`）；任务设 `wim_index`；
  3. 抓取 install.bat，确认：
     - `set "PXELAB_HOST=<渲染基址主机>"`
     - `set "PXELAB_SHARE=<首个共享名>"`（未配置为 `isos`）
     - `set "WIMINDEX=<任务 wim_index>"`（0 → 1）
     - `net use Z: \\%PXELAB_HOST%\%PXELAB_SHARE%`
  4. Answer Templates 的「预览」也可验证（可传 `wim_index`/`smb_share`/`smb_server` 覆盖）。
- **预期**：均为渲染值，无残留 `{{…}}`。
- **记录**：结果 = 　　；关键行 = 　　。

### E3 — Profile 默认驱动包继承（B2）

- **步骤**：
  1. Profiles → 编辑某 Boot Menu → 勾选「默认驱动包」（如 `nic-intel`），保存；
  2. 让一台**未绑定驱动包**的主机使用该 Profile，触发 install.bat；
  3. 确认 install.bat 注入了该驱动块；
  4. 再给该主机**显式绑定**另一个驱动包，确认主机级绑定优先。
- **预期**：未绑定时继承 Profile；绑定时主机优先。
- **记录**：结果 = 　　；两轮注入的包名 = 　　。

### F1 — Linux 装机回归

- **步骤**：用一个 Linux 发行版（kickstart/preseed 任一）走一遍无人值守。
- **预期**：不受 Windows 相关改动影响，正常注入应答并安装。
- **记录**：结果 = 　　；现象 = 　　。

---

## 3. 已知风险与排查提示（预期可能踩的坑）

| 现象 | 可能原因 | 排查 |
|---|---|---|
| SMB 连接 `System error 1727` | WinPE 24H2 的认证 SMB 缺陷（KB5051987），修于 KB5058411（26100.4061） | 换 Win10/Server 或更新 PE 补丁；确认账号正确 |
| SMB `System error 53/67` | 445 未监听 / 被 LanmanServer 占用 / 共享名错 | `netstat -ano \| findstr :445`；停 Server 服务；核对共享名 |
| PE 内 `net use` 提示端口被拒 | 服务端非 Windows 或服务未启动 | 检查 `Settings → SMB` 启用状态与服务状态 |
| `dism /Apply-Image` 报「找不到 install.wim」 | 共享名/PXELAB_SHARE 不匹配，或 sources 路径不同 | 核对 E2 渲染出的共享名；`dir Z:\sources` |
| initrd 模式引导变慢/失败、驱动不生效 | 文件过多、注入行数过大、或注入名不一致 | 用 C3 记录规模；逐文件核对菜单 vs install.bat（C2 第 3 步）；必要时改回 `http` |
| `inject-drivers` 报权限/COM 错误 | 服务端非管理员运行；WIM 被占用 | 以管理员/服务身份运行；确认无其他进程挂载该 WIM |
| `wim-info`/注入返回 501 | 服务端非 Windows | 这是预期行为，改用 PE 阶段运行时加载 |

---

## 4. 结果汇总（测试时填写）

| 编号 | 功能 | 结果 (P/F/NA) | 证据（截图/日志/命令输出） | 备注 |
|---|---|---|---|---|
| P0-1..5 | 前置检查 | | | |
| A1 | SMB 认证连接 | | | |
| B1 | 离线引导进 PE | | | |
| B2 | wimboot `index=` | | | |
| B3 | `[LaunchApps]` 拉起 install.bat | | | |
| B4 | Recipe B 装完进系统 | | | |
| C1 | 驱动交付 `http` | | | |
| C2 | 驱动交付 `initrd` | | | |
| C3 | initrd 规模/内存压力 | | | |
| C4 | 平台能力探测 | | | |
| D1 | 列出 WIM 索引 | | | |
| D2 | 注入 + 备份 + 验证 | | | |
| D3 | 注入后仍可引导/认盘 | | | |
| D4 | 注入幂等（.orig 不覆盖） | | | |
| D5 | 非 Windows 返回 501 | | | |
| D6 | 还原原始 WIM | | | |
| D7 | 并发注入互斥(409) | | | |
| E1 | 架构上报 + 过滤 | | | |
| E2 | 预设参数化 | | | |
| E3 | Profile 驱动包继承 | | | |
| F1 | Linux 装机回归 | | | |

### 4.1 汇总结论

- 通过 / 失败 / 不适用：　　/　　/　　
- 阻塞问题（必须修复）：　　（附编号）
- 非阻塞问题（可后续）：　　（附编号）
- 测试人 / 日期：　　/　　
