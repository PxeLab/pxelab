# 版本历史

> PxeLab 各版本的新增功能与变更记录。

**相关文档**: [快速开始](getting-started.md) | [架构与概述](architecture.md)

---

## v0.4.0-dev（当前开发版）

**新增**:
- 完整 iPXE 架构支持（ARM32、RISC-V 32/64、LoongArch32/64）
- Secure Boot 支持（x86_64 + ARM64）
- NFS 多挂载点支持
- NFS IP 访问控制
- DNS 子网感知解析
- DHCP 预留（IP+MAC 绑定）
- 访问控制（黑白名单）
- BMC/IPMI 带外管理
- 安装任务跟踪
- 应答文件模板版本管理
- 日志轮转和清理
- 审计日志
- 网络诊断（Ping/Traceroute）
- 本地缓存（加速重复引导）
- Windows 系统托盘

**变更**:
- NBP 架构重构：Option 93 作为主要依据
- 服务生命周期管理
- Profile 简化为单引导项
- 路由重构

---

## v0.3.0

- iPXE 引导脚本系统
- DHCP 四种模式
- Web UI 全功能
- CLI 管理工具
- 事件总线和实时日志

---

## v0.2.0

- TFTP/DNS 服务
- 主机管理
- Profile 管理
- REST API v1

---

## v0.1.0

- 初始版本
- DHCP + HTTP 基础服务
- iPXE 引导

---

> **文档版本**: v1.0 · **适用于**: PxeLab v0.4.0-dev · **维护者**: PxeLab Team
