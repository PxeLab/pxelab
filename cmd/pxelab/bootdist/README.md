# Boot Firmware Attribution

This directory contains third-party boot firmware and network boot loaders that are redistributed with PxeLab. Each file below is governed by its own open source license. Source code for these components can be obtained from the upstream projects linked in this document.

> PxeLab itself is licensed under the GNU Affero General Public License v3.0 (AGPL-3.0). The presence of these third-party binaries does not change the license of PxeLab's own source code.

---

## Included Firmware

| File(s) | Upstream Project | Version | License | Source Code |
|---------|------------------|---------|---------|-------------|
| `ipxe*.efi`, `ipxe.pxe`, `snponly*.efi` | [iPXE](https://ipxe.org/) | 2.0.0+ (g84668) | [GPL-2.0-or-later](https://github.com/ipxe/ipxe/blob/master/COPYING.GPLv2) | <https://github.com/ipxe/ipxe> |
| `ipxe-x86_64-sb.efi`, `ipxe-arm64-sb.efi` | [iPXE](https://ipxe.org/) (signed for Secure Boot) | 2.0.0+ | [GPL-2.0-or-later](https://github.com/ipxe/ipxe/blob/master/COPYING.GPLv2) | <https://github.com/ipxe/ipxe> |
| `shim-x86_64.efi`, `shim-arm64.efi` | [shim](https://github.com/rhboot/shim) | 16.1 | [BSD-2-Clause / GPLv2 with exception](https://github.com/rhboot/shim/blob/main/COPYRIGHT) | <https://github.com/rhboot/shim> |
| `grubx64.efi`, `grubaa64.efi` | [GRUB 2](https://www.gnu.org/software/grub/) | 2.03 | [GPL-3.0](https://www.gnu.org/licenses/gpl-3.0.html) | <https://git.savannah.gnu.org/cgit/grub.git> |
| `pxelinux.0`, `pxelinux.bios` | [SYSLINUX / PXELINUX](https://wiki.syslinux.org/) | 4.04 | [GPL-2.0](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html) | <https://wiki.syslinux.org/wiki/index.php?title=Development> |
| `pxelinux.efi`, `ldlinux.*`, `menu.c32`, `memdisk`, `poweroff.com` | [SYSLINUX / PXELINUX](https://wiki.syslinux.org/) | 6.04 | [GPL-2.0](https://www.gnu.org/licenses/old-licenses/gpl-2.0.html) | <https://wiki.syslinux.org/wiki/index.php?title=Development> |
| `autoexec.ipxe`, `embedd.ipxe` | PxeLab custom scripts | — | AGPL-3.0 (same as PxeLab) | This repository |
| `grub.cfg`, `grub.cfg.common` | PxeLab custom configs | — | AGPL-3.0 (same as PxeLab) | This repository |
| `pxelinux.cfg/default` | PxeLab custom config | — | AGPL-3.0 (same as PxeLab) | This repository |

---

## Rebuilding iPXE

PxeLab's iPXE binaries are built from upstream source with an embedded boot script. You can rebuild them using the provided Makefile targets:

```bash
# Build x86_64 EFI iPXE (non-embedded)
make ipxe-build

# Build x86_64 EFI iPXE with embedded script
make ipxe-build-embed

# Build all supported architectures
make ipxe-build-all
```

The default iPXE version is defined by `IPXE_VERSION` in the `Makefile` (currently `v2.0.0`).

---

## Rebuilding GRUB2 / PXELINUX / Syslinux

The GRUB2 and SYSLINUX/PXELINUX binaries in this directory are pre-built upstream releases. Corresponding source code is available from the upstream project links above. If you replace these files with your own builds, update the boot file mappings in PxeLab's configuration accordingly.

---

## License Compliance Notes

- iPXE, GRUB2, and SYSLINUX/PXELINUX are licensed under the GNU General Public License family. When you redistribute PxeLab (including these binaries), you must also make the corresponding source code available to recipients.
- PxeLab satisfies this requirement by providing source code at <https://github.com/PxeLab/pxelab> and by documenting upstream source locations in this file.
- For the full text of each license, see the upstream repositories linked above or refer to the `LICENSE` file in this repository for PxeLab's own AGPL-3.0 license.
