/*
Author: Joyjeet Roy
*/
package main

import (
	"advanced-lb/balancer"
	"advanced-lb/features"
	"advanced-lb/health"
	"context"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"gopkg.in/yaml.v2"
)

type Config struct {
	Port        int    `yaml:"port"`
	Algorithm   string `yaml:"algorithm"`
	HealthCheck string `yaml:"health_check_interval"`
	Backends    []struct {
		URL    string `yaml:"url"`
		Weight int    `yaml:"weight"`
	} `yaml:"backends"`
}

var (
	configPath  string
	mu          sync.RWMutex
	globalLB    balancer.LoadBalancer
	rateLimiter *features.RateLimiter
)

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = yaml.Unmarshal(data, &cfg)
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

func initLB(cfg *Config) balancer.LoadBalancer {
	pool := &balancer.ServerPool{
		Backends: make([]*balancer.Backend, 0),
	}

	for _, b := range cfg.Backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			log.Printf("Invalid backend URL %s: %v", b.URL, err)
			continue
		}
		pool.Backends = append(pool.Backends, balancer.NewBackend(u, b.Weight))
	}

	var lb balancer.LoadBalancer
	switch cfg.Algorithm {
	case "round-robin":
		lb = balancer.NewRoundRobin(pool)
	case "least-connections":
		lb = balancer.NewLeastConnections(pool)
	case "q-learning":
		lb = balancer.NewQLearning(pool)
	case "weighted-round-robin":
		lb = balancer.NewWeightedRoundRobin(pool)
	case "ip-hash":
		lb = balancer.NewIPHash(pool)
	case "least-response-time":
		lb = balancer.NewLeastResponseTime(pool)
	default:
		lb = balancer.NewRoundRobin(pool)
	}
	return lb
}

func validateConfig(cfg *Config) error {
	if cfg.Port <= 0 || cfg.Port > 65535 {
		return fmt.Errorf("invalid port: %d", cfg.Port)
	}

	validAlgos := map[string]bool{
		"round-robin": true, "least-connections": true, "q-learning": true,
		"weighted-round-robin": true, "ip-hash": true, "least-response-time": true,
	}

	if !validAlgos[cfg.Algorithm] {
		return fmt.Errorf("invalid algorithm: %s", cfg.Algorithm)
	}

	if len(cfg.Backends) == 0 {
		return fmt.Errorf("no backends configured")
	}

	for _, b := range cfg.Backends {
		if _, err := url.Parse(b.URL); err != nil {
			return fmt.Errorf("invalid backend URL %s: %v", b.URL, err)
		}
	}

	return nil
}

func reloadConfigHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("Reloading configuration...")
	newCfg, err := loadConfig(configPath)
	if err != nil {
		http.Error(w, "Failed to reload config", http.StatusInternalServerError)
		return
	}

	if err := validateConfig(newCfg); err != nil {
		http.Error(w, fmt.Sprintf("Invalid configuration: %v", err), http.StatusBadRequest)
		log.Printf("Configuration validation failed: %v", err)
		return
	}

	mu.Lock()
	globalLB = initLB(newCfg)
	mu.Unlock()

	log.Println("Configuration reloaded successfully")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Configuration reloaded"))
}

func main() {
	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.Parse()

	cfg, err := loadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	globalLB = initLB(cfg)

	rateLimiter = features.NewRateLimiter(1000, 500)

	if cfg.Algorithm == "q-learning" {
		if ql, ok := globalLB.(*balancer.QLearning); ok {
			qTablePath := "qtable.json"
			if err := ql.Load(qTablePath); err != nil {
				log.Printf("Could not load Q-table (starting fresh): %v", err)
			} else {
				log.Println("Q-table loaded successfully")
			}

			go func() {
				ticker := time.NewTicker(5 * time.Minute)
				defer ticker.Stop()
				for range ticker.C {
					if err := ql.Persist(qTablePath); err != nil {
						log.Printf("Failed to persist Q-table: %v", err)
					} else {
						log.Println("Q-table persisted successfully")
					}
				}
			}()
		}
	}

	healthInterval, err := time.ParseDuration(cfg.HealthCheck)
	if err != nil {
		healthInterval = 10 * time.Second
	}

	health.StartHealthCheck(func() balancer.LoadBalancer {
		mu.RLock()
		defer mu.RUnlock()
		return globalLB
	}, healthInterval)

	log.Printf("Starting Load Balancer on port %d with algorithm %s", cfg.Port, cfg.Algorithm)
	rand.Seed(time.Now().UnixNano())

	server := &http.Server{
		Addr: fmt.Sprintf(":%d", cfg.Port),
	}

	http.HandleFunc("/reload", reloadConfigHandler)
	http.HandleFunc("/stats", features.MetricsHandler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if !rateLimiter.Allow() {
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		cookie, err := r.Cookie("lb_session")
		var peer *balancer.Backend

		mu.RLock()
		lb := globalLB
		mu.RUnlock()

		if err == nil {
			for _, b := range lb.GetBackends() {
				if b.URL.String() == cookie.Value && b.IsAlive() {
					peer = b
					break
				}
			}
		}

		if peer == nil {
			peer = lb.NextBackend(r)
		}

		if peer == nil {
			http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:  "lb_session",
			Value: peer.URL.String(),
			Path:  "/",
		})

		log.Printf("Forwarding to %s", peer.URL)

		atomic.AddInt64(&peer.ActiveConnections, 1)
		defer atomic.AddInt64(&peer.ActiveConnections, -1)

		peer.ReverseProxy.ServeHTTP(w, r)

		duration := time.Since(time.Now())
		features.RecordRequest(duration, false)
		lb.OnRequestCompletion(peer.URL, duration, nil)
	})

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
		log.Println("Server exited")
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Could not listen on %s: %v", server.Addr, err)
	}
}
