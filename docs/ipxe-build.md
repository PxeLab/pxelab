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
dhcp || clear
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || shell
```

This script:
1. `dhcp` — Forces iPXE to do its own DHCP (ignores PXE BIOS/UEFI cached data)
2. `chain http://...` — Fetches the iPXE boot menu from PxeGo HTTP server
3. `|| shell` — Falls back to iPXE shell on error for debugging

## Build Commands

### 1) Clone iPXE source

```bash
git clone --depth 1 https://github.com/ipxe/ipxe.git
cd ipxe/src
```

### 2) Create embedded script

```bash
cat > embedd.ipxe << "EOF"
#!ipxe
dhcp || clear
chain http://${next-server}:8080/boot/ipxe/script?mac=${net0/mac} || shell
EOF
```

### 3) Disable PXE_STACK

Edit `src/config/general.h` and comment out:

```c
// #define PXE_STACK
// #define PXE_MENU
// #define PXEXT
```

This prevents iPXE from reading PXE BIOS/UEFI cached DHCP data.

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
