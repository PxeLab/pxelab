package netboot

import "sync"

// Manager provides thread-safe access to the distro catalog
type Manager struct {
	mu      sync.RWMutex
	catalog *Catalog
	byName  map[string]*Distro
}

// NewManager creates a catalog manager
func NewManager(c *Catalog) *Manager {
	m := &Manager{
		catalog: c,
		byName:  make(map[string]*Distro),
	}
	m.rebuildIndex()
	return m
}

func (m *Manager) rebuildIndex() {
	m.byName = make(map[string]*Distro)
	for _, d := range m.catalog.Distros {
		m.byName[d.Name] = d
	}
}

// GetDistro returns a distro by name
func (m *Manager) GetDistro(name string) *Distro {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.byName[name]
}

// Catalog returns the full catalog
func (m *Manager) Catalog() *Catalog {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.catalog
}

// Groups returns enabled distros grouped by category
func (m *Manager) Groups() []Group {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.catalog.Groups()
}

// Reload replaces the catalog and rebuilds the index
func (m *Manager) Reload(c *Catalog) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.catalog = c
	m.rebuildIndex()
}
