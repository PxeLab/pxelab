# PxeGo 实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**目标：** 构建一个 Go 语言开发的一体化 PXE 服务器，完整复现 Tiny PXE Server 功能，支持跨平台单二进制运行。

**架构：** 模块化 Server 接口设计，每个服务（DHCP/TFTP/DNS/HTTP）独立实现 Start/Stop 生命周期。App 编排器通过 errgroup 统一管理。多网卡通过 `0.0.0.0:67` + `ReadMsgUDP` 目标 IP 匹配实现。

**技术栈：** Go 1.25+, cobra+viper, chi/v5, insomniacslk/dhcp, pin/tftp, miekg/dns, go-ipmi, GORM+glebarez/sqlite, React 18+Vite+Tailwind+react-i18next

---

## 文件结构

```
PxeGo/
├── cmd/pxego/
│   ├── main.go
│   ├── embed.go
│   ├── service_windows.go
│   ├── service_linux.go
│   └── service_darwin.go
├── internal/
│   ├── app/
│   │   ├── app.go           # 编排器
│   │   └── server.go        # Server 接口定义
│   ├── config/
│   │   ├── config.go        # 配置结构体
│   │   ├── defaults.go      # 默认值
│   │   └── flags.go         # Viper/cobra 绑定
│   ├── models/
│   │   ├── host.go
│   │   ├── profile.go
│   │   ├── bootfile.go
│   │   ├── event.go
│   │   └── lease.go
│   ├── store/
│   │   ├── store.go         # 接口定义
│   │   ├── sqlite.go        # GORM SQLite 实现
│   │   └── memory.go        # 内存实现（测试用）
│   ├── eventbus/
│   │   └── bus.go
│   ├── dhcp/
│   │   ├── server.go
│   │   ├── handler.go
│   │   ├── lease.go
│   │   └── options.go
│   ├── tftp/
│   │   ├── server.go
│   │   └── handler.go
│   ├── dns/
│   │   ├── server.go
│   │   └── handler.go
│   ├── boot/
│   │   ├── serve.go
│   │   ├── archmap.go
│   │   ├── ipxe/
│   │   │   ├── engine.go
│   │   │   ├── templates.go
│   │   │   └── engine_test.go
│   │   └── pxelinux/
│   │       ├── ast.go
│   │       ├── parser.go
│   │       ├── parser_test.go
│   │       └── generator.go
│   ├── httpd/
│   │   ├── server.go
│   │   ├── router.go
│   │   └── middleware.go
│   ├── api/
│   │   ├── response.go
│   │   ├── hosts.go
│   │   ├── profiles.go
│   │   ├── files.go
│   │   ├── events.go
│   │   └── status.go
│   ├── ipmi/
│   │   └── ipmi.go
│   └── wds/
│       └── wds.go
├── web/                      # React SPA
│   ├── src/
│   │   ├── api/
│   │   ├── hooks/
│   │   ├── i18n/
│   │   ├── pages/
│   │   └── components/
│   ├── package.json
│   └── vite.config.ts
├── boot/                     # 默认引导文件
├── contrib/                  # systemd / launchd / 示例配置
├── go.mod
├── go.sum
├── Makefile
└── .goreleaser.yaml
```

---

## 前置准备

### Task 0：初始化项目

**文件：**
- Create: `go.mod`
- Create: `Makefile`

- [ ] **Step 1: 初始化 Go module**

```bash
mkdir -p PxeGo && cd PxeGo
go mod init github.com/pxego/pxego
```

- [ ] **Step 2: 安装依赖**

```bash
go get github.com/spf13/cobra
go get github.com/spf13/viper
go get github.com/go-chi/chi/v5
go get github.com/insomniacslk/dhcp
go get github.com/pin/tftp
go get github.com/miekg/dns
go get github.com/bougou/go-ipmi
go get gorm.io/gorm
go get github.com/glebarez/sqlite
go get github.com/google/uuid
go get github.com/google/gopacket
go get golang.org/x/sys/windows
```

- [ ] **Step 3: 创建 Makefile**

```makefile
.PHONY: build test run clean

build:
	go build -o bin/pxego ./cmd/pxego

test:
	go test ./...

run:
	go run ./cmd/pxego

clean:
	rm -rf bin/

.PHONY: frontend
frontend:
	cd web && npm ci && npm run build
```

- [ ] **Step 4: 提交**

```bash
git init && git add -A && git commit -m "chore: 初始化项目结构和依赖"
```

---

## Phase 1：基础设施

### Task 1：Server 接口与 App 编排器

**文件：**
- Create: `internal/app/server.go`
- Create: `internal/app/app.go`

- [ ] **Step 1: 定义 Server 接口**

`internal/app/server.go`:
```go
package app

import "context"

type Server interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}
```

- [ ] **Step 2: 实现 App 编排器**

`internal/app/app.go`:
```go
package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

type App struct {
	servers []Server
	config  interface{}
	db      interface{}
	eventBus interface{}
}

func New() *App {
	return &App{}
}

func (a *App) Register(srv Server) {
	a.servers = append(a.servers, srv)
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	g, ctx := errgroup.WithContext(ctx)

	for _, srv := range a.servers {
		s := srv
		g.Go(func() error {
			slog.Info("启动服务", "name", s.Name())
			if err := s.Start(ctx); err != nil {
				return fmt.Errorf("%s: %w", s.Name(), err)
			}
			return nil
		})
	}

	// 信号处理
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		slog.Info("收到信号", "signal", sig)
	}

	// 反向顺序优雅关闭
	for i := len(a.servers) - 1; i >= 0; i-- {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		slog.Info("关闭服务", "name", a.servers[i].Name())
		a.servers[i].Stop(shutdownCtx)
		cancel()
	}

	return g.Wait()
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/app/ && git commit -m "feat: 添加 Server 接口和 App 编排器"
```

### Task 2：配置模块

**文件：**
- Create: `internal/config/config.go`
- Create: `internal/config/defaults.go`
- Create: `internal/config/flags.go`

- [ ] **Step 1: 定义配置结构体**

`internal/config/config.go`:
```go
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Global    GlobalConfig    `mapstructure:"global"`
	Interfaces []InterfaceConfig `mapstructure:"interfaces"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Boot      BootConfig      `mapstructure:"boot"`
	Store     StoreConfig     `mapstructure:"store"`
	Log       LogConfig       `mapstructure:"log"`
}

type GlobalConfig struct {
	DataDir string `mapstructure:"data_dir"`
	AppMode bool   `mapstructure:"app_mode"`
}

type InterfaceConfig struct {
	Name    string         `mapstructure:"name"`
	IP      string         `mapstructure:"ip"`
	Subnets []SubnetConfig `mapstructure:"subnets"`
	DHCP    string         `mapstructure:"dhcp"` // full | proxy | hybrid | off
	TFTP    bool           `mapstructure:"tftp"`
	HTTP    bool           `mapstructure:"http"`
	DNS     bool           `mapstructure:"dns"`
}

type SubnetConfig struct {
	CIDR       string `mapstructure:"cidr"`
	DHCP       string `mapstructure:"dhcp"`
	Pool       string `mapstructure:"pool"`
	Gateway    string `mapstructure:"gateway"`
	DNSServers string `mapstructure:"dns_servers"`
	NextServer string `mapstructure:"next_server"`
	LeaseTime  int    `mapstructure:"lease_time"`
}

type AuthConfig struct {
	Token string `mapstructure:"token"`
}

type BootConfig struct {
	RootDir string `mapstructure:"root_dir"`
}

type StoreConfig struct {
	DSN string `mapstructure:"dsn"`
}

type LogConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

func (c *Config) Validate() error {
	for _, iface := range c.Interfaces {
		if iface.DHCP != "" && iface.DHCP != "full" && iface.DHCP != "proxy" && iface.DHCP != "hybrid" && iface.DHCP != "off" {
			return fmt.Errorf("interface %s: 无效的 DHCP 模式: %s", iface.Name, iface.DHCP)
		}
	}
	return nil
}

func DefaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".pxego")
}
```

- [ ] **Step 2: 定义默认值**

`internal/config/defaults.go`:
```go
package config

const (
	DefaultPortDHCP    = 67
	DefaultPortDHCP4011 = 4011
	DefaultPortTFTP    = 69
	DefaultPortDNS     = 53
	DefaultPortHTTP    = 8080
	DefaultLeaseTime   = 3600
	DefaultLogLevel    = "info"
)

func DefaultConfig() *Config {
	return &Config{
		Global: GlobalConfig{
			DataDir: DefaultDataDir(),
		},
		Boot: BootConfig{
			RootDir: "/etc/pxego/boot",
		},
		Store: StoreConfig{
			DSN: "pxego.db",
		},
		Log: LogConfig{
			Level: DefaultLogLevel,
		},
	}
}
```

- [ ] **Step 3: 绑定 cobra/viper**

`internal/config/flags.go`:
```go
package config

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func BindFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("config", "", "配置文件路径")
	cmd.PersistentFlags().String("data-dir", DefaultDataDir(), "数据目录")
	cmd.PersistentFlags().String("log-level", "info", "日志级别")
	cmd.PersistentFlags().Bool("app-mode", false, "应用模式（自动打开浏览器）")

	viper.BindPFlag("global.data_dir", cmd.PersistentFlags().Lookup("data-dir"))
	viper.BindPFlag("log.level", cmd.PersistentFlags().Lookup("log-level"))
	viper.BindPFlag("global.app_mode", cmd.PersistentFlags().Lookup("app-mode"))
}

