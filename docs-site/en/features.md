# Features

> Complete feature list of PxeLab.

**Docs**: [Product Overview](/en/product) | [Advantages](/en/advantages) | [Architecture](/en/architecture)

---

## Network Services

| Service | Port | Description |
|---------|------|-------------|
| **DHCP** | 67 | Supports full / proxy / hybrid / off modes, per-interface config |
| **ProxyDHCP** | 4011 | Overlays onto existing DHCP, no client config changes |
| **TFTP** | 69 | Configurable port and timeout, serves NBP files |
| **HTTP** | 8080 | Serves boot scripts, Web UI, SPA, boot files |
| **DNS** | 53 | Local resolution + upstream forwarding, A/AAAA/CNAME records |
| **NFS** | 2049 | Built-in NFSv3, multiple mount points, IP-based access control |

---

## Boot Capabilities

### iPXE Support

- Custom-compiled iPXE with embedded boot scripts
- 11 CPU architectures: x86 BIOS, EFI x86-64, ARM64, etc.
- Secure Boot support (x86_64 and ARM64)
- Auto architecture detection via DHCP Option 93

### Boot Types

| Type | Description |
|------|-------------|
| **direct** | Directly load kernel + initrd |
| **chain** | Chain-load to another iPXE script |
| **wds** | Windows WDS deployment (WIM files) |
| **sanboot** | iSCSI SAN boot |
| **local** | Local disk boot |

### PXELinux Compatibility

- pxelinux.cfg format parser + AST
- Auto-generates iPXE scripts
- Seamless migration to iPXE

---

## Management Features

### Web UI

- React 19 SPA, dark/light theme
- Bilingual (Chinese/English)
- Real-time data refresh (SSE event stream)
- Responsive design, mobile-friendly

### REST API

- v1 with full CRUD operations
- JSON request/response format
- Cookie-based session authentication
- `/api/v1/metrics` Prometheus metrics endpoint

### Host Management

- Host CRUD, grouping
- Profile binding (boot configuration)
- MAC address management
- Online status tracking

### Profile Management

- Boot configuration files with script versioning
- Diff comparison, rollback
- Supports direct / chain / wds / sanboot / local types

### OS Install Catalog

- 10 preset Linux distro groups
- Drag-and-drop sorting
- Custom groups and entries
- Answer file template integration

### Answer File Templates

- Preset + custom templates
- Preview and validation
- Variable substitution

---

## Hardware Management

| Feature | Description |
|---------|-------------|
| **WOL** | Wake-on-LAN with scheduled tasks |
| **BMC/IPMI** | Out-of-band power control (on/off/reboot/status) |
| **CSV Import** | Batch BMC info import |
| **OS Images** | ISO upload, mount, distro detection, mount tracking |

---

## Operations Tools

| Tool | Description |
|------|-------------|
| **Real-time Logs** | Multi-panel SSE log stream, filter by service |
| **Audit Logs** | Track all configuration changes |
| **Access Control** | MAC whitelist/blacklist |
| **Network Diagnostics** | Ping / Traceroute (streaming output) |
| **Prometheus Metrics** | Full performance and status metrics |
| **Log Rotation** | Auto-rotation by size/days/backups |
| **System Tray** | Background mode with tray icon |

---

## Deployment Modes

| Mode | Description |
|------|-------------|
| **Server** | Standalone server mode, for production |
| **App** | Application mode, for dev and testing |
| **systemd** | Linux service mode, auto-start on boot |
