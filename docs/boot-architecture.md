# Boot Architecture

## Overview

PxeLab uses a two-stage network boot architecture:

```
Stage 1                    Stage 2
┌─────────┐   TFTP    ┌──────────┐   HTTP    ┌───────────┐
│ PXE ROM ├──────────►│  iPXE   ├──────────►│ Boot Menu │
│ (BIOS/  │ undionly  │ (custom │ /boot/    │ (Kernel + │
│  UEFI)  │ .kpxe/.efi│  build) │ ipxe/     │ Initrd /  │
└─────────┘           │         │ script    │ WIM /     │
                      │         │           │ Chain /   │
                      │         │           │ Local)    │
                      └──────────┘           └───────────┘
```

### Stage 1: PXE ROM → iPXE

1. Client PXE ROM sends DHCP Discover
2. PxeLab DHCP 服务器响应 Offer/Ack，包含:
   - IP 地址（full/hybrid 模式）
   - next-server（TFTP 服务器地址）
   - bootfile（NBP 文件名）
   - Option 175.178（iPXE 引导脚本 URL）
3. Client 通过 TFTP 下载 NBP（undionly.kpxe / ipxe.efi）
4. PXE ROM 加载并执行 NBP → iPXE 启动

### Stage 2: iPXE → Boot Menu

1. 自定义 iPXE 的嵌入脚本自动执行：
   ```
   #!ipxe
   dhcp || clear
   chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || shell
   ```
2. iPXE 执行 DHCP（自己做，不读 PXE 缓存）
3. PxeLab 再次响应 DHCP，包含 Option 175.178
4. iPXE 通过 HTTP 获取引导菜单脚本
5. 菜单显示配置的引导项

## Custom iPXE Build

所有 iPXE 二进制都经自定义编译：

- **禁用 PXE_STACK/PXE_MENU** — 避免读取 PXE BIOS/UEFI 缓存数据
- **嵌入引导脚本** — 启动后自动执行 dhcp + chain HTTP
- **UNDI/SNP 接口优先** — 使用 PXE/UEFI 固件网络栈，避免原生驱动兼容性问题

详见 [docs/ipxe-build.md](ipxe-build.md).

## Supported Boot Methods

### 1. iPXE (默认，完整支持)

通过自定义编译的 iPXE 二进制作为 NBP，支持以下引导类型：

| BootType | 用途 | 示例 |
|----------|------|------|
| `direct` | 直接加载内核 + initrd | Linux 发行版安装 |
| `chain` | Chain-load 其他引导器/ISO | GRUB2, Windows Boot Manager |
| `wds` | Windows WIM 引导 | Windows PE/安装 |
| `sanboot` | iSCSI SAN 启动 | 无盘工作站 |
| `local` | 从本地硬盘启动 | 跳过网络引导 |

### 2. PXELinux (SYSLINUX) — 部分实现

PXELinux 配置文件解析器位于 `internal/boot/pxelinux/`，功能包括：

- **Parser** — 解析 pxelinux.cfg 语法（default, label, kernel, append, initrd, ipappend, menu label/default, timeout, ontimout, onerror 等）
- **AST** — 抽象语法树
- **Generator** — AST → iPXE 脚本转换

**当前状态：已完成解析器和生成器，但未接入服务器 HTTP 端点。** 需要有额外的 HTTP 端点如 `GET /boot/pxelinux.cfg/{identifier}` 来提供实时翻译。

启用方式：如需要此功能，需在 `internal/httpd/server.go` 中添加 pxelinux 处理路由。

### 3. GRUB2 — 未实现

GRUB2 的网络引导（HTTP Boot / PXE）暂不支持。要支持 GRUB2：

- 需要编写 GRUB2 配置文件的解析器
- 或者直接让 iPXE chain-load GRUB2 的 boot image（`chain grub2.efi`）
- 后者更简单——GRUB2 作为独立引导器通过 `chain` 类型即可加载

## Boot File Mapping

详见 `internal/boot/archmap.go`：

| 客户端架构 | AL 码 | 引导文件 |
|-----------|-------|---------|
| BIOS x86 | 0 | undionly.kpxe |
| EFI IA32 | 6 | ipxe32.efi |
| EFI x64 | 7, 9 | ipxe.efi |
| EFI ARM64 | 11 | ipxe-arm64.efi |
| EFI RISCV64 | 21 | ipxe-riscv64.efi |

## DHCP Modes

| Mode | 行为 | 适用场景 |
|------|------|---------|
| `full` | DHCP 服务器分配 IP + PXE 选项 | 独立网络，PxeLab 作为唯一 DHCP |
| `proxy` | 仅提供 PXE 选项，不分配 IP | 现有 DHCP 环境中叠加 PXE 服务 |
| `hybrid` | PXE 客户端用 proxy，其他用 full | 混合环境，默认模式 |
