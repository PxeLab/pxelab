# Boot Config

> iPXE boot script system, Profile management, and default menu configuration.

**Docs**: [Architecture Mapping & Secure Boot](../reference/boot-settings.md) | [DHCP Config](dhcp.md) | [Netboot Catalog](netboot.md)

---

## iPXE Boot Script System

PxeLab's iPXE boot uses a **configuration-driven decision tree** — no need to write raw iPXE scripts, configure visually through the Web UI:

```
Client requests boot script
    │
    ├─ 1. Custom script? → Yes → Return directly, ignore all below
    │
    ├─ 2. Host has Profile? → Profile with menu → Return Profile menu
    │
    ├─ 3. Catalog redirect? → Enabled → Chain to OS install catalog
    │
    └─ 4. Default boot menu → Return configured default menu
```

---

## Boot Menu Types

| BootType | Purpose | Example |
|----------|---------|---------|
| `local` | Boot from local disk | Skip network boot |
| `direct` | Directly load kernel + initrd | Linux distro installation |
| `chain` | Chain-load other bootloaders | GRUB2, Windows Boot Manager |
| `wds` | Windows WIM boot | Windows PE / installation |
| `sanboot` | iSCSI SAN boot | Diskless workstations |

### sanboot Use Cases

| Scenario | Suitable? | Reason |
|----------|-----------|--------|
| DOS boot disk | ✅ | Boots and runs immediately |
| Live Linux | ✅ | Kernel + initramfs self-contained |
| WinPE maintenance disk | ✅ | Gets tools via network after entering PE |
| Memtest86+ | ✅ | Doesn't access disk after boot |
| iSCSI LUN direct boot | ✅ | Installed system disk |
| CentOS/RHEL install | ⚠️→❌ | Anaconda needs explicit inst.repo |
| Windows install | ❌ | Needs wim + BCD extraction, use wimboot |

---

## Profile (Boot Configuration)

Profile is a boot config bound to a specific host, containing a boot menu:

- **Create Profile**: Specify name, architecture, boot type, and parameters
- **Bind Host**: Bind Profile to host MAC address
- **Script Versioning**: Auto-saves version snapshots on every modification, supports diff and rollback
- **Create from Netboot**: One-click Profile creation from OS install catalog

Each Profile is a single boot entry for simplified config:

```
Profile: "Install Ubuntu 22.04"
  ├─ Architecture: x86_64
  ├─ Type: direct
  ├─ Kernel: vmlinuz
  ├─ Initrd: initrd.img
  └─ Cmdline: net.ifnames=0 console=tty0
```

---

## Default Boot Menu

Displayed when client has **no associated Profile** and **Netboot catalog redirect is disabled**.

Two modes supported (Web UI: **Service Config → Netboot Catalog**, or sidebar bottom **Settings → Netboot**):

1. **Show default Profile's boot entry** — Only show Profiles marked as default
2. **List all Profiles** — All Profiles as menu items

Configuration:
- **Menu Title** — Default `PxeLab Boot Menu`
- **Timeout** — 0 = no auto-select, >0 = auto-select default entry after timeout
- **Menu Entries** — Add multiple boot entries

---

## Custom iPXE Script

Fill in **Settings → Netboot → Custom iPXE Script** to **completely replace** all visual configuration:

<v-pre>

```
#!ipxe
dhcp || clear
echo Booting custom script for {{.MAC}}
chain {{.URL}}/boot/custom.ipxe || shell
```

</v-pre>

Available template variables:
- `{{'{{'}}.URL}}` — Server address (e.g., `http://192.168.1.10:8080`)
- `{{'{{'}}.MAC}}` — Client MAC address
- `{{'{{'}}.NextServer}}` — Server IP (without port)
