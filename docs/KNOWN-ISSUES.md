# PxeLab 已知问题清单

> 本清单用于梳理当前版本中已识别的问题。每条包含：**模块**、**严重度**、**问题描述**、**证据位置**、**影响**、**建议**。
> 严重度：`P0 阻断`（功能不可用/数据损坏）｜`P1 高`（核心流程缺陷）｜`P2 中`（体验/边界问题）｜`P3 低`（清理/优化）。

---

## 一、Windows 系统部署

### W-01 【P0】本地 Windows ISO 无引导闭环

- **模块**：`internal/osimage` + `internal/netboot`
- **问题**：OS Images 上传/提取 Windows ISO 仅完成"识别 + 落盘"，无法用于网络引导。
  - `FindKernelInitrd()`（`internal/osimage/engine.go:363`）对 `windows` 发行版无 case，落入 default 的 `casper/vmlinuz` → Windows 镜像 kernel/initrd 路径恒为空。
  - 提取目录 `/boot/isos/<id>-<name>` 之后没有任何机制生成指向本地 `bootmgr`/`boot.sdi`/`boot.wim` 的 wimboot 引导配置。
- **影响**：用户上传 Windows ISO 后（Store 或 OS Images）无法在本地启动任何 Windows 安装/PE 流程；Windows 部署只能依赖外网模板（见 W-02）。
- **建议**：为 Windows ISO 增加 `sources/boot.wim` 探测与本地 wimboot 文件映射；在本机提供 `/boot/winserver/*` 这类静态服务（替代 W-08 死代码）；自动生成 `wds` 类型 Profile 指向本地文件。
- **状态：已修复**：`FindKernelInitrd` 增加 `case "windows"`（明确返回空 kernel/initrd，由 wds 类型映射）；`POST /profiles/from-os-image` 新端点 + OSImages 页"创建 wds Profile"按钮，一键生成 `wds` Profile（URL=`http://<host>/netboot/menu/wimboot`，WIM=`http://<host>/boot/isos/<dir>`，wimboot 为项目内嵌二进制）。

### W-02 【P1】Windows PE 引导全链路依赖外网 `boot.netboot.xyz`

- **模块**：`internal/netboot/embedded/windows.yaml`
- **问题**：两个内置版本（win11-x64、win10-x64）均为 `type: wimboot` 且 `remote` 指向 `boot.netboot.xyz`。无 `local` 替代，无离线降级。
- **影响**：内网/离线实验室（PxeLab 核心场景）Windows PE 启动直接失败；无任何 UI 提示存在外网依赖。
- **建议**：提供本地镜像回退（优先 `local`，其次 `remote`），并在 Store/Profile 界面标注"需外网"。
- **状态：已修复**：NetbootCatalog 缓存列为 remote-only 版本显示"需外网"徽标（含 tooltip）；Profiles"从 OS 目录选择"、InstallTasks、HostDetail 在选中无 `local` 的版本时给出离线不可用提示；本地回退由 W-01 的"创建 wds Profile"接管。

### W-03 【P0】`wds` / `sanboot` / `custom` 引导项在 pxelinux / grub2 NBP 下被静默丢弃

- **模块**：`internal/boot/configgen`
- **问题**：`configgen` 的 `generatePXELinux()`（`pxelinux.go:10`）与 `generateGRUB2()`（`grub2.go:10`）仅输出 `direct`/`local`/`chain` 三类；其余类型直接不进入菜单，且无任何警告。
- **影响**：当 DHCP 下发的是 pxelinux 或 grub2 引导加载器时，包含 `wds`/`sanboot`/`custom` 条目的 Profile 在客户端菜单中"凭空消失"，排障困难。
- **建议**：configgen 对不支持的类型生成"不支持"占位条目，或在 Profile / 启动日志中告警指出 NBP 不兼容。
- **状态：已修复**：`generatePXELinux()`/`generateGRUB2()` 增加 default 分支，对 `wds`/`sanboot`/`custom` 输出注释 `# 需要 iPXE NBP，已跳过`，不再静默丢弃；`TestSkippedEntries` 覆盖验证。

### W-04 【P0】主机 Profile 的 iPXE 菜单对 `wds` 生成错误脚本

- **模块**：`internal/boot/ipxe`
- **问题**：主机经 `generateBootMenu()`（`internal/httpd/server.go:391`）渲染内置 `menu` 模板，其中 `wds` 分支（`internal/boot/ipxe/templates.go:37-43`）生成：
  ```
  set wds-server ${next-server}
  kernel wdsmgfw.efi
  initrd bootmgr.exe
  initrd boot.sdi
  initrd {{$entry.WIM}}
  boot
  ```
  这与 `internal/netboot/script.go:289` 的 `GenerateBootLine()`（正确的 `kernel <wimboot>` + `initrd -n bootmgr .../bootmgr` 等）**自相矛盾**。前者用 `bootmgr.exe`/`boot.sdi` 裸名（无 URL）、把 WIM 字段（实际存 win base **URL**）当 initrd 文件用。
- **影响**：同一个 `wds` Profile——从 netboot catalog（`/netboot/menu`）启动正常；从主机菜单（`/boot/ipxe/script`）启动必然失败。
- **建议**：主机菜单 `wds` 分支改为复用 `GenerateBootLine` 的 wimboot 逻辑，消除两处实现分叉。
- **状态：已修复**：`templates.go` 的 `wds` 分支重写为正确 wimboot 脚本；`server.go` 两处 `MenuEntryData` 的 `WIM` 字段统一走 `urlJoin(serverAddr, replaceBootVars(...))`。

