# Release Notes

> New features and changes in each PxeLab version.

**Docs**: [Getting Started](getting-started.md) | [Architecture](architecture.md)

---

## v0.4.0-dev (Current Development)

**New**:
- Full iPXE architecture support (ARM32, RISC-V 32/64, LoongArch32/64)
- Secure Boot support (x86_64 + ARM64)
- NFS multiple mount points
- NFS IP access control
- DNS subnet-aware resolution
- DHCP reservations (IP+MAC binding)
- Access control (whitelist/blacklist)
- BMC/IPMI out-of-band management
- Install task tracking
- Answer file template versioning
- Log rotation and cleanup
- Audit logs
- Network diagnostics (Ping/Traceroute)
- Local cache (faster repeated boots)
- Windows system tray

**Changes**:
- NBP architecture refactor: Option 93 as primary source
- Service lifecycle management
- Profile simplified to single boot entry
- Route refactoring

---

## v0.3.0

- iPXE boot script system
- DHCP four modes
- Full-featured Web UI
- CLI management tools
- Event bus and real-time logs

---

## v0.2.0

- TFTP/DNS services
- Host management
- Profile management
- REST API v1

---

## v0.1.0

- Initial release
- DHCP + HTTP basic services
- iPXE booting

---

> **Doc Version**: v1.0 · **Applies to**: PxeLab v0.4.0-dev · **Maintainer**: PxeLab Team
