# Architecture Overview

> PxeLab's core positioning, features, and service architecture.

**Docs**: [Getting Started](getting-started.md) | [DHCP Config](guides/dhcp.md) | [Boot Config](guides/boot-config.md)

---

## Overview

**PxeLab** is an all-in-one PXE network boot server that integrates DHCP, TFTP, HTTP, DNS, and NFS into a single binary, managed via Web UI and REST API.

### Core Positioning

| Dimension | Description |
|-----------|-------------|
| **Product Form** | Single binary + embedded Web UI, ready out of the box |
| **Target Users** | IT ops, IDC engineers, system administrators |
| **Core Scenarios** | Batch deployment, diskless workstations, OS installation, server maintenance |
| **Tech Stack** | Go 1.23+ / React 19 / TypeScript / Tailwind CSS 4 / SQLite |
| **Platform Support** | Windows / Linux / macOS |

### Comparison with Other Solutions

| Feature | PxeLab | Traditional PXE (TFTP-only) | Foreman/Cobbler |
|---------|--------|---------------------------|-----------------|
| Setup complexity | Single binary, zero deps | Manual multi-service config | Heavy dependencies |
| iPXE support | Built-in custom compilation | Self-compile required | Self-integration required |
| Multi-arch | 11 CPU architectures + Secure Boot | Usually x86 only | Limited |
| Web management | Built-in, full-featured | None | Yes, but complex |
| DHCP modes | full / proxy / hybrid / off | Usually one mode | Limited |
| NFS | Built-in NFSv3 | External needed | External needed |

---

## Features

### Network Services

- **DHCP Server** — Supports full / proxy / hybrid / off modes, per-interface configuration
- **ProxyDHCP** — Port 4011, overlays onto existing DHCP environments
- **TFTP Server** — Configurable port and timeout, serves NBP files
- **HTTP Server** — Serves boot scripts, Web UI, SPA, boot files
- **DNS Server** — Local DNS resolution + upstream forwarding, A/AAAA/CNAME records
- **NFS Server** — Built-in NFSv3, multiple mount points, IP-based access control

### Boot Capabilities

- **iPXE** — Custom-compiled, embedded boot scripts, 11 architectures
- **Secure Boot** — x86_64 and ARM64 UEFI Secure Boot support
- **Boot Types** — direct (kernel+initrd), chain (chain-load), wds (Windows WIM), sanboot (iSCSI SAN), local (local disk)
- **PXELinux** — Config parser + AST + iPXE script generator
- **Architecture Auto-detection** — Automatic boot file selection via DHCP Option 93

### Management

- **Web UI** — React SPA, dark theme, bilingual
- **REST API** — v1, full CRUD operations
- **Host Management** — CRUD, grouping, Profile binding
- **Profile** — Boot config files with script versioning, diff, rollback
- **OS Install Catalog** — 10 preset distro groups, drag-and-drop sorting
- **Answer File Templates** — Preset + custom, preview and validation
- **Install Tasks** — Track deployment progress

### Hardware Management

- **WOL** — Wake-on-LAN with scheduled tasks
- **BMC/IPMI** — Out-of-band power control (on/off/reboot/status), CSV batch import
- **OS Images** — ISO upload, mount, distro detection

### Operations Tools

- **Real-time Logs** — Multi-panel SSE log stream, filter by service
- **Audit Logs** — Track all configuration changes
- **Access Control** — MAC whitelist/blacklist
- **Network Diagnostics** — Ping / Traceroute (streaming output)
- **Prometheus Metrics** — `/api/v1/metrics` endpoint
- **Log Rotation** — Auto-rotation by size/days/backups

---

## Two-Stage Network Boot

PxeLab uses a two-stage boot architecture, progressively upgrading from the limited PXE ROM to the full-featured iPXE:

```
Stage 1                           Stage 2
┌─────────────┐   TFTP/HTTP    ┌──────────┐   HTTP     ┌──────────────┐
│  PXE ROM    ├───────────────►│  iPXE    ├───────────►│  Boot Menu   │
│  (BIOS/UEFI)│  undionly.kpxe │  (custom │  /boot/    │  (kernel+initrd│
│             │  /ipxe.efi     │   build) │  ipxe/     │   /WIM/Chain │
└─────────────┘                │          │  script    │   /Local)    │
                               └──────────┘           └──────────────┘
```

**Stage 1: PXE ROM → iPXE**

1. Client PXE ROM sends DHCP Discover
2. PxeLab DHCP responds with Offer/Ack containing:
   - IP address (full/hybrid mode)
   - next-server (TFTP server address)
   - bootfile (NBP filename, e.g., `ipxe.efi`)
   - Option 175.178 (iPXE boot script URL)
3. Client downloads NBP via TFTP
4. PXE ROM loads and executes NBP → iPXE starts

**Stage 2: iPXE → Boot Menu**

1. iPXE embedded script auto-executes: `dhcp` → `chain http://server:8080/boot/ipxe/script?mac=xx`
2. PxeLab looks up the host's bound Profile by MAC address
3. Returns the corresponding boot menu script
4. Client displays menu, user selects boot entry

---

## Service Architecture

```
┌─────────────────────────────────────────────────────┐
│                    PxeLab Binary                     │
├──────────┬──────────┬──────────┬──────────┬─────────┤
│  HTTP    │  DHCP    │  TFTP    │  DNS     │  NFS    │
│  :8080   │  :67     │  :69     │  :53     │  :2049  │
│  TCP     │  UDP     │  UDP     │  UDP     │  TCP    │
├──────────┴──────────┴──────────┴──────────┴─────────┤
│              Service Manager (Lifecycle)             │
├─────────────────────────────────────────────────────┤
│  SQLite (pxelab.db)  │  Event Bus  │  Log Bus      │
├─────────────────────────────────────────────────────┤
│              Config (config.yaml)                    │
└─────────────────────────────────────────────────────┘
```

| Service | Default Port | Protocol | Auto-start | Description |
|---------|-------------|----------|------------|-------------|
| HTTP | 8080 | TCP | ✅ | Web UI + API + boot file serving |
| DHCP | 67 | UDP | Config-dependent | IP assignment + PXE options |
| ProxyDHCP | 4011 | UDP | Config-dependent | PXE options only (overlay mode) |
| TFTP | 69 | UDP | ❌ | NBP file transfer |
| DNS | 53 | UDP | ❌ | Local DNS resolution |
| NFS | 2049 | TCP | ❌ | NFSv3 file sharing |

---

## Deployment Modes

### Server Mode (Default)

```bash
pxelab --mode server
# or
pxelab   # defaults to server mode
```

- Foreground run, logs to stderr
- Suitable for server deployment, background running
- Can be managed via systemd on Linux

### App Mode

```bash
pxelab --mode app
```

- Auto-opens browser
- Runs as system tray on Windows (hides console window)
- Suitable for desktop environments, personal use

### Windows System Tray

On Windows, PxeLab automatically enables system tray mode when a desktop environment is detected:

- Right-click tray icon: Open browser / Open data directory / Exit
- Tray icon shows service running status
- Closing the browser doesn't stop the service — exit via tray
