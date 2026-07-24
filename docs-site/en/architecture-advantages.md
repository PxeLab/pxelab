# Architecture

> Technical architecture design highlights of PxeLab.

**Docs**: [Architecture Overview](/en/architecture) | [Advantages](/en/advantages) | [Config Reference](/en/reference/config-file)

---

## Overall Architecture

```
┌─────────────────────────────────────────────────────┐
│                    PxeLab Binary                     │
├─────────────┬─────────────┬─────────────┬───────────┤
│  Web UI     │  REST API   │  Services   │  Store    │
│  (React SPA)│  (chi)      │  Manager    │  (SQLite) │
├─────────────┴─────────────┴─────────────┴───────────┤
│              Service Manager                        │
│  ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐ ┌─────┐         │
│  │DHCP │ │TFTP │ │HTTP │ │DNS  │ │NFS  │         │
│  └─────┘ └─────┘ └─────┘ └─────┘ └─────┘         │
├─────────────────────────────────────────────────────┤
│              Event Bus (Pub/Sub)                    │
└─────────────────────────────────────────────────────┘
```

---

## 1. Go Single Binary Deployment

### Why Go?

| Feature | Benefit |
|---------|---------|
| **Static compilation** | No runtime dependencies, simple deployment |
| **Cross-platform** | One codebase, Windows/Linux/macOS binaries |
| **Concurrency model** | Goroutines efficiently handle network connections |
| **Standard library** | Built-in HTTP, DNS, NFS network service support |
| **Performance** | Near-C performance, far exceeds scripting languages |

### Embedded Resources

All frontend assets (HTML/CSS/JS) and boot files (iPXE binaries, templates) are embedded into the binary:

```go
//go:embed webdist/*
var webDist embed.FS

//go:embed ipxe/*
var ipxeBinaries embed.FS
```

**Benefits**:
- No separate frontend server needed
- No filesystem path maintenance
- Upgrade by replacing one file
- No file permission issues

---

## 2. Service Manager

Unified lifecycle management for all services:

```
ServiceManager
├── StartAll()     # Start all services
├── StopAll()      # Stop all services
├── RestartAll()   # Restart all services
├── Start(name)    # Start single service
├── Stop(name)     # Stop single service
└── Status()       # Get all service statuses
```

**Design Highlights**:
- Each service implements a unified `Service` interface
- Independent start/stop, no mutual interference
- Hot-reload configuration without process restart
- Graceful shutdown

---

## 3. Event Bus (Pub/Sub)

Internal publish/subscribe system for decoupled inter-service communication:

```go
// Publish event
eventbus.Publish(Event{
    Type:    "dhcp.lease",
    Payload: LeaseInfo{...},
})

// Subscribe to event
eventbus.Subscribe("dhcp.lease", func(e Event) {
    // Handle DHCP lease event
})
```

**Supported Event Types**:
- DHCP lease events
- Boot events (BOOT)
- WOL wake events
- IPMI/BMC operation events
- DNS query events

**Benefits**:
- Loose coupling between services
- Easy to extend with new event types
- Async processing support
- Logging and audit trail

---

## 4. Store Interface Separation

Data layer uses interface design with multiple implementations:

```go
type Interface interface {
    HostStore
    ProfileStore
    SettingsStore
    // ... other sub-interfaces
}

// SQLite implementation (production)
type SQLiteStore struct { ... }

// In-memory implementation (testing/dev)
type MemoryStore struct { ... }
```

**Benefits**:
- Fast, isolated testing with in-memory storage
- Lightweight, reliable SQLite for production
- Easy to switch to other databases (e.g., PostgreSQL)
- Clear interface definitions, easy to extend

---

## 5. Dependency Injection

`main.go` explicitly constructs all service dependencies:

```go
func run() {
    // Explicit construction, no global singletons
    store := sqlite.New(dbPath)
    eventBus := eventbus.New()
    dhcpServer := dhcp.New(store, eventBus)
    tftpServer := tftp.New(bootFileServer)
    httpServer := httpd.New(store, ...)
    
    // Service manager unifies management
    manager := servicemanager.New(
        dhcpServer,
        tftpServer,
        httpServer,
        dnsServer,
        nfsServer,
    )
    
    // Start all services
    manager.StartAll()
}
```

**Benefits**:
- Clear dependency relationships
- Easy to test and mock
- No hidden global state
- Easy to understand and maintain

---

## 6. CSS Variable Theme System

Frontend uses CSS variables for theme switching:

```css
:root {
    --bg-base: oklch(0.145 0 0);
    --text-primary: oklch(0.985 0 0);
    --accent-blue: oklch(0.546 0.245 262.881);
}

.dark {
    --bg-base: oklch(0.145 0 0);
    --text-primary: oklch(0.985 0 0);
}
```

**Benefits**:
- One-click dark/light theme switching
- All components auto-adapt
- No need to maintain two style sets
- User preference persistence

---

## 7. Code Splitting & Lazy Loading

React route-level code splitting:

```typescript
const Dashboard = React.lazy(() => import('./pages/Dashboard'))
const Hosts = React.lazy(() => import('./pages/Hosts'))

// Route config
<Route path="/dashboard" element={
    <Suspense fallback={<Skeleton />}>
        <Dashboard />
    </Suspense>
} />
```

**Benefits**:
- Fast first-screen loading
- On-demand loading, smaller initial bundle
- Skeleton screens for better UX
- Build tools auto-optimize

---

## Tech Stack Summary

| Layer | Technology | Description |
|-------|-----------|-------------|
| **Backend** | Go 1.23+ | High performance, static compilation |
| **Frontend** | React 19 + TypeScript | Modern UI framework |
| **Styling** | Tailwind CSS 4 | Atomic CSS |
| **Database** | SQLite (GORM) | Lightweight embedded database |
| **Router** | chi | Lightweight HTTP router |
| **Build** | Vite 6 | Fast frontend build |
| **Release** | GoReleaser | Automated releases |
