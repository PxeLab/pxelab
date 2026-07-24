# Host Management

> Device management, WOL wake-on-LAN, and BMC/IPMI out-of-band management.

**Docs**: [Web UI Guide](web-ui.md) | [REST API Reference](../reference/api-reference.md)

---

## Host CRUD

Manage devices on the network:

```
GET    /api/v1/hosts                # List all hosts
POST   /api/v1/hosts                # Create host
GET    /api/v1/hosts/{id}           # Get host details
PUT    /api/v1/hosts/{id}           # Update host
DELETE /api/v1/hosts/{id}           # Delete host
```

Each host can be configured with:
- Name, description
- MAC address
- Bound Profile (boot config)
- IP address
- Group

---

## WOL Wake-on-LAN

Remote power-on via Wake-on-LAN:

```
POST /api/v1/hosts/{id}/wake       # Wake single host
POST /api/v1/hosts/batch/wake      # Batch wake
GET  /api/v1/wol/history           # Wake history
POST /api/v1/wol/schedule          # Create scheduled wake
GET  /api/v1/wol/schedules         # List scheduled tasks
```

Scheduled wake features:
- Specify MAC address and wake time
- Support one-time or recurring schedules
- Wake history records

---

## BMC/IPMI Out-of-Band Management

Remote server power management via IPMI protocol:

```
POST /api/v1/bmc/probe              # Probe BMC
POST /api/v1/bmc/{id}/power-on      # Power on
POST /api/v1/bmc/{id}/power-off     # Power off
POST /api/v1/bmc/{id}/restart       # Restart
GET  /api/v1/bmc/{id}/status        # Query status
POST /api/v1/bmc/{id}/boot-device   # Set boot device
```

Batch operations supported:
```
POST /api/v1/bmc/batch/power-on     # Batch power on
POST /api/v1/bmc/batch/power-off    # Batch power off
POST /api/v1/bmc/batch/restart      # Batch restart
POST /api/v1/bmc/batch/status       # Batch status query
```

CSV batch import BMC config: `POST /api/v1/bmc/configs/import`
