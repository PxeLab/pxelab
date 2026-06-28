# Access Control — Blacklist & Whitelist Design

## Overview

Add MAC-based blacklist (global) and whitelist (subnet-level) filtering to PxeGo's DHCP handler. Blacklisted MACs are silently ignored at the DHCP level; whitelist-enforced subnets only respond to known MACs.

## Config Model

### GlobalConfig — new fields

```yaml
global:
  whitelist_enabled: false   # global whitelist toggle
```

### SubnetConfig — new field

```yaml
interfaces:
  - name: eth0
    subnets:
      - cidr: 192.168.1.0/24
        whitelist_enabled: true   # per-subnet whitelist toggle
```

### Seed entries (top-level, startup-only)

```yaml
blacklist:
  - mac: "00:11:22:33:44:55"
    reason: "Compromised device"

whitelist:
  - mac: "AA:BB:CC:DD:EE:FF"
    reason: "Management station"
    subnet: "192.168.1.0/24"
```

Seed entries are imported into the database on first startup (idempotent — skipped if the MAC already exists). Runtime CRUD only touches the database.

## Data Model (SQLite, GORM)

```go
type BlacklistEntry struct {
    ID        uint      `gorm:"primaryKey"`
    MAC       string    `gorm:"uniqueIndex;size:17"`
    Reason    string    `gorm:"size:255"`
    Source    string    `gorm:"size:32;default:db"`   // "db" | "seed"
    CreatedAt time.Time
    UpdatedAt time.Time
}

type WhitelistEntry struct {
    ID         uint      `gorm:"primaryKey"`
    MAC        string    `gorm:"index:idx_mac_subnet,unique;size:17"`
    SubnetCIDR string    `gorm:"index:idx_mac_subnet,unique;size:43"`
    Reason     string    `gorm:"size:255"`
    Source     string    `gorm:"size:32;default:db"`
    CreatedAt  time.Time
    UpdatedAt  time.Time
}
```

### Store interface additions

```go
type BlacklistStore interface {
    ListBlacklist(ctx context.Context) ([]models.BlacklistEntry, error)
    CreateBlacklist(ctx context.Context, entry *models.BlacklistEntry) error
    DeleteBlacklist(ctx context.Context, id uint) error
    IsBlacklisted(ctx context.Context, mac string) (bool, error)
}

type WhitelistStore interface {
    ListWhitelist(ctx context.Context) ([]models.WhitelistEntry, error)
    CreateWhitelist(ctx context.Context, entry *models.WhitelistEntry) error
    DeleteWhitelist(ctx context.Context, id uint) error
    IsWhitelisted(ctx context.Context, mac, subnetCIDR string) (bool, error)
}
```

## DHCP Handler — Interception Chain

Within `Handle()`, after subnet matching and before `switch mt`:

1. Check blacklist → `store.IsBlacklisted(mac)` → if true, log + return silently
2. Check global whitelist → `config.Global.WhitelistEnabled` → if true, `store.IsWhitelisted(mac, subnetCfg.CIDR)` → if false, log + return
3. Check subnet whitelist → `subnetCfg.WhitelistEnabled` → if true, `store.IsWhitelisted(mac, subnetCfg.CIDR)` → if false, log + return
4. Continue to normal DISCOVER/REQUEST handling

All rejections log an info-level event to the event bus for auditability.

## Startup Seed Import

In `cmd/pxego/main.go`, after `st.Migrate()` and before service startup:

```go
for _, entry := range cfg.BlacklistSeeds {
    exists, _ := st.IsBlacklisted(ctx, entry.MAC)
    if !exists {
        st.CreateBlacklist(ctx, &models.BlacklistEntry{MAC: entry.MAC, Reason: entry.Reason, Source: "seed"})
    }
}
for _, entry := range cfg.WhitelistSeeds {
    exists, _ := st.IsWhitelisted(ctx, entry.MAC, entry.Subnet)
    if !exists {
        st.CreateWhitelist(ctx, &models.WhitelistEntry{MAC: entry.MAC, SubnetCIDR: entry.Subnet, Reason: entry.Reason, Source: "seed"})
    }
}
```

## API Endpoints

```
GET    /api/v1/access/blacklist         → list
POST   /api/v1/access/blacklist         → create { mac, reason? }
DELETE /api/v1/access/blacklist/{id}    → delete

GET    /api/v1/access/whitelist         → list (?subnet= filter)
POST   /api/v1/access/whitelist         → create { mac, subnet_cidr, reason? }
DELETE /api/v1/access/whitelist/{id}    → delete
```

Settings API extended: `GET /api/v1/settings` returns `whitelist_enabled: bool` top-level; `PUT /api/v1/settings` accepts it.

## Web UI

### Navigation

New menu item "访问控制" in the Manage section, routing to `/netboot/access-control`.

### Settings page

General tab gains a `全局白名单开关` toggle for `whitelist_enabled`.

### Access Control page (`/netboot/access-control`)

Two-column layout:
- Left: Blacklist table (MAC, reason, delete button) + add form
- Right: Whitelist table (MAC, subnet CIDR, reason, delete button) + add form (with subnet CIDR selector)

## Files Changed

| Layer | File | Change |
|-------|------|--------|
| Config | `internal/config/config.go` | Add fields + MACEntry struct |
| Models | `internal/models/access.go` | New file |
| Store | `internal/store/store.go` | Interface additions |
| Store | `internal/store/sqlite.go` | Migrate registration |
| Store | `internal/store/sqlite_access.go` | New file |
| DHCP | `internal/dhcp/handler.go` | Interception chain |
| API | `internal/api/handler.go` | Route registration |
| API | `internal/api/access.go` | New file |
| API | `internal/api/settings.go` | Whitelist toggle |
| App | `cmd/pxego/main.go` | Seed import |
| UI | `web/src/pages/AccessControl.tsx` | New file |
| UI | `web/src/App.tsx` | Route |
| UI | `web/src/components/layout/AppShell.tsx` | Menu item |
| UI | `web/src/pages/Settings.tsx` | Toggle |
| UI | `web/src/api/client.ts` | API methods |
