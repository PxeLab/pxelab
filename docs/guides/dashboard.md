# 仪表盘（Dashboard）

> 首页仪表盘提供全局概览，每 5 秒自动刷新。

**相关文档**: [快速开始](../getting-started.md) | [主机管理](host-management.md) | [服务配置](services.md)

---

## 统计卡片

顶行展示 5 个核心指标：

| 卡片 | 数据来源 | 说明 |
|------|---------|------|
| 在线主机 | `hosts.last_online` | 最近 10 分钟内有记录的主机数 |
| 运行中服务 | `GET /api/v1/services` | status=running 的服务数 |
| 活跃租约 | Prometheus `dhcp.activeLeases` | 当前 DHCP 活跃租约数 |
| DNS 记录数 | `GET /api/v1/dns/records` | 总 DNS 记录数 |
| 今日引导次数 | events（type=BOOT, 今天） | 当天 BOOT 事件数 |

## 服务状态栏

显示 DHCP / TFTP / HTTP / DNS / NFS 五个服务的运行状态（绿/黄/红圆点）和端口。

## 图表区域

- **流量趋势图** — TFTP / HTTP / NFS 带宽，支持 5m / 30m / 1h 时间范围切换
- **HTTP 状态码分布** — 饼图显示 2xx / 3xx / 4xx / 5xx 占比，支持时间范围切换
- **DHCP 架构分布** — 饼图显示各客户端架构（x86 BIOS / EFI x86-64 / ARM64 等）占比
- **最近事件** — 最新 12 条事件，带类型标签（DHCP / TFTP / HTTP / BOOT / IPMI / DNS）

## 在线主机列表

显示最近引导的主机，带在线状态圆点。
