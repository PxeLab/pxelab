# iPXE Build Guide

## Overview

PxeLab uses custom-compiled iPXE binaries for two-stage network booting. Each binary embeds the same iPXE script that performs DHCP then loads the boot menu via HTTP, avoiding PXE BIOS/UEFI DHCP data caching issues that cause chain-load loops.

## Build Environment

- Linux host with:
  - `git`, `make`, `gcc`, `xz`
  - Cross-compilers for target architectures:
    - `gcc-aarch64-linux-gnu` (ARM64 UEFI)
    - `gcc-x86-64-linux-gnu` (x86 UEFI, usually built-in)
    - `gcc-i686-linux-gnu` (IA32 UEFI, optional)
  - Network access (to clone iPXE source)

## Embedded Script

All PxeLab iPXE binaries embed the same script:

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

Script logic:

1. **DHCP first** — Runs `dhcp` to get IP, also receives ProxyDHCP OFFER if present
2. **isset proxydhcp/next-server** — Checks if ProxyDHCP data exists (note: `isset` parameter is the setting name, don't wrap with `${}`)
3. **Proxy mode** — `proxydhcp/next-server` exists → use it as PxeLab address (the siaddr field from ProxyDHCP)
4. **Full/Server mode** — No proxy data → use `${dhcp-server}` (PxeLab itself is the DHCP server)
5. **TFTP fallback** — Falls back to TFTP if HTTP chain-load fails
6. **DHCP failure** — Drops to iPXE shell for manual debugging

**Advantage**: No hardcoded IPs, no dependency on `${next-server}` scope priority, no dependency on `PXE_STACK` compile option. Both Proxy and Full modes share the same script.

### PXE_STACK Note

**No longer needed.** Testing revealed PXE_STACK cannot properly import ProxyDHCP data in Legacy BIOS (undionly.kpxe) — PXE ROM stores proxy data as Option 43 sub-options that `PXE_STACK` can't read. The current approach uses iPXE's `dhcp` command to natively receive `yiaddr=0` ProxyDHCP OFFERs and store them in the `proxydhcp` scope, without needing `PXE_STACK`.

### ProxyDHCP Identification

iPXE's `dhcp_offer()` identifies an OFFER as ProxyDHCP with two required conditions:

1. **`yiaddr == 0.0.0.0`** — Key indicator meaning "not assigning an IP"
2. **Option 60 = `"PXEClient"`** — UEFI PXE Base Code requires this option to be echoed back

PxeLab's `appendProxyPXEOptions()` function ensures both are satisfied, along with setting siaddr, Option 54, Option 66, and Option 43.

## Build Commands

### 1) Clone iPXE Source

```bash
git clone --depth 1 https://github.com/ipxe/ipxe.git
cd ipxe/src
```

### 2) Create Embedded Script

```bash
cat > embedd.ipxe << "IPXE_EOF"
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
IPXE_EOF
```

### 3) Enable HTTPS

Edit `src/config/general.h`, uncomment or add:

```c
#define DOWNLOAD_PROTOCOL_HTTPS
```

`DOWNLOAD_PROTOCOL_HTTPS` enables iPXE to download kernels and initrds from HTTPS addresses referenced in the netboot catalog (e.g., `https://github.com/...`).

> Other config options (`PXE_MENU`, `PXEXT`, `PXE_STACK`, etc.) don't need changes — defaults work fine. `PXE_STACK` is no longer required (see above).

### 4) Build All Targets

```bash
# BIOS x86 — UNDI (uses PXE ROM network stack, no native NIC drivers)
make bin/undionly.kpxe EMBED=embedd.ipxe

# BIOS x86 — Full drivers (larger, some NIC compatibility issues)
make bin/ipxe.pxe EMBED=embedd.ipxe

# UEFI x86-64 — SNP (uses UEFI network stack)
make bin-x86_64-efi/ipxe.efi EMBED=embedd.ipxe

# UEFI IA32
make bin-i386-efi/ipxe.efi EMBED=embedd.ipxe

# UEFI ARM64 (requires aarch64 cross-compiler)
make bin-arm64-efi/ipxe.efi EMBED=embedd.ipxe CROSS=aarch64-linux-gnu-
```

## Output Files

| Build Artifact | Architecture | PxeLab Filename | Size |
|---------------|-------------|-----------------|------|
| `bin/undionly.kpxe` | BIOS x86 (UNDI) | `undionly.kpxe` | ~71KB |
| `bin/ipxe.pxe` | BIOS x86 (full) | `ipxe.pxe` | ~392KB |
| `bin-x86_64-efi/ipxe.efi` | UEFI x86-64 | `ipxe.efi` | ~1.1MB |
| `bin-i386-efi/ipxe.efi` | UEFI IA32 | `ipxe32.efi` | ~1.0MB |
| `bin-arm64-efi/ipxe.efi` | UEFI ARM64 | `ipxe-arm64.efi` | ~1.2MB |

## Integration into PxeLab

Build artifacts need to be copied to two locations:

```bash
# Runtime boot directory
cp bin/undionly.kpxe /path/to/PxeLab/boot/
cp bin-x86_64-efi/ipxe.efi /path/to/PxeLab/boot/
# ... and so on

# Embedded bootdist (extracted on first run)
cp bin/undionly.kpxe /path/to/PxeLab/cmd/pxelab/bootdist/
cp bin-x86_64-efi/ipxe.efi /path/to/PxeLab/cmd/pxelab/bootdist/
# ... and so on
```

Then rebuild PxeLab:

```bash
cd /path/to/PxeLab
go build ./cmd/pxelab/
```

## Architecture Mapping

See `internal/boot/archmap.go` — the mapping logic from client architecture types to boot filenames.
