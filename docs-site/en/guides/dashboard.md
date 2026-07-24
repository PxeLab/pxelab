# Dashboard

> Home dashboard provides a global overview, auto-refreshing every 5 seconds.

**Docs**: [Getting Started](../getting-started.md) | [Host Management](host-management.md) | [Service Config](services.md)

---

## Statistics Cards

Top row shows 5 core metrics:

| Card | Data Source | Description |
|------|------------|-------------|
| Online Hosts | `hosts.last_online` | Hosts with records in last 10 minutes |
| Running Services | `GET /api/v1/services` | Services with status=running |
| Active Leases | Prometheus `dhcp.activeLeases` | Current DHCP active leases |
| DNS Records | `GET /api/v1/dns/records` | Total DNS records |
| Today's Boots | events (type=BOOT, today) | Boot events today |

## Service Status Bar

Shows running status (green/yellow/red dot) and port for DHCP / TFTP / HTTP / DNS / NFS services.

## Charts Area

- **Traffic Trend** — TFTP / HTTP / NFS bandwidth, switchable between 5m / 30m / 1h time ranges
- **HTTP Status Codes** — Pie chart showing 2xx / 3xx / 4xx / 5xx distribution
- **DHCP Architecture Distribution** — Pie chart showing client architectures (x86 BIOS / EFI x86-64 / ARM64, etc.)
- **Recent Events** — Latest 12 events with type tags (DHCP / TFTP / HTTP / BOOT / IPMI / DNS)

## Online Hosts List

Shows recently booted hosts with online status indicator.
