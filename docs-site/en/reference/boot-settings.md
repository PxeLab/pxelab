# Architecture Mapping & Secure Boot

> 11 CPU architecture support, Secure Boot chain, and chain_to_ipxe.

**Related**: [Boot Config](../guides/boot-config.md) | [iPXE Build](ipxe-build.md)

---

## Supported Client Architectures

PxeLab supports 11 CPU architectures, auto-detected via DHCP Option 93:

| Architecture | AL Code | Boot File | Secure Boot |
|-------------|---------|-----------|-------------|
| Intel x86 (BIOS) | 0 | ipxe.pxe / undionly.kpxe | ❌ |
| EFI IA32 | 6 | ipxe32.efi | ❌ |
| EFI x86-64 | 7 | ipxe.efi | ✅ |
| EFI BC | 9 | snponly.efi | ❌ |
| EFI ARM32 | 10 | ipxe-arm32.efi | ❌ |
| EFI ARM64 | 11 | ipxe-arm64.efi | ✅ |
| EFI RISC-V 32 | 25 | ipxe-riscv32.efi | ❌ |
| EFI RISC-V 64 | 27 | ipxe-riscv64.efi | ❌ |
| EFI LoongArch32 | 37 | ipxe-loong64.efi | ❌ |
| EFI LoongArch64 | 39 | ipxe-loong64.efi | ❌ |

---

## Secure Boot Chain

x86-64 and ARM64 architectures support UEFI Secure Boot. Boot chain:

```
UEFI Firmware → shim-x86_64.efi (Microsoft-signed) → ipxe-x86_64-sb.efi (PxeLab-signed) → Boot Menu
```

View Secure Boot status for each architecture in **Service Config → Boot Settings**.

---

## chain_to_ipxe

When interface bootloader is set to `pxelinux` or `grub2`, enable `chain_to_ipxe` to auto-upgrade clients to iPXE:

```
Client PXE → PXELinux/GRUB2 loads
  → Requests config file
  → Server intercepts, returns iPXE chainload config
  → Client downloads ipxe.efi and executes
  → Enters PxeLab boot decision tree
```
