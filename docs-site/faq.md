# 常见问题

> PxeLab 使用过程中的常见问题与解答。

**相关文档**: [快速开始](getting-started.md) | [故障排查](troubleshooting.md) | [配置文件参考](reference/config-file.md)

---

## 安装与启动

### Q: PxeLab 支持哪些操作系统？

**A:** PxeLab 支持以下平台：

| 操作系统 | 架构 | 说明 |
|---------|------|------|
| Linux | amd64, arm64 | 推荐用于生产环境 |
| Windows | amd64 | 支持 Windows 10+ |
| macOS | arm64, amd64 | 支持 macOS 12+ |

### Q: 需要安装什么依赖？

**A:** 无需任何依赖。PxeLab 是单个可执行文件，下载后直接运行。

```bash
# Linux/macOS
chmod +x pxelab
./pxelab

# Windows
pxelab.exe
```

### Q: 如何以系统服务方式运行？

**A:** Linux 系统可以使用 systemd：

```bash
# 创建服务文件
sudo tee /etc/systemd/system/pxelab.service << 'EOF'
[Unit]
Description=PxeLab PXE Server
After=network.target

[Service]
ExecStart=/usr/local/bin/pxelab
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

# 启用并启动
sudo systemctl enable --now pxelab
```

### Q: 首次启动后访问不了 Web UI？

**A:** 检查以下几点：

1. 确认服务已启动：`curl http://localhost:8080/api/v1/services`
2. 检查端口是否被占用：`netstat -tlnp | grep 8080`
3. 检查防火墙规则是否放行 8080 端口
4. 查看日志：`./pxelab --log-level debug`

---

## 网络配置

### Q: 如何选择 DHCP 模式？

**A:** 根据网络环境选择：

| 场景 | 推荐模式 | 说明 |
|------|---------|------|
| 独立网络，无现有 DHCP | **full** | PxeLab 作为完整 DHCP 服务器 |
| 已有 DHCP 服务器 | **proxy** | 叠加 PXE 功能，不影响现有 DHCP |
| 部分接口需要 PXE | **hybrid** | 混合模式 |
| 仅使用 TFTP/HTTP | **off** | 关闭 DHCP，手动配置客户端 |

### Q: ProxyDHCP 和 Full DHCP 有什么区别？

**A:**

- **Full DHCP**：PxeLab 作为完整 DHCP 服务器，分配 IP 地址和 PXE 引导信息
- **ProxyDHCP**：PxeLab 仅提供 PXE 引导信息，IP 地址由现有 DHCP 服务器分配

ProxyDHCP 适合已有 DHCP 服务器的环境，无需修改现有 DHCP 配置。

### Q: 如何配置多网卡？

**A:** 每个网络接口可以独立配置 DHCP 模式：

```yaml
dhcp:
  interfaces:
    - name: eth0
      mode: full
      subnet: 192.168.1.0/24
    - name: eth1
      mode: proxy
    - name: eth2
      mode: off
```

### Q: DHCP 端口 67 被占用怎么办？

**A:** 两种解决方案：

1. **停止占用端口的服务**（推荐）
2. **使用 ProxyDHCP 模式**（端口 4011，通常不冲突）

```yaml
dhcp:
  mode: proxy  # 使用 4011 端口
```

---

## iPXE 与引导

### Q: 什么是 iPXE？

**A:** iPXE 是开源网络引导固件，支持 HTTP、iSCSI、AoE 等协议。相比传统 PXE，iPXE 功能更强：

- 支持 HTTP 引导（更快、更灵活）
- 支持脚本编程（条件判断、变量）
- 支持更多架构和引导方式

PxeLab 内置预编译的 iPXE，无需自行编译。

### Q: 如何自定义引导菜单？

**A:** 在 Web UI 中：**设置 → Netboot → 自定义 iPXE 脚本**

```ipxe
#!ipxe
dhcp || clear
menu Choose an OS
item ubuntu Ubuntu 22.04
item centos CentOS 9
item local Boot from local disk
choose --timeout 10000 target
goto ${target}

:ubuntu
chain http://192.168.1.10:8080/netboot/ubuntu.ipxe || shell

:centos
chain http://192.168.1.10:8080/netboot/centos.ipxe || shell

:local
sanboot || exit
```

### Q: Secure Boot 是什么？如何启用？

**A:** Secure Boot 是 UEFI 安全启动机制，确保只加载经过签名的引导加载程序。

PxeLab 支持 x86_64 和 ARM64 架构的 Secure Boot。在 Web UI 中：**设置 → Boot Settings → Secure Boot**

### Q: 支持哪些引导类型？

**A:**

