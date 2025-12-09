package balancer

import (
	"hash/crc32"
	"net"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"
)

type RoundRobin struct {
	pool *ServerPool
}

func NewRoundRobin(pool *ServerPool) *RoundRobin {
	return &RoundRobin{
		pool: pool,
	}
}

func (rr *RoundRobin) NextBackend(r *http.Request) *Backend {
	backends := rr.pool.Backends
	l := len(backends)
	if l == 0 {
		return nil
	}

	start := atomic.AddUint64(&rr.pool.current, 1)
	for i := 0; i < l; i++ {
		idx := int((start + uint64(i)) % uint64(l))
		if backends[idx].IsAlive() {
			return backends[idx]
		}
	}
	return nil
}

func (rr *RoundRobin) AddBackend(b *Backend) {
	rr.pool.Backends = append(rr.pool.Backends, b)
}

func (rr *RoundRobin) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range rr.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (rr *RoundRobin) GetBackends() []*Backend {
	return rr.pool.Backends
}

func (rr *RoundRobin) OnRequestCompletion(u *url.URL, duration time.Duration, err error) {
}

type LeastConnections struct {
	pool *ServerPool
}

func NewLeastConnections(pool *ServerPool) *LeastConnections {
	return &LeastConnections{
		pool: pool,
	}
}

func (lc *LeastConnections) NextBackend(r *http.Request) *Backend {
	var best *Backend
	var min int64 = -1

	for _, b := range lc.pool.Backends {
		if !b.IsAlive() {
			continue
		}
		conn := atomic.LoadInt64(&b.ActiveConnections)
		if min == -1 || conn < min {
			min = conn
			best = b
		}
	}
	return best
}

func (lc *LeastConnections) AddBackend(b *Backend) {
	lc.pool.Backends = append(lc.pool.Backends, b)
}

func (lc *LeastConnections) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range lc.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (lc *LeastConnections) GetBackends() []*Backend {
	return lc.pool.Backends
}

func (lc *LeastConnections) OnRequestCompletion(u *url.URL, duration time.Duration, err error) {
}

type WeightedRoundRobin struct {
	pool    *ServerPool
	mu      sync.RWMutex
	indices []int
}

func NewWeightedRoundRobin(pool *ServerPool) *WeightedRoundRobin {
	wrr := &WeightedRoundRobin{
		pool:    pool,
		indices: make([]int, 0),
	}
	for i, b := range pool.Backends {
		w := b.Weight
		if w <= 0 {
			w = 1
		}
		for j := 0; j < w; j++ {
			wrr.indices = append(wrr.indices, i)
		}
	}
	return wrr
}

func (wrr *WeightedRoundRobin) NextBackend(r *http.Request) *Backend {
	wrr.mu.RLock()
	indices := wrr.indices
	wrr.mu.RUnlock()

	l := len(indices)
	if l == 0 {
		return nil
	}

	start := atomic.AddUint64(&wrr.pool.current, 1)
	for i := 0; i < l; i++ {
		idxVal := int((start + uint64(i)) % uint64(l))
		backendIdx := indices[idxVal]
		if backendIdx < len(wrr.pool.Backends) {
			b := wrr.pool.Backends[backendIdx]
			if b.IsAlive() {
				return b
			}
		}
	}
	return nil
}

func (wrr *WeightedRoundRobin) AddBackend(b *Backend) {
	wrr.pool.Backends = append(wrr.pool.Backends, b)
	wrr.mu.Lock()
	defer wrr.mu.Unlock()
	idx := len(wrr.pool.Backends) - 1
	w := b.Weight
	if w <= 0 {
		w = 1
	}
	for j := 0; j < w; j++ {
		wrr.indices = append(wrr.indices, idx)
	}
}

func (wrr *WeightedRoundRobin) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range wrr.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (wrr *WeightedRoundRobin) GetBackends() []*Backend {
	return wrr.pool.Backends
}

func (wrr *WeightedRoundRobin) OnRequestCompletion(u *url.URL, d time.Duration, e error) {}

type IPHash struct {
	pool *ServerPool
}

func NewIPHash(pool *ServerPool) *IPHash {
	return &IPHash{pool: pool}
}

func (iph *IPHash) NextBackend(r *http.Request) *Backend {
	backends := iph.pool.Backends
	if len(backends) == 0 {
		return nil
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	checksum := crc32.ChecksumIEEE([]byte(ip))
	startIdx := int(checksum % uint32(len(backends)))

	for i := 0; i < len(backends); i++ {
		idx := (startIdx + i) % len(backends)
		if backends[idx].IsAlive() {
			return backends[idx]
		}
	}
	return nil
}

func (iph *IPHash) AddBackend(b *Backend) {
	iph.pool.Backends = append(iph.pool.Backends, b)
}

func (iph *IPHash) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range iph.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (iph *IPHash) GetBackends() []*Backend {
	return iph.pool.Backends
}

func (iph *IPHash) OnRequestCompletion(u *url.URL, d time.Duration, e error) {}

type LeastResponseTime struct {
	pool  *ServerPool
	stats map[string]int64
	mux   sync.RWMutex
}

func NewLeastResponseTime(pool *ServerPool) *LeastResponseTime {
	return &LeastResponseTime{
		pool:  pool,
		stats: make(map[string]int64),
	}
}

func (lrt *LeastResponseTime) NextBackend(r *http.Request) *Backend {
	lrt.mux.RLock()
	defer lrt.mux.RUnlock()

	var best *Backend
	var minTime int64 = -1

	for _, b := range lrt.pool.Backends {
		if !b.IsAlive() {
			continue
		}
		t := lrt.stats[b.URL.String()]
		if minTime == -1 || t < minTime {
			minTime = t
			best = b
		}
	}
	if best == nil {
		return nil
	}
	return best
}

func (lrt *LeastResponseTime) AddBackend(b *Backend) {
	lrt.pool.Backends = append(lrt.pool.Backends, b)
}

func (lrt *LeastResponseTime) UpdateBackendStatus(u *url.URL, alive bool) {
	for _, b := range lrt.pool.Backends {
		if b.URL.String() == u.String() {
			b.SetAlive(alive)
			break
		}
	}
}

func (lrt *LeastResponseTime) GetBackends() []*Backend {
	return lrt.pool.Backends
}

func (lrt *LeastResponseTime) OnRequestCompletion(u *url.URL, d time.Duration, e error) {
	lrt.mux.Lock()
	defer lrt.mux.Unlock()

	old := lrt.stats[u.String()]
	if old == 0 {
		lrt.stats[u.String()] = int64(d)
	} else {
		lrt.stats[u.String()] = (old + int64(d)) / 2
	}
}
