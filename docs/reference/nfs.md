# NFS 服务

> 内置 NFSv3 服务器，多挂载点与 IP 访问控制。

**相关文档**: [网络启动目录](../guides/netboot.md) | [DNS 服务](dns.md)

---

## 多挂载点

PxeLab 内置 NFSv3 服务器，支持多个独立挂载点：

```yaml
nfs:
  enabled: true
  port: 2049
  mount_points:
    - label: "ISOs"
      export_path: /isos          # 客户端挂载路径
      local_dir: /data/isos       # 本地目录
      read_only: true
      allow_ips:
        - "10.0.0.0/24"

    - label: "Installs"
      export_path: /installs
      local_dir: /data/installs
      read_only: false
      allow_ips:
        - "192.168.1.0/24"
```

每个挂载点独立配置：
- **标签** — 显示名称
- **导出路径** — 客户端挂载别名
- **本地目录** — 服务器本地目录
- **只读** — 权限控制
- **IP/CIDR 白名单** — 访问控制

Web UI：**基础配置 → 服务配置 → NFS**

---

## IP 访问控制

- `allow_ips` 为空 = 不限制，所有客户端可挂载
- 支持 IP 地址和 CIDR 网段
- 内嵌 rpcbind（端口 111/UDP+TCP），自动注册 NFSv3/MOUNT 端口映射
- 版本感知 TCP 监听器：拦截 NFSv4 连接并回复 PROG_MISMATCH，强制回退到 v3
