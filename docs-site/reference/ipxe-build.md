# iPXE 自定义编译

> PxeLab 内嵌引导脚本与多架构编译产物。

**相关文档**: [架构映射](boot-settings.md) | [引导配置](../guides/boot-config.md)

---

## 内嵌引导脚本

PxeLab 使用自定义编译的 iPXE 二进制，所有二进制内嵌同一份引导脚本：

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

## 编译所有架构

```bash
# 使用 Docker 交叉编译（推荐）
make ipxe-build-all

# 手动编译 x86_64 EFI
make ipxe-build           # 非嵌入式
make ipxe-build-embed     # 嵌入式（failsafe）
```

---

## 编译产物

| 文件 | 架构 | 用途 |
|------|------|------|
| `ipxe.pxe` | x86 BIOS | iPXE 引导 |
| `ipxe32.efi` | EFI IA32 | iPXE 引导 |
| `ipxe.efi` | EFI x86-64 | iPXE 引导 |
| `snponly.efi` | EFI BC | iPXE 引导（SNP 驱动） |
| `ipxe-arm32.efi` | EFI ARM32 | iPXE 引导 |
| `ipxe-arm64.efi` | EFI ARM64 | iPXE 引导 |
| `ipxe-riscv32.efi` | RISC-V 32 | iPXE 引导 |
| `ipxe-riscv64.efi` | RISC-V 64 | iPXE 引导 |
| `ipxe-loong64.efi` | LoongArch64 | iPXE 引导 |
| `ipxe-x86_64-sb.efi` | EFI x86-64 | Secure Boot iPXE |
| `ipxe-arm64-sb.efi` | EFI ARM64 | Secure Boot iPXE |
| `shim-x86_64.efi` | EFI x86-64 | Secure Boot Shim |
| `shim-arm64.efi` | EFI ARM64 | Secure Boot Shim |

> 本文档涵盖 iPXE 编译的完整流程。更多架构细节请参考 [架构映射与 Secure Boot](boot-settings.md)。
