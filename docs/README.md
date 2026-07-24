# PxeLab Documentation

> PxeLab 用户文档 — 快速开始、功能指南、配置参考与故障排查。

---

## 快速开始

| 文档 | 说明 |
|------|------|
| [快速开始](getting-started.md) | 系统要求、安装方式、首次启动与验证 |
| [架构与概述](architecture.md) | 核心定位、功能特性、两阶段引导与服务架构 |

---

## 使用指南

| 文档 | 说明 |
|------|------|
| [DHCP 配置](guides/dhcp.md) | 四种 DHCP 模式、多接口部署、IP 预留、访问控制 |
| [引导配置](guides/boot-config.md) | iPXE 决策树、引导菜单类型、Profile 管理、自定义脚本 |
| [网络启动目录](guides/netboot.md) | OS 目录菜单、覆盖层、应答文件模板、安装任务 |
| [主机管理](guides/host-management.md) | 主机 CRUD、WOL 网络唤醒、BMC/IPMI 带外管理 |
| [OS 镜像管理](guides/os-images.md) | ISO 上传、挂载、解压、文件浏览 |
| [Web UI 指南](guides/web-ui.md) | 完整的 Web 界面操作指南（仪表盘、主机、配置、监控等） |
| [部署模式](guides/deployment.md) | Server 模式、App 模式、systemd 部署 |

---

## 参考文档

| 文档 | 说明 |
|------|------|
| [REST API 参考](reference/api-reference.md) | API v1 完整端点列表与使用约定 |
| [配置文件参考](reference/config-file.md) | config.yaml 完整结构与 CLI 参数 |
| [TFTP 服务](reference/tftp.md) | TFTP 配置与引导文件管理 |
| [DNS 服务](reference/dns.md) | 本地 DNS、上游转发与记录管理 |
| [NFS 服务](reference/nfs.md) | NFSv3 多挂载点与 IP 访问控制 |
| [架构映射与 Secure Boot](reference/boot-settings.md) | 11 种架构支持与 Secure Boot 链 |
| [iPXE 编译](reference/ipxe-build.md) | 内嵌引导脚本与多架构编译产物 |
| [环境变量与 CLI](reference/environment-variables.md) | CLI 参数、环境变量与数据目录结构 |
| [日志配置](reference/logging.md) | 日志级别、轮转配置与排查 |
| [贡献指南](contributing.md) | 开发环境、代码规范与 PR 流程 |

---

## 其他

| 文档 | 说明 |
|------|------|
| [故障排查与常见问题](troubleshooting.md) | 常见问题排查、日志分析与 FAQ |
| [版本历史](release-notes.md) | 各版本新增功能与变更记录 |


