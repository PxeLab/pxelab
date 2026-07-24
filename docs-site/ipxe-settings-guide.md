# iPXE 引导脚本配置使用说明

## 概述

PxeLab 的 iPXE 引导脚本系统采用**配置驱动决策树**设计。你无需编写原始 iPXE 脚本，通过 Web 管理界面（设置 → Netboot）即可可视化配置完整的 PXE 引导行为。

## 决策树流程

当客户端获取引导脚本（`GET /boot/ipxe/script?mac=xx:xx:xx:xx:xx:xx`）时，按以下优先级确定返回内容：

```
1. 自定义脚本 → 有填写？返回自定义脚本，忽略下方所有配置
2. 主机 Profile → 该 MAC 有绑定 Profile 且包含菜单条目？返回 Profile 菜单
   ├─ 追加「从本地硬盘启动」（可选）
   └─ 追加「OS 安装目录」入口（可选）
3. 安装目录跳转 → 已启用？直接 chain 到安装目录
   ├─ 自动检测架构（可选）
   └─ 执行前置脚本（可选）
4. 默认引导菜单 → 返回配置的默认菜单
```

## 配置模块

### 1. 自定义 iPXE 脚本（逃生口）

路径：设置 → Netboot → 自定义 iPXE 脚本

填写后**完全替代**所有下方可视化配置，直接作为客户端引导脚本。用于临时调试或高级定制。

可用模板变量：

| 变量 | 说明 |
|------|------|
| `{{'{{'}}.URL}}` | 替换为服务器地址，例如 `http://192.168.1.10:8080` |
| `{{'{{'}}.MAC}}` | 替换为客户端 MAC 地址 |

示例：

<v-pre>

```
#!ipxe
dhcp || clear
echo Booting custom script for {{.MAC}}
chain {{.URL}}/boot/custom.ipxe || shell
```

</v-pre>

留空则使用下方的可视化配置。

---

### 2. 默认引导菜单

路径：设置 → Netboot → 默认引导菜单

当客户端**无关联 Profile** 且**未启用 Netboot 安装目录跳转**时，客户端看到此菜单。

#### 菜单标题
默认值：`PxeLab Boot Menu`

#### 超时时间（秒）
- `0` = 不自动选择，等待用户操作
- `>0` = 超时后自动选择 default 条目

#### 菜单条目

每个条目包含：

| 字段 | 说明 |
|------|------|
| **标签** | 条目显示名称 |
| **类型** | 引导类型（见下方） |

**引导类型说明：**

| 类型 | 用途 | 额外字段 | 示例 |
|------|------|---------|------|
| `local` | 从本地硬盘启动 | 无 | 退出网络引导 |
| `direct` | 直接加载内核 + initrd | kernel, initrd, cmdline | Linux 安装 |
| `chain` | Chain-load 其他引导器 | URL | GRUB2, WDS |
| `sanboot` | iSCSI SAN 启动 | URL | 无盘工作站 |
| `wds` | Windows WIM 引导 | URL, WIM | Windows PE |

**direct 类型字段：**
- **kernel 路径** — 相对于 `/boot/` 的内核文件路径，例如 `vmlinuz`
- **initrd 路径** — 相对于 `/boot/` 的 initrd 路径，例如 `initrd.img`
- **cmdline** — 内核命令行参数，例如 `net.ifnames=0 console=tty0`

**chain/sanboot 类型字段：**
- **URL** — 跳转目标 URL

**wds 类型字段：**
- **URL** — WDS 服务器地址
- **WIM 路径** — WIM 文件路径

> **注意：** 所有文件路径相对于 HTTP 启动文件目录，由「HTTP → 启动文件目录」设置决定。前端自动补全为 `http://<服务器地址>/boot/`。

---

### 3. Profile 菜单行为

路径：设置 → Netboot → Profile 菜单行为

控制有关联 Profile 的主机在 iPXE 菜单中自动追加的条目。

| 选项 | 说明 |
|------|------|
| **追加「从本地硬盘启动」** | 在 Profile 菜单末尾/开头添加 local 条目 |
| **追加「OS 安装目录」** | 在 Profile 菜单末尾/开头添加 netboot 目录入口 |
| **追加位置** | 追加到菜单开头或末尾 |

典型场景：服务器维护模式时启用「从本地硬盘启动」和「OS 安装目录」，让运维人员可以临时选择引导方式。

---

### 4. 安装目录跳转

路径：设置 → Netboot → 安装目录跳转

当客户端**无关联 Profile** 且 **Netboot 已启用**时，自动跳转到 OS 安装目录菜单。

| 选项 | 说明 |
|------|------|
| **启用跳转** | 是否启用自动跳转 |
| **目标 URL** | 跳转目标，支持 `{{'{{'}}.URL}}` 变量 |
| **自动检测架构** | 跳转前检测客户端 CPU 架构和固件类型 |
| **前置脚本** | 跳转前额外执行的 iPXE 命令 |

**架构检测**启用时自动设置以下变量：

```ipxe
cpuid --ext 29 && set arch x86_64 || set arch x86
iseq ${buildarch} arm64 && set arch arm64 ||
iseq ${buildarch} armhf && set arch armhf ||
platform --is efi && set platform efi || set platform pc
```

**目标 URL 示例：**

<v-pre>

```
http://{{.URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}
```

</v-pre>

**前置脚本示例：**

