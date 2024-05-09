package main

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter represents a token bucket rate limiter.
type RateLimiter struct {
	rate       float64    // Rate of tokens to fill per second
	capacity   float64    // Maximum capacity of the token bucket
	tokens     float64    // Current number of tokens in the bucket
	lastUpdate time.Time  // Last time the token bucket was updated
	mu         sync.Mutex // Mutex for thread safety
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(ratePerSecond, capacity float64) *RateLimiter {
	return &RateLimiter{
		rate:     ratePerSecond,
		capacity: capacity,
		tokens:   capacity,
	}
}

// Allow checks if the rate limiter allows the request or not.
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(rl.lastUpdate).Seconds()
	rl.tokens += elapsed * rl.rate
	if rl.tokens > rl.capacity {
		rl.tokens = rl.capacity // Ensure tokens don't exceed capacity
	}
	rl.lastUpdate = now

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	return false
}

func main() {
	// Example usage
	limiter := NewRateLimiter(2, 5) // Allow 2 requests per second with a maximum burst of 5 tokens
	for i := 0; i < 10; i++ {
		if limiter.Allow() {
			fmt.Println("Request allowed")
		} else {
			fmt.Println("Request denied")
		}
		time.Sleep(time.Second)
	}
}
