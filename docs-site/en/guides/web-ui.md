# Web UI Guide

> PxeLab Web management interface navigation and page layout.

**Docs**: [Dashboard](dashboard.md) | [Service Config](services.md) | [Host Management](host-management.md)

---

Page path: `/` (root path)

## Main Layout

PxeLab uses a standard layout:

- **Top Navigation Bar**
  - Left: Logo + PxeLab title
  - Center: Search bar (⌘K to activate)
  - Right: Theme toggle (light/dark/system) + language switch + user dropdown (Change password, Sign out)
- **Left Sidebar**: Vertical navigation menu
- **Main Content Area**

## Sidebar Navigation

| Menu | Page Path | Content |
|------|-----------|---------|
| **Overview** | | |
| └ Dashboard | `/dashboard` | Statistics, charts, service status, online hosts |
| **Configuration** | | |
| └ Service Config | `/services/dhcp` | DHCP/DNS/NFS/TFTP services |
| └ Boot Config | `/boot-config` | iPXE scripts, Profiles, default boot menu |
| └ Netboot Catalog | `/netboot-catalog` | OS install catalog, groups, cache |
| └ Files | `/files` | Boot file management |
| **Management** | | |
| └ Hosts | `/hosts` | Device list and management |
| └ Profiles | `/profiles` | Boot configuration files (Profiles) |
| └ Answer Templates | `/answer-templates` | Automated installation answer files |
| └ Install Tasks | `/install-tasks` | Network install task management |
| └ Access Control | `/access-control` | MAC whitelist/blacklist/unauthorized devices |
| └ OS Images | `/os-images` | ISO upload, mount, extraction |
| └ Network Diagnostics | `/diagnostics` | Service detection, conflict diagnosis |
| └ BMC | `/bmc` | IPMI out-of-band management |
| └ WOL | `/wol` | Wake-on-LAN wake/scheduled tasks |
| **Monitoring** | | |
| └ Monitoring | `/monitoring` | Prometheus metrics, alerts, config |
| **Global** | | |
| └ Settings | `/settings` | Network, services, OS install, language, advanced |
| └ Deployment | `/deployment` | Release and update management |
