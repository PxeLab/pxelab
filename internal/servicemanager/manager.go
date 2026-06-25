package servicemanager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/pxego/pxego/internal/app"
)

type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusError   Status = "error"
)

type ServiceInfo struct {
	Name      string `json:"name"`
	Display   string `json:"display"`
	Status    Status `json:"status"`
	AutoStart bool   `json:"auto_start"`
}

type managedService struct {
	server    app.Server
	display   string
	status    Status
	autoStart bool
	mu        sync.Mutex
}

type Manager struct {
	services map[string]*managedService
	order    []string // registration order for deterministic stop
	mu       sync.RWMutex
}

func New() *Manager {
	return &Manager{
		services: make(map[string]*managedService),
	}
}

// Register 注册服务。name 是唯一标识，display 是展示名。
func (m *Manager) Register(name, display string, srv app.Server, autoStart bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.services[name] = &managedService{
		server:    srv,
		display:   display,
		status:    StatusStopped,
		autoStart: autoStart,
	}
	m.order = append(m.order, name)
}

func (m *Manager) Start(name string) error {
	m.mu.RLock()
	svc, ok := m.services[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.status == StatusRunning {
		return nil
	}

	slog.Info("启动服务", "name", name)
	// Start() 使用独立 context.Background()，因为当前所有服务实现的
	// Start() 启动监听后立即返回，Stop() 通过关闭 listener/Shutdown 来停止
	// 不依赖 context 传播取消信号。
	if err := svc.server.Start(context.Background()); err != nil {
		svc.status = StatusError
		return fmt.Errorf("start %s: %w", name, err)
	}
	svc.status = StatusRunning
	return nil
}

func (m *Manager) Stop(name string) error {
	m.mu.RLock()
	svc, ok := m.services[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.status == StatusStopped {
		return nil
	}

	slog.Info("停止服务", "name", name)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := svc.server.Stop(ctx); err != nil {
		svc.status = StatusError
		return fmt.Errorf("stop %s: %w", name, err)
	}
	svc.status = StatusStopped
	return nil
}

func (m *Manager) Restart(name string) error {
	if err := m.Stop(name); err != nil {
		return err
	}
	return m.Start(name)
}

// BatchResult maps service name to error string (empty on success).
type BatchResult map[string]string

func (m *Manager) BatchStart(names []string) BatchResult {
	return m.batchOp(names, m.Start)
}

func (m *Manager) BatchStop(names []string) BatchResult {
	return m.batchOp(names, m.Stop)
}

func (m *Manager) BatchRestart(names []string) BatchResult {
	return m.batchOp(names, m.Restart)
}

func (m *Manager) batchOp(names []string, op func(string) error) BatchResult {
	result := make(BatchResult, len(names))
	for _, name := range names {
		if err := op(name); err != nil {
			result[name] = err.Error()
		} else {
			result[name] = ""
		}
	}
	return result
}

// List 返回所有服务状态，按 display 排序以保持顺序稳定。
func (m *Manager) List() []ServiceInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]ServiceInfo, 0, len(m.services))
	for name, svc := range m.services {
		svc.mu.Lock()
		infos = append(infos, ServiceInfo{
			Name:      name,
			Display:   svc.display,
			Status:    svc.status,
			AutoStart: svc.autoStart,
		})
		svc.mu.Unlock()
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Display < infos[j].Display
	})
	return infos
}

func (m *Manager) Get(name string) (ServiceInfo, bool) {
	m.mu.RLock()
	svc, ok := m.services[name]
	m.mu.RUnlock()
	if !ok {
		return ServiceInfo{}, false
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()
	return ServiceInfo{
		Name:      name,
		Display:   svc.display,
		Status:    svc.status,
		AutoStart: svc.autoStart,
	}, true
}

// StopAll 按注册顺序反向停止所有服务。
func (m *Manager) StopAll() {
	m.mu.RLock()
	names := make([]string, len(m.order))
	copy(names, m.order)
	m.mu.RUnlock()

	for i := len(names) - 1; i >= 0; i-- {
		if err := m.Stop(names[i]); err != nil {
			slog.Error("服务关闭失败", "name", names[i], "error", err)
		}
	}
}

// Run 启动所有 auto-start 服务，阻塞等待 SIGINT/SIGTERM，然后停止所有服务。
func (m *Manager) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// 启动 auto-start 服务
	autoStartNames := func() []string {
		m.mu.RLock()
		defer m.mu.RUnlock()
		var names []string
		for _, name := range m.order {
			svc := m.services[name]
			svc.mu.Lock()
			if svc.autoStart {
				names = append(names, name)
			}
			svc.mu.Unlock()
		}
		return names
	}()

	for _, name := range autoStartNames {
		if err := m.Start(name); err != nil {
			slog.Error("服务启动失败", "name", name, "error", err)
		}
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	select {
	case <-ctx.Done():
	case sig := <-sigCh:
		slog.Info("收到信号", "signal", sig)
	}

	cancel()
	m.StopAll()
	return nil
}
