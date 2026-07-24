# TFTP Service

> TFTP service configuration and architecture mapping.

**Related**: [Architecture Mapping](boot-settings.md) | [Boot Config](../guides/boot-config.md)

---

TFTP service provides NBP (Network Bootstrap Program) file transfer:

| Config | Default | Description |
|--------|---------|-------------|
| Port | 69 | UDP |
| Timeout | Default | Client connection timeout |

Web UI: **Service Config → TFTP**

The TFTP service automatically selects the correct boot file based on architecture mapping and returns it to the client. Boot files are embedded in binaries and released to the data directory on first run.

---

## Boot File Management

The TFTP page in Web UI includes a Boot File Management tab:

- **iPXE / PXELinux / GRUB2 three-column display**: Architecture mapping at a glance
- **Upload files**: Manually upload custom boot files
- **Delete files**: Remove unwanted boot files
- **Table view**: Name, size, modified time, MD5 hash
