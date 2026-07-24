# Product Overview

> All-in-one PXE network boot server, ready to use out of the box.

**Docs**: [Getting Started](/en/getting-started) | [Features](/en/features) | [Advantages](/en/advantages)

---

## What is PxeLab

**PxeLab** is an all-in-one PXE network boot server that integrates DHCP, TFTP, HTTP, DNS, and NFS services into a single binary, managed through a modern Web UI and REST API.

Traditional PXE deployments require installing and configuring multiple services (DHCP, TFTP, HTTP, etc.) separately — a process that's tedious and error-prone. PxeLab simplifies everything into one executable: download, run, and start using. Whether you're doing batch OS deployment, diskless workstation setup, or daily server maintenance, PxeLab provides an efficient and reliable network boot solution.

---

## Target Users

| User Role | Typical Scenarios |
|-----------|-------------------|
| **IT Operations** | Batch OS deployment, remote server maintenance |
| **IDC Engineers** | Data center batch provisioning, diskless workstation deployment |
| **System Administrators** | Daily server management, network boot configuration |
| **Dev/Test Teams** | Quick test environment setup, automated deployment |
| **Education/Training** | Network boot teaching, lab environment setup |

---

## Core Value

| Value | Description |
|-------|-------------|
| **Zero-dependency deployment** | Single binary, no extra dependencies needed |
| **Full architecture coverage** | 11 CPU architectures including x86 BIOS/EFI, ARM64, Secure Boot |
| **Modern management UI** | Built-in React Web UI, dark theme, bilingual, real-time monitoring |
| **Flexible deployment modes** | 4 DHCP modes for various network environments |
| **Ready out of the box** | Built-in iPXE, NFS, OS install catalog |

---

## Comparison

| Feature | PxeLab | Traditional PXE (TFTP-only) | Foreman/Cobbler |
|---------|--------|---------------------------|-----------------|
| Setup complexity | Single binary, zero deps | Manual multi-service config | Heavy dependencies |
| iPXE support | Built-in custom compilation | Self-compile required | Self-integration required |
| Multi-arch | 11 archs + Secure Boot | Usually x86 only | Limited |
| Web management | Built-in, full-featured | None | Yes, but complex |
| DHCP modes | full / proxy / hybrid / off | Usually one mode | Limited |
| NFS | Built-in NFSv3 | External NFS needed | External needed |
| Platform support | Windows / Linux / macOS | Usually Linux only | Usually Linux only |
| Resource usage | ≤ 512 MB RAM | Depends on services | High |

---

## Quick Start

```bash
# Download and run
./pxelab

# Open browser
open http://localhost:8080
```

On first start, PxeLab auto-initializes default configuration. All network boot services can be configured and managed through the Web UI.
