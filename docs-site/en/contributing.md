# Contributing

> How to participate in PxeLab development and contribution.

---

## Development Environment

### Prerequisites

- Go 1.23+
- Node.js 20+
- Make (optional)

### Clone Repository

```bash
git clone https://github.com/user/pxelab.git
cd pxelab
```

### Build Backend

```bash
make build          # Generates bin/pxelab
```

### Build Frontend

```bash
cd web
npm install
npm run dev         # Vite dev server, /api proxied to :8080
```

### Run Tests

```bash
make test           # go test ./...
```

---

## Code Standards

### Backend

- Go standard format (gofumpt)
- Follow Go code review comments
- New features must include tests
- 250-line file limit, split if exceeded

### Frontend

- TypeScript strict mode
- React 19 + Tailwind CSS 4
- Use shared components from `components/ui/`
- All new UI text must use i18n (`locales/` bilingual)

---

## Commit Convention

Use Conventional Commits:

```
feat(dhcp): add DHCP reservation conflict detection
fix(tftp): fix architecture mapping error
docs: update user manual
refactor(api): refactor REST API handlers
```

---

## Pull Request Workflow

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Commit changes
4. Push branch: `git push origin feat/my-feature`
5. Create a Pull Request
6. Wait for CI to pass and code review

---

## Project Structure

```
pxelab/
├── cmd/pxelab/           # CLI entry point
├── internal/
│   ├── api/              # REST API handlers
│   ├── boot/             # Boot file serving
│   ├── config/           # Configuration management
│   ├── dhcp/             # DHCP server
│   ├── dns/              # DNS server
│   ├── httpd/            # HTTP server
│   ├── nfs/              # NFS server
│   ├── tftp/             # TFTP server
│   ├── store/            # Data storage layer
│   └── ...
├── web/                  # Frontend (React SPA)
├── docs/                 # Documentation
├── design-demos/         # Design mockups
└── Makefile              # Build commands
```
