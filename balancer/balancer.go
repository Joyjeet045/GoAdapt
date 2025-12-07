/*
Author: Joyjeet Roy
*/
package balancer

import (
	"advanced-lb/features"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

type Backend struct {
	URL               *url.URL
	Alive             bool
	mux               sync.RWMutex
	ReverseProxy      *httputil.ReverseProxy
	Weight            int
	ActiveConnections int64
	Stats             BackendStats
	CircuitBreaker    *features.CircuitBreaker
}

type BackendStats struct {
	Requests     int64
	ResponseTime int64
	Errors       int64
}

func (b *Backend) SetAlive(alive bool) {
	b.mux.Lock()
	b.Alive = alive
	b.mux.Unlock()
}

func (b *Backend) IsAlive() bool {
	b.mux.RLock()
	defer b.mux.RUnlock()
	return b.Alive && b.CircuitBreaker.Allow()
}

type ServerPool struct {
	Backends []*Backend
	current  uint64
}

type LoadBalancer interface {
	NextBackend(r *http.Request) *Backend
	AddBackend(b *Backend)
	UpdateBackendStatus(u *url.URL, alive bool)
	GetBackends() []*Backend
	OnRequestCompletion(u *url.URL, duration time.Duration, err error)
}

func NewBackend(u *url.URL, weight int) *Backend {
	b := &Backend{
		URL:            u,
		Alive:          true,
		Weight:         weight,
		CircuitBreaker: features.NewCircuitBreaker(3, 10*time.Second),
	}

	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
	}

	proxy := httputil.NewSingleHostReverseProxy(u)
	proxy.Transport = transport

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		b.CircuitBreaker.RecordFailure()
		b.SetAlive(false)
		w.WriteHeader(http.StatusBadGateway)
		w.Write([]byte("Bad Gateway"))
	}

	proxy.ModifyResponse = func(resp *http.Response) error {
		if resp.StatusCode >= 500 {
			b.CircuitBreaker.RecordFailure()
			b.SetAlive(false)
		} else {
			b.CircuitBreaker.RecordSuccess()
		}
		return nil
	}

	b.ReverseProxy = proxy
	return b
}
