# DHCP 模式说明

PxeLab 支持 4 种 DHCP 模式，在接口配置中通过 `dhcp` 字段设置。

## 快速对照

| 模式 | 分配 IP | 提供 PXE 选项 | 非 PXE 客户端 | 适用场景 |
|------|---------|---------------|---------------|----------|
| **full** | ✅ | ✅ | ✅ 正常分配 | PxeLab 作为唯一 DHCP 服务器 |
| **proxy** | ❌ 全程 yiaddr=0 | ✅ | ❌ 忽略 | 叠加到现有 DHCP 环境 |
| **hybrid** | ✅ | ✅（仅 PXE 客户端） | ✅ 仅分配 IP，不含 PXE 选项 | 默认模式，兼顾两方 |
| **off** | ❌ | ❌ | ❌ 忽略 | 完全关闭 DHCP 功能 |

---

## 各模式详解

### full（完整 DHCP）

PxeLab 作为该网络的**唯一 DHCP 服务器**，对所有客户端（无论是否 PXE 请求）响应。

**行为：**

```
DHCP Discover ──► PxeLab
    │
    ├─ PXE/BIOS 客户端 ──► Offer：IP 地址 + 子网掩码 + 网关 + DNS + 启动文件 + PXE 选项
    │                      ├─ Option 54 (Server Identifier)
    │                      ├─ Option 1 (Subnet Mask)
    │                      ├─ Option 3 (Router)
    │                      ├─ Option 6 (DNS)
    │                      ├─ Option 51 (Lease Time)
    │                      ├─ Option 43 (PXE Discovery Control)
    │                      └─ Option 175.178 (iPXE 脚本 URL)
    │
    ├─ iPXE 客户端 ──► Offer：IP 地址 + 标准选项 + 脚本 URL（启动文件字段设为脚本 URL）
    │
    └─ 非 PXE 客户端 ──► Offer：IP 地址 + 标准选项（无 PXE 相关选项）
```

**特点：**
- 管理整个 DHCP 生命周期：Discover → Offer → Request → Ack
- 所有客户端都获得 IP 地址
- PXE 客户端额外获取 NBP 文件名和引导脚本 URL（Option 175.178）
- 非 PXE 客户端获得标准 DHCP 响应，正常上网

**适用场景：**
- 新建网络，PxeLab 作为网络中的唯一 DHCP 服务
- 实验/测试环境，不需要保留现有 DHCP 基础架构
- 隔离网络（无上行 DHCP 服务器）

---

### proxy（代理 DHCP）

PxeLab **仅提供 PXE 相关选项**，IP 地址由网络中现有的 DHCP 服务器分配。

**行为：**

```
DHCP Discover ──► 现有 DHCP + PxeLab
    │
    ├─ PXE 客户端（Legacy + UEFI）：
    │   ├─ 现有 DHCP ──► Offer：IP 地址（标准 DHCP）
    │   └─ PxeLab ──► Offer：yiaddr=0.0.0.0 + 启动文件 + PXE 选项
    │                  ├─ siaddr = PxeLab IP
    │                  ├─ Option 54 (Server Identifier)
    │                  ├─ Option 60 = "PXEClient"（UEFI 识别 ProxyDHCP 的关键）
    │                  ├─ Option 66 (TFTP Server Name)
    │                  ├─ Option 67 (Boot File Name)
    │                  ├─ Option 43 (PXE Discovery Control)
    │                  └─ Option 175.178 (iPXE 脚本 URL)
    │
    ├─ iPXE 客户端 ──► Offer：yiaddr=0.0.0.0 + 脚本 URL + 同上 PXE 选项
    │
    └─ 非 PXE 客户端 ──► 忽略，不响应
```

**参数说明：**

- **yiaddr=0.0.0.0** — 全程保持。iPXE `dhcp_offer()` 以此为关键判据识别 ProxyDHCP
- **Option 60 = "PXEClient"** — UEFI PXE Base Code 要求在响应中回写此选项才承认 PXE OFFER
- **siaddr** — 指向 PxeLab 自身（作为 TFTP/HTTP 服务器），iPXE 将其存入 `proxydhcp/next-server`
- 不发送网关、DNS、租期 — 严格遵循 ProxyDHCP 语义

**特点：**
- Legacy BIOS 和 UEFI **统一使用 yiaddr=0.0.0.0** — Option 60 修复后 UEFI 不再需要分配临时 IP
- 不干扰现有 DHCP 服务器的地址分配
- 非 PXE 客户端完全不受影响
- 不消耗地址池

**适用场景：**
- 已有 DHCP 服务器的网络，需要叠加 PXE 引导服务
- 不想改动现有网络基础设施

---

### hybrid（混合）

PxeLab **对 PXE 客户端以 proxy 模式响应，对其他客户端以 full 模式响应**。这是默认模式。

**行为：**

