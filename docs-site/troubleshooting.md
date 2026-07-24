# 故障排查与常见问题

> 常见问题排查、日志分析与 FAQ。

**相关文档**: [Web UI 指南](guides/web-ui.md) | [配置文件参考](reference/config-file.md)

---

## 常见问题

| 现象 | 可能原因 | 排查步骤 |
|------|---------|---------|
| 客户端无法 PXE 启动 | DHCP 未配置 / 网络不通 | 1. 检查 PxeLab 是否运行 2. 检查客户端和服务器是否在同一 VLAN 3. 检查防火墙是否放行 UDP 67 |
| 客户端启动后直接进本地磁盘 | 默认菜单只有 local 条目 | 检查默认菜单配置，确保有网络引导条目 |
| 客户端看不到安装目录 | Netboot 未启用 | 检查「启用 OS 安装目录菜单」和「Profile 菜单行为」 |
| DHCP Offer 被 UEFI 固件拒绝 | 缺少 Option 53 | 确保 PxeLab 版本包含 Option 53 修复 |
| iPXE 循环重启 | PXE_STACK 缓存问题 | PxeLab 已通过自定义编译 iPXE 解决（禁用 PXE_STACK） |
| vmxnet3 虚拟网卡启动失败 | TXE:1 兼容问题 | 使用 undionly.kpxe（UNDI 接口） |
| 修改配置不生效 | 浏览器缓存 | 硬刷新 (Ctrl+F5) |
| NFS 挂载失败（Windows） | 路径分隔符问题 | 确保使用 PxeLab 最新版（已修复 path.Clean） |
| DNS 不解析 | 上游 DNS 未配置 | 检查 DNS 设置中的 upstream 字段 |

---

## 日志排查

```bash
# 查看实时日志
# Web UI: 日志页面 → 选择服务过滤

# 或查看日志文件
tail -f ~/.pxelab/logs/pxelab.log

# 按服务过滤
grep "DHCP" ~/.pxelab/logs/pxelab.log
```

---

## 网络诊断

Web UI 内置网络诊断工具：
- **Ping** — 测试网络连通性
- **Traceroute** — 追踪路由路径
- 支持流式输出，实时显示结果

API：
```bash
# Ping
curl -X POST http://localhost:8080/api/v1/network/ping \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1", "count": 4}'

# Traceroute
curl -X POST http://localhost:8080/api/v1/network/traceroute \
  -H "Content-Type: application/json" \
  -d '{"host": "192.168.1.1"}'
```

---

## 常见问题（FAQ）

### Q: PxeLab 需要什么权限？

**A**: Linux/macOS 需要 root 权限（DHCP 端口 67 是特权端口）。Windows 需要管理员权限。

### Q: 可以同时运行多个 DHCP 服务器吗？

**A**: 可以。使用 `proxy` 模式叠加到现有 DHCP 环境，或使用 `hybrid` 模式智能分流。

### Q: 支持哪些操作系统安装？

**A**: 通过 iPXE 引导支持：
- Linux：Ubuntu、Debian、CentOS、Fedora、Arch、Gentoo 等 50+ 发行版
- Windows：通过 WDS 仿真或 wimboot
- BSD：FreeBSD、OpenBSD、NetBSD
- Live CD：Kali、GParted、SystemRescue 等

### Q: 如何自定义引导菜单？

**A**: 三种方式：
1. **默认菜单** — 侧边栏底部 **设置 → Boot Menu** → 默认引导菜单
2. **Profile** — 为主机创建专用引导配置
3. **自定义脚本** — 侧边栏底部 **设置 → Netboot → 自定义 iPXE 脚本**（高级用户）

### Q: 数据存储在哪里？

**A**: 默认在 `~/.pxelab/`：
- `config.yaml` — 配置文件
- `pxelab.db` — SQLite 数据库
- `boot/` — 引导文件
- `netboot/` — 网络启动目录
- `logs/` — 日志文件

### Q: 如何备份数据？

**A**: 备份 `~/.pxelab/` 整个目录即可。核心数据在 `pxelab.db` 和 `config.yaml`。

### Q: 支持 Docker 部署吗？

**A**: 暂不支持，在 Roadmap 中。当前推荐直接运行二进制。

### Q: 如何升级 PxeLab？

**A**:
1. 停止当前运行的 PxeLab
2. 下载新版本二进制替换旧文件
3. 数据目录 `~/.pxelab/` 无需变动，新版本自动迁移
4. 重新启动

### Q: 多网卡如何配置？

**A**: 在 `config.yaml` 的 `interfaces` 部分配置多个接口，每个接口独立设置 DHCP 模式和子网。
