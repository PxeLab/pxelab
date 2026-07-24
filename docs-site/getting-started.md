# 快速开始

> 本文档介绍 PxeLab 的安装与首次启动。

**相关文档**: [架构与概述](architecture.md) | [部署模式](guides/deployment.md) | [故障排查](troubleshooting.md)

---

## 系统要求

| 项目 | 要求 |
|------|------|
| **操作系统** | Windows 10+ / Linux (kernel 3.10+) / macOS 12+ |
| **架构** | amd64 / arm64 |
| **内存** | ≥ 512 MB |
| **磁盘** | ≥ 1 GB 可用空间 |
| **网络** | 需要管理员/root 权限（DHCP 端口 67） |
| **依赖** | 无（单二进制，所有文件内嵌） |

---

## 安装方式

### 方式一：下载 Release（推荐）

从 GitHub Releases 下载对应平台的二进制文件：

```bash
# Linux amd64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_linux_amd64.tar.gz
tar xzf pxelab_linux_amd64.tar.gz

# macOS arm64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_darwin_arm64.tar.gz
tar xzf pxelab_darwin_arm64.tar.gz

# Windows
# 下载 pxelab_windows_amd64.zip，解压得到 pxelab.exe
```

### 方式二：从源码编译

```bash
# 克隆仓库
git clone https://github.com/user/pxelab.git
cd pxelab

# 编译后端
make build          # 生成 bin/pxelab

# 编译前端（可选，已内嵌）
make frontend       # cd web && npm ci && npm run build

# 完整发布构建
make release        # goreleaser release --clean
```

### 方式三：Docker（计划中）

> Docker 容器化部署在 Roadmap 中，尚未实现。

---

## 首次启动

### Linux / macOS

```bash
# 需要 root 权限（DHCP 端口 67 需要特权）
sudo ./bin/pxelab

# 或指定数据目录
sudo ./bin/pxelab --data-dir /opt/pxelab-data

# 或以 App 模式启动（自动打开浏览器）
sudo ./bin/pxelab --mode app
```

### Windows

```cmd
# 管理员权限运行
pxelab.exe

# 或 App 模式
pxelab.exe --mode app
```

> **Windows 系统托盘**: 在 Windows 上，PxeLab 默认以系统托盘模式运行，自动隐藏控制台窗口。托盘图标提供打开浏览器、查看数据目录、退出等功能。

### 首次启动行为

1. 自动创建数据目录 `~/.pxelab/`
2. 初始化 SQLite 数据库 `~/.pxelab/pxelab.db`
3. 生成默认配置文件 `~/.pxelab/config.yaml`
4. 释放内嵌的引导文件到 `~/.pxelab/boot/`
5. 释放内嵌的 netboot 目录到 `~/.pxelab/netboot/`
6. 启动 HTTP 服务器（端口 8080）
7. 自动打开浏览器（`--mode app` 时）

---

## 验证服务

启动后，访问以下地址验证服务状态：

| 地址 | 用途 |
|------|------|
| `http://localhost:8080` | Web 管理界面 |
| `http://localhost:8080/api/v1/status` | 服务状态 JSON |
| `http://localhost:8080/api/v1/metrics` | Prometheus 指标 |
| `http://localhost:8080/boot/ipxe/script` | iPXE 引导脚本 |

```bash
# 检查服务状态
curl -s http://localhost:8080/api/v1/status | python -m json.tool
```
