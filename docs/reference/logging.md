# 日志配置

> PxeLab 日志系统的配置、轮转与排查。

**相关文档**: [配置文件参考](config-file.md) | [故障排查](../troubleshooting.md)

---

## 日志级别

| 级别 | 说明 | 适用场景 |
|------|------|---------|
| `debug` | 详细调试信息 | 开发调试、问题排查 |
| `info` | 正常运行信息（默认） | 生产环境 |
| `warn` | 警告信息 | 需要关注但不影响运行 |
| `error` | 错误信息 | 需要修复的问题 |

---

## 配置项

在 `config.yaml` 的 `log` 部分配置：

```yaml
log:
  level: info                    # 日志级别
  format: text                   # text / json
  file: ""                       # 日志文件路径（空=默认 ~/.pxelab/logs/）
  max_size_mb: 0                 # 单文件最大体积 (MB)，0=不限制
  max_backups: 0                 # 保留轮转文件数
  max_age_days: 0                # 保留天数
  compress: false                # gzip 压缩旧日志
  cleanup_interval: 0            # 清理检查间隔（小时）
```

Web UI：**侧边栏底部 → 设置 → 日志管理**

---

## 日志文件

- 默认路径：`~/.pxelab/logs/pxelab.log`
- 格式：文本（每行一条日志）
- 包含：时间戳、级别、服务名、消息

---

## 日志轮转

PxeLab 内置日志轮转功能，支持：

- **按大小轮转**：单文件超过 `max_size_mb` 时自动轮转
- **按天数保留**：超过 `max_age_days` 天的日志自动清理
- **备份数限制**：最多保留 `max_backups` 个轮转文件
- **gzip 压缩**：`compress: true` 时自动压缩旧日志

---

## 日志排查

```bash
# 实时查看日志
tail -f ~/.pxelab/logs/pxelab.log

# 按服务过滤
grep "DHCP" ~/.pxelab/logs/pxelab.log
grep "TFTP" ~/.pxelab/logs/pxelab.log
grep "HTTP" ~/.pxelab/logs/pxelab.log

# 按级别过滤
grep "ERROR" ~/.pxelab/logs/pxelab.log

# Web UI
# 侧边栏 → 监控 → 日志（实时 SSE 流，支持按服务过滤）
```

---

## Prometheus 指标

PxeLab 提供 Prometheus 指标端点：

```
GET /api/v1/metrics
```

包含的服务指标：
- DHCP 活跃租约数
- HTTP 请求数（按状态码）
- TFTP 传输量
- DNS 查询数
