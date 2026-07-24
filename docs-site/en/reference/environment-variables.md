# Environment Variables & CLI Reference

> Supported environment variables, CLI parameters, and configuration priority for PxeLab.

**Related**: [Config File Reference](config-file.md) | [Deployment](../guides/deployment.md)

---

## CLI Parameters

```
pxelab [flags]

Flags:
  --config string       Config file path
  --data-dir string     Data directory (default "~/.pxelab/")
  --log-level string    Log level: debug/info/warn/error (default "info")
  --mode string         Run mode: app/server ("app" auto-opens browser)
```

---

## Configuration Priority

CLI parameters > Config file > Environment variables > Default values

| Priority | Source | Description |
|----------|--------|-------------|
| 1 (Highest) | `--config` / `--data-dir` CLI parameters | Explicitly specified on command line |
| 2 | `config.yaml` | Config file |
| 3 | Environment variables | System environment variables |
| 4 (Lowest) | Built-in defaults | Hardcoded default values |

---

## Common Environment Variables

PxeLab primarily manages settings via config file. These environment variables are available for special scenarios:

| Environment Variable | Description | Default |
|---------------------|-------------|---------|
| `PXELAB_DATA_DIR` | Data directory path | `~/.pxelab/` |
| `PXELAB_LOG_LEVEL` | Log level | `info` |
| `PXELAB_LISTEN_ADDR` | HTTP listen address | `:8080` |

---

## Data Directory Structure

```
~/.pxelab/
├── config.yaml          # Config file
├── pxelab.db            # SQLite database
├── boot/                # Boot files (iPXE, PXELinux, GRUB2 binaries)
│   ├── ipxe.efi
│   ├── undionly.kpxe
│   └── ...
├── netboot/             # Netboot catalog (distro index)
├── os-images/           # OS image storage
├── logs/                # Log files
│   └── pxelab.log
└── volumes/             # NFS mount point data
```