func LoadConfig(cfgPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME/.pxego")
	v.AddConfigPath("/etc/pxego")

	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
	}

	v.SetDefault("store.dsn", "pxego.db")
	v.SetDefault("boot.root_dir", "/etc/pxego/boot")
	v.SetDefault("log.level", "info")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}
```

- [ ] **Step 4: 创建默认配置文件**

`contrib/config.example.yaml`:
```yaml
global:
  data_dir: ~/.pxego
  app_mode: false

interfaces:
  - name: eth0
    ip: 0.0.0.0
    dhcp: hybrid
    tftp: true
    http: true
    dns: false
    subnets:
      - cidr: 192.168.1.0/24
        pool: 192.168.1.100-200
        gateway: 192.168.1.1
        dns_servers: 8.8.8.8
        next_server: 192.168.1.10
        lease_time: 3600

boot:
  root_dir: /etc/pxego/boot

auth:
  token: ""

store:
  dsn: pxego.db

log:
  level: info
  format: text
```

- [ ] **Step 5: 提交**

```bash
git add internal/config/ contrib/ && git commit -m "feat: 添加配置模块和示例配置"
```

### Task 3：数据模型

**文件：**
- Create: `internal/models/host.go`
- Create: `internal/models/profile.go`
- Create: `internal/models/event.go`
- Create: `internal/models/lease.go`

- [ ] **Step 1: Host 模型**

`internal/models/host.go`:
```go
package models

import "time"

type Host struct {
	ID           string     `json:"id" gorm:"primaryKey"`
	Name         string     `json:"name" gorm:"uniqueIndex"`
	MAC          string     `json:"mac" gorm:"uniqueIndex"`
	IP           string     `json:"ip"`
	ProfileID    *string    `json:"profile_id"`
	BMCAddr      string     `json:"bmc_addr"`
	BMCUser      string     `json:"bmc_user"`
	BMCPass      string     `json:"-" gorm:"column:bmc_pass"` // 敏感字段不序列化
	MenuOverride *string    `json:"menu_override"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
	LastOnline   *time.Time `json:"last_online"`
	BootCount    int        `json:"boot_count"`
}
```

- [ ] **Step 2: Profile 模型**

`internal/models/profile.go`:
```go
package models

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type Profile struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"uniqueIndex"`
	Description string    `json:"description"`
	MenuJSON    string    `json:"-" gorm:"column:menu"`
	IsDefault   bool      `json:"is_default" gorm:"index"`
	Arch        *uint16   `json:"arch,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *Profile) GetMenu() (*BootMenu, error) {
	if p.MenuJSON == "" {
		return &BootMenu{}, nil
	}
	var menu BootMenu
	if err := json.Unmarshal([]byte(p.MenuJSON), &menu); err != nil {
		return nil, err
	}
	return &menu, nil
}

func (p *Profile) SetMenu(menu *BootMenu) error {
	data, err := json.Marshal(menu)
	if err != nil {
		return err
	}
	p.MenuJSON = string(data)
	return nil
}

type BootMenu struct {
	Entries []MenuEntry `json:"entries"`
}

type MenuEntry struct {
	Label   string  `json:"label"`
	Type    string  `json:"type"` // local | direct | chain | sanboot | wds
	Kernel  *string `json:"kernel,omitempty"`
	Initrd  *string `json:"initrd,omitempty"`
	Cmdline *string `json:"cmdline,omitempty"`
	URL     *string `json:"url,omitempty"`
	WIM     *string `json:"wim,omitempty"`
}

// Scan 和 Value 实现 sql.Scanner 和 driver.Valuer 以便 GORM 存储
func (bm *BootMenu) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	return json.Unmarshal(value.([]byte), bm)
}

func (bm BootMenu) Value() (driver.Value, error) {
	return json.Marshal(bm)
}
```

- [ ] **Step 3: Event 和 Lease 模型**

`internal/models/event.go`:
```go
package models

import "time"

type EventType string

const (
	EventDHCP EventType = "DHCP"
	EventTFTP EventType = "TFTP"
	EventHTTP EventType = "HTTP"
	EventBoot EventType = "BOOT"
	EventIPMI EventType = "IPMI"
	EventDNS  EventType = "DNS"
)

type EventLevel string

const (
	EventInfo  EventLevel = "INFO"
	EventWarn  EventLevel = "WARN"
	EventError EventLevel = "ERROR"
)

type Event struct {
	ID        string         `json:"id" gorm:"primaryKey"`
	Type      EventType      `json:"type" gorm:"index"`
	Level     EventLevel     `json:"level" gorm:"index"`
	Message   string         `json:"message"`
	MAC       *string        `json:"mac,omitempty"`
	IP        *string        `json:"ip,omitempty"`
	Detail    map[string]any `json:"detail,omitempty" gorm:"-:all"` // JSONB 或序列化存储
	Timestamp time.Time      `json:"timestamp" gorm:"index"`
}
```

`internal/models/lease.go`:
```go
package models

import "time"

type Lease struct {
	MAC       string    `json:"mac" gorm:"primaryKey"`
	IP        string    `json:"ip"`
	SubnetID  string    `json:"subnet_id" gorm:"index"`
	Hostname  *string   `json:"hostname"`
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/models/ && git commit -m "feat: 添加数据模型"
```

### Task 4：EventBus

**文件：**
- Create: `internal/eventbus/bus.go`
- Create: `internal/eventbus/bus_test.go`

- [ ] **Step 1: 实现事件总线**

`internal/eventbus/bus.go`:
```go
package eventbus

import (
	"sync"
	"sync/atomic"
)

type Event struct {
	Topic   string
	Payload interface{}
}

type Handler func(Event)

type Bus struct {
	mu         sync.RWMutex
	subscribers map[string]map[int64]Handler
	counter    int64
}

func New() *Bus {
	return &Bus{
		subscribers: make(map[string]map[int64]Handler),
	}
}

func (b *Bus) Subscribe(topic string, handler Handler) int64 {
	b.mu.Lock()
	defer b.mu.Unlock()

	id := atomic.AddInt64(&b.counter, 1)
	if _, ok := b.subscribers[topic]; !ok {
		b.subscribers[topic] = make(map[int64]Handler)
	}
	b.subscribers[topic][id] = handler
	return id
}

func (b *Bus) Unsubscribe(topic string, id int64) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if handlers, ok := b.subscribers[topic]; ok {
		delete(handlers, id)
	}
}

func (b *Bus) Publish(topic string, payload interface{}) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if handlers, ok := b.subscribers[topic]; ok {
		for _, handler := range handlers {
			handler(Event{Topic: topic, Payload: payload})
		}
	}
}

func (b *Bus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers = make(map[string]map[int64]Handler)
}
```

- [ ] **Step 2: 写单元测试**

`internal/eventbus/bus_test.go`:
```go
package eventbus_test

import (
	"testing"
	"eventbus"
)

func TestBus(t *testing.T) {
	b := eventbus.New()

	received := make(chan eventbus.Event, 1)
	id := b.Subscribe("test", func(e eventbus.Event) {
		received <- e
	})

	b.Publish("test", "hello")

	select {
	case e := <-received:
		if e.Topic != "test" || e.Payload != "hello" {
			t.Fatalf("unexpected event: %+v", e)
		}
	default:
		t.Fatal("expected event")
	}

	b.Unsubscribe("test", id)
	b.Publish("test", "world")

	select {
	case <-received:
		t.Fatal("expected no event after unsubscribe")
	default:
	}
}
```

- [ ] **Step 3: 运行测试并提交**

```bash
go test ./internal/eventbus/ -v
git add internal/eventbus/ && git commit -m "feat: 添加事件总线"
```

### Task 5：Store 接口与 SQLite 实现

**文件：**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite.go`
- Create: `internal/store/memory.go`
- Create: `internal/store/store_test.go`

- [ ] **Step 1: 定义 Store 接口**

`internal/store/store.go`:
```go
package store

import (
	"context"
	"pxego/internal/models"
)

type Interface interface {
	HostStore
	ProfileStore
	EventStore
	LeaseStore
	Close() error
	Migrate() error
}

type HostStore interface {
	ListHosts(ctx context.Context, search string, page, size int) ([]models.Host, int64, error)
	GetHost(ctx context.Context, id string) (*models.Host, error)
	GetHostByMAC(ctx context.Context, mac string) (*models.Host, error)
	CreateHost(ctx context.Context, host *models.Host) error
	UpdateHost(ctx context.Context, host *models.Host) error
	DeleteHost(ctx context.Context, id string) error
}

type ProfileStore interface {
	ListProfiles(ctx context.Context) ([]models.Profile, error)
	GetDefaultProfile(ctx context.Context) (*models.Profile, error)
	GetProfile(ctx context.Context, id string) (*models.Profile, error)
	CreateProfile(ctx context.Context, profile *models.Profile) error
	UpdateProfile(ctx context.Context, profile *models.Profile) error
	DeleteProfile(ctx context.Context, id string) error
}

type EventStore interface {
	ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, int64, error)
	CreateEvent(ctx context.Context, event *models.Event) error
	PruneEvents(ctx context.Context, before int64) error
}