```
DHCP Discover ──► PxeLab
    │
    ├─ PXE 客户端（检测到 Option 60 "PXEClient" 等）：
    │   └─ 以 proxy 模式处理：yiaddr=0.0.0.0 + PXE 选项
    │
    ├─ iPXE 客户端：
    │   └─ 以 proxy 模式处理：yiaddr=0.0.0.0 + 脚本 URL
    │
    └─ 非 PXE 客户端：
        └─ 以 full 模式处理：分配 IP + 标准 DHCP 选项（无 PXE 选项）
```

在代码中判定逻辑（`handler.go:147-151`）：

```
detect PXE client? → proxy 模式
  └─ 非 PXE? → 检查接口 DHCP 模式
      ├─ hybrid → full 模式（分配 IP）
      └─ full → full 模式
```

**特点：**
- **一个接口同时承担两种角色**：对 PXE 客户端是 ProxyDHCP，对普通客户端是标准 DHCP
- 不需要两台 DHCP 服务器，也不需要 DHCP 中继
- 普通客户端获得完整网络配置（IP、网关、DNS）
- PXE 客户端获得引导选项但不影响 IP 分配

**适用场景：**
- 小型网络，PxeLab 承担 DHCP 服务但同时要叠加 PXE
- 不想架设两台 DHCP 服务器的场景
- **推荐默认模式**，兼顾各方需求

---

### off（关闭）

PxeLab 在该接口上**完全关闭 DHCP 功能**，不处理任何 DHCP 请求。

**特点：**
- 接口如同 DHCP 不存在一样
- 不会发送任何 DHCP 响应
- 不影响同接口上的 HTTP/TFTP/DNS 等其他服务

**适用场景：**
- 接口只做 HTTP/TFTP 启动文件服务，DHCP 由其他设备提供
- 排查 DHCP 冲突时临时关闭
- 管理接口不需要提供 DHCP

---

## 决策流程

当收到 DHCP 请求时，PxeLab 按以下流程确定模式：

```
收到 DHCP 请求
    │
    ├─ 是否为 PXE/iPXE 客户端？
    │   ├─ 是 → 检查接口 DHCP 模式
    │   │       ├─ proxy → proxy 模式处理
    │   │       ├─ hybrid → proxy 模式处理
    │   │       ├─ full → full 模式处理
    │   │       └─ off → 不处理
    │   │
    │   └─ 否 → 检查接口 DHCP 模式
    │           ├─ full → full 模式处理（分配 IP）
    │           ├─ hybrid → full 模式处理（分配 IP，不附加 PXE 选项）
    │           ├─ proxy → 不处理（非 PXE 客户端在 proxy 下被忽略）
    │           └─ off → 不处理
    │
    └─ 响应客户端
```

## 实际部署示例

### 示例 1：有公司 DHCP 服务器，叠加 PXE

```
    公司 DHCP          PxeLab
  192.168.1.1      192.168.1.100
       │                 │
       │                 │  dhcp: proxy
       │                 │  bootloader: ipxe
       │                 │
       │                 └── PXE 客户端提供引导选项
       │
       └── 所有客户端获得 IP
           非 PXE 客户端不受 PxeLab 影响
```

### 示例 2：新网络，PxeLab 一机包办

```
    PxeLab (192.168.1.100)
       │
       │  dhcp: full
       │  bootloader: ipxe
       │
       ├── PXE 客户端：分配 IP + 引导选项
       ├── 普通客户端：分配 IP + 网络配置
       └── 无其他 DHCP 冲突
```

### 示例 3：hybrid 默认模式

```
    PxeLab (192.168.1.100)
       │
       │  dhcp: hybrid（默认）
       │
       ├── PXE 客户端 → proxy 模式（yiaddr=0.0.0.0 + 引导选项）
       ├── 普通客户端 → full 模式（分配 IP）
       └── 相当于"智能双模"
```

### 示例 4：双接口，管理口 + 业务口

```yaml
interfaces:
  - name: eth0       # 管理口
    ip: 10.0.0.1
    dhcp: full       # 管理网段自建 DHCP
    subnets:
      - cidr: 10.0.0.0/24
        pool: 10.0.0.100-10.0.0.200

  - name: eth1       # 业务口，叠加 PXE
    ip: 192.168.1.100
    dhcp: proxy      # 不干扰公司 DHCP
    subnets:
      - cidr: 192.168.1.0/24
```

### 示例 5：仅提供文件服务

```yaml
interfaces:
  - name: eth0
    ip: 192.168.1.100
    dhcp: off        # DHCP 由其他设备负责
    tftp: true       # 仅提供 TFTP
    http: true       # 仅提供 HTTP
    bootloader: grub2
```

## Web 界面配置

PxeLab 界面上对每个接口可以独立设置 DHCP 模式：「设置 → 接口」，每个接口的「DHCP 模式」下拉框。

> **注意：** 同接口下的子网共享该接口的 DHCP 模式。如需要多个不同行为，通过多接口配置实现。
