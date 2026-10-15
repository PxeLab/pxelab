# 使用 PxeLab 网络安装 Windows

本文档说明如何通过 PxeLab 给目标机器（x64 UEFI）网络安装 Windows：从 PXE 引导到自动应答部署，全程可离线。

> 真机验证清单（事项 / 步骤 / 预期 / 结果记录）：`docs/design/windows-install-hw-test-plan.md`。

## 0. 总体机制

两条互补路线:

| 路线 | 入口 | 能力 | 依赖 |
|---|---|---|---|
| A：本地 ISO + wds Profile | OS Images → 创建 wds Profile → 绑定主机 | PXE 引导到 Windows PE，本机提供 wimboot 与所有文件 | 无（离线可用） |
| B：Catallog + 安装任务 | Install Tasks + Answer Templates | 引导 PE 并自动注入应答文件，无人值守安装 | Catalog 的 Windows PE 默认走 boot.netboot.xyz（需外网），上传本地 ISO 并“设为本地源”后即可离线 |

PXE 引导链：DHCP（PxeLab 内建，option 66/67 + iPXE 175/177）→ 按架构下发 NBP → iPXE → 菜单。

## 1. 前置条件

1. 运行中的 PxeLab（建议 x86_64 Linux 部署；开发机在 macOS/ARM64 上编译受限，见 §6）。
2. 目标机：**UEFI + 网络启动**。wimboot 是 x64 UEFI 方案，Legacy BIOS 请另用 BIOS 版 NBP。
3. DHCP：PxeLab 内建 DHCP 或 Settings → DHCP 将子网 `next-server` 指向 PxeLab 服务器 IP。
4. Windows ISO：必须包含 `boot\bootmgr`、`boot\bootmgr.efi`、`boot\boot.sdi`、`boot\bcd`、`sources\boot.wim`（标准安装 ISO 均满足）。
5. **（路线 B 无人值守）先在 Settings → SMB 启用内置只读 SMB 服务**：填共享名（Recipe B 预设用 `isos`）、共享目录（提取后的 Windows ISO 目录，需含 `sources\install.wim`）、NTLMv2 账号与密码。**Windows 主机部署时须先停用占用 445 端口的「Server」(LanmanServer) 服务**，否则 PxeLab 无法监听 SMB。

## 2. 路线 A：本地 ISO 离线引导进 PE

1. **上传 ISO**：OS Images → 上传（或“导入目录”）。
2. **提取**：点击“提取”，等待状态变为 `ready`。产物目录即 `http://<server>/boot/isos/<dir>/`。
3. **一键创建 wds Profile**：点击该镜像行的 **“创建 wds Profile”**，输入菜单名（仅英文/数字/符号，中文会乱码）。
   - 自动生成：`URL = <server>/netboot/menu/wimboot`，`WIM = <server>/boot/isos/<dir>`。
4. **绑定主机**：Hosts → 新建目标机，填 MAC / IP，Profile 选择刚创建的 wds Profile。
5. **目标机开机** → 网络启动 → PxeLab 菜单显示该条目 → wimboot 本机拉取 bootmgr/bcd/boot.sdi/boot.wim → **进入 Windows PE**。
6. **装系统**：PE 内访问 `http://<server>/boot/isos/<dir>/sources/setup.exe` 手动安装；自动安装见路线 B。
7. **收尾**：装机完成后在 Hosts 把该主机 Profile 切回“本地硬盘/默认”，避免反复重装。

## 3. 路线 B：安装任务无人值守部署

1. **建应答模板**：Answer Templates → 新建。
   - `autounattend`：占位符 `{{.ProductKey}}`、`{{.ComputerName}}`、`{{.AdminPassword}}`（由 `AnswerDataFromHost` 提供；ComputerName 默认主机名，ProductKey/AdminPassword 无运行时来源时留空，安装器会提示）。按需配置分区 / install.wim 索引 / 语言。
   - `winpeshl`：模板内容作为 PE 内 `%SYSTEMROOT%\System32\install.bat`；`winpeshl.ini` 由后端生成（内容为 `[LaunchApps]` + `%SYSTEMROOT%\System32\install.bat`，与 install.bat 同目录，winpeshl.exe 在此读取并执行），`?file=winpeshl.ini` 单独返回该包装。内置预设为 **Recipe B**：`net use Z: \\<server>\<共享名>`（PxeLab 只读 SMB 共享）→ `dism /Apply-Image /ImageFile:%SRC%\install.wim /Index:%WIMINDEX%` → `bcdboot` 写引导 → 重启，不经交互式 `setup.exe`（需先在 Settings → SMB 配置只读共享，见 §1；`install.wim` 索引由安装任务的 `wim_index` 控制）。若主机绑定了驱动包（Hosts → 驱动包），后端会在 `install.bat` 顶部、`dism` **之前**自动注入一段 **PE 阶段驱动加载**（从 `/boot/drivers/<包>/` 下载 → `pnputil /add-driver ... /install`，缺失 `pnputil` 时回退 `drvload`），确保存储/网卡等 boot-critical 驱动在 `dism` 扫描磁盘前就绪；驱动包内按 `x64/`、`x86/`、`arm64/` 子目录约定时按目标架构过滤。
