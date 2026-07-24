# REST API Reference

> Complete endpoint list and usage conventions for PxeLab REST API v1.

**Related**: [Config File Reference](config-file.md) | [Web UI Guide](../guides/web-ui.md)

---

## General Conventions

**Base URL**: `http://<host>:8080/api/v1`

**Request Format**: JSON (`Content-Type: application/json`)

**Response Format**:

```json
{
  "success": true,
  "data": { ... }
}
```

Error response:

```json
{
  "success": false,
  "error": "Error description"
}
```

---

## Authentication

```
POST /api/v1/auth/login     # Login
POST /api/v1/auth/logout    # Logout
GET  /api/v1/auth/session   # Check session
```

Sessions are maintained via cookies.

---

## Endpoint List

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/auth/login` | Login |
| POST | `/auth/logout` | Logout |
| GET | `/auth/session` | Check session |

### Hosts

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/hosts` | List hosts |
| POST | `/hosts` | Create host |
| GET | `/hosts/{id}` | Get host |
| PUT | `/hosts/{id}` | Update host |
| DELETE | `/hosts/{id}` | Delete host |
| POST | `/hosts/{id}/wake` | WOL wake |
| POST | `/hosts/{id}/power` | IPMI power control |
| GET | `/hosts/{id}/boot-config` | Preview boot config |
| POST | `/hosts/batch/wake` | Batch wake |

### Boot Config (Profile)

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/profiles` | List profiles |
| POST | `/profiles` | Create profile |
| GET | `/profiles/{id}` | Get profile |
| PUT | `/profiles/{id}` | Update profile |
| DELETE | `/profiles/{id}` | Delete profile |
| POST | `/profiles/from-netboot` | Create from netboot |
| GET | `/profiles/{id}/script-versions` | Script version list |
| GET | `/profiles/{id}/script-diff/{verId}` | Version diff |
| POST | `/profiles/{id}/script-rollback/{verId}` | Version rollback |

### File Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/files` | List files |
| GET | `/files/root` | Get root directory |
| POST | `/files/upload` | Upload file |
| DELETE | `/files` | Delete file |

### Leases

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/leases` | List leases |
| GET | `/leases/stats` | Lease statistics |
| DELETE | `/leases/{mac}` | Delete lease |
| POST | `/leases/batch-delete` | Batch delete |
| POST | `/leases/prune` | Prune expired leases |

### Settings

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/settings/general` | General settings |
| PUT | `/settings/general` | Update general settings |
| GET | `/settings/interfaces` | Interface config |
| PUT | `/settings/interfaces` | Update interface config |
| GET | `/settings/netboot` | Netboot settings |
| PUT | `/settings/netboot` | Update netboot settings |
| GET | `/settings/logging` | Logging settings |
| PUT | `/settings/logging` | Update logging settings |

### Services

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/services/tftp` | TFTP settings |
| PUT | `/services/tftp` | Update TFTP |
| GET | `/services/dhcp` | DHCP settings |
| PUT | `/services/dhcp` | Update DHCP |
| GET | `/services/dns` | DNS settings |
| PUT | `/services/dns` | Update DNS |
| GET | `/services/nfs` | NFS settings |
| PUT | `/services/nfs` | Update NFS |
| GET | `/services/archmap` | Architecture mapping |
| PUT | `/services/archmap` | Update architecture mapping |
| GET | `/services/archmap/defaults` | Default architecture mapping |
| GET | `/services/ipxe-script` | iPXE script |
| PUT | `/services/ipxe-script` | Update iPXE script |

### Service Lifecycle

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/services` | List all services |
| POST | `/services/{name}/start` | Start service |
| POST | `/services/{name}/stop` | Stop service |
| POST | `/services/{name}/restart` | Restart service |
| POST | `/services/batch/{action}` | Batch operation |
| PUT | `/services/{name}/auto-start` | Set auto-start |

### Netboot

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/netboot/catalog` | OS catalog |
| GET | `/netboot/catalog/{distro}` | Distro details |
| GET | `/netboot/groups` | Group list |
| GET | `/netboot/check-files` | Check files |
| GET | `/netboot/cache-stats` | Cache statistics |
| GET/PUT/DELETE | `/netboot/overlays/{distro}` | Overlay |
| GET/POST/PUT/DELETE | `/netboot/answer-templates` | Answer file templates |
| GET/POST/PUT/DELETE | `/netboot/tasks` | Install tasks |

### Access Control

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST/DELETE | `/access/blacklist` | Blacklist |
| GET/POST/DELETE | `/access/whitelist` | Whitelist |
| GET | `/access/unauthorized` | Unauthorized devices |

### DNS

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/dns/records` | DNS records |
| GET/PUT/DELETE | `/dns/records/{id}` | Single record |

### BMC

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/bmc/configs` | BMC config |
| POST | `/bmc/configs/import` | CSV import |
| POST | `/bmc/probe` | Probe BMC |
| POST | `/bmc/{id}/power-on` | Power on |
| POST | `/bmc/{id}/power-off` | Power off |
| POST | `/bmc/{id}/restart` | Restart |
| GET | `/bmc/{id}/status` | Query status |
| POST | `/bmc/{id}/boot-device` | Set boot device |
| POST | `/bmc/batch/power-on` | Batch power on |
| POST | `/bmc/batch/power-off` | Batch power off |
| POST | `/bmc/batch/restart` | Batch restart |
| POST | `/bmc/batch/status` | Batch status query |

### DHCP Reservations

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/dhcp/reservations` | List / Create |
| GET/PUT/DELETE | `/dhcp/reservations/{id}` | Single operation |

### Network Diagnostics

| Method | Endpoint | Description |
|--------|----------|-------------|
| POST | `/network/ping` | Ping |
| POST | `/network/ping/stream` | Streaming ping |
| POST | `/network/traceroute` | Traceroute |
| POST | `/network/traceroute/stream` | Streaming traceroute |
| GET | `/network/interfaces` | Network interface list |

### WOL

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST/DELETE | `/wol/history` | Wake history |
| GET/POST/DELETE | `/wol/schedules` | Scheduled tasks |
| GET | `/wol/interfaces` | WOL interfaces |

### OS Images

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET/POST | `/os-images` | List / Upload |
| GET/PUT/DELETE | `/os-images/{id}` | Single operation |
| POST | `/os-images/{id}/extract` | Extract |
| POST | `/os-images/{id}/mount` | Mount |
| POST | `/os-images/{id}/unmount` | Unmount |
| GET | `/fs/browse` | File browser |

### Miscellaneous

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/status` | Service status |
| GET | `/metrics` | Prometheus metrics |
| GET | `/events` | Event list |
| GET | `/events/stream` | Event stream (SSE) |
| GET | `/logs/stream` | Log stream (SSE) |
| GET | `/logs/files` | Log file list |
| GET | `/logs/disk-usage` | Log disk usage |
| POST | `/logs/cleanup` | Clean up logs |
| GET | `/interfaces` | Network interfaces |
| GET | `/bootloader/check` | Boot file check |
| GET | `/bootloader/files` | Boot file list |
