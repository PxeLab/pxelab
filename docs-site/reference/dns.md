# DNS 服务

> 本地 DNS 解析、上游转发与记录管理。

**相关文档**: [DHCP 配置](../guides/dhcp.md) | [NFS 服务](nfs.md)

---

## 本地 DNS

PxeLab 内置 DNS 服务器，支持：

- **上游转发** — 未匹配的查询转发到上游 DNS
- **本地解析** — A / AAAA / CNAME 记录
- **子网感知** — 根据客户端来源子网返回对应网段的服务器 IP
- **自动记录** — 启动时自动创建 `@` A 记录和服务器名称 A 记录

| 配置项 | 默认值 | 说明 |
|--------|--------|------|
| 端口 | 53 | UDP |
| 本地域名 | pxelab.local | 后缀域名 |
| 上游 DNS | 系统默认 | 转发目标 |

Web UI：**基础配置 → 服务配置 → DNS**

---

## DNS 记录管理

通过 Web UI 或 API 管理 DNS 记录：

```
GET    /api/v1/dns/records          # 列出所有记录
POST   /api/v1/dns/records          # 创建记录
GET    /api/v1/dns/records/{id}     # 获取单条记录
PUT    /api/v1/dns/records/{id}     # 更新记录
DELETE /api/v1/dns/records/{id}     # 删除记录
```

支持记录类型：A、AAAA、CNAME
