# FAQ

> Common questions and answers about using PxeLab.

**Docs**: [Getting Started](/en/getting-started) | [Troubleshooting](/en/troubleshooting) | [Config Reference](/en/reference/config-file)

---

## Installation & Startup

### Q: What operating systems does PxeLab support?

**A:** PxeLab supports the following platforms:

| OS | Architecture | Notes |
|----|-------------|-------|
| Linux | amd64, arm64 | Recommended for production |
| Windows | amd64 | Windows 10+ required |
| macOS | arm64, amd64 | macOS 12+ required |

### Q: What dependencies do I need to install?

**A:** None. PxeLab is a single executable — download and run:

```bash
# Linux/macOS
chmod +x pxelab
./pxelab

# Windows
pxelab.exe
```

### Q: How to run as a system service?

**A:** On Linux, use systemd:

```bash
# Create service file
sudo tee /etc/systemd/system/pxelab.service << 'EOF'
[Unit]
Description=PxeLab PXE Server
After=network.target

[Service]
ExecStart=/usr/local/bin/pxelab
Restart=always
User=root

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
sudo systemctl enable --now pxelab
```

### Q: Can't access Web UI after first start?

**A:** Check the following:

1. Confirm service is running: `curl http://localhost:8080/api/v1/services`
2. Check port usage: `netstat -tlnp | grep 8080`
3. Check firewall rules for port 8080
4. Check logs: `./pxelab --log-level debug`

---

## Network Configuration

### Q: How to choose a DHCP mode?

**A:** Choose based on your network environment:

| Scenario | Recommended Mode | Notes |
|----------|-----------------|-------|
| Standalone network, no existing DHCP | **full** | PxeLab as complete DHCP server |
| Existing DHCP server | **proxy** | Overlay PXE functionality |
| Partial PXE needed | **hybrid** | Hybrid mode |
| TFTP/HTTP only | **off** | Disable DHCP, configure clients manually |

### Q: What's the difference between ProxyDHCP and Full DHCP?

**A:**

- **Full DHCP**: PxeLab acts as complete DHCP server, assigns IPs and PXE boot info
- **ProxyDHCP**: PxeLab only provides PXE boot info, IPs come from existing DHCP server

ProxyDHCP is ideal for environments with existing DHCP servers — no changes to existing DHCP config needed.

### Q: How to configure multiple NICs?

**A:** Each network interface can have independent DHCP mode:

```yaml
dhcp:
  interfaces:
    - name: eth0
      mode: full
      subnet: 192.168.1.0/24
    - name: eth1
      mode: proxy
    - name: eth2
      mode: off
```

### Q: DHCP port 67 is occupied, what to do?

**A:** Two solutions:

1. **Stop the service occupying the port** (recommended)
2. **Use ProxyDHCP mode** (port 4011, usually not conflicting)

```yaml
dhcp:
  mode: proxy  # Uses port 4011
```

---

## iPXE & Boot

### Q: What is iPXE?

**A:** iPXE is open-source network boot firmware supporting HTTP, iSCSI, AoE and more. Compared to traditional PXE:

- HTTP boot support (faster, more flexible)
- Script programming (conditionals, variables)
- More architectures and boot methods

PxeLab includes pre-compiled iPXE — no self-compilation needed.

### Q: How to customize the boot menu?

**A:** In Web UI: **Settings → Netboot → Custom iPXE Script**

```ipxe
#!ipxe
dhcp || clear
menu Choose an OS
item ubuntu Ubuntu 22.04
item centos CentOS 9
item local Boot from local disk
choose --timeout 10000 target
goto ${target}

:ubuntu
chain http://192.168.1.10:8080/netboot/ubuntu.ipxe || shell

:centos
chain http://192.168.1.10:8080/netboot/centos.ipxe || shell

:local
sanboot || exit
```

### Q: What is Secure Boot? How to enable it?

**A:** Secure Boot is a UEFI security mechanism ensuring only signed bootloaders are loaded.

PxeLab supports Secure Boot for x86_64 and ARM64. In Web UI: **Settings → Boot Settings → Secure Boot**

### Q: What boot types are supported?

**A:**

