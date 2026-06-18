# Changelog

## [Unreleased]

### Added
- 全架构自定义 iPXE 编译（BIOS/undionly.kpxe, BIOS/full ipxe.pxe, UEFI x64 ipxe.efi, UEFI IA32 ipxe32.efi, UEFI ARM64 ipxe-arm64.efi）
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
