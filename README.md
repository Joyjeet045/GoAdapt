<div align="center">

<img src="https://raw.githubusercontent.com/kubernetes/kubernetes/master/logo/logo.svg" alt="Logo" width="120"/>

# Go-Adapt
### Adaptive Reinforcement Learning Load Balancer

[![Go Version](https://img.shields.io/badge/Go-1.19+-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

</div>

---

## 📖 Overview

**Go-Adapt** is a high-performance, adaptive layer-7 load balancer engineered in Go. It distinguishes itself from traditional load balancers by incorporating **Reinforcement Learning (Q-Learning)** to dynamically adapt to backend server states in real-time.

Designed for resilience and efficiency, Go-Adapt overcomes the limitations of static algorithms (like Round Robin) by learning from latency patterns, error rates, and connection loads to optimize traffic distribution autonomously.

---

## 🚀 Key Features

### Intelligent Traffic Routing
The core of Go-Adapt is its suite of routing strategies, headlined by its adaptive engine:

*   **Q-Learning (Adaptive)**: Utilizes a Reinforcement Learning agent to balance traffic based on historical performance rewards.
    *   **Reward Function**: `100.0 - (latency_ms / 10.0)` — Balances latency minimization with stability.
    *   **Exploration**: Adaptive epsilon-greedy strategy with decay.
    *   **Persistence**: State preservation across restarts for continuous learning.
*   **Weighted Round Robin**: Standard traffic distribution respecting server capacity weights.
*   **Least Connections**: Dynamically routes to the server with the lowest active load.
*   **Least Response Time**: Prioritizes the backend with the fastest recent response metrics.
*   **IP Hash**: Ensures session consistency by hashing client IP addresses.

### Reliability & Resilience
Engineered for production environments where uptime is non-negotiable:

*   **Circuit Breaking**: Automatically detects and isolates failing backends to prevent cascading system failures.
*   **Active Health Checking**: Periodically probes backend health to ensure traffic is only routed to healthy nodes.
*   **Rate Limiting**: Token-bucket based request limiting to protect against DoS attacks and traffic spikes.
*   **Connection Pooling**: Optimized HTTP transport with persistent connections to minimize handshake overhead.

### Operational Excellence
*   **Hot Configuration Reload**: Update routing rules and backend pools without zero downtime via the `/reload` endpoint.
*   **Real-Time Observability**: Comprehensive metrics exposed via `/stats` for monitoring throughput, latency, and error rates.
*   **Session Persistence**: Sticky sessions via cookies to maintain user state across requests.

---

## 🏗️ Architecture

The project follows a modular, clean architecture designed for maintainability and scalability.

```
.
├── main.go                     # Entry point & HTTP server
├── balancer/                   # Core Load Balancing Logic
│   ├── algorithms.go           # Static Algorithms (RR, WRR, LC, etc.)
│   ├── q_learning.go           # Q-Learning Implementation
│   ├── q_learning_state.go     # State Persistence & Management
│   └── balancer.go             # Common Interfaces & Connection Pooling
├── features/                   # Cross-Cutting Concerns
│   ├── circuit_breaker.go      # Failure Isolation Logic
│   ├── rate_limiter.go         # Traffic Control
│   └── metrics.go              # Telemetry & Stats
├── health/                     # Health Monitoring
│   └── check.go                # Periodic Probe Logic
└── scripts/                    # Testing & Benchmarking tools
```

---

## 🛠️ Quick Start

### Prerequisites
*   **Go**: Version 1.19 or higher
*   **Python**: Version 3.8+ (Required only for running benchmark suites)

### Installation

```bash
git clone https://github.com/Joyjeet045/GoAdapt.git
cd GoAdapt
go mod download
```

### Configuration
Configure the listener and backends in `config.yaml`:

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

### Execution

1.  **Start the Load Balancer**:
    ```bash
    go run main.go
    ```

2.  **Start Mock Backends (Optional)**:
    ```bash
    python simulation/mock_servers.py
    ```

---

## 📊 Benchmarking & Testing

Go-Adapt includes a comprehensive suite of performance tests to validate its behavior under various conditions.

### Comprehensive Suite
Runs the load balancer against **10 distinct scenarios** (e.g., Jitter, Dead Nodes, Latency Traps) to verify adaptability.

```bash
python scripts/comprehensive_benchmark_suite.py
```
> **Output**: `results/comprehensive_suite_results.csv`

### Q-Learning Efficiency Showcase
Specifically demonstrates the superiority of Q-Learning in "Hidden Latency Trap" scenarios against standard algorithms.

```bash
python scripts/q_learning_showcase_benchmark.py
```

### Feature Validation
Validates operational features such as Hot Reloading, Rate Limiting, and Circuit Breaking.

```bash
python scripts/comprehensive_test.py
```

---

## 📡 API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/` | `ANY` | Proxies traffic to the selected backend. |
| `/reload` | `GET` | Triggers a zero-downtime configuration reload. |
| `/stats` | `GET` | Returns JSON-formatted metrics and system status. |

---

## ⚙️ Configuration Details

| Parameter | Default | Description |
| :--- | :--- | :--- |
| **Q-Learning Epsilon** | `0.01` | Initial exploration rate (decays over time). |
| **Q-Learning Alpha** | `0.3` | Learning rate (speed of adaptation). |
| **Q-Learning Gamma** | `0.95` | Discount factor for future rewards. |
| **Rate Limit** | `1000/s` | Maximum request capacity (burst). |
| **Circuit Breaker** | `3 fails` | Threshold to trip the circuit. |

---

## 👨‍💻 Author

**Joyjeet Roy**

---

## 📄 License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
