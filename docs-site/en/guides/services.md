# Service Config

> Six service configuration sub-pages accessible from the Service Config navigation.

**Docs**: [DHCP Config](dhcp.md) | [TFTP Reference](../reference/tftp.md) | [DNS Reference](../reference/dns.md) | [NFS Reference](../reference/nfs.md) | [Architecture Mapping](../reference/boot-settings.md)

---

## DHCP (/services/dhcp)

- **Subnet Config**: Per-subnet DHCP mode (full/proxy/hybrid/off), address pool range, gateway, DNS, lease time
- **Reservations (Tab)**: IP + MAC binding, conflict detection, only available for full-mode subnets
- **Lease List (Tab)**: View active leases, delete and batch cleanup

See [DHCP Config Guide](dhcp.md).

## DNS (/services/dns)

- Upstream DNS configuration
- Local domain settings
- DNS record management (A / AAAA / CNAME)
- Subnet-aware resolution config

See [DNS Service Reference](../reference/dns.md).

## NFS (/services/nfs)

- Multiple mount point management (dynamic card list)
  - Add/remove mount points
  - Per mount point: label, export path, local directory, read-only toggle, IP/CIDR whitelist
- NFS connection status: current connections, client list
- Port configuration

See [NFS Service Reference](../reference/nfs.md).

## TFTP (/services/tftp)

- Port and timeout settings
- Boot file management (Tab): iPXE / PXELinux / GRUB2 three-column architecture mapping display
- File management: upload/delete boot files, table view (name, size, modified time, MD5)

See [TFTP Service Reference](../reference/tftp.md).

## Boot Settings (/boot-settings)

Architecture mapping management page:

- **Boot File Health Check**: Verify all architecture boot files exist and sizes are normal
- **Architecture Mapping Table**: 10 architecture configuration table
  - Columns: Architecture name, AL code, NBP type (iPXE/PXELinux/GRUB2 dropdown), boot filename, Secure Boot support status
  - Auto-update boot filename on NBP switch
  - ARM64 + PXELinux fallback notice
- **iPXE Script Settings**: Custom iPXE script (template variables `{{'{{'}}.URL}}` / `{{'{{'}}.MAC}}`)
- **Actions**: Save, Restore defaults

See [Architecture Mapping & Secure Boot](../reference/boot-settings.md).

## Netboot Catalog (/netboot-catalog)

OS install catalog management:

- Distro list: Show all available distros with version and architecture
- Group management: Enable/disable 10 built-in groups, edit titles, drag-and-drop sorting
- Cache stats: Cache path, file count, disk usage

See [Netboot Catalog](netboot.md).