2. **建安装任务**：Install Tasks → 新建 → 选主机、发行版 `Windows PE`、版本 `win11-x64`/`win10-x64`、应答模板与应答类型（autounattend / winpeshl）。任务会把应答 URL 注入该主机的引导行。
3. **引导目标机** → Catalog 菜单（`/netboot/menu.ipxe`）→ 选 Windows PE 条目 → 引导行注入应答 → PE 读取 `autounattend.xml`（或走 winpeshl 路线执行 `%SYSTEMROOT%\System32\install.bat`）自动安装。
4. **验证**：任务状态、安装日志、重启后进入系统。

> **驱动覆盖补充（Phase 1/2）**：
> - **Profile 默认驱动包**：驱动包可在 **Boot Menu（Profile）** 上配置为默认值；未显式绑定驱动包的主机会自动继承（Hosts 上的主机级绑定优先）。驱动包内 `x64/`、`x86/`、`arm64/` 子目录会按目标架构过滤。
> - **驱动交付方式**（Settings → Netboot → 驱动交付方式，`netboot.driver_delivery`）：
>   - `http`（默认）：PE 内用 `curl`/`certutil` 从 `/boot/drivers/` 下载后 `pnputil` 安装；预启动占用内存更少。
>   - `initrd`：iPXE 在启动前把每个驱动文件作为具名 initrd 注入 PE 内存盘，`install.bat` 落地后 `pnputil` 安装；**完全离线**，但预启动即传输全部驱动文件、占用更多 PE 内存，且仅对 winpeshl 路线生效。
> - **离线注入**（可选，仅 Windows 主机）：对已提取的 Windows ISO 调 `POST /api/v1/os-images/{id}/inject-drivers`（body `{"driver_packs":["<包名>"],"target":"boot","index":1}`）把驱动打进 `sources/boot.wim`，覆盖须在 PE 启动前注册 driver store 的驱动。原 `boot.wim` 先备份为 `boot.wim.orig`（不覆盖已有备份），注入结果随响应返回；`GET /api/v1/os-images/{id}/wim-info` 列出可选索引（`target` 可选 `boot`/`install`）。**非 Windows 主机返回 501**（`GET /api/v1/os-images/capabilities` 可预判），请改用上面的 PE 阶段运行时加载。

> 默认（remote-only）Windows PE 的 wimboot/文件来自 `boot.netboot.xyz`，离线网络不可用。见 §4 本地化。

## 4. 让 Catalog 的 Windows PE 走本地源（离线 + 仍支持应答注入）

已提取的本地 Windows ISO 一键替换 Catalog 中 Windows PE 条目的远端源：

1. OS Images → 提取完成（`ready`）的 Windows ISO。
2. 点击行的 **“设为本地源”**：把 `Windows PE` 条目下所有启用的 wimboot 版本改写为
   - `kernel = <server>/netboot/menu/wimboot`
   - `initrd  = <server>/boot/isos/<dir>`（即 Windows base，bootmgr/bcd/boot.sdi/boot.wim 所在目录）
3. 后端持久化到 `<data_dir>/netboot/catalog/windows.yaml` 并在内存中热重载。

效果：Catalog 菜单的 Windows PE 全部文件由本机提供（NetbootCatalog“缓存”列不再显示“需外网”），安装任务的应答注入不受影响——即 **离线 + 无人值守** 同时成立。

接口：`POST /api/v1/os-images/{id}/set-catalog-local`（读/写磁盘、热重载、记录审计）。

## 5. 推荐落地顺序

1. 上传 + 提取 Windows ISO。
2. OS Images → “创建 wds Profile”（先拿到本地可引导的 PE）。
3. “设为本地源”（让 Catalog 的 Windows PE 也离线）。
4. 写 `autounattend` 应答模板。
5. 建安装任务并绑定目标机。
6. 测试机网络启动验证，再批量。

## 6. 常见问题

- **macOS/ARM64 上 `go build ./cmd/pxelab` 报 `icon.syso: unknown ARM64 relocation type 3`**
  `cmd/pxelab/icon.syso` 是 windows/amd64 的 COFF 图标资源对象（由 `gen_icon.go` + `windres` 生成），Go 会把 `*.syso` 无差别链接进所有平台构建——本机 darwin/arm64 链接器无法解析 amd64 的 `.rsrc` 重定位。
  - 本机只交叉打 Windows 包：`GOOS=windows GOARCH=amd64 go build ./cmd/pxelab`（已验证可通过）。
  - 本机构建非 Windows 目标：构建前临时移走 `cmd/pxelab/icon.syso`（构建脚本里 mv/cp 即可）。
- **wimboot 只支持 x64 UEFI**：ARM64 Windows 与 Legacy BIOS 需走其他 NBP（如 `sbmboot`/相应固件）。
- **中文菜单名乱码**：启动菜单（pxelinux/GRUB2/iPXE 文本）渲染受限，Profile 名请用英文。
- **`GOSUMDB`**：若 Go 报 “checksum database disabled”，执行 `go env -w GOSUMDB=sum.golang.org`。
- **构建产物污染**：`web/` 前端 `npm run build` 会写入 `cmd/pxelab/webdist/`，其中 `index.html` 是跟踪文件，构建后如 `git status` 出现其变更，回归 `git checkout -- cmd/pxelab/webdist/index.html`（assets 目录已被忽略）。
- **未覆盖测试运行命令**：`go vet ./internal/... && go test ./internal/...`（`cmd/pxelab` 在本机受 §6 第一条限制，部署环境可全量）。