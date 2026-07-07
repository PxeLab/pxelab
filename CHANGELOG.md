# Changelog

## [Unreleased]

### Added
- NFSv3 服务器（基于 go-nfs），默认端口 2049/TCP，只读导出 `~/.pxego/boot/isos`
- NFS IP 访问控制：支持按 IP 或 CIDR 网段限制挂载（`allow_ips`），空列表不限制
- 内嵌 rpcbind（端口 111/UDP+TCP），自动注册 NFSv3/MOUNT 端口映射
- 版本感知 TCP 监听器：拦截 NFSv4 连接并回复 PROG_MISMATCH（low=3, high=3），强制 Linux 客户端自动回退到 v3
- NFS 设置页面（Web UI：根目录、端口、只读开关）
- NFS 服务自动启动持久化（config.yaml `service_auto_start.nfs`）
- `{{.NextServer}}` 模板变量，展开为服务器 IP（不含端口）
- NFS 日志过滤和颜色标签（Logs 页面）
- Settings → Netboot 新增「本地缓存」开关（默认启用），缓存下载的引导文件到磁盘，加快重复引导速度
- 缓存统计 API（GET /api/v1/netboot/cache-stats），显示缓存路径、文件数、磁盘占用
- Netboot 设置弹窗中缓存开启时实时显示缓存路径和磁盘占用信息
- DNS 服务启动时重新读取配置，上游 DNS 变更不再需要重启进程

### Changed
- Netboot 默认启用 HTTPS 代理（ProxyHTTPS）和本地缓存（CacheEnabled）

### Fixed
- GParted 引导失败：替换 live_endpoint 为本地 HTTP 代理地址，禁用签名校验
- 保存 netboot 设置时丢失 CatalogDisplay.Groups 配置
- 文件管理表格显示修改时间、MD5 列，带表头展示
- 后端文件列表 API 返回 MD5 哈希
- data_dir 路径校验（创建 + 可写测试）
- DNS 自动创建服务器名称 A 记录（server_name.domain.local）
- DNS 子网感知解析：根据客户端来源子网返回对应网段的服务器 IP
- TFTP 基本设置页签（URL 参数持久化）
- 设置弹窗「每页条数」字段
- DHCP 设置 / 访问控制 / OS 安装目录页签 URL 参数持久化（刷新不丢失）
- DHCP 预留管理（IP+MAC 绑定）：DB 存储 + CRUD API + 前端「预留管理」页面标签
- IP 预留冲突检测：新建/编辑时检查 IP 是否已被其他预留或活跃租约占用，前端实时提示
- IP 地址池范围显示：预留管理页面展示所选子网的可用池范围
- 子网 DHCP 模式标识：预留管理子网下拉仅显示 full 模式，带模式标签
- 首次启动自动创建默认引导配置（Boot from local disk），无需人工干预
- Settings → Netboot 新增「列出所有引导配置作为菜单项」开关，支持两种默认菜单模式
- 后端 Seed() 方法，存储层统一初始化入口

### Changed
- /files 页面移至 /settings/tftp 文件管理页签
- TFTP 引导文件映射改为三个切换页签（iPXE / PXELinux / GRUB2）
- 文件管理改为表格布局（名称 / 大小 / 修改时间 / MD5 / 删除）
- 设置弹窗保存后同步 pageSize 到 UIConfigContext
- DNS 默认 @ 记录创建跳过 loopback IP，取第一个非 loopback 接口 IP
- 路由重构：/settings/dhcp → /services/dhcp，/settings/tftp → /services/tftp，/settings/dns → /services/dns（前端路由 + 后端 API 同步）
- 导航调整：OS 安装目录从「管理」移至「设置」分组；移除 /settings 首页（重定向到 /services/dhcp）
- Toast 通知 z-index 提升至 [200]，避免被模态框遮挡
- 统一引导菜单管理：移除动态追加（AppendLocal/AppendNetboot），所有引导项通过 Profile 管理
- 每个 Profile 改为单引导项，简化创建/编辑界面（移除多条目列表，直接选择引导类型和参数）
- 默认菜单支持两种模式：仅显示默认 Profile 的引导项，或列出所有 Profile

### Removed
- 移除 ProfileBehavior 配置（append_local / append_netboot / append_position），不再需要

