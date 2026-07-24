# DNS Service

> Local DNS resolution, upstream forwarding, and record management.

**Related**: [DHCP Config](../guides/dhcp.md) | [NFS Service](nfs.md)

---

## Local DNS

PxeLab has a built-in DNS server supporting:

- **Upstream forwarding** — Forward unmatched queries to upstream DNS
- **Local resolution** — A / AAAA / CNAME records
- **Subnet-aware** — Return server IP based on client source subnet
- **Auto-records** — Auto-create `@` A record and server name A record on startup

| Config | Default | Description |
|--------|---------|-------------|
| Port | 53 | UDP |
| Local domain | pxelab.local | Suffix domain |
| Upstream DNS | System default | Forward target |

Web UI: **Service Config → DNS**

---

## DNS Record Management

Manage DNS records via Web UI or API:

```
GET    /api/v1/dns/records          # List all records
POST   /api/v1/dns/records          # Create record
GET    /api/v1/dns/records/{id}     # Get single record
PUT    /api/v1/dns/records/{id}     # Update record
DELETE /api/v1/dns/records/{id}     # Delete record
```

Supported record types: A, AAAA, CNAME
