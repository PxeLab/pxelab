# Logging Configuration

> Log system configuration, rotation, and troubleshooting for PxeLab.

**Related**: [Config File Reference](config-file.md) | [Troubleshooting](../troubleshooting.md)

---

## Log Levels

| Level | Description | Use Case |
|-------|-------------|----------|
| `debug` | Detailed debug info | Development debugging, issue investigation |
| `info` | Normal operation info (default) | Production |
| `warn` | Warning messages | Needs attention but doesn't affect operation |
| `error` | Error messages | Issues requiring fix |

---

## Configuration

Configure in the `log` section of `config.yaml`:

```yaml
log:
  level: info                    # Log level
  format: text                   # text / json
  file: ""                       # Log file path (empty=default ~/.pxelab/logs/)
  max_size_mb: 0                 # Max file size (MB), 0=unlimited
  max_backups: 0                 # Number of rotated files to keep
  max_age_days: 0                # Retention days
  compress: false                # gzip compress old logs
  cleanup_interval: 0            # Cleanup check interval (hours)
```

Web UI: **Sidebar bottom → Settings → Log Management**

---

## Log Files

- Default path: `~/.pxelab/logs/pxelab.log`
- Format: Text (one log entry per line)
- Contains: timestamp, level, service name, message

---

## Log Rotation

PxeLab has built-in log rotation supporting:

- **Size-based rotation**: Auto-rotate when file exceeds `max_size_mb`
- **Age-based cleanup**: Auto-delete logs older than `max_age_days`
- **Backup limit**: Keep up to `max_backups` rotated files
- **gzip compression**: Auto-compress old logs when `compress: true`

---

## Log Troubleshooting

```bash
# View logs in real-time
tail -f ~/.pxelab/logs/pxelab.log

# Filter by service
grep "DHCP" ~/.pxelab/logs/pxelab.log
grep "TFTP" ~/.pxelab/logs/pxelab.log
grep "HTTP" ~/.pxelab/logs/pxelab.log

# Filter by level
grep "ERROR" ~/.pxelab/logs/pxelab.log

# Web UI
# Sidebar → Monitoring → Logs (real-time SSE stream, filterable by service)
```

---

## Prometheus Metrics

PxeLab provides Prometheus metrics endpoint:

```
GET /api/v1/metrics
```

Included service metrics:
- DHCP active leases
- HTTP requests (by status code)
- TFTP transfers
- DNS queries
