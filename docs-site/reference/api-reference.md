# REST API 参考

> PxeLab REST API v1 的完整端点列表与使用约定。

**相关文档**: [配置文件参考](config-file.md) | [Web UI 指南](../guides/web-ui.md)

---

## 通用约定

**Base URL**: `http://<host>:8080/api/v1`

**请求格式**: JSON (`Content-Type: application/json`)

**响应格式**:

```json
{
  "success": true,
  "data": { ... }
}
```

错误响应：

```json
{
  "success": false,
  "error": "错误描述"
}
```

---

## 认证

```
POST /api/v1/auth/login     # 登录
POST /api/v1/auth/logout    # 登出
GET  /api/v1/auth/session   # 检查会话
```

会话通过 Cookie 维持。

---

## 端点列表

### 认证

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/auth/login` | 登录 |
| POST | `/auth/logout` | 登出 |
| GET | `/auth/session` | 检查会话 |

### 主机

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/hosts` | 列出主机 |
| POST | `/hosts` | 创建主机 |
| GET | `/hosts/{id}` | 获取主机 |
| PUT | `/hosts/{id}` | 更新主机 |
| DELETE | `/hosts/{id}` | 删除主机 |
| POST | `/hosts/{id}/wake` | WOL 唤醒 |
| POST | `/hosts/{id}/power` | IPMI 电源控制 |
| GET | `/hosts/{id}/boot-config` | 预览引导配置 |
| POST | `/hosts/batch/wake` | 批量唤醒 |

### 引导配置（Profile）

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/profiles` | 列出 Profile |
| POST | `/profiles` | 创建 Profile |
| GET | `/profiles/{id}` | 获取 Profile |
| PUT | `/profiles/{id}` | 更新 Profile |
| DELETE | `/profiles/{id}` | 删除 Profile |
| POST | `/profiles/from-netboot` | 从 Netboot 创建 |
| GET | `/profiles/{id}/script-versions` | 脚本版本列表 |
| GET | `/profiles/{id}/script-diff/{verId}` | 版本差异 |
| POST | `/profiles/{id}/script-rollback/{verId}` | 版本回滚 |

### 文件管理

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/files` | 列出文件 |
| GET | `/files/root` | 获取根目录 |
| POST | `/files/upload` | 上传文件 |
| DELETE | `/files` | 删除文件 |

### 租约

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/leases` | 列出租约 |
| GET | `/leases/stats` | 租约统计 |
| DELETE | `/leases/{mac}` | 删除租约 |
| POST | `/leases/batch-delete` | 批量删除 |
| POST | `/leases/prune` | 清理过期租约 |

### 设置

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/settings/general` | 通用设置 |
| PUT | `/settings/general` | 更新通用设置 |
| GET | `/settings/interfaces` | 接口配置 |
| PUT | `/settings/interfaces` | 更新接口配置 |
| GET | `/settings/netboot` | Netboot 设置 |
| PUT | `/settings/netboot` | 更新 Netboot 设置 |
| GET | `/settings/logging` | 日志设置 |
| PUT | `/settings/logging` | 更新日志设置 |

### 服务

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/services/tftp` | TFTP 设置 |
| PUT | `/services/tftp` | 更新 TFTP |
| GET | `/services/dhcp` | DHCP 设置 |
| PUT | `/services/dhcp` | 更新 DHCP |
| GET | `/services/dns` | DNS 设置 |
| PUT | `/services/dns` | 更新 DNS |
| GET | `/services/nfs` | NFS 设置 |
| PUT | `/services/nfs` | 更新 NFS |
| GET | `/services/archmap` | 架构映射 |
| PUT | `/services/archmap` | 更新架构映射 |
| GET | `/services/archmap/defaults` | 默认架构映射 |
| GET | `/services/ipxe-script` | iPXE 脚本 |
| PUT | `/services/ipxe-script` | 更新 iPXE 脚本 |

### 服务生命周期

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/services` | 列出所有服务 |
| POST | `/services/{name}/start` | 启动服务 |
| POST | `/services/{name}/stop` | 停止服务 |
| POST | `/services/{name}/restart` | 重启服务 |
| POST | `/services/batch/{action}` | 批量操作 |
| PUT | `/services/{name}/auto-start` | 设置自动启动 |

