package features

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type Metrics struct {
	TotalRequests  uint64
	TotalErrors    uint64
	TotalLatencyMs uint64
	Status2xx      uint64
	Status3xx      uint64
	Status4xx      uint64
	Status5xx      uint64
}

var globalMetrics = &Metrics{}

func RecordRequest(duration time.Duration, statusCode int) {
	atomic.AddUint64(&globalMetrics.TotalRequests, 1)
	atomic.AddUint64(&globalMetrics.TotalLatencyMs, uint64(duration.Milliseconds()))

	if statusCode >= 200 && statusCode < 300 {
		atomic.AddUint64(&globalMetrics.Status2xx, 1)
	} else if statusCode >= 300 && statusCode < 400 {
		atomic.AddUint64(&globalMetrics.Status3xx, 1)
	} else if statusCode >= 400 && statusCode < 500 {
		atomic.AddUint64(&globalMetrics.Status4xx, 1)
	} else if statusCode >= 500 {
		atomic.AddUint64(&globalMetrics.Status5xx, 1)
		atomic.AddUint64(&globalMetrics.TotalErrors, 1)
	}
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	reqs := atomic.LoadUint64(&globalMetrics.TotalRequests)
	errs := atomic.LoadUint64(&globalMetrics.TotalErrors)
	lat := atomic.LoadUint64(&globalMetrics.TotalLatencyMs)
	s2xx := atomic.LoadUint64(&globalMetrics.Status2xx)
	s3xx := atomic.LoadUint64(&globalMetrics.Status3xx)
	s4xx := atomic.LoadUint64(&globalMetrics.Status4xx)
	s5xx := atomic.LoadUint64(&globalMetrics.Status5xx)

	var avgLat uint64 = 0
	if reqs > 0 {
		avgLat = lat / reqs
	}

	response := fmt.Sprintf(`{
		"total_requests": %d,
		"total_errors": %d,
		"avg_latency_ms": %d,
		"status_2xx": %d,
		"status_3xx": %d,
		"status_4xx": %d,
		"status_5xx": %d
	}`, reqs, errs, avgLat, s2xx, s3xx, s4xx, s5xx)
	w.Write([]byte(response))

	log.Printf("Metrics: %s", response)
}
