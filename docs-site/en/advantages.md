# Advantages

> Why choose PxeLab?

**Docs**: [Product Overview](/en/product) | [Features](/en/features) | [Architecture](/en/architecture-advantages)

---

## 1. Single Binary, Zero Dependencies

Traditional PXE solutions require installing and configuring DHCP, TFTP, HTTP, DNS, NFS and other services separately, each with its own config files, dependencies, and management approach.

PxeLab packages everything into a single executable:

```bash
# Download one file, run it
./pxelab

# No extra dependencies
# No multi-service config
# No multiple tools to learn
```

**Benefits**:
- Deployment time reduced from hours to seconds
- No dependency conflicts
- Consistent cross-platform experience (Windows / Linux / macOS)
- Upgrade by replacing one file

---

## 2. Full Architecture Coverage

Supports 11 CPU architectures, far exceeding similar tools:

| Category | Supported Architectures |
|----------|------------------------|
| **x86** | BIOS (legacy), EFI x86-64 |
| **ARM** | ARM64 (EFI), ARM (EFI) |
| **Other** | EFI IA32, EFI x86-64 Compressed, EFI BC, EFI ARM64, EFI ARM64 HTTP, EFI x86-64 HTTP, EFI ARM64 TFTP |
| **Secure Boot** | x86_64 + ARM64 secure boot |

**Practical Impact**:
- One PxeLab instance covers all server architectures in a data center
- No need to maintain separate PXE servers for different architectures
- Secure Boot support for modern server security requirements

---

## 3. Built-in iPXE, No External Compilation

iPXE is a powerful network boot firmware, but traditionally requires self-compilation.

PxeLab includes pre-compiled iPXE:

- Pre-compiled iPXE binaries for all 11 architectures
- Embedded boot scripts, ready to use
- Custom script replacement supported
- Version management and diff comparison

```bash
# No need to compile iPXE separately
# No need to maintain iPXE source
# Manage boot scripts via Web UI
```

---

## 4. Modern Web Management Interface

Say goodbye to command-line config files:

| Feature | Description |
|---------|-------------|
| **React 19 SPA** | Modern frontend framework, responsive |
| **Dark/Light Theme** | Adapts to different environments |
| **Bilingual** | Chinese and English support |
| **Real-time Monitoring** | SSE event stream, auto-refresh |
| **REST API** | Supports automation and integration |

**Management Capabilities**:
- Dashboard: global overview, service status, traffic charts
- Host management: CRUD, grouping, profile binding
- Boot config: profile management, script versioning
- OS catalog: built-in distros, custom groups
- Hardware: WOL, BMC/IPMI control
- Operations: logs, audit, diagnostics

---

## 5. Flexible DHCP Modes

4 DHCP modes for various network environments:

| Mode | Description | Use Case |
|------|-------------|----------|
| **full** | Full DHCP server | Standalone networks |
| **proxy** | ProxyDHCP overlay | Existing DHCP servers |
| **hybrid** | Hybrid mode | Partial PXE needed |
| **off** | DHCP disabled | TFTP/HTTP only |

**Key Features**:
- Per-network-interface configuration
- Whitelist and access control
- Subnet address pool management
- IP reservations

---

## 6. Ready-to-Use NFS

Built-in NFSv3 server, no external dependencies:

```yaml
# config.yaml
nfs:
  enabled: true
  mountpoints:
    - path: /data/installs
      allowed_ips:
        - 192.168.1.0/24
```

**Benefits**:
- Tight integration with PXE services
- IP-based access control
- Multiple mount point support
- Connection tracking and monitoring

---

## Summary

| Advantage | Traditional Solution | PxeLab |
|-----------|---------------------|--------|
| Setup complexity | High (multi-service) | Low (single binary) |
| Architecture support | Limited | 11 archs + Secure Boot |
| iPXE | Self-compile | Built-in |
| Management UI | CLI / complex Web | Modern Web UI |
| DHCP flexibility | Limited | 4 modes |
| NFS | External dependency | Built-in |
| Cross-platform | Usually Linux only | Windows/Linux/macOS |
