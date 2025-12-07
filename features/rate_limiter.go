/*
    Author: Joyjeet Roy
*/
package features

import (
	"fmt"
	"sync"
	"time"
)

type RateLimiter struct {
	tokens         float64
	capacity       float64
	refillRate     float64
	lastRefillTime time.Time
	mu             sync.Mutex
}

func NewRateLimiter(capacity float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:         capacity,
		capacity:       capacity,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastRefillTime).Seconds()

	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity
	}
	rl.lastRefillTime = now

	
	fmt.Printf("DEBUG: Tokens: %f, Capacity: %f\n", rl.tokens, rl.capacity)

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	fmt.Println("DEBUG: Rate Limit Hit!")
	return false
}