| Type | Description | Typical Use |
|------|-------------|-------------|
| direct | Directly load kernel + initrd | Linux installation |
| chain | Chain-load to another script | Multi-stage boot |
| wds | Windows WDS deployment | Windows installation |
| sanboot | iSCSI SAN boot | Diskless workstations |
| local | Local disk boot | Default boot entry |

---

## Hosts & Profiles

### Q: What's the relationship between Hosts and Profiles?

**A:**

- **Host**: Represents a device on the network, identified by MAC address
- **Profile**: Boot configuration file defining how a host boots

One host binds to one profile, which determines its boot behavior.

### Q: How to batch import hosts?

**A:** Via REST API or CSV:

```bash
# CSV format
mac,name,profile
AA:BB:CC:DD:EE:01,server-01,ubuntu-install
AA:BB:CC:DD:EE:02,server-02,centos-install
```

```bash
# API import
curl -X POST http://localhost:8080/api/v1/hosts/import \
  -F "file=@hosts.csv"
```

### Q: What is Profile script versioning?

**A:** PxeLab records every modification history of Profile scripts:

- View historical versions
- Compare version differences
- Roll back to any version

Ideal for team collaboration and change auditing.

---

## OS Install Catalog

### Q: How to add a new Linux distro?

**A:** Via Web UI: **Netboot Catalog → Add Group**

1. Upload ISO image to OS Image Management
2. Create a group (e.g., "Ubuntu")
3. Add entries pointing to ISO mount path
4. Configure answer file template (optional)

### Q: What are answer file templates for?

**A:** Answer file templates automate installation without manual interaction:

- **Ubuntu/Debian**: preseed.cfg
- **CentOS/RHEL**: kickstart.cfg
- **Windows**: unattend.xml

PxeLab provides preset templates with variable substitution and customization.

---

## Hardware Management

### Q: How does WOL work?

**A:** Wake-on-LAN sends magic packets to wake network devices:

1. Add target host MAC address in Web UI
2. Click wake button
3. PxeLab sends UDP broadcast magic packet
4. Target device powers on

Scheduled wake tasks are also supported.

### Q: What BMC/IPMI operations are supported?

**A:**

| Operation | Description |
|-----------|-------------|
| power on | Power on |
| power off | Power off |
| power cycle | Restart |
| power status | Query power status |
| SOL | Serial over LAN (remote terminal) |

CSV batch import for BMC info is supported, ideal for large-scale deployments.

---

## Performance & Scaling

### Q: How many hosts can PxeLab support?

**A:** Depends on hardware configuration:

| Config | Recommended Hosts |
|--------|------------------|
| Single core / 512 MB | ≤ 50 |
| Dual core / 1 GB | ≤ 200 |
| Quad core / 2 GB | ≤ 500 |
| Octa core / 4 GB | ≤ 1000 |

DHCP and TFTP are main performance bottlenecks — use high-performance hardware for large deployments.

### Q: How to monitor PxeLab status?

**A:** Three monitoring methods:

1. **Web UI Dashboard**: Real-time service status, traffic, events
2. **Prometheus Metrics**: `GET /api/v1/metrics`
3. **Logs**: Real-time log stream + audit logs

### Q: Where is data stored?

**A:** Default storage locations:

| Platform | Path |
|----------|------|
| Linux | `~/.pxelab/` |
| macOS | `~/Library/Application Support/pxelab/` |
| Windows | `%APPDATA%\pxelab\` |

Customizable via `--data-dir` flag.

---

## Troubleshooting

### Q: Client can't get IP address?

**A:** Checklist:

1. Is DHCP service running: `curl localhost:8080/api/v1/services | jq .dhcp.status`
2. Is port 67 occupied: `netstat -tlnp | grep :67`
3. Are firewall rules allowing it: `iptables -L -n | grep 67`
4. Is network interface configured correctly

### Q: Client gets IP but can't boot?

**A:** Checklist:

1. Is TFTP service running
2. Do boot files exist: `ls -la /path/to/pxelinux.0`
3. Are next-server and boot-file configured correctly
4. Are client and server on the same subnet

### Q: How to view detailed logs?

**A:** Enable debug mode:

```bash
./pxelab --log-level debug
```

Or in Web UI: **Settings → Logs → Log Level → Debug**
