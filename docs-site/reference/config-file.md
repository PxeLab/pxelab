# 配置文件参考

> PxeLab 配置文件的完整结构与 CLI 参数说明。

**相关文档**: [快速开始](../getting-started.md) | [部署模式](../guides/deployment.md)

---

## 配置文件位置

PxeLab 按以下顺序搜索配置文件：

1. `--config` 参数指定的路径
2. 当前目录 `./config.yaml`
3. 数据目录 `~/.pxelab/config.yaml`
4. `/etc/pxelab/config.yaml`

---

## 完整配置结构

```yaml
# ── 全局设置 ──
global:
  listen_addr: ":8080"           # HTTP 监听地址
  data_dir: ""                   # 数据目录（默认 ~/.pxelab/）
  server_name: "PxeLab"          # 服务器名称
  app_mode: false                # App 模式（自动打开浏览器）

# ── 接口配置 ──
interfaces:
  - name: eth0
    ip: 192.168.1.100
    auto_start: true
    subnets:
      - cidr: 192.168.1.0/24
        dhcp: hybrid             # full / proxy / hybrid / off
        pool: 192.168.1.100-192.168.1.200
        gateway: 192.168.1.1
        dns: 8.8.8.8
        lease_time: 86400

# ── TFTP ──
tftp:
  port: 69
  timeout: 30

# ── DNS ──
dns:
  port: 53
  upstream: "8.8.8.8"            # 上游 DNS
  local_domain: "pxelab.local"   # 本地域名

# ── NFS ──
nfs:
  enabled: false
  port: 2049
  mount_points:
    - label: "Default"
      export_path: "/"
      local_dir: ""              # 默认 ~/.pxelab/boot/isos
      read_only: true
      allow_ips: []

# ── 引导配置 ──
boot:
  root_dir: ""                   # 引导文件目录（默认 ~/.pxelab/boot/）
  arch_map:                      # 架构映射（通常自动管理）
    - arch_code: 0
      arch_name: "Intel x86PC"
      nbp: "ipxe"
      ipxe: "ipxe.pxe"
    # ... 更多架构

# ── Netboot ──
netboot:
  enabled: true
  failsafe_prompt: true
  script_template: ""            # 自定义 iPXE 脚本（留空用可视化配置）
  sync:
    url: "https://github.com/netbootxyz/netboot.xyz.git"
    repo: "contrib/netboot.xyz"
  boot:
    default_menu:
      title: "PxeLab Boot Menu"
      timeout: 5000              # 毫秒，0=不自动选择
      default: 0
      entries:
        - label: "Boot from local disk"
          type: local
    catalog_redirect:
      enabled: true
      target_url: "http://{{ "{" }}{".URL}}/netboot/menu.ipxe?arch=${arch}&platform=${platform}"
      detect_arch: true
    catalog_display:
      title: "[OS] Netboot OS Install Catalog"
      groups:
        - name: linux
          title: "Linux Distributions"
          enabled: true
          order: 1
        # ... 更多分组

# ── 服务自动启动 ──
service_auto_start:
  http: true
  tftp: false
  dns: false
  nfs: false

# ── 存储 ──
store:
  dsn: ""                        # SQLite 路径（默认 ~/.pxelab/pxelab.db）

# ── 日志 ──
log:
  level: info                    # debug / info / warn / error
  format: text
  file: ""                       # 日志文件路径（空=默认 ~/.pxelab/logs/）
  max_size_mb: 0                 # 单文件最大体积 (MB)，0=不限制
  max_backups: 0                 # 保留轮转文件数
  max_age_days: 0                # 保留天数
  compress: false                # gzip 压缩旧日志
  cleanup_interval: 0            # 清理检查间隔（小时）

# ── 访问控制种子 ──
blacklist_seeds:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "已报废设备"

whitelist_seeds:
  - mac: "11:22:33:44:55:66"
    subnet: "10.0.0.0/24"
    reason: "服务器区"
```

---

## CLI 参数

```
pxelab [flags]

Flags:
  --config string       配置文件路径
  --data-dir string     数据目录 (default "~/.pxelab/")
  --log-level string    日志级别: debug/info/warn/error (default "info")
  --mode string         运行模式: app/server ("app" 自动打开浏览器)
```
