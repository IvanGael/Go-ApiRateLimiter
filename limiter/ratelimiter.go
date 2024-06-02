package limiter

import (
	"fmt"
	"github.com/jedib0t/go-pretty/v6/table"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// RateLimiter represents a token bucket rate limiter.
type RateLimiter struct {
	rate       float64
	capacity   float64
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(ratePerSecond, capacity float64) *RateLimiter {
	return &RateLimiter{
		rate:       ratePerSecond,
		capacity:   capacity,
		tokens:     capacity,
		lastUpdate: time.Now(),
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
		rl.tokens = rl.capacity
	}
	rl.lastUpdate = now

	if rl.tokens >= 1.0 {
		rl.tokens--
		return true
	}
	return false
}

// APIRegistry holds the rate limiters for registered APIs.
type APIRegistry struct {
	globalLimiters map[string]*RateLimiter
	methodLimiters map[string]map[string]*RateLimiter
	stats          map[string]map[string]*APIMethodStats
	mu             sync.Mutex
}

// APIMethodStats holds statistics for a specific API method.
type APIMethodStats struct {
	TotalRequests int
	Allowed       int
	Denied        int
}

// NewAPIRegistry creates a new APIRegistry instance.
func NewAPIRegistry() *APIRegistry {
	return &APIRegistry{
		globalLimiters: make(map[string]*RateLimiter),
		methodLimiters: make(map[string]map[string]*RateLimiter),
		stats:          make(map[string]map[string]*APIMethodStats),
	}
}

// RegisterAPI registers a new API with the given rate limits.
func (r *APIRegistry) RegisterAPI(apiName string, ratePerSecond, capacity float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.globalLimiters[apiName] = NewRateLimiter(ratePerSecond, capacity)
}

// RegisterAPIMethod registers a new API method with the given rate limits.
func (r *APIRegistry) RegisterAPIMethod(apiName, method string, ratePerSecond, capacity float64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.methodLimiters[apiName]; !exists {
		r.methodLimiters[apiName] = make(map[string]*RateLimiter)
	}
	r.methodLimiters[apiName][strings.ToUpper(method)] = NewRateLimiter(ratePerSecond, capacity)

	if _, exists := r.stats[apiName]; !exists {
		r.stats[apiName] = make(map[string]*APIMethodStats)
	}
	r.stats[apiName][strings.ToUpper(method)] = &APIMethodStats{}
}

// getLimiter retrieves the appropriate rate limiter (method-specific or global).
func (r *APIRegistry) getLimiter(apiName, method string) *RateLimiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	if methodLimiter, exists := r.methodLimiters[apiName][strings.ToUpper(method)]; exists {
		return methodLimiter
	}
	return r.globalLimiters[apiName]
}

// Middleware creates an HTTP middleware for rate limiting.
func (r *APIRegistry) Middleware(apiName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		limiter := r.getLimiter(apiName, req.Method)
		r.mu.Lock()
		stats := r.stats[apiName][strings.ToUpper(req.Method)]
		stats.TotalRequests++
		r.mu.Unlock()

		if limiter == nil || !limiter.Allow() {
			r.mu.Lock()
			stats.Denied++
			r.mu.Unlock()
			http.Error(w, "Too Many Requests", http.StatusTooManyRequests)
			return
		}

		r.mu.Lock()
		stats.Allowed++
		r.mu.Unlock()

		next.ServeHTTP(w, req)
	})
}

// DisplayStats displays the API statistics in the terminal.
func (r *APIRegistry) DisplayStats() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.Lock()
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleLight)
		t.AppendHeader(table.Row{"API Name", "Method", "Total Requests", "Allowed", "Denied", "Last Update"})
		for apiName, methods := range r.stats {
			for method, stats := range methods {
				lastUpdateFormatted := r.globalLimiters[apiName].lastUpdate.Format("2006-01-02 15:04:05")
				t.AppendRow(table.Row{apiName, method, stats.TotalRequests, stats.Allowed, stats.Denied, lastUpdateFormatted})
			}
		}
		t.Render()
		r.mu.Unlock()
		fmt.Println()
	}
}
