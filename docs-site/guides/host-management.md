# 主机管理

> 设备管理、WOL 网络唤醒与 BMC/IPMI 带外管理。

**相关文档**: [Web UI 指南](web-ui.md) | [REST API 参考](../reference/api-reference.md)

---

## 主机 CRUD

管理网络中的设备：

```
GET    /api/v1/hosts                # 列出所有主机
POST   /api/v1/hosts                # 创建主机
GET    /api/v1/hosts/{id}           # 获取主机详情
PUT    /api/v1/hosts/{id}           # 更新主机
DELETE /api/v1/hosts/{id}           # 删除主机
```

每个主机可配置：
- 名称、描述
- MAC 地址
- 绑定 Profile（引导配置）
- IP 地址
- 分组

---

## WOL 网络唤醒

通过 Wake-on-LAN 远程开机：

```
POST /api/v1/hosts/{id}/wake       # 唤醒单台主机
POST /api/v1/hosts/batch/wake      # 批量唤醒
GET  /api/v1/wol/history           # 唤醒历史
POST /api/v1/wol/schedule          # 创建定时唤醒
GET  /api/v1/wol/schedules         # 列出定时任务
```

定时调度功能：
- 指定 MAC 地址和唤醒时间
- 支持一次性或周期性调度
- 唤醒历史记录

---

## BMC/IPMI 带外管理

通过 IPMI 协议远程管理服务器电源：

```
POST /api/v1/bmc/probe              # 探测 BMC
POST /api/v1/bmc/{id}/power-on      # 开机
POST /api/v1/bmc/{id}/power-off     # 关机
POST /api/v1/bmc/{id}/restart       # 重启
GET  /api/v1/bmc/{id}/status        # 查询状态
POST /api/v1/bmc/{id}/boot-device   # 设置启动设备
```

支持批量操作：
```
POST /api/v1/bmc/batch/power-on     # 批量开机
POST /api/v1/bmc/batch/power-off    # 批量关机
POST /api/v1/bmc/batch/restart      # 批量重启
POST /api/v1/bmc/batch/status       # 批量查询状态
```

CSV 批量导入 BMC 配置：`POST /api/v1/bmc/configs/import`