### W-05 【P2】`wds` 编辑 UI 字段不完整且预览失准

- **模块**：`web/src/pages/Profiles.tsx`
- **问题**：
  - `wds` 类型（`Profiles.tsx:534-539`）只渲染 **WIM** 一个输入框，**URL（wimboot 位置）字段完全不可见/不可编辑**。wimboot 需要两个 URL（wimboot 二进制 + win base），UI 只给一个 → 新建 `wds` Profile 时 URL 恒为空，脚本缺失 kernel。
  - `generatePreview()` 的 `wds` 分支（`Profiles.tsx:230-238`）同样输出 `kernel wdsmgfw.efi` 风格，与实际期望的 wimboot 脚本不一致，误导用户。
- **影响**：手动创建 `wds` Profile 几乎必然失败或产生错误脚本。
- **建议**：`wds` 类型展示 `URL`（wimboot）与 `WIM`（win base）两个输入框并加示例；修正预览为 wimboot 脚本。
- **状态：已修复**：wds 表单显示 `URL` + `WIM` 两个输入框与 NBP 兼容性提示；`generatePreview()` 的 wds 分支输出与服务端一致的 wimboot 脚本。

### W-06 【P2】winpeshl 应答注入两个文件同源

- **模块**：`internal/netboot/script.go`
- **问题**：`generateWimbootLine()` 的 `winpeshl` 分支（`script.go:285-286`）中，`install.bat` 与 `winpeshl.ini` 均取 `task.AnswerURL`（同一个 URL），而 answer 端点只返回单个渲染文件 → 两者拉起相同内容，必然错误。
- **影响**：通过 winpeshl 方式自动执行 Windows PE 脚本的链路不可用。
- **建议**：answer 端点按文件名/路径区分返回内容，或为 `install.bat`/`winpeshl.ini` 分别生成 URL。
- **状态：已修复**：`script.go` winpeshl 分支中 `winpeshl.ini` 改为 `?file=winpeshl.ini`；`GetAnswerFile` 对该参数返回引用 `X:\install.bat` 的独立 INI 包装。

### W-07 【P2】autounattend 模板变量错乱

- **模块**：`internal/api/answer_templates.go` + `internal/netboot/answer.go`
- **问题**：
  - 内置 `autounattend` 模板（`answer_templates.go:405-423`）中 `ProductKey`、`AdministratorPassword`、AutoLogon 密码均渲染为 `{{.HostName | upper}}`（大写的计算机名），而模板声明的变量 `product_key` / `admin_password` **从未被使用**。
  - `AnswerDataFromHost()`（`answer.go:33`）根本不填充 `ProductKey`/`AdminPassword`/`ComputerName` 等 Windows 专用字段。
- **影响**：生成的 `autounattend.xml` 会写入错误的产品密钥/管理员密码，自动化安装无法完成，或留下非预期凭据。
- **建议**：修正模板变量映射；`AnswerDataFromHost` 从任务/主机上下文回填 Windows 字段。
- **状态：已修复**：autounattend 模板改用 `{{.ProductKey}}`/`{{.ComputerName}}`/`{{.AdminPassword}}`（与声明的变量一致）；`AnswerDataFromHost` 回填 `ComputerName=HostName`（ProductKey/AdminPassword 无运行时来源，保持为空由安装器提示）。

### W-08 【P3】`internal/wds` 为死代码

- **模块**：`internal/wds`
- **问题**：`wds.RegisterRoutes()` 定义后**从未被调用**（全局搜索无引用），注释所描述的 `/boot/wds/` 文件服务不存在于任何路由注册。
- **影响**：无（但容易让人误以为已实现）。
- **建议**：删除该包，或在实现 W-01 的本地 Windows 文件服务时复用其思路。
- **状态：已修复**：`internal/wds` 包已删除（wimboot 由内嵌 `/netboot/menu/wimboot` 提供，Windows 文件由 `/boot/isos/*` 提供，无需该包）。

---

## 二、引导类型 / NBP 兼容性（通用）

### B-01 【P1】Profile 引导类型与 NBP 兼容性无任何提示

- **模块**：`web/src/pages/Profiles.tsx`、`web/src/pages/Store.tsx`、`web/src/pages/StoreDetail.tsx`
- **问题**：前端在 Profile 创建/编辑、Store 导入时，对 6 种 `MenuEntry.Type`（`direct`/`chain`/`local`/`sanboot`/`wds`/`custom`）**没有说明文字**（tooltip/placeholder/示例），也不提示各类型所需的 NBP（iPXE/pxelinux/grub2）与必填字段。
- **影响**：用户按直觉选类型、乱填字段，容易踩中 W-03/W-04/W-05；排障成本高。
- **建议**（后续专项讨论）：
  1. 类型下拉框加 tooltip / 各类型说明文案；
  2. 各字段 placeholder 加示例；
  3. 导入成功后按类型给出 toast 引导；
  4. 长期：类型自动推断 + 一键转换（`custom`⇄`direct`）。

---

## 三、记录与后续

- 本清单由 Windows 部署专项分析产出，后续可继续追加其他模块（DHCP/TFTP/NFS/安全等）的已知问题。
- 每条修复前应先在 `docs/design/` 补充设计方案，涉及前端改动遵循 `CLAUDE.md` 组件复用与 i18n 约定。