```ipxe
# 重新 DHCP 获取租约
dhcp || clear

# 设置自定义变量
set keep-san 1
```

---

### 5. 安装目录菜单结构

路径：设置 → Netboot → 安装目录菜单结构

控制 `/netboot/menu.ipxe` 的标题和分组顺序。

| 选项 | 说明 |
|------|------|
| **菜单标题** | 安装目录菜单的标题 |
| **分组列表** | 10 个内置分组的顺序/启用/标题编辑 |

**分组列表支持拖拽排序：** 拖拽分组行调整显示顺序。每个分组可配置：

- **内部名称**（只读）— 如 `linux`、`bsd`、`windows`
- **显示标题** — 例如将 `Linux Distributions` 改为 `Linux 发行版`
- **启用开关** — 禁用后该分组不显示在菜单中

默认分组：

| 内部名称 | 默认标题 | 内容 |
|----------|---------|------|
| `linux` | Linux Distributions | x86_64 Linux 发行版 |
| `linux-i386` | Linux Distributions (32-bit) | 32 位 Linux |
| `linux-arm64` | Linux Distributions (arm64) | ARM64 Linux |
| `bsd` | BSD Systems | FreeBSD/OpenBSD 等 |
| `live` | Live CDs | 图形化 Live 环境 |
| `live-arm` | Live CDs (arm64) | ARM64 Live 环境 |
| `tools` | System Tools | 系统工具/救援镜像 |
| `windows` | Windows | Windows PE/安装 |
| `dos` | DOS | DOS 引导 |
| `unix` | Unix | 其他 Unix 系统 |

---

## 高级配置

### chain_to_ipxe

当接口配置的引导加载器为 `pxelinux` 或 `grub2`，且启用了 `chain_to_ipxe` 时，PxeLab 会在 PXELinux/GRUB2 客户端请求配置文件时，自动返回 iPXE chainload 配置，引导客户端升级到 iPXE。

工作原理：

```
客户端 PXE → PXELinux/GRUB2 加载
  → 请求 pxelinux.cfg/default 或 grub.cfg
  → 服务器拦截，返回 iPXE chainload 配置
  → 客户端下载 ipxe.efi/undionly.kpxe 并执行
  → iPXE 再请求引导脚本 → 进入配置驱动决策树
```

配置示例（YAML）：

```yaml
interfaces:
  - name: eth0
    dhcp: full
    bootloader: grub2
    chain_to_ipxe: true
```

---

## 常见场景配置

### 场景 1：仅显示网络安装目录

```
默认菜单：保留默认条目即可
超时：5 秒
安装目录跳转：启用
→ 无 Profile 客户端：自动转到 OS 安装目录
→ 有 Profile 客户端：显示 Profile 菜单
```

### 场景 2：纯本地启动 + 管理用 netboot

```
默认菜单：仅保留「从本地硬盘启动」条目
安装目录跳转：关闭
Profile 追加本地启动：开启
→ 所有客户端默认本地启动
→ 需安装 OS 的客户端：绑定 Profile
```

### 场景 3：多架构混合环境

```
安装目录跳转：启用 + 架构检测
→ x86_64 EFI 客户端自动链到对应架构的安装目录
→ ARM64 客户端进入 ARM64 发行版目录
```

### 场景 4：维护模式

```
Profile 菜单行为：追加位置 → 开头
追加本地启动 + 追加 OS 目录：均开启
→ 客户端启动时先看到本地启动和安装目录选项
→ 避免因 Profile 自动选择导致无法干预
```

---

## 后端 YAML 配置参考

上述所有配置均可直接在 `config.yaml` 中配置，Web 界面修改后自动保存到文件。

```yaml
netboot:
  enabled: true
  script_template: ""  # 留空则使用可视化配置
  boot:
    default_menu:
      title: "PxeLab Boot Menu"
      timeout: 5000
      default: 0
      entries:
        - label: "Boot from local disk"
          type: local
        - label: "Install Ubuntu 22.04"
          type: direct
          kernel: "vmlinuz"
          initrd: "initrd.img"
          cmdline: "net.ifnames=0"
    profile_behavior:
      append_local: true
      append_netboot: true
      append_position: "last"
    catalog_redirect:
      enabled: true
      target_url: "http://{{ "{" }}{".URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}"
      detect_arch: true
      preamble: ""
    catalog_display:
      title: "[OS] Netboot OS Install Catalog"
      groups:
        - name: linux
          title: "Linux Distributions"
          enabled: true
          order: 1
        - name: linux-i386
          title: "Linux Distributions (32-bit)"
          enabled: true
          order: 2
        # ... 更多分组
```

## 故障排查

| 现象 | 原因 | 检查 |
|------|------|------|
| 客户端启动后直接进入本地磁盘 | 默认菜单有 local 条目且超时归零 | 检查默认菜单是否只有 local 条目 |
| 客户端看不到安装目录 | Netboot 未启用或 Profile 未追加 | 检查「启用 OS 安装目录菜单」和「Profile 菜单行为」 |
| 修改配置不生效 | 浏览器缓存 | 硬刷新 (Ctrl+F5) |
| 自定义脚本不执行 | 脚本语法错误 | 查看服务器日志中有无 iPXE 报错 |
| chain_to_ipxe 不触发 | 接口配置的 bootloader 不匹配 | 检查接口配置中 `bootloader: grub2` 或 `pxelinux` |
