package metrics

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Window5m  = 5 * time.Minute
	Window15m = 15 * time.Minute
	Window1h  = 1 * time.Hour

	bucket10s = 10 * time.Second
)

type Counter struct {
	total atomic.Int64
}

func (c *Counter) Inc()   { c.total.Add(1) }
func (c *Counter) Add(n int64) { c.total.Add(n) }
func (c *Counter) Value() int64 { return c.total.Load() }

type Gauge struct {
	val atomic.Int64
}

func (g *Gauge) Set(v int64) { g.val.Store(v) }
func (g *Gauge) Inc()        { g.val.Add(1) }
func (g *Gauge) Dec()        { g.val.Add(-1) }
func (g *Gauge) Value() int64 { return g.val.Load() }

type TimeBucket struct {
	Timestamp int64   `json:"t"`
	Value     float64 `json:"v"`
}

type TimeSeries struct {
	mu      sync.Mutex
	buckets []TimeBucket
	maxAge  time.Duration
	maxLen  int
}

func NewTimeSeries(resolution, maxAge time.Duration) *TimeSeries {
	return &TimeSeries{
		maxLen: int(maxAge/resolution) + 1,
		maxAge: maxAge,
	}
}

func (ts *TimeSeries) Add(v float64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	now := time.Now().Unix()
	ts.buckets = append(ts.buckets, TimeBucket{Timestamp: now, Value: v})
	cutoff := time.Now().Add(-ts.maxAge).Unix()
	for len(ts.buckets) > 0 && ts.buckets[0].Timestamp < cutoff {
		ts.buckets = ts.buckets[1:]
	}
	if len(ts.buckets) > ts.maxLen*2 {
		ts.buckets = ts.buckets[len(ts.buckets)-ts.maxLen:]
	}
}

func (ts *TimeSeries) Snapshot() []TimeBucket {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	out := make([]TimeBucket, len(ts.buckets))
	copy(out, ts.buckets)
	return out
}

type ServiceMetrics struct {
	Requests    atomic.Int64
	Errors      atomic.Int64
	BytesIn     atomic.Int64
	BytesOut    atomic.Int64
	ActiveConns atomic.Int64
	Rejected    atomic.Int64

	requestRate *TimeSeries
	errorRate   *TimeSeries
	bandwidth   *TimeSeries
	latencyMs   *TimeSeries
}

func NewServiceMetrics() *ServiceMetrics {
	return &ServiceMetrics{
		requestRate: NewTimeSeries(bucket10s, Window1h),
		errorRate:   NewTimeSeries(1*time.Minute, Window1h),
		bandwidth:   NewTimeSeries(bucket10s, Window1h),
		latencyMs:   NewTimeSeries(1*time.Minute, Window1h),
	}
}

func (sm *ServiceMetrics) RecordRequest() {
	sm.Requests.Add(1)
	sm.requestRate.Add(1)
}

func (sm *ServiceMetrics) RecordError() {
	sm.Errors.Add(1)
	sm.errorRate.Add(1)
}

func (sm *ServiceMetrics) RecordBytes(n int64, out bool) {
	if out {
		sm.BytesOut.Add(n)
	} else {
		sm.BytesIn.Add(n)
	}
	sm.bandwidth.Add(float64(n))
}

func (sm *ServiceMetrics) RecordRejected() {
	sm.Rejected.Add(1)
}

func (sm *ServiceMetrics) RecordLatency(ms float64) {
	sm.latencyMs.Add(ms)
}

type MetricSnapshot struct {
	Requests    int64        `json:"requests"`
	Errors      int64        `json:"errors"`
	BytesIn     int64        `json:"bytesIn"`
	BytesOut    int64        `json:"bytesOut"`
	ActiveConns int64        `json:"activeConns"`
	Rejected    int64        `json:"rejected"`
	RequestRate []TimeBucket `json:"requestRate"`
	ErrorRate   []TimeBucket `json:"errorRate"`
	Bandwidth   []TimeBucket `json:"bandwidth"`
	LatencyMs   []TimeBucket `json:"latencyMs"`
}