type EventFilter struct {
	Type    string
	Level   string
	Search  string
	From    int64
	To      int64
	Page    int
	Size    int
}

type LeaseStore interface {
	ListLeases(ctx context.Context) ([]models.Lease, error)
	CreateLease(ctx context.Context, lease *models.Lease) error
	DeleteLease(ctx context.Context, mac string) error
	PruneLeases(ctx context.Context) error
}
```

- [ ] **Step 2: SQLite 实现**

`internal/store/sqlite.go`:
```go
package store

import (
	"context"
	"pxego/internal/models"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type sqliteStore struct {
	db *gorm.DB
}

func NewSQLite(dsn string) (Interface, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1) // SQLite 不支持并发写入
	sqlDB.SetMaxIdleConns(1)
	return &sqliteStore{db: db}, nil
}

func (s *sqliteStore) Migrate() error {
	return s.db.AutoMigrate(
		&models.Host{},
		&models.Profile{},
		&models.Event{},
		&models.Lease{},
	)
}

func (s *sqliteStore) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Host 操作
func (s *sqliteStore) ListHosts(ctx context.Context, search string, page, size int) ([]models.Host, int64, error) {
	var hosts []models.Host
	var total int64
	query := s.db.WithContext(ctx).Model(&models.Host{})
	if search != "" {
		query = query.Where("name LIKE ? OR mac LIKE ? OR ip LIKE ?", "%"+search+"%", "%"+search+"%", "%"+search+"%")
	}
	query.Count(&total)
	offset := (page - 1) * size
	if err := query.Order("created_at DESC").Offset(offset).Limit(size).Find(&hosts).Error; err != nil {
		return nil, 0, err
	}
	return hosts, total, nil
}

func (s *sqliteStore) GetHost(ctx context.Context, id string) (*models.Host, error) {
	var host models.Host
	if err := s.db.WithContext(ctx).First(&host, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *sqliteStore) GetHostByMAC(ctx context.Context, mac string) (*models.Host, error) {
	var host models.Host
	if err := s.db.WithContext(ctx).First(&host, "mac = ?", mac).Error; err != nil {
		return nil, err
	}
	return &host, nil
}

func (s *sqliteStore) CreateHost(ctx context.Context, host *models.Host) error {
	return s.db.WithContext(ctx).Create(host).Error
}

func (s *sqliteStore) UpdateHost(ctx context.Context, host *models.Host) error {
	return s.db.WithContext(ctx).Save(host).Error
}

func (s *sqliteStore) DeleteHost(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Host{}, "id = ?", id).Error
}

// Profile 操作
func (s *sqliteStore) ListProfiles(ctx context.Context) ([]models.Profile, error) {
	var profiles []models.Profile
	if err := s.db.WithContext(ctx).Order("created_at DESC").Find(&profiles).Error; err != nil {
		return nil, err
	}
	return profiles, nil
}

