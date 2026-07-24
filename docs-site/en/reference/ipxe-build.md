# iPXE Custom Build

> PxeLab embedded boot scripts and multi-architecture build artifacts.

**Related**: [Architecture Mapping](boot-settings.md) | [Boot Config](../guides/boot-config.md)

---

## Embedded Boot Script

PxeLab uses custom-compiled iPXE binaries. All binaries embed the same boot script:

```ipxe
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

---

## Build All Architectures

```bash
# Cross-compile with Docker (recommended)
make ipxe-build-all

# Manual build x86_64 EFI
make ipxe-build           # Non-embedded
make ipxe-build-embed     # Embedded (failsafe)
```

---

## Build Artifacts

| File | Architecture | Purpose |
|------|-------------|---------|
| `ipxe.pxe` | x86 BIOS | iPXE boot |
| `ipxe32.efi` | EFI IA32 | iPXE boot |
| `ipxe.efi` | EFI x86-64 | iPXE boot |
| `snponly.efi` | EFI BC | iPXE boot (SNP driver) |
| `ipxe-arm32.efi` | EFI ARM32 | iPXE boot |
| `ipxe-arm64.efi` | EFI ARM64 | iPXE boot |
| `ipxe-riscv32.efi` | RISC-V 32 | iPXE boot |
| `ipxe-riscv64.efi` | RISC-V 64 | iPXE boot |
| `ipxe-loong64.efi` | LoongArch64 | iPXE boot |
| `ipxe-x86_64-sb.efi` | EFI x86-64 | Secure Boot iPXE |
| `ipxe-arm64-sb.efi` | EFI ARM64 | Secure Boot iPXE |
| `shim-x86_64.efi` | EFI x86-64 | Secure Boot Shim |
| `shim-arm64.efi` | EFI ARM64 | Secure Boot Shim |

> This document covers the complete iPXE build process. For more architecture details, see [Architecture Mapping & Secure Boot](boot-settings.md).
