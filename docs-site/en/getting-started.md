# Getting Started

> Installing and first run of PxeLab.

**Docs**: [Architecture](architecture.md) | [Deployment](guides/deployment.md) | [Troubleshooting](troubleshooting.md)

---

## System Requirements

| Item | Requirement |
|------|-------------|
| **OS** | Windows 10+ / Linux (kernel 3.10+) / macOS 12+ |
| **Architecture** | amd64 / arm64 |
| **Memory** | ≥ 512 MB |
| **Disk** | ≥ 1 GB free space |
| **Network** | Admin/root privileges required (DHCP port 67) |
| **Dependencies** | None (single binary, all files embedded) |

---

## Installation

### Option 1: Download Release (Recommended)

Download the binary for your platform from GitHub Releases:

```bash
# Linux amd64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_linux_amd64.tar.gz
tar xzf pxelab_linux_amd64.tar.gz

# macOS arm64
wget https://github.com/user/pxelab/releases/latest/download/pxelab_darwin_arm64.tar.gz
tar xzf pxelab_darwin_arm64.tar.gz

# Windows
# Download pxelab_windows_amd64.zip, extract to get pxelab.exe
```

### Option 2: Build from Source

```bash
git clone https://github.com/user/pxelab.git
cd pxelab

make build          # Generates bin/pxelab
make frontend       # cd web && npm ci && npm run build (optional, already embedded)
make release        # goreleaser release --clean
```

### Option 3: Docker (Planned)

> Docker containerized deployment is on the roadmap, not yet implemented.

---

## First Run

### Linux / macOS

```bash
# Root required (DHCP port 67 needs privileges)
sudo ./bin/pxelab

# Or specify data directory
sudo ./bin/pxelab --data-dir /opt/pxelab-data

# Or App mode (auto-opens browser)
sudo ./bin/pxelab --mode app
```

### Windows

```cmd
# Run as Administrator
pxelab.exe

# Or App mode
pxelab.exe --mode app
```

> **Windows System Tray**: On Windows, PxeLab runs in system tray mode by default, hiding the console window. The tray icon provides access to open browser, view data directory, and exit.

### First Start Behavior

1. Auto-creates data directory `~/.pxelab/`
2. Initializes SQLite database `~/.pxelab/pxelab.db`
3. Generates default config `~/.pxelab/config.yaml`
4. Extracts embedded boot files to `~/.pxelab/boot/`
5. Extracts embedded netboot catalog to `~/.pxelab/netboot/`
6. Starts HTTP server (port 8080)
7. Auto-opens browser (in `--mode app`)

---

## Verify Services

After starting, verify service status at:

| URL | Purpose |
|-----|---------|
| `http://localhost:8080` | Web Management UI |
| `http://localhost:8080/api/v1/status` | Service status JSON |
| `http://localhost:8080/api/v1/metrics` | Prometheus metrics |
| `http://localhost:8080/boot/ipxe/script` | iPXE boot script |

```bash
curl -s http://localhost:8080/api/v1/status | python -m json.tool
```