func (s *sqliteStore) GetDefaultProfile(ctx context.Context) (*models.Profile, error) {
	var profile models.Profile
	if err := s.db.WithContext(ctx).First(&profile, "is_default = ?", true).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *sqliteStore) GetProfile(ctx context.Context, id string) (*models.Profile, error) {
	var profile models.Profile
	if err := s.db.WithContext(ctx).First(&profile, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &profile, nil
}

func (s *sqliteStore) CreateProfile(ctx context.Context, profile *models.Profile) error {
	if profile.IsDefault {
		s.db.WithContext(ctx).Model(&models.Profile{}).Where("is_default = ?", true).Update("is_default", false)
	}
	return s.db.WithContext(ctx).Create(profile).Error
}

func (s *sqliteStore) UpdateProfile(ctx context.Context, profile *models.Profile) error {
	if profile.IsDefault {
		s.db.WithContext(ctx).Model(&models.Profile{}).Where("is_default = ?", true).Update("is_default", false)
	}
	return s.db.WithContext(ctx).Save(profile).Error
}

func (s *sqliteStore) DeleteProfile(ctx context.Context, id string) error {
	return s.db.WithContext(ctx).Delete(&models.Profile{}, "id = ?", id).Error
}

// Event 操作
func (s *sqliteStore) ListEvents(ctx context.Context, filter EventFilter) ([]models.Event, int64, error) {
	var events []models.Event
	var total int64
	query := s.db.WithContext(ctx).Model(&models.Event{})
	if filter.Type != "" {
		query = query.Where("type = ?", filter.Type)
	}
	if filter.Level != "" {
		query = query.Where("level = ?", filter.Level)
	}
	query.Count(&total)
	if err := query.Order("timestamp DESC").Offset((filter.Page - 1) * filter.Size).Limit(filter.Size).Find(&events).Error; err != nil {
		return nil, 0, err
	}
	return events, total, nil
}

func (s *sqliteStore) CreateEvent(ctx context.Context, event *models.Event) error {
	return s.db.WithContext(ctx).Create(event).Error
}

func (s *sqliteStore) PruneEvents(ctx context.Context, before int64) error {
	return s.db.WithContext(ctx).Where("timestamp < ?", time.Unix(before, 0)).Delete(&models.Event{}).Error
}

// Lease 操作
func (s *sqliteStore) ListLeases(ctx context.Context) ([]models.Lease, error) {
	var leases []models.Lease
	if err := s.db.WithContext(ctx).Find(&leases).Error; err != nil {
		return nil, err
	}
	return leases, nil
}

func (s *sqliteStore) CreateLease(ctx context.Context, lease *models.Lease) error {
	return s.db.WithContext(ctx).Create(lease).Error
}

func (s *sqliteStore) DeleteLease(ctx context.Context, mac string) error {
	return s.db.WithContext(ctx).Delete(&models.Lease{}, "mac = ?", mac).Error
}

func (s *sqliteStore) PruneLeases(ctx context.Context) error {
	return s.db.WithContext(ctx).Where("expires_at < ?", time.Now()).Delete(&models.Lease{}).Error
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/store/ && git commit -m "feat: 添加 Store 接口和 SQLite 实现"
```

### Task 6：主入口

**文件：**
- Create: `cmd/pxego/main.go`

- [ ] **Step 1: 实现 cobra 主命令**

`cmd/pxego/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
	"pxego/internal/config"
	"pxego/internal/app"
	"pxego/internal/store"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "pxego",
	Short: "PxeGo - 一体化 PXE 服务器",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func init() {
	config.BindFlags(rootCmd)
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "配置文件路径")
}

func run(cmd *cobra.Command) error {
	cfg, err := config.LoadConfig(cfgFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化日志
	var level slog.Level
	switch cfg.Log.Level {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	slog.SetLogLoggerLevel(level)

	slog.Info("PxeGo 启动", "data_dir", cfg.Global.DataDir)

	// 初始化数据库
	st, err := store.NewSQLite(cfg.Store.DSN)
	if err != nil {
		return fmt.Errorf("初始化数据库失败: %w", err)
	}
	if err := st.Migrate(); err != nil {
		return fmt.Errorf("数据库迁移失败: %w", err)
	}
	defer st.Close()

	// 创建 App
	pxeApp := app.New()
	// TODO: 后续注册各服务
	_ = pxeApp

	return pxeApp.Run(context.Background())
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		slog.Error("程序退出", "error", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 2: 验证编译**

```bash
go build ./cmd/pxego
./pxego --help
```

- [ ] **Step 3: 提交**

```bash
git add cmd/pxego/main.go && git commit -m "feat: 添加 PxeGo 主入口"
```

---

## Phase 2：DHCP 核心

### Task 7：DHCP Lease 管理器

**文件：**
- Create: `internal/dhcp/lease.go`
- Create: `internal/dhcp/lease_test.go`

- [ ] **Step 1: 实现 IP 池管理**

`internal/dhcp/lease.go`:
```go
package dhcp

import (
	"fmt"
	"net"
	"sync"
	"time"
	"pxego/internal/models"
	"pxego/internal/store"
)

type LeaseManager struct {
	mu      sync.RWMutex
	store   store.LeaseStore
	subnets map[string]*SubnetPool
}

type SubnetPool struct {
	CIDR    string
	Gateway net.IP
	Pool    *IPRange
	Leases  map[string]*models.Lease // MAC → Lease
}

type IPRange struct {
	Start net.IP
	End   net.IP
}

func NewIPRange(start, end string) (*IPRange, error) {
	s := net.ParseIP(start)
	e := net.ParseIP(end)
	if s == nil || e == nil {
		return nil, fmt.Errorf("无效的 IP 范围: %s - %s", start, end)
	}
	return &IPRange{Start: s, End: e}, nil
}

func (r *IPRange) Contains(ip net.IP) bool {
	return bytesCompare(ip, r.Start) >= 0 && bytesCompare(ip, r.End) <= 0
}

func bytesCompare(a, b net.IP) int {
	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func NewLeaseManager(store store.LeaseStore) *LeaseManager {
	return &LeaseManager{
		store:   store,
		subnets: make(map[string]*SubnetPool),
	}
}

func (lm *LeaseManager) AddSubnet(cidr string, pool *IPRange, gateway net.IP) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.subnets[cidr] = &SubnetPool{
		CIDR:    cidr,
		Gateway: gateway,
		Pool:    pool,
		Leases:  make(map[string]*models.Lease),
	}
}

func (lm *LeaseManager) Allocate(cidr, mac string) (net.IP, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	pool, ok := lm.subnets[cidr]
	if !ok {
		return nil, fmt.Errorf("子网 %s 未配置", cidr)
	}

	// 检查是否已有租约
	if lease, ok := pool.Leases[mac]; ok {
		if time.Now().Before(lease.ExpiresAt) {
			return net.ParseIP(lease.IP), nil
		}
	}

	// 尝试分配第一个可用的 IP
	ip := make(net.IP, len(pool.Pool.Start))
	copy(ip, pool.Pool.Start)
	for {
		if bytesCompare(ip, pool.Pool.End) > 0 {
			return nil, fmt.Errorf("子网 %s 无可用 IP", cidr)
		}
		// 检查是否已被占用
		used := false
		for _, lease := range pool.Leases {
			if lease.IP == ip.String() {
				used = true
				break
			}
		}
		if !used {
			break
		}
		incIP(ip)
	}

	lease := &models.Lease{
		MAC:       mac,
		IP:        ip.String(),
		SubnetID:  cidr,
		ExpiresAt: time.Now().Add(1 * time.Hour),
		CreatedAt: time.Now(),
	}
	pool.Leases[mac] = lease
	lm.store.CreateLease(nil, lease)
	return ip, nil
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/dhcp/lease.go && git commit -m "feat: 添加 DHCP 租约管理"
```

### Task 8：PXE Option 构建器

**文件：**
- Create: `internal/dhcp/options.go`
- Create: `internal/dhcp/options_test.go`

- [ ] **Step 1: 实现 PXE option 43 子选项构建**

`internal/dhcp/options.go`:
```go
package dhcp

import (
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/iana"
)

// BuildPXEOptions 构建 PXE 相关的 DHCP 选项
func BuildPXEOptions(arch iana.Arch, nextServer string, bootFile string, menuData []byte) []dhcpv4.Option {
	options := []dhcpv4.Option{
		dhcpv4.OptTFTPServerName(nextServer),
		dhcpv4.OptBootFileName(bootFile),
	}

	// Option 43 - Vendor Specific Information
	// PXE sub-option 6: Discovery Control (不启用 multicast，允许广播)
	discoveryCtrl := []byte{6, 1, 0x08} // sub-opt 6, len 1, bit3=1 (禁用广播发现用直接方式)
	_ = discoveryCtrl // 后续使用

	// PXE sub-option 8: Boot Menu
	if menuData != nil {
		// sub-option 8 格式：类型(2)+层(2)+描述长度(1)+描述
		// 简化实现：添加一个默认启动项
		bootMenu := buildSubOption(8, menuData)
		option43 := dhcpv4.OptGeneric(dhcpv4.OptionVendorSpecificInformation, bootMenu)
		options = append(options, option43)
	}

	return options
}

func buildSubOption(subOpt byte, data []byte) []byte {
	// 子选项编码：子选项代码(1)+长度(1)+数据(n)
	length := len(data)
	result := make([]byte, 0, 2+length)
	result = append(result, subOpt)
	result = append(result, byte(length))
	result = append(result, data...)
	return result
}

// DetectClientArch 从 DHCP 包读取架构类型
func DetectClientArch(pkt *dhcpv4.DHCPv4) (iana.Arch, bool) {
	if pkt == nil {
		return 0, false
	}
	// 读取 Option 93
	archs := pkt.ClientArch()
	if len(archs) > 0 {
		return archs[0], true
	}
	return 0, false
}

// IsPXEClient 检测客户端是否为 PXE 客户端（Option 60）
func IsPXEClient(pkt *dhcpv4.DHCPv4) bool {
	if pkt == nil {
		return false
	}
	vci := pkt.ClassIdentifier()
	return vci == "PXEClient"
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/dhcp/options.go && git commit -m "feat: 添加 PXE option 构建器"
```

### Task 9：DHCP Handler 与 Server

**文件：**
- Create: `internal/dhcp/handler.go`
- Create: `internal/dhcp/server.go`
- Create: `internal/dhcp/handler_test.go`

- [ ] **Step 1: DHCP Handler 实现**

`internal/dhcp/handler.go`:
```go
package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"pxego/internal/config"
	"pxego/internal/eventbus"
	"pxego/internal/models"
	"pxego/internal/store"

	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/insomniacslk/dhcp/dhcpv4/serverudp"
)

type Handler struct {
	config    *config.Config
	store     store.Interface
	leaseMgr  *LeaseManager
	eventBus  *eventbus.Bus
	nextSrvIP net.IP
}

func NewHandler(cfg *config.Config, st store.Interface, bus *eventbus.Bus) *Handler {
	return &Handler{
		config:   cfg,
		store:    st,
		leaseMgr: NewLeaseManager(st),
		eventBus: bus,
	}
}

func (h *Handler) InitSubnets() {
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			r, _ := NewIPRange(subnet.Pool, subnet.Pool)
			_ = r
			// TODO: 解析 pool 字符串为 start-end
			// 简化: pool 格式 "192.168.1.100-200"
		}
	}
}

func (h *Handler) Handle(ctx context.Context, peer net.Addr, pkt *dhcpv4.DHCPv4) {
	if pkt == nil || pkt.MessageType() != dhcpv4.MessageTypeDiscover && pkt.MessageType() != dhcpv4.MessageTypeRequest {
		return
	}

	mac := pkt.ClientHWAddr.String()
	isPXE := IsPXEClient(pkt)

	slog.Debug("收到 DHCP 包",
		"mac", mac,
		"type", pkt.MessageType(),
		"isPXE", isPXE,
	)

	var msgType string
	if isPXE {
		msgType = "PXE"
	} else {
		msgType = "DHCP"
	}

	h.eventBus.Publish("event", models.Event{
		Type:    models.EventDHCP,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("%s 请求: %s", msgType, mac),
		MAC:     &mac,
	})

	// 确定 DHCP 模式
	dhcpMode := "hybrid"
	if isPXE {
		// PXE 客户端 → 按接口配置走
		for _, iface := range h.config.Interfaces {
			if iface.DHCP == "proxy" {
				dhcpMode = "proxy"
			} else if iface.DHCP == "full" {
				dhcpMode = "full"
			}
		}
		if dhcpMode == "hybrid" {
			dhcpMode = "proxy"
		}
	}

	// 获取 NextServer
	var nextServer net.IP
	for _, iface := range h.config.Interfaces {
		for _, subnet := range iface.Subnets {
			nextServer = net.ParseIP(subnet.NextServer)
		}
	}

	var reply *dhcpv4.DHCPv4

	switch pkt.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		reply = h.handleDiscover(pkt, dhcpMode, nextServer)
	case dhcpv4.MessageTypeRequest:
		reply = h.handleRequest(pkt, dhcpMode, nextServer)
	}

	if reply != nil {
		reply.UpdateOption(dhcpv4.OptServerIdentifier(nextServer))
		if _, err := serverudp.Send(peer, reply); err != nil {
			slog.Error("发送 DHCP 响应失败", "error", err)
		}
	}
}

func (h *Handler) handleDiscover(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}

	switch mode {
	case "proxy":
		// ProxyDHCP：只填 PXE 选项，不分配 IP
		reply.YourIPAddr = net.IP{0, 0, 0, 0}
		reply.ServerIPAddr = nextServer
		if arch, ok := DetectClientArch(pkt); ok {
			bootFile := BootFileForArch(arch)
			reply.BootFileName = bootFile
		}
	case "full":
		// Full DHCP：分配 IP + PXE 选项
		if ip, err := h.leaseMgr.Allocate("", pkt.ClientHWAddr.String()); err == nil {
			reply.YourIPAddr = ip
		}
		reply.ServerIPAddr = nextServer
	}

	return reply
}

func (h *Handler) handleRequest(pkt *dhcpv4.DHCPv4, mode string, nextServer net.IP) *dhcpv4.DHCPv4 {
	reply, err := dhcpv4.NewReplyFromRequest(pkt)
	if err != nil {
		return nil
	}
	reply.ServerIPAddr = nextServer
	return reply
}
```

- [ ] **Step 2: DHCP Server 实现**

`internal/dhcp/server.go`:
```go
package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"pxego/internal/config"
	"pxego/internal/eventbus"
	"pxego/internal/store"

	"github.com/insomniacslk/dhcp/dhcpv4/serverudp"
)

type Server struct {
	name     string
	addr     string
	handler  *Handler
	srv      *serverudp.Server
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus) (*Server, error) {
	handler := NewHandler(cfg, st, bus)
	handler.InitSubnets()

	addr := fmt.Sprintf("0.0.0.0:%d", config.DefaultPortDHCP)
	srv, err := serverudp.NewServer(addr, func(peer net.Addr, pkt []byte) {
		dhcpPkt, err := dhcpv4.FromBytes(pkt)
		if err != nil {
			slog.Error("解析 DHCP 包失败", "error", err)
			return
		}
		handler.Handle(context.Background(), peer, dhcpPkt)
	})
	if err != nil {
		return nil, fmt.Errorf("创建 DHCP 服务器失败: %w", err)
	}

	return &Server{
		name:    "DHCP",
		addr:    addr,
		handler: handler,
		srv:     srv,
	}, nil
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	slog.Info("DHCP 服务启动", "addr", s.addr)
	go func() {
		if err := s.srv.Serve(); err != nil {
			slog.Error("DHCP 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DHCP 服务关闭")
	return s.srv.Close()
}
```

`internal/dhcp/server_4011.go`:
```go
package dhcp

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"pxego/internal/config"
	"pxego/internal/eventbus"
	"pxego/internal/store"
)

type ProxyServer4011 struct {
	name    string
	addr    string
	handler *Handler
	conn    *net.UDPConn
}

func NewProxyServer4011(cfg *config.Config, st store.Interface, bus *eventbus.Bus) (*ProxyServer4011, error) {
	handler := NewHandler(cfg, st, bus)
	addr := fmt.Sprintf("0.0.0.0:%d", config.DefaultPortDHCP4011)
	return &ProxyServer4011{
		name:    "ProxyDHCP-4011",
		addr:    addr,
		handler: handler,
	}, nil
}

func (s *ProxyServer4011) Name() string { return s.name }

func (s *ProxyServer4011) Start(ctx context.Context) error {
	udpAddr, err := net.ResolveUDPAddr("udp", s.addr)
	if err != nil {
		return err
	}
	s.conn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("监听 :4011 失败: %w", err)
	}

	slog.Info("ProxyDHCP 服务启动", "addr", s.addr)

	buf := make([]byte, 1500)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			n, peer, err := s.conn.ReadFromUDP(buf)
			if err != nil {
				if ctx.Err() != nil {
					return nil
				}
				slog.Error("ProxyDHCP 读取错误", "error", err)
				continue
			}
			pkt, err := dhcpv4.FromBytes(buf[:n])
			if err != nil {
				continue
			}
			s.handler.Handle(ctx, peer, pkt)
		}
	}
}

func (s *ProxyServer4011) Stop(ctx context.Context) error {
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}
```

- [ ] **Step 3: 编译验证**

```bash
go build ./...
```

- [ ] **Step 4: 提交**

```bash
git add internal/dhcp/ && git commit -m "feat: 添加 DHCP 服务器 (full/proxy/hybrid + :4011)"
```

---

## Phase 3：TFTP + BootFileServer

### Task 10：BootFileServer

**文件：**
- Create: `internal/boot/serve.go`
- Create: `internal/boot/archmap.go`

- [ ] **Step 1: 统一文件服务**

`internal/boot/serve.go`:
```go
package boot

import (
	"fmt"
	"os"
	"path/filepath"
)

type BootFileServer struct {
	rootDir string
}

func NewBootFileServer(rootDir string) *BootFileServer {
	return &BootFileServer{rootDir: rootDir}
}

func (b *BootFileServer) Root() string {
	return b.rootDir
}

func (b *BootFileServer) Read(path string) ([]byte, error) {
	fullPath := filepath.Join(b.rootDir, path)
	// 安全检查：防止目录遍历
	absRoot, _ := filepath.Abs(b.rootDir)
	absFile, _ := filepath.Abs(fullPath)
	if len(absFile) < len(absRoot) || absFile[:len(absRoot)] != absRoot {
		return nil, fmt.Errorf("路径越权: %s", path)
	}
	return os.ReadFile(fullPath)
}

func (b *BootFileServer) Exists(path string) bool {
	fullPath := filepath.Join(b.rootDir, path)
	_, err := os.Stat(fullPath)
	return err == nil
}

func (b *BootFileServer) List(dir string) ([]os.FileInfo, error) {
	fullPath := filepath.Join(b.rootDir, dir)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, err
	}
	infos := make([]os.FileInfo, 0, len(entries))
	for _, e := range entries {
		info, _ := e.Info()
		infos = append(infos, info)
	}
	return infos, nil
}
```

- [ ] **Step 2: 架构 → 引导文件映射**

`internal/boot/archmap.go`:
```go
package boot

import "github.com/insomniacslk/dhcp/iana"

// BootFileForArch 根据 PXE 客户端架构返回引导文件
func BootFileForArch(arch iana.Arch) string {
	switch arch {
	case iana.INTEL_X86PC:
		return "undionly.kpxe"
	case iana.EFI_IA32:
		return "ipxe32.efi"
	case iana.EFI_X86_64, iana.EFI_BC:
		return "ipxe.efi"
	case iana.EFI_ARM64:
		return "ipxe-arm64.efi"
	case iana.EFI_RISCV64:
		return "ipxe-riscv64.efi"
	default:
		return "undionly.kpxe"
	}
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/boot/serve.go internal/boot/archmap.go && git commit -m "feat: 添加 BootFileServer 和架构映射"
```

### Task 11：TFTP Server

**文件：**
- Create: `internal/tftp/server.go`
- Create: `internal/tftp/handler.go`

- [ ] **Step 1: TFTP Server**

`internal/tftp/server.go`:
```go
package tftp

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"pxego/internal/boot"
	"pxego/internal/eventbus"
	"pxego/internal/models"

	"github.com/pin/tftp"
)

type Server struct {
	name        string
	port        int
	server      *tftp.Server
	bootFS      *boot.BootFileServer
	maxConns    int
	activeConns chan struct{}
	eventBus    *eventbus.Bus
}

func NewServer(port int, bootFS *boot.BootFileServer, bus *eventbus.Bus) *Server {
	s := &Server{
		name:        "TFTP",
		port:        port,
		bootFS:      bootFS,
		maxConns:    50,
		activeConns: make(chan struct{}, 50),
		eventBus:    bus,
	}

	readHandler := func(filename string, rf io.ReaderFrom) error {
		// 限流
		s.activeConns <- struct{}{}
		defer func() { <-s.activeConns }()

		slog.Debug("TFTP 读取请求", "file", filename)

		s.eventBus.Publish("event", models.Event{
			Type:    models.EventTFTP,
			Level:   models.EventInfo,
			Message: fmt.Sprintf("TFTP 读取: %s", filename),
		})

		data, err := s.bootFS.Read(filename)
		if err != nil {
			slog.Warn("TFTP 文件未找到", "file", filename)
			return fmt.Errorf("文件未找到: %s", filename)
		}

		_, err = rf.ReadFrom(io.NopCloser(bytes.NewReader(data)))
		return err
	}

	s.server = tftp.NewServer(readHandler, nil)
	s.server.SetReadTimeout(5 * time.Second) // 5s 超时
	return s
}

func (s *Server) Name() string { return s.name }
func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("TFTP 服务启动", "addr", addr)
	go func() {
		if err := s.server.ListenAndServe(addr); err != nil {
			slog.Error("TFTP 服务异常退出", "error", err)
		}
	}()
	return nil
}
func (s *Server) Stop(ctx context.Context) error {
	slog.Info("TFTP 服务关闭")
	s.server.Shutdown()
	return nil
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/tftp/ && git commit -m "feat: 添加 TFTP 服务器"
```

---

## Phase 4：HTTP + iPXE

### Task 12：HTTP 服务器 + 路由

**文件：**
- Create: `internal/httpd/server.go`
- Create: `internal/httpd/router.go`
- Create: `internal/httpd/middleware.go`

- [ ] **Step 1: Chi 路由和 HTTP Server**

`internal/httpd/server.go`:
```go
package httpd

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"pxego/internal/api"
	"pxego/internal/boot"
	"pxego/internal/config"
	"pxego/internal/eventbus"
	"pxego/internal/store"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	name   string
	cfg    *config.Config
	router chi.Router
	srv    *http.Server
	bootFS *boot.BootFileServer
	api    *api.Handler
}

func NewServer(cfg *config.Config, st store.Interface, bus *eventbus.Bus, bootFS *boot.BootFileServer) *Server {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(CORSMiddleware)
	if cfg.Auth.Token != "" {
		r.Use(AuthMiddleware(cfg.Auth.Token))
	}

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	apiHandler := api.NewHandler(st, bus)
	apiHandler.RegisterRoutes(r)

	return &Server{
		name:   "HTTP",
		cfg:    cfg,
		router: r,
		bootFS: bootFS,
		api:    apiHandler,
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	addr := fmt.Sprintf(":%d", config.DefaultPortHTTP)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}
	slog.Info("HTTP 服务启动", "addr", addr)
	go func() {
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("HTTP 服务关闭")
	return s.srv.Shutdown(ctx)
}
```

`internal/httpd/middleware.go`:
```go
package httpd

import (
	"net/http"
	"strings"
)

func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") != "Bearer "+token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/httpd/ && git commit -m "feat: 添加 HTTP 服务器基础架构"
```

### Task 13：iPXE 模板引擎

**文件：**
- Create: `internal/boot/ipxe/engine.go`
- Create: `internal/boot/ipxe/templates.go`
- Create: `internal/boot/ipxe/engine_test.go`

- [ ] **Step 1: 模板引擎**

`internal/boot/ipxe/engine.go`:
```go
package ipxe

import (
	"bytes"
	"text/template"
)

type BootType string

const (
	BootMenu    BootType = "menu"
	BootDirect  BootType = "direct"
	BootLocal   BootType = "local"
	BootChain   BootType = "chain"
	BootSANBoot BootType = "sanboot"
	BootWDS     BootType = "wds"
)

type TemplateData struct {
	NextServer string
	BootFile   string
	MAC        string
	IP         string
	Hostname   string
	Menu       *MenuData
	OS         *OSData
}

type MenuData struct {
	Title   string
	Entries []MenuEntryData
	Timeout int
	Default int
}

type MenuEntryData struct {
	Label   string
	Type    BootType
	Kernel  string
	Initrd  string
	Cmdline string
	URL     string
	WIM     string
}

type OSData struct {
	Kernel  string
	Initrd  string
	Cmdline string
}

type Engine struct {
	templates map[string]*template.Template
}

func New() *Engine {
	e := &Engine{templates: make(map[string]*template.Template)}
	for name, text := range builtinTemplates {
		e.templates[name] = template.Must(template.New(name).Parse(text))
	}
	return e
}

func (e *Engine) Render(name string, data TemplateData) (string, error) {
	tmpl, ok := e.templates[name]
	if !ok {
		return "", nil
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
```

- [ ] **Step 2: 内置模板**

`internal/boot/ipxe/templates.go`:
```go
package ipxe

var builtinTemplates = map[string]string{
	"menu": `#!ipxe
set menu-timeout {{.Menu.Timeout}}
set menu-default {{.Menu.Default}}

:menu
menu {{.Menu.Title}}
{{range $i, $entry := .Menu.Entries}}
item{{if eq $i $.Menu.Default}} --default{{end}} --key {{$i}} {{$i}} {{$entry.Label}}
{{end}}
choose --timeout ${menu-timeout} --default ${menu-default} selected || goto shell
goto ${selected}

:shell
shell
goto menu

{{range $i, $entry := .Menu.Entries}}
:{{$i}}
{{if eq $entry.Type "direct"}}
kernel {{$entry.Kernel}} {{$entry.Cmdline}}
initrd {{$entry.Initrd}}
boot
{{else if eq $entry.Type "local"}}
exit
{{else if eq $entry.Type "chain"}}
chain {{$entry.URL}}
{{else if eq $entry.Type "sanboot"}}
sanboot {{$entry.URL}}
{{else if eq $entry.Type "wds"}}
set wds-server {{$.NextServer}}
kernel wdsmgfw.efi
initrd bootmgr.exe
initrd boot.sdi
initrd {{$entry.WIM}}
boot
{{end}}
{{end}}
`,

	"direct": `#!ipxe
kernel {{.OS.Kernel}} {{.OS.Cmdline}}
initrd {{.OS.Initrd}}
boot
`,

	"local": `#!ipxe
exit
`,

	"chain": `#!ipxe
chain {{.URL}}
`,

	"sanboot": `#!ipxe
sanboot {{.URL}}
`,
}
```

- [ ] **Step 3: 提交**

```bash
git add internal/boot/ipxe/ && git commit -m "feat: 添加 iPXE 模板引擎和内置模板"
```

---

## Phase 5：DNS + PXELinux

### Task 14：DNS 服务器

**文件：**
- Create: `internal/dns/server.go`
- Create: `internal/dns/handler.go`

- [ ] **Step 1: DNS 转发服务器**

`internal/dns/server.go`:
```go
package dns

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"pxego/internal/eventbus"
	"pxego/internal/models"

	"github.com/miekg/dns"
)

type Server struct {
	name    string
	port    int
	handler *Handler
	dnsSrv  *dns.Server
}

func NewServer(port int, upstream string, bus *eventbus.Bus) *Server {
	return &Server{
		name:    "DNS",
		port:    port,
		handler: NewHandler(upstream, bus),
	}
}

func (s *Server) Name() string { return s.name }

func (s *Server) Start(ctx context.Context) error {
	mux := dns.NewServeMux()
	mux.Handle(".", s.handler)

	s.dnsSrv = &dns.Server{
		Addr:    fmt.Sprintf(":%d", s.port),
		Net:     "udp",
		Handler: mux,
	}

	slog.Info("DNS 服务启动", "addr", s.dnsSrv.Addr)
	go func() {
		if err := s.dnsSrv.ListenAndServe(); err != nil {
			slog.Error("DNS 服务异常退出", "error", err)
		}
	}()
	return nil
}

func (s *Server) Stop(ctx context.Context) error {
	slog.Info("DNS 服务关闭")
	return s.dnsSrv.Shutdown()
}
```

`internal/dns/handler.go`:
```go
package dns

import (
	"log/slog"
	"net"
	"pxego/internal/eventbus"
	"pxego/internal/models"

	"github.com/miekg/dns"
)

type Handler struct {
	upstream string
	client   *dns.Client
	eventBus *eventbus.Bus
}

func NewHandler(upstream string, bus *eventbus.Bus) *Handler {
	return &Handler{
		upstream: upstream,
		client:   &dns.Client{},
		eventBus: bus,
	}
}

func (h *Handler) ServeDNS(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	slog.Debug("DNS 查询", "name", q.Name, "type", q.Qtype)

	if h.upstream != "" {
		// 转发模式
		resp, _, err := h.client.Exchange(r, h.upstream)
		if err != nil {
			slog.Error("DNS 转发失败", "error", err)
			m.Rcode = dns.RcodeServerFailure
		} else {
			m = resp
		}
	}

	h.eventBus.Publish("event", models.Event{
		Type:    models.EventDNS,
		Level:   models.EventInfo,
		Message: fmt.Sprintf("DNS 查询: %s", q.Name),
	})
	w.WriteMsg(m)
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/dns/ && git commit -m "feat: 添加 DNS 服务器"
```

### Task 15：PXELinux 解析器

**文件：**
- Create: `internal/boot/pxelinux/ast.go`
- Create: `internal/boot/pxelinux/parser.go`
- Create: `internal/boot/pxelinux/parser_test.go`
- Create: `internal/boot/pxelinux/generator.go`

- [ ] **Step 1: AST 定义**

`internal/boot/pxelinux/ast.go`:
```go
package pxelinux

type AST struct {
	Defaults  Label
	Labels    []Label
	OnTimeout string
	OnError   string
	MenuTitle string
	Timeout   int
	Default   string
}

type Label struct {
	Name       string
	Kernel     string
	Append     []string
	Initrd     string
	MenuLabel  string
	MenuTitle  string
	MenuIndent int
}
```

- [ ] **Step 2: 解析器**

`internal/boot/pxelinux/parser.go`:
```go
package pxelinux

import (
	"bufio"
	"fmt"
	"strings"
)

func Parse(data string) (*AST, error) {
	ast := &AST{}
	scanner := bufio.NewScanner(strings.NewReader(data))
	var currentLabel *Label

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		tokens := strings.Fields(line)
		if len(tokens) == 0 {
			continue
		}

		cmd := strings.ToLower(tokens[0])
		args := tokens[1:]

		switch cmd {
		case "default":
			ast.Default = strings.Join(args, " ")
		case "timeout":
			fmt.Sscanf(strings.Join(args, " "), "%d", &ast.Timeout)
		case "ontimeout":
			ast.OnTimeout = strings.Join(args, " ")
		case "onerror":
			ast.OnError = strings.Join(args, " ")
		case "menu":
			handleMenuCommand(ast, args)
		case "label":
			if currentLabel != nil {
				ast.Labels = append(ast.Labels, *currentLabel)
			}
			currentLabel = &Label{Name: strings.Join(args, " ")}
		case "kernel":
			if currentLabel != nil {
				currentLabel.Kernel = strings.Join(args, " ")
			}
		case "append":
			if currentLabel != nil {
				currentLabel.Append = args
			}
		case "initrd":
			if currentLabel != nil {
				currentLabel.Initrd = strings.Join(args, " ")
			}
		case "menu label":
			if currentLabel != nil {
				currentLabel.MenuLabel = strings.Join(args, " ")
			}
		}
	}

	if currentLabel != nil {
		ast.Labels = append(ast.Labels, *currentLabel)
	}

	return ast, nil
}

func handleMenuCommand(ast *AST, args []string) {
	if len(args) >= 2 && strings.ToLower(args[0]) == "title" {
		ast.MenuTitle = strings.Join(args[1:], " ")
	}
}
```

- [ ] **Step 3: 生成器**

`internal/boot/pxelinux/generator.go`:
```go
package pxelinux

import (
	"fmt"
	"strings"
)

func Generate(ast *AST, nextServer string) string {
	var sb strings.Builder

	sb.WriteString("#!ipxe\n")
	if ast.Timeout > 0 {
		sb.WriteString(fmt.Sprintf("set menu-timeout %d\n", ast.Timeout/10))
	}
	if ast.Default != "" {
		sb.WriteString(fmt.Sprintf("set menu-default %s\n", ast.Default))
	}

	// 生成菜单
	sb.WriteString(":menu\n")
	title := ast.MenuTitle
	if title == "" {
		title = "PXE Boot Menu"
	}
	sb.WriteString(fmt.Sprintf("menu %s\n", title))

	for _, label := range ast.Labels {
		menuLabel := label.MenuLabel
		if menuLabel == "" {
			menuLabel = label.Name
		}
		sb.WriteString(fmt.Sprintf("item %s %s\n", label.Name, menuLabel))
	}

	sb.WriteString("choose --timeout ${menu-timeout} --default ${menu-default} selected || goto shell\n")
	sb.WriteString("goto ${selected}\n\n")

	// 生成条目
	for _, label := range ast.Labels {
		sb.WriteString(fmt.Sprintf(":%s\n", label.Name))
		if label.Kernel != "" {
			kernel := rewritePath(label.Kernel, nextServer)
			sb.WriteString(fmt.Sprintf("kernel %s %s\n", kernel, strings.Join(label.Append, " ")))
			if label.Initrd != "" {
				initrd := rewritePath(label.Initrd, nextServer)
				sb.WriteString(fmt.Sprintf("initrd %s\n", initrd))
			}
			sb.WriteString("boot\n")
		} else {
			sb.WriteString("exit\n")
		}
	}

	return sb.String()
}

// 重写相对路径为 HTTP URL
func rewritePath(path, server string) string {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") || strings.HasPrefix(path, "tftp://") {
		return path
	}
	return fmt.Sprintf("http://%s/boot/%s", server, strings.TrimPrefix(path, "/"))
}
```

- [ ] **Step 4: 解析器测试**

`internal/boot/pxelinux/parser_test.go`:
```go
package pxelinux

import "testing"

func TestParse(t *testing.T) {
	input := `default ubuntu
timeout 100

menu title PXE Boot Menu

label ubuntu
  menu label Ubuntu 24.04
  kernel /ubuntu/vmlinuz
  append initrd=/ubuntu/initrd.img root=/dev/nfs netboot=nfs

label local
  menu label Boot from Local Disk
  local
`
	ast, err := Parse(input)
	if err != nil {
		t.Fatal(err)
	}
	if ast.Default != "ubuntu" {
		t.Fatalf("expected default=ubuntu, got %s", ast.Default)
	}
	if len(ast.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d", len(ast.Labels))
	}
	if ast.Labels[0].Kernel != "/ubuntu/vmlinuz" {
		t.Fatalf("expected kernel=/ubuntu/vmlinuz, got %s", ast.Labels[0].Kernel)
	}

	// 测试生成
	script := Generate(ast, "192.168.1.10")
	if !strings.Contains(script, "http://192.168.1.10/boot/ubuntu/vmlinuz") {
		t.Fatal("generated script should have HTTP URL")
	}
}
```

- [ ] **Step 5: 提交**

```bash
git add internal/boot/pxelinux/ && git commit -m "feat: 添加 PXELinux 解析器和 iPXE 生成器"
```

---

## Phase 6：REST API + IPMI + WDS

### Task 16：API 处理层

**文件：**
- Create: `internal/api/response.go`
- Create: `internal/api/hosts.go`
- Create: `internal/api/profiles.go`
- Create: `internal/api/events.go`
- Create: `internal/api/status.go`
- Create: `internal/api/files.go`

- [ ] **Step 1: 统一响应格式**

`internal/api/response.go`:
```go
package api

import (
	"encoding/json"
	"net/http"
)

type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    *Meta       `json:"meta,omitempty"`
}

type Meta struct {
	Page  int   `json:"page"`
	Size  int   `json:"size"`
	Total int64 `json:"total"`
}

func JSON(w http.ResponseWriter, status int, resp Response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func OK(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusOK, Response{Success: true, Data: data})
}

func Created(w http.ResponseWriter, data interface{}) {
	JSON(w, http.StatusCreated, Response{Success: true, Data: data})
}

func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, Response{Success: false, Error: msg})
}
```

- [ ] **Step 2: 主机 API**

`internal/api/hosts.go`:
```go
package api

import (
	"net/http"
	"pxego/internal/models"
	"pxego/internal/store"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

type HostHandler struct {
	store store.Interface
}

func (h *HostHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	search := r.URL.Query().Get("search")
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	hosts, total, err := h.store.ListHosts(r.Context(), search, page, size)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, hosts, &Meta{Page: page, Size: size, Total: total})
}

func (h *HostHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	host, err := h.store.GetHost(r.Context(), id)
	if err != nil {
		Error(w, http.StatusNotFound, "主机未找到")
		return
	}
	OK(w, host)
}

func (h *HostHandler) Create(w http.ResponseWriter, r *http.Request) {
	var host models.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	host.ID = uuid.New().String()
	if err := h.store.CreateHost(r.Context(), &host); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	Created(w, host)
}

func (h *HostHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var host models.Host
	if err := json.NewDecoder(r.Body).Decode(&host); err != nil {
		Error(w, http.StatusBadRequest, "无效的请求体")
		return
	}
	host.ID = id
	if err := h.store.UpdateHost(r.Context(), &host); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, host)
}

func (h *HostHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteHost(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	JSON(w, http.StatusNoContent, Response{Success: true})
}
```

- [ ] **Step 3: 事件 API + SSE**

`internal/api/events.go`:
```go
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"pxego/internal/eventbus"
	"pxego/internal/models"
	"pxego/internal/store"
	"strconv"
)

type EventHandler struct {
	store    store.Interface
	eventBus *eventbus.Bus
}

func (h *EventHandler) List(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	size, _ := strconv.Atoi(r.URL.Query().Get("size"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 50
	}

	filter := store.EventFilter{
		Type:  r.URL.Query().Get("type"),
		Level: r.URL.Query().Get("level"),
		Page:  page,
		Size:  size,
	}

	events, total, err := h.store.ListEvents(r.Context(), filter)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	OK(w, events)
}

func (h *EventHandler) Stream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		Error(w, http.StatusInternalServerError, "不支持 SSE")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := make(chan eventbus.Event, 100)
	id := h.eventBus.Subscribe("event", func(e eventbus.Event) {
		select {
		case ch <- e:
		default:
		}
	})
	defer h.eventBus.Unsubscribe("event", id)

	for {
		select {
		case <-r.Context().Done():
			return
		case e := <-ch:
			evt := e.Payload.(models.Event)
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
```

- [ ] **Step 4: 提交**

```bash
git add internal/api/ && git commit -m "feat: 添加 REST API 处理层"
```

### Task 17：IPMI 封装

**文件：**
- Create: `internal/ipmi/ipmi.go`

- [ ] **Step 1: 实现 IPMI 控制**

`internal/ipmi/ipmi.go`:
```go
package ipmi

import (
	"fmt"
	"net"
	"pxego/internal/models"
	"time"

	"github.com/bougou/go-ipmi"
)

type Client struct{}

func NewClient() *Client {
	return &Client{}
}

func (c *Client) connect(host *models.Host) (*ipmi.Connection, error) {
	conn, err := ipmi.NewConnection(
		ipmi.WithHost(net.JoinHostPort(host.BMCAddr, "")),
		ipmi.WithUsername(host.BMCUser),
		ipmi.WithPassword(host.BMCPass),
		ipmi.WithTimeout(5*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("IPMI 连接失败: %w", err)
	}
	return conn, nil
}

func (c *Client) PowerOn(host *models.Host) error {
	conn, err := c.connect(host)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ControlChassisPower(ipmi.ChassisControlPowerUp)
}

func (c *Client) PowerOff(host *models.Host) error {
	conn, err := c.connect(host)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ControlChassisPower(ipmi.ChassisControlPowerDown)
}

func (c *Client) PowerCycle(host *models.Host) error {
	conn, err := c.connect(host)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.ControlChassisPower(ipmi.ChassisControlPowerCycle)
}

func (c *Client) PowerStatus(host *models.Host) (string, error) {
	conn, err := c.connect(host)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	status, err := conn.GetChassisPowerStatus()
	if err != nil {
		return "", err
	}
	if status.PowerOn {
		return "on", nil
	}
	return "off", nil
}
```

- [ ] **Step 2: 提交**

```bash
git add internal/ipmi/ && git commit -m "feat: 添加 IPMI 电源控制"
```

### Task 18：WDS 仿真（简略实现）

**文件：**
- Create: `internal/wds/wds.go`

- [ ] **Step 1: WDS 文件服务**

`internal/wds/wds.go`:
```go
package wds

import "net/http"

// RegisterRoutes 注册 WDS 相关的 HTTP 路由
func RegisterRoutes(mux *http.ServeMux, rootDir string) {
	fs := http.FileServer(http.Dir(rootDir))
	mux.Handle("/boot/wds/", http.StripPrefix("/boot/wds/", fs))
}

/*
WDS 启动流程：
1. 客户端请求 bootmgr.exe → 从 boot/winpe/ 提供
2. 客户端请求 BCD → 返回预配置的 BCD 文件
3. 客户端请求 boot.sdi → 返回 boot.sdi
4. 客户端请求 boot.wim → 返回 Windows PE 映像
*/
```

- [ ] **Step 2: 提交**

```bash
git add internal/wds/ && git commit -m "feat: 添加 WDS 仿真"
```

---

## Phase 7：前端 SPA

### Task 19：React 脚手架

**文件：**
- Create: `web/package.json`
- Create: `web/vite.config.ts`
- Create: `web/tsconfig.json`
- Create: `web/tailwind.config.js`
- Create: `web/index.html`
- Create: `web/src/main.tsx`
- Create: `web/src/App.tsx`

- [ ] **Step 1: 初始化前端项目**

```bash
cd web
npm create vite@latest . -- --template react-ts
npm install react-router-dom@6
npm install react-i18next i18next i18next-browser-languagedetector
npm install lucide-react
npm install -D tailwindcss @tailwindcss/vite
```

- [ ] **Step 2: 配置 Tailwind**

`web/tailwind.config.js`:
```js
export default {
  content: ['./index.html', './src/**/*.{js,ts,jsx,tsx}'],
  theme: {
    extend: {},
  },
  plugins: [],
}
```

`web/vite.config.ts`:
```ts
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  build: { outDir: 'dist' },
})
```

- [ ] **Step 3: 嵌入配置**

`cmd/pxego/embed.go`:
```go
package main

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed web/dist/*
var spaFS embed.FS

func spaHandler() http.Handler {
	subFS, _ := fs.Sub(spaFS, "web/dist")
	return http.FileServer(http.FS(subFS))
}
```

- [ ] **Step 4: 提交**

```bash
git add web/ && git commit -m "feat: 初始化前端 React 项目"
```

### Task 20：前端国际化 + 主题

**文件：**
- Create: `web/src/i18n/index.ts`
- Create: `web/src/i18n/zh.json`
- Create: `web/src/i18n/en.json`
- Create: `web/src/hooks/useTheme.ts`
- Create: `web/src/components/ThemeToggle.tsx`
- Create: `web/src/components/LangSwitch.tsx`

- [ ] **Step 1: 国际化配置**

`web/src/i18n/index.ts`:
```ts
import i18n from 'i18next'
import { initReactI18next } from 'react-i18next'
import LanguageDetector from 'i18next-browser-languagedetector'
import zh from './zh.json'
import en from './en.json'

i18n.use(initReactI18next).use(LanguageDetector).init({
  resources: { zh: { translation: zh }, en: { translation: en } },
  fallbackLng: 'zh',
  interpolation: { escapeValue: false },
})

export default i18n
```

- [ ] **Step 2: 翻译文件（部分内容）**

`web/src/i18n/zh.json`：
```json
{
  "nav": {
    "dashboard": "仪表板",
    "hosts": "主机管理",
    "profiles": "引导配置",
    "files": "文件管理",
    "events": "事件日志",
    "settings": "设置"
  },
  "common": {
    "save": "保存",
    "cancel": "取消",
    "delete": "删除",
    "search": "搜索",
    "create": "新建",
    "edit": "编辑"
  }
}
```

`web/src/i18n/en.json`：
```json
{
  "nav": {
    "dashboard": "Dashboard",
    "hosts": "Hosts",
    "profiles": "Profiles",
    "files": "Files",
    "events": "Events",
    "settings": "Settings"
  },
  "common": {
    "save": "Save",
    "cancel": "Cancel",
    "delete": "Delete",
    "search": "Search",
    "create": "Create",
    "edit": "Edit"
  }
}
```

- [ ] **Step 3: 主题 Hook**

`web/src/hooks/useTheme.ts`：
```ts
import { useState, useEffect } from 'react'

type Theme = 'dark' | 'light'

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    return (localStorage.getItem('theme') as Theme) || 'dark'
  })

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem('theme', theme)
  }, [theme])

  const toggle = () => setTheme(t => t === 'dark' ? 'light' : 'dark')

  return { theme, setTheme, toggle }
}
```

- [ ] **Step 4: 提交**

```bash
git add web/src/i18n/ web/src/hooks/useTheme.ts && git commit -m "feat: 添加国际化配置和主题切换"
```

---

## Phase 8：加固 + 服务集成 + 发布

### Task 21：服务集成

**文件：**
- Create: `cmd/pxego/service_windows.go`
- Create: `cmd/pxego/service_linux.go`
- Create: `cmd/pxego/service_darwin.go`
- Create: `contrib/pxego.service` (systemd)
- Create: `contrib/com.pxego.plist` (launchd)

- [ ] **Step 1: Windows 服务**

`cmd/pxego/service_windows.go`:
```go
//go:build windows

package main

import (
	"log/slog"
	"os"
	"golang.org/x/sys/windows/svc"
)

type pxegoService struct{}

func (s *pxegoService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.Running}
	for c := range r {
		switch c.Cmd {
		case svc.Interrogate:
			changes <- c.CurrentStatus
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			return false, 0
		}
	}
	return false, 0
}

func runService() {
	if ok, _ := svc.IsWindowsService(); ok {
		err := svc.Run("pxego", &pxegoService{})
		if err != nil {
			slog.Error("Windows 服务运行失败", "error", err)
			os.Exit(1)
		}
	}
}
```

- [ ] **Step 2: Linux systemd**

`contrib/pxego.service`:
```ini
[Unit]
Description=PxeGo - All-in-One PXE Server
After=network.target

[Service]
Type=notify
ExecStart=/usr/local/bin/pxego --config /etc/pxego/config.yaml
Restart=always
RestartSec=5
AmbientCapabilities=CAP_NET_BIND_SERVICE CAP_NET_RAW
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```

- [ ] **Step 3: 提交**

```bash
git add cmd/pxego/service_*.go contrib/ && git commit -m "feat: 添加 OS 服务集成"
```

### Task 22：goreleaser 发布配置

**文件：**
- Create: `.goreleaser.yaml`

- [ ] **Step 1: goreleaser 配置**

`.goreleaser.yaml`:
```yaml
version: 2
project_name: pxego

before:
  hooks:
    - go mod tidy
    - cd web && npm ci && npm run build

builds:
  - id: pxego
    main: ./cmd/pxego
    binary: pxego
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - windows
      - darwin
    goarch:
      - amd64
      - arm64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - files:
      - contrib/*
      - README.md
      - config.example.yaml

nfpms:
  - package_name: pxego
    vendor: PxeGo
    formats: [deb, rpm]
    files:
      contrib/pxego.service: /lib/systemd/system/pxego.service
```

- [ ] **Step 2: 编译验证**

```bash
goreleaser build --snapshot --clean
bin/pxego --help
```

- [ ] **Step 3: 提交**

```bash
git add .goreleaser.yaml && git commit -m "chore: 添加 goreleaser 发布配置"
```

---

## 验证策略

### 开发验证
```bash
# 编译
go build ./cmd/pxego

# 单元测试
go test ./...

# API 测试
go run ./cmd/pxego &
curl http://localhost:8080/api/v1/status
curl http://localhost:8080/api/v1/hosts | jq .

# QEMU PXE 启动测试
qemu-system-x86_64 -boot n -netdev user,id=net0,tftp=./boot,bootfile=undionly.kpxe -device virtio-net,netdev=net0

# Windows 交叉编译
GOOS=windows GOARCH=amd64 go build ./cmd/pxego
GOOS=darwin GOARCH=arm64 go build ./cmd/pxego
```

### 阶段交付检查

| Phase | 检查点 |
|-------|--------|
| Phase 1 | `pxego --help` 正常，SQLite 文件创建成功 |
| Phase 2 | DHCP 请求有响应，Wireshark 可看到 option 43/60/66/67/93 |
| Phase 3 | TFTP 客户端能下载 `undionly.kpxe` |
| Phase 4 | iPXE 客户端能从 HTTP 获取菜单并启动内核 |
| Phase 5 | PXELinux 配置文件转换正确，DNS 解析正常 |
| Phase 6 | API CRUD 均正常，IPMI 可控制电源，SSE 实时推流 |
| Phase 7 | SPA 在浏览器中可用，中英文切换正常，主题切换正常 |
| Phase 8 | `pxego service install` 在各平台正常，goreleaser 构建成功 |
