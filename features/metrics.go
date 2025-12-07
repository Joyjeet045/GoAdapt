/*
    Author: Joyjeet Roy
*/
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
}

var globalMetrics = &Metrics{}

func RecordRequest(duration time.Duration, isError bool) {
	atomic.AddUint64(&globalMetrics.TotalRequests, 1)
	atomic.AddUint64(&globalMetrics.TotalLatencyMs, uint64(duration.Milliseconds()))
	if isError {
		atomic.AddUint64(&globalMetrics.TotalErrors, 1)
	}
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	reqs := atomic.LoadUint64(&globalMetrics.TotalRequests)
	errs := atomic.LoadUint64(&globalMetrics.TotalErrors)
	lat := atomic.LoadUint64(&globalMetrics.TotalLatencyMs)
	var avgLat uint64 = 0
	if reqs > 0 {
		avgLat = lat / reqs
	}

	response := fmt.Sprintf(`{"total_requests": %d, "total_errors": %d, "avg_latency_ms": %d}`, reqs, errs, avgLat)
	w.Write([]byte(response))

	log.Printf("Metrics: %s", response)
}
