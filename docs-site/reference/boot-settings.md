# 架构映射与 Secure Boot

> 11 种 CPU 架构支持、Secure Boot 链与 chain_to_ipxe。

**相关文档**: [引导配置](../guides/boot-config.md) | [iPXE 编译](ipxe-build.md)

---

## 支持的客户端架构

PxeLab 支持 11 种 CPU 架构，通过 DHCP Option 93 自动检测：

| 架构 | AL 码 | 引导文件 | Secure Boot |
|------|-------|---------|-------------|
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

## Secure Boot 链

x86-64 和 ARM64 架构支持 UEFI Secure Boot，启动链为：

```
UEFI 固件 → shim-x86_64.efi（微软签名）→ ipxe-x86_64-sb.efi（PxeLab 签名）→ 引导菜单
```

在 **基础配置 → 服务配置 → Boot Settings** 中可查看各架构的 Secure Boot 支持状态。

---

## chain_to_ipxe

当接口配置的引导加载器为 `pxelinux` 或 `grub2` 时，启用 `chain_to_ipxe` 可自动将客户端升级到 iPXE：

```
客户端 PXE → PXELinux/GRUB2 加载
  → 请求配置文件
  → 服务器拦截，返回 iPXE chainload 配置
  → 客户端下载 ipxe.efi 并执行
  → 进入 PxeLab 引导决策树
```