| 类型 | 说明 | 典型用途 |
|------|------|---------|
| direct | 直接加载内核 + initrd | Linux 安装 |
| chain | 链式加载到另一个脚本 | 多级引导 |
| wds | Windows WDS 部署 | Windows 安装 |
| sanboot | iSCSI SAN 引导 | 无盘工作站 |
| local | 本地硬盘引导 | 默认引导项 |

---

## 主机与 Profile

### Q: 主机和 Profile 是什么关系？

**A:**

- **主机 (Host)**：代表网络中的一台设备，通过 MAC 地址标识
- **Profile**：引导配置文件，定义主机的引导行为

一台主机可以绑定一个 Profile，Profile 决定该主机如何引导。

### Q: 如何批量导入主机？

**A:** 通过 REST API 或 CSV 导入：

```bash
# CSV 格式
mac,name,profile
AA:BB:CC:DD:EE:01,server-01,ubuntu-install
AA:BB:CC:DD:EE:02,server-02,centos-install
```

```bash
# API 导入
curl -X POST http://localhost:8080/api/v1/hosts/import \
  -F "file=@hosts.csv"
```

### Q: Profile 脚本版本管理是什么？

**A:** PxeLab 记录 Profile 脚本的每次修改历史：

- 查看历史版本
- 对比版本差异
- 回滚到任意版本

适合多人协作和变更审计。

---

## OS 安装目录

### Q: 如何添加新的 Linux 发行版？

**A:** 通过 Web UI：**网络启动目录 → 添加分组**

1. 上传 ISO 镜像到 OS 镜像管理
2. 创建分组（如 "Ubuntu"）
3. 添加条目，指向 ISO 挂载路径
4. 配置应答文件模板（可选）

### Q: 应答文件模板有什么用？

**A:** 应答文件模板用于自动化安装，无需手动交互：

- **Ubuntu/Debian**：preseed.cfg
- **CentOS/RHEL**：kickstart.cfg
- **Windows**：unattend.xml

PxeLab 提供预设模板，支持变量替换和自定义。

---

## 硬件管理

### Q: WOL 如何工作？

**A:** Wake-on-LAN 通过发送魔术包唤醒网络中的设备：

1. 在 Web UI 中添加目标主机的 MAC 地址
2. 点击唤醒按钮
3. PxeLab 发送 UDP 广播魔术包
4. 目标设备收到后开机

支持定时调度，可设置周期性唤醒任务。

### Q: BMC/IPMI 支持哪些操作？

**A:**

| 操作 | 说明 |
|------|------|
| power on | 开机 |
| power off | 关机 |
| power cycle | 重启 |
| power status | 查询电源状态 |
| SOL | 串口重定向（远程终端） |

支持 CSV 批量导入 BMC 信息，适合大规模部署。

---

## 性能与扩展

### Q: PxeLab 能支持多少台主机？

**A:** 取决于硬件配置：

| 配置 | 建议主机数 |
|------|-----------|
| 单核 / 512 MB | ≤ 50 台 |
| 双核 / 1 GB | ≤ 200 台 |
| 四核 / 2 GB | ≤ 500 台 |
| 八核 / 4 GB | ≤ 1000 台 |

DHCP 和 TFTP 是主要性能瓶颈，大规模部署建议使用高性能硬件。

### Q: 如何监控 PxeLab 状态？

**A:** 三种监控方式：

1. **Web UI 仪表盘**：实时查看服务状态、流量、事件
2. **Prometheus 指标**：`GET /api/v1/metrics`
3. **日志**：实时日志流 + 审计日志

### Q: 数据存储在哪里？

**A:** 默认存储在用户目录：

| 平台 | 路径 |
|------|------|
| Linux | `~/.pxelab/` |
| macOS | `~/Library/Application Support/pxelab/` |
| Windows | `%APPDATA%\pxelab\` |

可通过 `--data-dir` 参数自定义。

---

## 故障排查

### Q: 客户端无法获取 IP 地址？

**A:** 检查清单：

1. DHCP 服务是否运行：`curl localhost:8080/api/v1/services | jq .dhcp.status`
2. 端口 67 是否被占用：`netstat -tlnp | grep :67`
3. 防火墙是否放行：`iptables -L -n | grep 67`
4. 网络接口配置是否正确

### Q: 客户端获取 IP 后无法引导？

**A:** 检查清单：

1. TFTP 服务是否运行
2. 引导文件是否存在：`ls -la /path/to/pxelinux.0`
3. next-server 和 boot-file 配置是否正确
4. 客户端和服务器是否在同一子网

### Q: 如何查看详细日志？

**A:** 启用调试模式：

```bash
./pxelab --log-level debug
```

或在 Web UI 中：**设置 → 日志 → 日志级别 → Debug**
