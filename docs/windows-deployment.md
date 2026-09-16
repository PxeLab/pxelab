# 使用 PxeLab 网络安装 Windows

本文档说明如何通过 PxeLab 给目标机器（x64 UEFI）网络安装 Windows：从 PXE 引导到自动应答部署，全程可离线。

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
   - `winpeshl`：模板内容作为 `X:\install.bat`，`winpeshl.ini` 由后端独立返回（引用 `AppPath = X:\install.bat`），在 PE 启动时执行安装脚本。
2. **建安装任务**：Install Tasks → 新建 → 选主机、发行版 `Windows PE`、版本 `win11-x64`/`win10-x64`、应答模板与应答类型（autounattend / winpeshl）。任务会把应答 URL 注入该主机的引导行。
3. **引导目标机** → Catalog 菜单（`/netboot/menu.ipxe`）→ 选 Windows PE 条目 → 引导行注入应答 → PE 读取 `autounattend.xml` 自动安装。
4. **验证**：任务状态、安装日志、重启后进入系统。

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