### Netboot

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/netboot/catalog` | OS 目录 |
| GET | `/netboot/catalog/{distro}` | 发行版详情 |
| GET | `/netboot/groups` | 分组列表 |
| GET | `/netboot/check-files` | 检查文件 |
| GET | `/netboot/cache-stats` | 缓存统计 |
| GET/PUT/DELETE | `/netboot/overlays/{distro}` | 覆盖层 |
| GET/POST/PUT/DELETE | `/netboot/answer-templates` | 应答文件模板 |
| GET/POST/PUT/DELETE | `/netboot/tasks` | 安装任务 |

### 访问控制

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST/DELETE | `/access/blacklist` | 黑名单 |
| GET/POST/DELETE | `/access/whitelist` | 白名单 |
| GET | `/access/unauthorized` | 未授权设备 |

### DNS

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/dns/records` | DNS 记录 |
| GET/PUT/DELETE | `/dns/records/{id}` | 单条记录 |

### BMC

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/bmc/configs` | BMC 配置 |
| POST | `/bmc/configs/import` | CSV 导入 |
| POST | `/bmc/probe` | 探测 BMC |
| POST | `/bmc/{id}/power-on` | 开机 |
| POST | `/bmc/{id}/power-off` | 关机 |
| POST | `/bmc/{id}/restart` | 重启 |
| GET | `/bmc/{id}/status` | 查询状态 |
| POST | `/bmc/{id}/boot-device` | 设置启动设备 |
| POST | `/bmc/batch/power-on` | 批量开机 |
| POST | `/bmc/batch/power-off` | 批量关机 |
| POST | `/bmc/batch/restart` | 批量重启 |
| POST | `/bmc/batch/status` | 批量查询 |

### DHCP 预留

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/dhcp/reservations` | 预留列表/创建 |
| GET/PUT/DELETE | `/dhcp/reservations/{id}` | 单条操作 |

### 网络诊断

| 方法 | 端点 | 说明 |
|------|------|------|
| POST | `/network/ping` | Ping |
| POST | `/network/ping/stream` | 流式 Ping |
| POST | `/network/traceroute` | Traceroute |
| POST | `/network/traceroute/stream` | 流式 Traceroute |
| GET | `/network/interfaces` | 网络接口列表 |

### WOL

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST/DELETE | `/wol/history` | 唤醒历史 |
| GET/POST/DELETE | `/wol/schedules` | 定时任务 |
| GET | `/wol/interfaces` | WOL 接口 |

### OS 镜像

| 方法 | 端点 | 说明 |
|------|------|------|
| GET/POST | `/os-images` | 镜像列表/上传 |
| GET/PUT/DELETE | `/os-images/{id}` | 单条操作 |
| POST | `/os-images/{id}/extract` | 解压 |
| POST | `/os-images/{id}/mount` | 挂载 |
| POST | `/os-images/{id}/unmount` | 卸载 |
| GET | `/fs/browse` | 文件浏览 |

### 其他

| 方法 | 端点 | 说明 |
|------|------|------|
| GET | `/status` | 服务状态 |
| GET | `/metrics` | Prometheus 指标 |
| GET | `/events` | 事件列表 |
| GET | `/events/stream` | 事件流（SSE） |
| GET | `/logs/stream` | 日志流（SSE） |
| GET | `/logs/files` | 日志文件列表 |
| GET | `/logs/disk-usage` | 日志磁盘占用 |
| POST | `/logs/cleanup` | 清理日志 |
| GET | `/interfaces` | 网络接口 |
| GET | `/bootloader/check` | 引导文件检查 |
| GET | `/bootloader/files` | 引导文件列表 |
