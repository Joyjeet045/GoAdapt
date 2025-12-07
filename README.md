![Load Balancer](https://raw.githubusercontent.com/kubernetes/kubernetes/master/logo/logo.png)

# Go-Adapt: Intelligent Load Balancer

> A production-ready HTTP load balancer with adaptive Q-Learning algorithm, built in Go.

## Overview

Go-Adapt is an advanced load balancing solution that intelligently distributes traffic across backend servers. Unlike traditional static algorithms, it employs reinforcement learning to adapt to real-time conditions, optimizing for both performance and reliability.

## Key Features

### 🧠 Intelligent Routing Algorithms
- **Q-Learning** - Adaptive algorithm with:
  - Reinforcement learning with adaptive epsilon decay
  - Discount factor (γ=0.95) for temporal credit assignment
  - Q-table persistence (survives restarts)
  - Lock-free concurrent access (sync.Map)
  - **Best Performance**: 249.46 RPS in production tests
- **Round Robin** - Classic sequential distribution
- **Weighted Round Robin** - Priority-based traffic distribution
- **Least Connections** - Routes to servers with fewest active connections
- **Least Response Time** - Selects fastest responding backend
- **IP Hash** - Consistent routing based on client IP

**Implementation**: `balancer/algorithms.go`, `balancer/q_learning.go`

### 🛡️ Production-Grade Protection

#### Circuit Breaker
Automatically isolates failing backends to prevent cascade failures. Configurable failure threshold and recovery timeout.

**Implementation**: `features/circuit_breaker.go`

#### Rate Limiting
Token bucket algorithm prevents abuse and ensures fair resource allocation. Configurable capacity and refill rate.

**Implementation**: `features/rate_limiter.go`

#### Active Health Checks
Periodic health monitoring with automatic backend status updates. Unhealthy servers are removed from rotation.

**Implementation**: `health/check.go`

#### Connection Pooling
HTTP transport optimization with connection reuse. Reduces latency through persistent connections.

**Implementation**: `balancer/balancer.go` (MaxIdleConns: 100, MaxIdleConnsPerHost: 10)

### 🔄 Operational Features

#### Hot Reload
Update configuration without downtime via `/reload` endpoint. Includes validation to prevent invalid configs.

**Implementation**: `main.go` (reloadConfigHandler with validateConfig)

#### Metrics & Monitoring
Real-time performance metrics exposed via `/stats` endpoint.

**Implementation**: `features/metrics.go`

#### Session Persistence
Cookie-based sticky sessions ensure client requests route to the same backend.

**Implementation**: `main.go` (lb_session cookie)

## Architecture

```
├── main.go                 # Entry point and HTTP server
├── balancer/
│   ├── balancer.go        # Core interfaces and backend management
│   ├── algorithms.go      # Traditional load balancing algorithms
│   └── q_learning.go      # Adaptive Q-Learning implementation
├── features/
│   ├── circuit_breaker.go # Failure isolation
│   ├── rate_limiter.go    # Traffic throttling
│   └── metrics.go         # Performance tracking
├── health/
│   └── check.go           # Health monitoring
├── scripts/
│   ├── comprehensive_benchmark_suite.py  # 15-scenario test suite
│   ├── comprehensive_test.py             # Feature validation
│   └── benchmark_runner.py               # Single-run benchmarks
└── simulation/
    └── mock_servers.py    # Configurable backend simulator
```

## Quick Start

### Prerequisites
- Go 1.19+
- Python 3.8+ (for testing)

### Installation

```bash
git clone https://github.com/yourusername/advanced-lb.git
cd advanced-lb
go mod download
```

### Configuration

Edit `config.yaml`:

```yaml
port: 8080
algorithm: q-learning
health_check_interval: 1s
backends:
  - url: http://localhost:8081
    weight: 1
  - url: http://localhost:8082
    weight: 1
```

### Running

```bash
# Start the load balancer
go run main.go

# In another terminal, start mock backends
python simulation/mock_servers.py
```

## Testing & Benchmarking

### Comprehensive Test Suite
Validates all features across 15 diverse scenarios:

```bash
python scripts/comprehensive_benchmark_suite.py
```

Results saved to `results/comprehensive_suite_results.csv`

### Feature Testing
Tests rate limiting, hot reload, and circuit breakers:

```bash
python scripts/comprehensive_test.py
```

### Single Algorithm Benchmark
Quick performance test:

```bash
python scripts/benchmark_runner.py
```

## API Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Proxies request to backend |
| `/reload` | GET | Hot-reloads configuration |
| `/stats` | GET | Returns performance metrics |

## Configuration Parameters

### Rate Limiter
```go
rateLimiter = features.NewRateLimiter(1000, 500)
// Capacity: 1000 tokens
// Refill: 500 tokens/second
```

### Circuit Breaker
```go
CircuitBreaker: features.NewCircuitBreaker(3, 10*time.Second)
// Threshold: 3 failures
// Timeout: 10 seconds
```

### Q-Learning Parameters
```go
epsilon: 0.05  // Exploration rate
alpha: 0.5     // Learning rate
gamma: 0.95    // Discount factor (temporal credit)
```

## Performance

Tested across 15 scenarios including high jitter, failure injection, and variable latency. Q-Learning demonstrates superior adaptability in complex environments.

See `results/` directory for detailed benchmark data.

## Author

**Joyjeet Roy**

## License

MIT License - see LICENSE file for details
