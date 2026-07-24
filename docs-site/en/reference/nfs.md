# NFS Service

> Built-in NFSv3 server with multiple mount points and IP access control.

**Related**: [Netboot Catalog](../guides/netboot.md) | [DNS Service](dns.md)

---

## Multiple Mount Points

PxeLab has a built-in NFSv3 server supporting multiple independent mount points:

```yaml
nfs:
  enabled: true
  port: 2049
  mount_points:
    - label: "ISOs"
      export_path: /isos          # Client mount path
      local_dir: /data/isos       # Local directory
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

Each mount point is independently configured:
- **Label** — Display name
- **Export path** — Client mount alias
- **Local directory** — Server local directory
- **Read-only** — Permission control
- **IP/CIDR whitelist** — Access control

Web UI: **Service Config → NFS**

---

## IP Access Control

- `allow_ips` empty = no restrictions, all clients can mount
- Supports IP addresses and CIDR subnets
- Built-in rpcbind (port 111/UDP+TCP), auto-registers NFSv3/MOUNT port mappings
- Version-aware TCP listener: intercepts NFSv4 connections and replies PROG_MISMATCH, forcing fallback to v3
