# Netboot Catalog

> OS install catalog menu, overlays, answer file templates, and install task management.

**Docs**: [Boot Config](boot-config.md) | [OS Images](os-images.md) | [Web UI Guide](web-ui.md)

---

## OS Catalog Menu

PxeLab includes a built-in OS install catalog integrated with netboot.xyz distro index, supporting 10 groups:

| Group | Content |
|-------|---------|
| Linux Distributions | x86_64 Linux distros |
| Linux Distributions (32-bit) | 32-bit Linux |
| Linux Distributions (arm64) | ARM64 Linux |
| BSD Systems | FreeBSD / OpenBSD, etc. |
| Live CDs | Graphical live environments |
| Live CDs (arm64) | ARM64 live environments |
| System Tools | System tools / rescue images |
| Windows | Windows PE / installation |
| DOS | DOS boot |
| Unix | Other Unix systems |

Web UI: **Service Config → Netboot Catalog** (or sidebar bottom **Settings → Netboot → Catalog Menu Structure**)

- Enable/disable groups
- Customize display titles
- Drag-and-drop group ordering

---

## Overlay

Customize boot parameters for specific distros without affecting defaults:

- API: `PUT /api/v1/netboot/overlays/{distro}`
- Override kernel params, initrd params, etc.
- Per-distro independent configuration

---

## Answer File Templates

Automated installation answer file management:

- **Preset Templates**: Pre-built answer files for common distros
- **Custom Templates**: Create, edit, validate
- **Versioning**: Save historical versions, support rollback
- **Preview**: Generate final answer file preview
- **Validation**: Syntax and format checking

API: `/api/v1/netboot/answer-templates`

---

## Install Tasks

Track and manage network installation tasks:

- Create install tasks (specify distro, target, answer file)
- Query task status
- Query by MAC: `GET /api/v1/netboot/task/by-mac/{mac}`
- Get answer file: `GET /api/v1/netboot/answer/{task_id}`

---

## Local Cache

Enable **local cache** in sidebar bottom **Settings → Netboot** (enabled by default):

- Cache downloaded boot files to disk
- Speed up repeated boots
- Cache stats: `GET /api/v1/netboot/cache-stats`
- Web UI shows real-time cache path and disk usage
