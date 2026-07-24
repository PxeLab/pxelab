# Web UI 指南

> PxeLab Web 管理界面的完整操作指南。

**相关文档**: [快速开始](../getting-started.md) | [架构与概述](../architecture.md) | [配置文件参考](../reference/config-file.md)

---

## 侧边栏导航结构

侧边栏分为四个分区，每个分区下的页面通过 `<Route>` 懒加载：

```
侧边栏导航
│
├─ 概览
│   └─ 仪表盘（/）
│
├─ 基础配置
│   ├─ 服务配置 ▸（二级子导航，显示在右侧）
│   │   ├─ DHCP（/services/dhcp）
│   │   ├─ DNS（/services/dns）
│   │   ├─ NFS（/services/nfs）
│   │   ├─ TFTP（/services/tftp）
│   │   ├─ Boot Settings（/boot-settings）
│   │   └─ Netboot 目录（/netboot-catalog）
│   ├─ 文件管理（/files）
│   ├─ 引导配置（/profiles）
│   ├─ 应答文件模板（/answer-templates）
│   └─ OS 镜像（/os-images）
│
├─ 管理
│   ├─ 主机（/hosts → /hosts/:id）
│   ├─ 访问控制（/access-control）
│   ├─ 安装任务（/install-tasks）
│   ├─ BMC（/bmc）
│   ├─ WOL（/wol）
│   └─ 网络诊断（/network）
│
├─ 监控
│   ├─ 事件（/events）
│   ├─ 审计日志（/audit-logs）
│   └─ 日志（/logs）
│
└─ 底部
    └─ 设置（弹出 SettingsModal 弹窗）
```

**注意**：「服务配置」是带二级子导航的分组，点击后在右侧展开子页面列表（DHCP / DNS / NFS / TFTP / Boot Settings / Netboot 目录）。

---

## 功能菜单详解

按侧边栏分区，各功能页面的详细说明请参阅：

### 概览

- [仪表盘](dashboard.md) — 全局统计、服务状态、流量图表、最近事件

### 基础配置

- [服务配置](services.md) — DHCP / DNS / NFS / TFTP / Boot Settings / Netboot 目录
- [文件管理](files.md) — 引导文件的上传、删除和浏览
- [引导配置](profiles.md) — Profile 管理、脚本版本控制
- [应答文件模板](answer-templates.md) — 自动化安装应答文件
- [OS 镜像](os-images.md) — ISO 上传、挂载、解压

### 管理

- [主机管理](host-management.md) — 主机 CRUD、WOL、BMC 电源控制
- [访问控制](access-control.md) — MAC 黑白名单
- [安装任务](install-tasks.md) — 网络安装任务跟踪
- [BMC 带外管理](bmc.md) — IPMI 电源控制、批量操作
- [WOL 网络唤醒](wol.md) — 远程唤醒、定时调度
- [网络诊断](network-diagnostics.md) — Ping / Traceroute

### 监控

- [监控](monitoring.md) — 事件流、审计日志、实时日志

### 设置

- [设置弹窗](settings.md) — 通用配置、引导菜单、Netboot、服务自启动、日志管理

---

## 顶部栏功能

### 服务状态下拉

顶栏右侧的**服务状态指示器**，悬停/点击展开：

- 显示运行中 / 已停止 / 错误的服务数量
- 展开后显示所有服务列表（名称 + 端口 + 状态圆点）
- 每个服务可单独启动/停止/重启
- 一键全部启动/全部停止/全部重启
- HTTP 标记为 Core，禁止停止操作
- 5 秒自动轮询刷新

### 通知中心

顶栏的铃铛图标，点击展开通知面板：

- 实时推送系统通知
- 查看历史通知

### 搜索 / 命令面板（⌘K）

按 `⌘K`（macOS）或 `Ctrl+K`（Windows/Linux）打开：

- 快速导航到任意页面
- 显示最近访问的页面
- 搜索功能和设置
- 键盘快捷操作

### 语言切换

中/英文一键切换，设置保存到 localStorage。

### 主题切换

- 亮色 / 暗色 / 跟随系统 三态切换
- 主色选择（蓝色、紫色、青色等调色板）
- 圆角基准调节
- macOS 卡片风格开关
