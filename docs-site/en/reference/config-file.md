# Config File Reference

> Complete structure and CLI parameters for PxeLab configuration files.

**Related**: [Getting Started](../getting-started.md) | [Deployment](../guides/deployment.md)

---

## Config File Location

PxeLab searches for config files in the following order:

1. Path specified by `--config` parameter
2. Current directory `./config.yaml`
3. Data directory `~/.pxelab/config.yaml`
4. `/etc/pxelab/config.yaml`

---

## Complete Config Structure

```yaml
# ── Global Settings ──
global:
  listen_addr: ":8080"           # HTTP listen address
  data_dir: ""                   # Data directory (default ~/.pxelab/)
  server_name: "PxeLab"          # Server name
  app_mode: false                # App mode (auto-open browser)

# ── Interface Config ──
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
  upstream: "8.8.8.8"            # Upstream DNS
  local_domain: "pxelab.local"   # Local domain

# ── NFS ──
nfs:
  enabled: false
  port: 2049
  mount_points:
    - label: "Default"
      export_path: "/"
      local_dir: ""              # Default ~/.pxelab/boot/isos
      read_only: true
      allow_ips: []

# ── Boot Config ──
boot:
  root_dir: ""                   # Boot file directory (default ~/.pxelab/boot/)
  arch_map:                      # Architecture mapping (usually auto-managed)
    - arch_code: 0
      arch_name: "Intel x86PC"
      nbp: "ipxe"
      ipxe: "ipxe.pxe"
    # ... more architectures

# ── Netboot ──
netboot:
  enabled: true
  failsafe_prompt: true
  script_template: ""            # Custom iPXE script (leave empty for visual config)
  sync:
    url: "https://github.com/netbootxyz/netboot.xyz.git"
    repo: "contrib/netboot.xyz"
  boot:
    default_menu:
      title: "PxeLab Boot Menu"
      timeout: 5000              # Milliseconds, 0=no auto-select
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
        # ... more groups

# ── Service Auto-start ──
service_auto_start:
  http: true
  tftp: false
  dns: false
  nfs: false

# ── Storage ──
store:
  dsn: ""                        # SQLite path (default ~/.pxelab/pxelab.db)

# ── Logging ──
log:
  level: info                    # debug / info / warn / error
  format: text
  file: ""                       # Log file path (empty=default ~/.pxelab/logs/)
  max_size_mb: 0                 # Max file size (MB), 0=unlimited
  max_backups: 0                 # Number of rotated files to keep
  max_age_days: 0                # Retention days
  compress: false                # gzip compress old logs
  cleanup_interval: 0            # Cleanup check interval (hours)

# ── Access Control Seeds ──
blacklist_seeds:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "Decommissioned device"

whitelist_seeds:
  - mac: "11:22:33:44:55:66"
    subnet: "10.0.0.0/24"
    reason: "Server zone"
```

---

## CLI Parameters

```
pxelab [flags]

Flags:
  --config string       Config file path
  --data-dir string     Data directory (default "~/.pxelab/")
  --log-level string    Log level: debug/info/warn/error (default "info")
  --mode string         Run mode: app/server ("app" auto-opens browser)
```
