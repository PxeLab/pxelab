# Changelog

## [Unreleased]

### Added
- iPXE 脚本生成测试套件（标签冲突检测、choose 语法验证、全发行版覆盖验证）
- Settings 页面引导文件映射表（iPXE/PXELinux/GRUB2 三栏展示架构对应关系）
- 三 NBP 引导加载器支持：iPXE（默认）、PXELinux、GRUB2
- 每个网络接口可单独选择引导加载器（Settings 页面 + 后端 API + DHCP 分发）
- 全架构自定义 iPXE 编译（BIOS/undionly.kpxe, BIOS/full ipxe.pxe, UEFI x64 ipxe.efi, UEFI IA32 ipxe32.efi, UEFI ARM64 ipxe-arm64.efi）
- DHCP 响应增加 PXE Option 43（Discovery Control），UEFI PXE 兼容性

### Fixed
- iPXE 菜单选择跳转 Bug：`choose selected && goto ${selected}` 中 `${selected}` 在解析期即被展开（值为空），导致 `goto` 落入脚本中第一个子菜单（4MLinux）。改为两行分离模式，确保变量在运行时展开
- 标签（label）特殊字符过滤：`Pop!_OS`、`Memtest86+` 等名称中的 `!`、`+`、`'` 可能导致 iPXE 解析异常，现统一替换为 `_`
- API `SettingsData.ipmi` 字段标记为可选（omitempty），兼容无 IPMI 模块的配置
- DHCP 响应缺少必填 Option 53（DHCP Message Type），导致 UEFI 固件拒绝 OFFER/ACK
- bootloaderForSubnet 指针比较 bug（循环变量副本地址 ≠ 切片元素地址），导致 bootloader 配置始终不生效
- DHCP Offer/ACK 日志缺少 bootfile 和 bootloader 字段

### Changed
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
