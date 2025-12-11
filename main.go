package main

import (
	"advanced-lb/balancer"
	"advanced-lb/features"
	"advanced-lb/health"
	"context"
	"flag"
	"fmt"
	"log"
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

type statusCapture struct {
	http.ResponseWriter
	statusCode int
}

func (sc *statusCapture) WriteHeader(code int) {
	sc.statusCode = code
	sc.ResponseWriter.WriteHeader(code)
}

type Config struct {
	Port        int    `yaml:"port"`
	Algorithm   string `yaml:"algorithm"`
	HealthCheck string `yaml:"health_check_interval"`
	QLearning   struct {
		Alpha   float64 `yaml:"alpha"`
		Gamma   float64 `yaml:"gamma"`
		Epsilon float64 `yaml:"epsilon"`
	} `yaml:"q_learning"`
	Middleware struct {
		Compress        bool  `yaml:"compress"`
		MaxBodySize     int64 `yaml:"max_body_size"`
		SecurityHeaders bool  `yaml:"security_headers"`
	} `yaml:"middleware"`
	CircuitBreaker struct {
		Threshold int    `yaml:"threshold"`
		Timeout   string `yaml:"timeout"`
	} `yaml:"circuit_breaker"`
	RateLimiter struct {
		Enabled bool `yaml:"enabled"`
		Limit   int  `yaml:"limit"`
		Burst   int  `yaml:"burst"`
	} `yaml:"rate_limiter"`
	SSL struct {
		Enabled  bool   `yaml:"enabled"`
		CertFile string `yaml:"cert_file"`
		KeyFile  string `yaml:"key_file"`
	} `yaml:"ssl"`
	Backends []struct {
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

	cbThreshold := cfg.CircuitBreaker.Threshold
	if cbThreshold <= 0 {
		cbThreshold = 3
	}

	cbTimeout, err := time.ParseDuration(cfg.CircuitBreaker.Timeout)
	if err != nil {
		cbTimeout = 10 * time.Second
	}

	for _, b := range cfg.Backends {
		u, err := url.Parse(b.URL)
		if err != nil {
			log.Printf("Invalid backend URL %s: %v", b.URL, err)
			continue
		}
		pool.Backends = append(pool.Backends, balancer.NewBackend(u, b.Weight, cbThreshold, cbTimeout))
	}

	var lb balancer.LoadBalancer
	switch cfg.Algorithm {
	case "round-robin":
		lb = balancer.NewRoundRobin(pool)
	case "least-connections":
		lb = balancer.NewLeastConnections(pool)
	case "q-learning":
		epsilon := cfg.QLearning.Epsilon
		if epsilon == 0 {
			epsilon = 0.01
		}
		alpha := cfg.QLearning.Alpha
		if alpha == 0 {
			alpha = 0.3
		}
		gamma := cfg.QLearning.Gamma
		if gamma == 0 {
			gamma = 0.95
		}
		lb = balancer.NewQLearning(pool, epsilon, alpha, gamma)
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

	var oldQTable map[string]float64
	var oldCounts map[string]int64
	var oldEpsilon, oldGamma, oldMaxQValue, oldLastQDelta float64

	mu.RLock()
	if ql, ok := globalLB.(*balancer.QLearning); ok {
		oldQTable = make(map[string]float64)
		oldCounts = make(map[string]int64)
		ql.ExportState(&oldQTable, &oldCounts, &oldEpsilon, &oldGamma, &oldMaxQValue, &oldLastQDelta)
		log.Println("Saved Q-Learning state for reload")
	}
	mu.RUnlock()

	mu.Lock()
	globalLB = initLB(newCfg)

	if ql, ok := globalLB.(*balancer.QLearning); ok && oldQTable != nil {
		ql.ImportState(oldQTable, oldCounts, oldEpsilon, oldGamma, oldMaxQValue, oldLastQDelta)
		log.Println("Q-Learning state restored after reload")
	}
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

	rlLimit := cfg.RateLimiter.Limit
	if rlLimit <= 0 {
		rlLimit = 1000
	}
	rlBurst := cfg.RateLimiter.Burst
	if rlBurst <= 0 {
		rlBurst = 500
	}

	rateLimiter = features.NewRateLimiter(float64(rlBurst), float64(rlLimit))

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

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	http.HandleFunc("/reload", reloadConfigHandler)
	http.HandleFunc("/stats", features.MetricsHandler)
	http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	mainHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.RateLimiter.Enabled && !rateLimiter.Allow() {
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
				if b.URL.String() == cookie.Value {
					if b.IsAlive() {
						peer = b
						break
					} else {
						http.SetCookie(w, &http.Cookie{
							Name:   "lb_session",
							Value:  "",
							Path:   "/",
							MaxAge: -1,
						})
						break
					}
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

		atomic.AddInt64(&peer.ActiveConnections, 1)
		defer atomic.AddInt64(&peer.ActiveConnections, -1)

		capture := &statusCapture{ResponseWriter: w, statusCode: http.StatusOK}

		start := time.Now()
		peer.ReverseProxy.ServeHTTP(capture, r)
		duration := time.Since(start)

		var requestErr error
		isError := capture.statusCode >= 500 || capture.statusCode == http.StatusBadGateway
		if isError {
			requestErr = fmt.Errorf("backend error: status %d", capture.statusCode)
		}

		features.RecordRequest(duration, capture.statusCode)
		lb.OnRequestCompletion(peer.URL, duration, requestErr)

		log.Printf(`{"time":"%s","client":"%s","method":"%s","path":"%s","backend":"%s","status":%d,"duration_ms":%d,"error":"%v"}`,
			start.Format(time.RFC3339),
			r.RemoteAddr,
			r.Method,
			r.URL.Path,
			peer.URL.String(),
			capture.statusCode,
			duration.Milliseconds(),
			requestErr,
		)
	})

	middlewares := []features.Middleware{
		features.TracingMiddleware,
		features.ProxyHeadersMiddleware,
	}

	if cfg.Middleware.MaxBodySize > 0 {
		middlewares = append(middlewares, features.MaxBodySizeMiddleware(cfg.Middleware.MaxBodySize))
	}

	if cfg.Middleware.SecurityHeaders {
		middlewares = append(middlewares, features.SecurityHeadersMiddleware)
	}

	if cfg.Middleware.Compress {
		middlewares = append(middlewares, features.GzipMiddleware)
	}

	finalHandler := features.Chain(mainHandler, middlewares...)
	log.Println("Initializing Middleware chain and registering handlers...")
	http.Handle("/", finalHandler)

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit
		log.Println("Shutting down server...")

		mu.RLock()
		if ql, ok := globalLB.(*balancer.QLearning); ok {
			qTablePath := "qtable.json"
			if err := ql.Persist(qTablePath); err != nil {
				log.Printf("Failed to save Q-table on shutdown: %v", err)
			} else {
				log.Println("Q-table saved successfully on shutdown")
			}
		}
		mu.RUnlock()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %v", err)
		}
		log.Println("Server exited")
	}()

	if cfg.SSL.Enabled {
		log.Printf("Starting HTTPS Load Balancer on port %d", cfg.Port)
		if err := server.ListenAndServeTLS(cfg.SSL.CertFile, cfg.SSL.KeyFile); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v", server.Addr, err)
		}
	} else {
		log.Printf("Starting HTTP Load Balancer on port %d", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not listen on %s: %v", server.Addr, err)
		}
	}
}
