# DHCP 配置

> PxeLab 的 DHCP 服务器支持四种运行模式，每个网络接口可独立配置。

**相关文档**: [架构与概述](../architecture.md) | [引导配置](boot-config.md) | [网络启动目录](netboot.md)

---

## 四种 DHCP 模式

每个网络接口（子网）可独立设置 DHCP 模式：

| 模式 | 分配 IP | PXE 选项 | 非 PXE 客户端 | 适用场景 |
|------|---------|---------|--------------|---------|
| **full** | ✅ | ✅ | ✅ 正常分配 | 唯一 DHCP 服务器 |
| **proxy** | ❌ yiaddr=0 | ✅ | ❌ 忽略 | 叠加到现有 DHCP |
| **hybrid** | ✅ | ✅（仅 PXE） | ✅ 仅分配 IP | 默认，兼顾两方 |
| **off** | ❌ | ❌ | ❌ 忽略 | 关闭 DHCP |

### full 模式

PxeLab 作为网络中的**唯一 DHCP 服务器**，管理整个 DHCP 生命周期：

```
客户端 Discover → PxeLab Offer（IP + 网关 + DNS + PXE 选项）→ Request → Ack
```

适用：新建网络、实验环境、隔离网络。

### proxy 模式

PxeLab **仅提供 PXE 相关选项**，IP 地址由现有 DHCP 服务器分配：

```
客户端 Discover → 现有 DHCP Offer（IP）+ PxeLab ProxyOffer（PXE 选项，yiaddr=0）
```

关键参数：
- `yiaddr=0.0.0.0` — iPXE 识别 ProxyDHCP 的关键判据
- `Option 60 = "PXEClient"` — UEFI PXE Base Code 要求
- `siaddr` — 指向 PxeLab（TFTP/HTTP 服务器地址）

适用：已有 DHCP 服务器的网络，叠加 PXE 服务。

### hybrid 模式（默认）

**智能双模**：对 PXE 客户端用 proxy 模式，对其他客户端用 full 模式：

```
PXE 客户端 → proxy 模式（yiaddr=0 + PXE 选项）
普通客户端 → full 模式（分配 IP + 标准选项）
```

适用：大多数场景的推荐默认模式。

### off 模式

完全关闭该接口的 DHCP 功能，不影响 HTTP/TFTP/DNS 等其他服务。

---

## 多接口部署

支持多网卡多子网配置，每个接口独立设置 DHCP 模式：

```yaml
# 示例：管理口 + 业务口
interfaces:
  - name: eth0          # 管理口
    ip: 10.0.0.1
    subnets:
      - cidr: 10.0.0.0/24
        dhcp: full       # 自建 DHCP
        pool: 10.0.0.100-10.0.0.200

  - name: eth1          # 业务口
    ip: 192.168.1.100
    subnets:
      - cidr: 192.168.1.0/24
        dhcp: proxy      # 叠加 PXE，不干扰公司 DHCP
```

---

## IP 预留（DHCP Reservation）

将特定 IP 地址永久绑定到 MAC 地址，确保关键设备始终获得相同 IP：

- Web UI：**基础配置 → 服务配置 → DHCP → 预留管理**
- API：`POST /api/v1/dhcp/reservations`
- 冲突检测：创建/编辑时自动检查 IP 是否已被其他预留或活跃租约占用
- 仅 full 模式子网支持预留

---

## 访问控制（黑白名单）

| 列表 | 行为 | 适用场景 |
|------|------|---------|
| **白名单** | 仅允许列表中的 MAC 地址获取 IP | 严格控制接入设备 |
| **黑名单** | 拒绝列表中的 MAC 地址 | 封禁特定设备 |
| **未授权设备** | 不在任何列表中的设备 | 监控和审计 |

Web UI：**管理 → 访问控制**

配置支持从 YAML 种子文件预导入：

```yaml
blacklist_seeds:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "已报废设备"

whitelist_seeds:
  - mac: "11:22:33:44:55:66"
    subnet: "10.0.0.0/24"
    reason: "服务器区"
```
