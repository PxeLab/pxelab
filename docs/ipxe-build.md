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

All PxeGo iPXE binaries embed the same script (`embedd.ipxe`):

```bash
#!ipxe
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} && goto done
dhcp || clear
chain http://${proxydhcp/next-server}:8080/boot/ipxe/script?mac=${net0/mac} && goto done
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || shell
:done
```

This script:
1. **Try PXE ROM cached next-server** — 直接用 PXE ROM 缓存的 `${next-server}`（在 Proxy DHCP 环境下为 PxeGo IP）直连 HTTP 脚本，成功则跳过
2. **DHCP fallback** — 第 1 步失败时执行 `dhcp` 获取真实 IP 和网络配置
3. **Try proxyDHCP next-server** — 使用 `proxydhcp/next-server`（ProxyDHCP 响应的 siaddr，即 PxeGo IP）重试 HTTP
4. **Fallback to DHCP next-server** — 最后尝试使用 DHCP 分配的 `${next-server}`（Full DHCP 模式可用）
5. `:done` — 成功跳转标签

**优势**: Proxy DHCP 环境下，第 1 步或第 3 步都能命中 PxeGo，无需硬编码 IP；Full DHCP 下第 4 步同样有效。三层回退覆盖所有网络拓扑。

### PXE_STACK 说明

`PXE_STACK` 在 iPXE 编译时必须启用（见第 3 步配置）。启用后 iPXE 能继承 PXE ROM 缓存的 ProxyDHCP 响应，使得 `${next-server}` 在 `dhcp` 命令后仍能通过 `${proxydhcp/next-server}` 获取到代理服务器的正确 IP。

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

### 3) Enable PXE_STACK and HTTPS

Edit `src/config/general.h`:

- **Enable** `PXE_STACK` — allows iPXE to inherit the proxy DHCP `next-server` from PXE ROM cache, critical for proxy DHCP mode where `${next-server}` must resolve to PxeGo's IP (not the real DHCP server).
- Comment out `PXE_MENU` and `PXEXT` if desired, though they are generally harmless.
- Uncomment or add `DOWNLOAD_PROTOCOL_HTTPS` to enable HTTPS download support:

```c
// ... keep PXE_STACK enabled:
#define PXE_STACK		/* PXE stack in iPXE - you want this! */

// ... comment out (optional):
// #define PXE_MENU
// #define PXEXT

// ... add or uncomment:
#define DOWNLOAD_PROTOCOL_HTTPS
```

`PXE_STACK` enables iPXE to cache the proxy DHCP response, so `${next-server}` resolves to PxeGo's IP even after iPXE performs its own `dhcp` command. The embedded script also tries `${proxydhcp/next-server}` as a fallback.

This also enables iPXE to fetch kernel/initrd files from `https://github.com/...` URLs used by the netboot catalog.

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
