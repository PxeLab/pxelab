package dhcp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pxego/pxego/internal/models"
	"github.com/pxego/pxego/internal/store"
)

type ClientInfo struct {
	MAC      string
	Arch     string // x86_64, i386, arm64, armhf
	Platform string // efi or pc (legacy BIOS)
}

type LeaseManager struct {
	mu           sync.RWMutex
	store        store.LeaseStore
	subnets      map[string]*SubnetPool
	ipToClient   map[string]ClientInfo // IP → client arch/platform
}

type SubnetPool struct {
	CIDR    string
	Gateway net.IP
	Pools   []*IPRange
	Leases  map[string]*models.Lease
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

func NewLeaseManager(st store.LeaseStore) *LeaseManager {
	return &LeaseManager{
		store:      st,
		subnets:    make(map[string]*SubnetPool),
		ipToClient: make(map[string]ClientInfo),
	}
}

func (lm *LeaseManager) ClearSubnets() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.subnets = make(map[string]*SubnetPool)
	lm.ipToClient = make(map[string]ClientInfo)
}

func (lm *LeaseManager) AddSubnet(cidr string, pools []*IPRange, gateway net.IP) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.subnets[cidr] = &SubnetPool{
		CIDR:    cidr,
		Gateway: gateway,
		Pools:   pools,
		Leases:  make(map[string]*models.Lease),
	}
}

// Allocate 分配 IP 并记录客户端架构信息
func (lm *LeaseManager) Allocate(cidr, mac string) (net.IP, error) {
	return lm.AllocateWithInfo(cidr, mac, "", "")
}

// AllocateWithInfo 分配 IP 并记录客户端架构信息
func (lm *LeaseManager) AllocateWithInfo(cidr, mac, arch, platform string) (net.IP, error) {
	lm.mu.Lock()
	defer lm.mu.Unlock()

	pool, ok := lm.subnets[cidr]
	if !ok {
		return nil, fmt.Errorf("子网 %s 未配置", cidr)
	}

	// 检查是否已有租约
	if lease, ok := pool.Leases[mac]; ok {
		if time.Now().Before(lease.ExpiresAt) {
			ip := net.ParseIP(lease.IP)
			if ip != nil && arch != "" {
				lm.ipToClient[lease.IP] = ClientInfo{MAC: mac, Arch: arch, Platform: platform}
			}
			return ip, nil
		}
	}

	// 遍历所有地址池
	for _, r := range pool.Pools {
		ip := make(net.IP, len(r.Start))
		copy(ip, r.Start)
		for {
			if bytesCompare(ip, r.End) > 0 {
				break
			}
			used := false
			for _, lease := range pool.Leases {
				if lease.IP == ip.String() {
					used = true
					break
				}
			}
			if !used {
				lease := &models.Lease{
					MAC:       mac,
					IP:        ip.String(),
					SubnetID:  cidr,
					ExpiresAt: time.Now().Add(1 * time.Hour),
					CreatedAt: time.Now(),
				}
				pool.Leases[mac] = lease
				lm.store.CreateLease(context.Background(), lease)
				if arch != "" {
					lm.ipToClient[ip.String()] = ClientInfo{MAC: mac, Arch: arch, Platform: platform}
				}
				return ip, nil
			}
			incIP(ip)
		}
	}

	return nil, fmt.Errorf("子网 %s 无可用 IP", cidr)
}

// GetClientByIP 根据 IP 查询客户端架构信息
func (lm *LeaseManager) GetClientByIP(ip string) (ClientInfo, bool) {
	lm.mu.RLock()
	defer lm.mu.RUnlock()
	info, ok := lm.ipToClient[ip]
	return info, ok
}

// SetClientInfo 记录客户端架构信息（用于 proxy 模式不分配 IP 的场景）
func (lm *LeaseManager) SetClientInfo(ip, mac, arch, platform string) {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.ipToClient[ip] = ClientInfo{MAC: mac, Arch: arch, Platform: platform}
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}
