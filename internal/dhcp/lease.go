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

type LeaseManager struct {
	mu      sync.RWMutex
	store   store.LeaseStore
	subnets map[string]*SubnetPool
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
		store:   st,
		subnets: make(map[string]*SubnetPool),
	}
}

func (lm *LeaseManager) ClearSubnets() {
	lm.mu.Lock()
	defer lm.mu.Unlock()
	lm.subnets = make(map[string]*SubnetPool)
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

	// 遍历所有地址池
	for _, r := range pool.Pools {
		ip := make(net.IP, len(r.Start))
		copy(ip, r.Start)
		for {
			if bytesCompare(ip, r.End) > 0 {
				break // 当前池无可用 IP，尝试下一个
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
				return ip, nil
			}
			incIP(ip)
		}
	}

	return nil, fmt.Errorf("子网 %s 无可用 IP", cidr)
}

func incIP(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] != 0 {
			break
		}
	}
}