func (sm *ServiceMetrics) Snapshot() MetricSnapshot {
	return MetricSnapshot{
		Requests:    sm.Requests.Load(),
		Errors:      sm.Errors.Load(),
		BytesIn:     sm.BytesIn.Load(),
		BytesOut:    sm.BytesOut.Load(),
		ActiveConns: sm.ActiveConns.Load(),
		Rejected:    sm.Rejected.Load(),
		RequestRate: sm.requestRate.Snapshot(),
		ErrorRate:   sm.errorRate.Snapshot(),
		Bandwidth:   sm.bandwidth.Snapshot(),
		LatencyMs:   sm.latencyMs.Snapshot(),
	}
}

func (sm *ServiceMetrics) Rates5m() (reqRate, errRate, bwRate float64) {
	reqRate = rateFromBuckets(sm.requestRate.Snapshot(), Window5m)
	errRate = rateFromBuckets(sm.errorRate.Snapshot(), Window5m)
	bwRate = rateFromBuckets(sm.bandwidth.Snapshot(), Window5m)
	return
}

func rateFromBuckets(buckets []TimeBucket, window time.Duration) float64 {
	if len(buckets) < 2 {
		return 0
	}
	cutoff := time.Now().Add(-window).Unix()
	var total float64
	var count int
	for _, b := range buckets {
		if b.Timestamp >= cutoff {
			total += b.Value
			count++
		}
	}
	if count == 0 {
		return 0
	}
	avg := total / float64(count)
	return math.Round(avg*100) / 100
}

type DHCPExtra struct {
	Offers            int64              `json:"offers"`
	Acks              int64              `json:"acks"`
	Naks              int64              `json:"naks"`
	Declines          int64              `json:"declines"`
	Discovers         int64              `json:"discovers"`
	Requests          int64              `json:"requests"`
	Unauthorized      int64              `json:"unauthorized"`
	ActiveLeases      int64              `json:"activeLeases"`
	ArchBreakdown     map[string]int64   `json:"archBreakdown"`
	PlatformBreakdown map[string]int64   `json:"platformBreakdown"`
}

type HTTPExtra struct {
	Status2xx   int64        `json:"status2xx"`
	Status3xx   int64        `json:"status3xx"`
	Status4xx   int64        `json:"status4xx"`
	Status5xx   int64        `json:"status5xx"`
	Duration    []TimeBucket `json:"duration"`
}

type ServiceSnapshot struct {
	Metrics MetricSnapshot `json:"metrics"`
	DHCP    *DHCPExtra     `json:"dhcp,omitempty"`
	HTTP    *HTTPExtra     `json:"http,omitempty"`
}

type GlobalSnapshot struct {
	Services map[string]ServiceSnapshot `json:"services"`
}

type Registry struct {
	mu       sync.RWMutex
	services map[string]*ServiceMetrics
	dhcp     *DHCPTracker
	http     *HTTPTracker
}

func NewRegistry() *Registry {
	return &Registry{
		services: make(map[string]*ServiceMetrics),
	}
}

func (r *Registry) GetOrCreate(name string) *ServiceMetrics {
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.services[name]; ok {
		return s
	}
	s := NewServiceMetrics()
	r.services[name] = s
	return s
}

func (r *Registry) Get(name string) *ServiceMetrics {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.services[name]
}

func (r *Registry) SetDHCPTracker(t *DHCPTracker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.dhcp = t
}

func (r *Registry) DHCPTracker() *DHCPTracker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.dhcp
}

func (r *Registry) SetHTTPTracker(t *HTTPTracker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.http = t
}

func (r *Registry) HTTPTracker() *HTTPTracker {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.http
}

func (r *Registry) Snapshot() GlobalSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()
	gs := GlobalSnapshot{Services: make(map[string]ServiceSnapshot)}
	for name, sm := range r.services {
		ss := ServiceSnapshot{Metrics: sm.Snapshot()}
		if name == "dhcp" && r.dhcp != nil {
			de := r.dhcp.Snapshot()
			ss.DHCP = &de
		}
		if name == "http" && r.http != nil {
			he := r.http.Snapshot()
			ss.HTTP = &he
		}
		gs.Services[name] = ss
	}
	return gs
}