- 服务管理页面（Web UI 表格 + 独立启停/重启 + 批量操作 + 自动刷新）
- 服务生命周期管理后端（ServiceManager 包，支持运行时启停重启单个服务）
- 每个服务和接口的 auto_start 配置项（HTTP 默认 true，其余默认 false）
- 服务管理 API（GET /services, POST /services/{name}/start|stop|restart, POST /services/batch/{action}）
- 自动启动运行时切换（PUT /services/{name}/auto-start，持久化到 config.yaml）
- 端口/协议显示（如 "67/UDP"、"8080/TCP"），前端服务表格新增端口列
- 服务错误详情展示（error_msg 字段 + 前端模态框）
- 保护服务机制（HTTP 标记为 protected，API 拒绝启停操作，前端显示 Core 徽章且禁用勾选）
- iPXE 脚本生成测试套件（标签冲突检测、choose 语法验证、全发行版覆盖验证）
- Settings 页面引导文件映射表（iPXE/PXELinux/GRUB2 三栏展示架构对应关系）
- 三 NBP 引导加载器支持：iPXE（默认）、PXELinux、GRUB2
- 每个网络接口可单独选择引导加载器（Settings 页面 + 后端 API + DHCP 分发）
- 全架构自定义 iPXE 编译（BIOS/undionly.kpxe, BIOS/full ipxe.pxe, UEFI x64 ipxe.efi, UEFI IA32 ipxe32.efi, UEFI ARM64 ipxe-arm64.efi）
- DHCP 响应增加 PXE Option 43（Discovery Control），UEFI PXE 兼容性

### Fixed
- 配置校验白名单缺少 "undionly" 引导加载器导致启动失败
- 状态 API 服务映射改为从 ServiceManager 实时读取，移除静态 Services map
- iPXE 菜单选择跳转 Bug：`choose selected && goto ${selected}` 中 `${selected}` 在解析期即被展开（值为空），导致 `goto` 落入脚本中第一个子菜单（4MLinux）。改为两行分离模式，确保变量在运行时展开
- 标签（label）特殊字符过滤：`Pop!_OS`、`Memtest86+` 等名称中的 `!`、`+`、`'` 可能导致 iPXE 解析异常，现统一替换为 `_`
- API `SettingsData.ipmi` 字段标记为可选（omitempty），兼容无 IPMI 模块的配置
- DHCP 响应缺少必填 Option 53（DHCP Message Type），导致 UEFI 固件拒绝 OFFER/ACK
- bootloaderForSubnet 指针比较 bug（循环变量副本地址 ≠ 切片元素地址），导致 bootloader 配置始终不生效
- DHCP Offer/ACK 日志缺少 bootfile 和 bootloader 字段

### Changed
- 重构服务启动流程：使用 servicemanager 替代原 app.App，仅启动标记 auto-start 的服务
- 运行时移除 --app-mode 标志，统一使用 --mode app|server
- Settings 页面移除 IPMI 配置页签
- 将 Choose/goto 生成的脚本从 `choose selected && goto ${selected} || goto exit` 改为 `choose selected || goto exit` + `goto ${selected}` 两行模式
- InterfaceConfig 新增 Bootloader 字段（config.yaml + API + 前端的完整链路）
- NBP 映射逻辑从 boot.BootFileForArch 改为 boot.NBPFilename(arch, bootloader)
- docs/ipxe-build.md iPXE 编译指南
- DHCP Option 175.178 (iPXE boot script URL) 支持二阶段引导
- GET /boot/ipxe/script iPXE 引导菜单 HTTP 端点
- 自动检测系统网卡，接口名称下拉选择
- 多面板实时日志查看器（logbus + SSE）
- 配置热重载机制
- 嵌入 boot 文件到二进制（go:embed），单文件即可运行
- BootFileServer + 架构映射（TFTP + HTTP 统一文件服务）
- PXELinux 配置解析器（pxelinux.cfg → iPXE 脚本转换）
- DNS 服务器
- iPXE 模板引擎（menu/direct/chain/sanboot/wds/bzImage 模板）
- REST API v1（hosts, profiles, files, events, status, leases）
- IPMI 电源控制（on/off/cycle/status）
- WDS 仿真的引导流程
- 前端 SPA（React + TypeScript + Tailwind CSS）
- Settings 页面，支持配置的读取和保存
- --mode app 自动打开浏览器

### Fixed
- PXE PXE_STACK 缓存问题：自定义编译 iPXE 禁用 PXE 缓存，嵌入引导脚本
- vmxnet3 TXE:1 问题：改用 undionly.kpxe（UNDI 接口，无原生驱动）
- PXEClient 检测：改为前缀匹配（原精确匹配不识别 PXEClient:Arch:xxxxx 格式）
- IsIPXEClient 检测：改为精确匹配（原用非 PXEClient 的判断逻辑）
- DHCP Ack 缺少 yiaddr：从 ciaddr 和 Option 50 获取
- DHCP Ack 缺少 bootfile：补充启动文件/脚本 URL
- SPA fallback 路由：支持子路径刷新
- Viper 配置文件搜索路径适配 Windows
- 设置保存到配置文件实际路径而非固定 DataDir
