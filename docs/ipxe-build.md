# iPXE Build Guide

## Overview

PxeGo uses custom-compiled iPXE binaries for two-stage network boot. The embedded script in each binary forces iPXE to do its own DHCP and chain-load the boot menu via HTTP, bypassing PXE BIOS/UEFI cached DHCP data that would otherwise cause a chain-load loop.

## Prerequisites

- Linux host with:
  - `git`, `make`, `gcc`, `xz`
  - Cross-compilers for target architectures:
    - `gcc-aarch64-linux-gnu` (ARM64 UEFI)
    - `gcc-x86-64-linux-gnu` (x86 UEFI, usually included)
    - `gcc-i686-linux-gnu` (IA32 UEFI, optional)
  - Internet access to clone iPXE source

## Embedded Script

All PxeGo iPXE binaries embed the same script:

```bash
#!ipxe
dhcp || goto dhcp_failed
isset proxydhcp/next-server && goto use_proxy

:use_dhcp
set next-server ${dhcp-server}
goto chain

:use_proxy
set next-server ${proxydhcp/next-server}

:chain
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || goto tftp_fallback
exit

:tftp_fallback
chain tftp://${next-server}/boot/menu.ipxe || shell
exit

:dhcp_failed
shell
```

This script:
1. **DHCP first** — 执行 `dhcp` 获取 IP，同时接收 ProxyDHCP OFFER（如果存在）
2. **issect proxydhcp/next-server** — 检测是否存在 ProxyDHCP 数据（注意：`isset` 参数是 setting 名，不用 `${}` 包裹）
3. **Proxy 模式** — `proxydhcp/next-server` 存在 → 使用它作为 PxeGo 地址（ProxyDHCP 的 siaddr）
4. **Full/Server 模式** — 无 proxy 数据 → 使用 `${dhcp-server}`（PxeGo 自身就是 DHCP 服务器）
5. **TFTP 兜底** — HTTP chain 失败时尝试 TFTP 加载菜单
6. **DHCP 失败** — 进入 iPXE shell 手动调试

**优势**: 无需硬编码 IP、不依赖 `${next-server}` 的 scope 优先级、不依赖 PXE_STACK 编译选项。Proxy 和 Full 两种模式共用同一份脚本。

### PXE_STACK 说明

**已不再需要。** 实测发现 PXE_STACK 在 Legacy BIOS (undionly.kpxe) 下无法正确导入 ProxyDHCP 数据（Pxe ROM 将 proxy 数据存为 Option 43 子选项，PXE_STACK 读不到）。当前方案通过 iPXE `dhcp` 命令原生接收 yiaddr=0 的 ProxyDHCP OFFER 并存入 `proxydhcp` scope，无需 PXE_STACK。

### ProxyDHCP 识别条件

iPXE `dhcp_offer()` 将 OFFER 识别为 ProxyDHCP 的两个必要条件：
1. `yiaddr == 0.0.0.0` — 关键判据，表示"我不分配 IP"
2. `Option 60 == "PXEClient"` — UEFI PXE Base Code 要求响应中必须包含

PxeGo 的 `appendProxyPXEOptions()` 函数确保两者都满足，同时设置 siaddr、Option 54、Option 66、Option 43。

## Build Commands

### 1) Clone iPXE source

```bash
git clone --depth 1 https://github.com/ipxe/ipxe.git
cd ipxe/src
```

### 2) Create embedded script

```bash
cat > embedd.ipxe << "IPXE_EOF"
#!ipxe
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} && goto done
dhcp || clear
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || shell
:done
IPXE_EOF
```

### 3) Enable HTTPS

Edit `src/config/general.h`:

- Comment out `PXE_MENU` and `PXEXT` if desired.
- Uncomment or add `DOWNLOAD_PROTOCOL_HTTPS` to enable HTTPS download support:

```c
// ... comment out (optional):
// #define PXE_MENU
// #define PXEXT

// ... add or uncomment:
#define DOWNLOAD_PROTOCOL_HTTPS
```

`DOWNLOAD_PROTOCOL_HTTPS` enables iPXE to fetch kernel/initrd files from `https://github.com/...` URLs used by the netboot catalog.

### 4) Build all targets

```bash
# BIOS x86 — UNDI (uses PXE ROM network stack, no native drivers)
make bin/undionly.kpxe EMBED=embedd.ipxe

# BIOS x86 — All drivers (bigger, may have NIC-specific issues)
make bin/ipxe.pxe EMBED=embedd.ipxe

# UEFI x86-64 — SNP (uses UEFI network stack)
make bin-x86_64-efi/ipxe.efi EMBED=embedd.ipxe

# UEFI IA32
make bin-i386-efi/ipxe.efi EMBED=embedd.ipxe

# UEFI ARM64 (requires aarch64 cross-compiler)
make bin-arm64-efi/ipxe.efi EMBED=embedd.ipxe CROSS=aarch64-linux-gnu-
```

## Output Files

| Binary | Architecture | PxeGo Filename | Size |
|--------|-------------|----------------|------|
| `bin/undionly.kpxe` | BIOS x86 (UNDI) | `undionly.kpxe` | ~71KB |
| `bin/ipxe.pxe` | BIOS x86 (all drivers) | `ipxe.pxe` | ~392KB |
| `bin-x86_64-efi/ipxe.efi` | UEFI x86-64 | `ipxe.efi` | ~1.1MB |
| `bin-i386-efi/ipxe.efi` | UEFI IA32 | `ipxe32.efi` | ~1.0MB |
| `bin-arm64-efi/ipxe.efi` | UEFI ARM64 | `ipxe-arm64.efi` | ~1.2MB |

## Integration with PxeGo

Copy built binaries to two places:

```bash
# Runtime boot directory
cp bin/undionly.kpxe /path/to/pxego/boot/
cp bin-x86_64-efi/ipxe.efi /path/to/pxego/boot/
# ... etc

# Embedded bootdist (extracted on first run)
cp bin/undionly.kpxe /path/to/pxego/cmd/pxego/bootdist/
cp bin-x86_64-efi/ipxe.efi /path/to/pxego/cmd/pxego/bootdist/
# ... etc
```

Then rebuild PxeGo:

```bash
cd /path/to/pxego
go build ./cmd/pxego/
```

## Architecture Mapping

See `internal/boot/archmap.go` for the architecture-to-filename mapping.