type DHCPTracker struct {
	mu     sync.Mutex
	offers, acks, naks, declines, discovers, requests, unauthorized int64
	activeLeases int64
	archBreakdown     map[string]int64
	platformBreakdown map[string]int64
}

func NewDHCPTracker() *DHCPTracker {
	return &DHCPTracker{
		archBreakdown:     make(map[string]int64),
		platformBreakdown: make(map[string]int64),
	}
}

func (d *DHCPTracker) IncOffer()       { atomic.AddInt64(&d.offers, 1) }
func (d *DHCPTracker) IncAck()         { atomic.AddInt64(&d.acks, 1) }
func (d *DHCPTracker) IncNak()         { atomic.AddInt64(&d.naks, 1) }
func (d *DHCPTracker) IncDecline()     { atomic.AddInt64(&d.declines, 1) }
func (d *DHCPTracker) IncDiscover()    { atomic.AddInt64(&d.discovers, 1) }
func (d *DHCPTracker) IncRequest()     { atomic.AddInt64(&d.requests, 1) }
func (d *DHCPTracker) IncUnauthorized() { atomic.AddInt64(&d.unauthorized, 1) }
func (d *DHCPTracker) SetActiveLeases(v int64) { atomic.StoreInt64(&d.activeLeases, v) }

func (d *DHCPTracker) RecordArch(arch string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.archBreakdown[arch]++
}

func (d *DHCPTracker) RecordPlatform(platform string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.platformBreakdown[platform]++
}

func (d *DHCPTracker) Snapshot() DHCPExtra {
	d.mu.Lock()
	defer d.mu.Unlock()
	arch := make(map[string]int64, len(d.archBreakdown))
	for k, v := range d.archBreakdown {
		arch[k] = v
	}
	plat := make(map[string]int64, len(d.platformBreakdown))
	for k, v := range d.platformBreakdown {
		plat[k] = v
	}
	return DHCPExtra{
		Offers:            atomic.LoadInt64(&d.offers),
		Acks:              atomic.LoadInt64(&d.acks),
		Naks:              atomic.LoadInt64(&d.naks),
		Declines:          atomic.LoadInt64(&d.declines),
		Discovers:         atomic.LoadInt64(&d.discovers),
		Requests:          atomic.LoadInt64(&d.requests),
		Unauthorized:      atomic.LoadInt64(&d.unauthorized),
		ActiveLeases:      atomic.LoadInt64(&d.activeLeases),
		ArchBreakdown:     arch,
		PlatformBreakdown: plat,
	}
}

type HTTPTracker struct {
	mu        sync.Mutex
	status2xx int64
	status3xx int64
	status4xx int64
	status5xx int64
	duration  *TimeSeries
}

func NewHTTPTracker() *HTTPTracker {
	return &HTTPTracker{
		duration: NewTimeSeries(1*time.Minute, Window1h),
	}
}

func (h *HTTPTracker) RecordStatus(code int) {
	switch {
	case code >= 200 && code < 300:
		atomic.AddInt64(&h.status2xx, 1)
	case code >= 300 && code < 400:
		atomic.AddInt64(&h.status3xx, 1)
	case code >= 400 && code < 500:
		atomic.AddInt64(&h.status4xx, 1)
	case code >= 500:
		atomic.AddInt64(&h.status5xx, 1)
	}
}

func (h *HTTPTracker) RecordDuration(ms float64) {
	h.duration.Add(ms)
}

func (h *HTTPTracker) Snapshot() HTTPExtra {
	return HTTPExtra{
		Status2xx: atomic.LoadInt64(&h.status2xx),
		Status3xx: atomic.LoadInt64(&h.status3xx),
		Status4xx: atomic.LoadInt64(&h.status4xx),
		Status5xx: atomic.LoadInt64(&h.status5xx),
		Duration:  h.duration.Snapshot(),
	}
}

var DefaultRegistry = NewRegistry()
