// limiter/ratelimiter.go
package limiter

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
)

type RateLimiter struct {
	rate       float64
	capacity   float64
	tokens     float64
	lastUpdate time.Time
	mu         sync.Mutex
}

// APIConfig holds configuration for an API
type APIConfig struct {
	BaseURL      string
	RateLimit    float64
	BurstLimit   float64
	Methods      map[string]MethodConfig
	Headers      map[string]string // Required headers for the API
	RequireAuth  bool              // Whether authentication is required
	IPBasedLimit bool              // Whether to apply limits per IP
	Timeout      time.Duration     // Request timeout
}

// MethodConfig holds configuration for specific HTTP methods
type MethodConfig struct {
	RateLimit    float64
	BurstLimit   float64
	PathPatterns []string // Specific path patterns this limit applies to
	Enabled      bool
}

// APIRegistry holds the rate limiters for registered APIs
type APIRegistry struct {
	apiConfigs     map[string]APIConfig
	globalLimiters map[string]*RateLimiter
	methodLimiters map[string]map[string]*RateLimiter
	ipLimiters     map[string]map[string]*RateLimiter
	stats          map[string]*APIStats
	mu             sync.RWMutex
}

// APIStats holds detailed statistics for an API
type APIStats struct {
	Methods   map[string]*MethodStats
	IPStats   map[string]*IPStats
	StartTime time.Time
	LastReset time.Time
}

// MethodStats holds statistics for a specific method
type MethodStats struct {
	TotalRequests    int64
	SuccessRequests  int64
	FailedRequests   int64
	RateLimitDenials int64
	AverageLatency   time.Duration
	LastAccess       time.Time
}

// IPStats holds statistics for IP-based limiting
type IPStats struct {
	TotalRequests   int64
	AllowedRequests int64
	DeniedRequests  int64
	LastAccess      time.Time
}

func NewRateLimiter(ratePerSecond, capacity float64) *RateLimiter {
	return &RateLimiter{
		rate:       ratePerSecond,
		capacity:   capacity,
		tokens:     capacity,
		lastUpdate: time.Now(),
	}
}

func NewAPIRegistry() *APIRegistry {
	return &APIRegistry{
		apiConfigs:     make(map[string]APIConfig),
		globalLimiters: make(map[string]*RateLimiter),
		methodLimiters: make(map[string]map[string]*RateLimiter),
		ipLimiters:     make(map[string]map[string]*RateLimiter),
		stats:          make(map[string]*APIStats),
	}
}

// RegisterAPI registers a new API with its configuration
func (r *APIRegistry) RegisterAPI(config APIConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Validate URL
	_, err := url.Parse(config.BaseURL)
	if err != nil {
		return fmt.Errorf("invalid base URL: %v", err)
	}

	// Initialize API configuration
	r.apiConfigs[config.BaseURL] = config
	r.globalLimiters[config.BaseURL] = NewRateLimiter(config.RateLimit, config.BurstLimit)

	// Initialize method limiters
	r.methodLimiters[config.BaseURL] = make(map[string]*RateLimiter)
	for method, methodConfig := range config.Methods {
		if methodConfig.Enabled {
			r.methodLimiters[config.BaseURL][strings.ToUpper(method)] =
				NewRateLimiter(methodConfig.RateLimit, methodConfig.BurstLimit)
		}
	}

	// Initialize statistics
	r.stats[config.BaseURL] = &APIStats{
		Methods:   make(map[string]*MethodStats),
		IPStats:   make(map[string]*IPStats),
		StartTime: time.Now(),
		LastReset: time.Now(),
	}

	return nil
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

// Middleware creates an HTTP middleware with enhanced features
func (r *APIRegistry) Middleware(apiURL string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		startTime := time.Now()

		r.mu.RLock()
		config, exists := r.apiConfigs[apiURL]
		r.mu.RUnlock()

		if !exists {
			http.Error(w, "API not registered", http.StatusNotFound)
			return
		}

		// Check required headers
		for header, value := range config.Headers {
			if req.Header.Get(header) != value {
				http.Error(w, "Missing required headers", http.StatusBadRequest)
				return
			}
		}

		// Get client IP for IP-based limiting
		clientIP := req.RemoteAddr
		if config.IPBasedLimit {
			r.mu.Lock()
			if _, exists := r.ipLimiters[apiURL]; !exists {
				r.ipLimiters[apiURL] = make(map[string]*RateLimiter)
			}
			if _, exists := r.ipLimiters[apiURL][clientIP]; !exists {
				r.ipLimiters[apiURL][clientIP] = NewRateLimiter(config.RateLimit, config.BurstLimit)
			}
			r.mu.Unlock()
		}

		// Get appropriate limiter
		limiter := r.getLimiter(apiURL, req.Method)
		ipLimiter := r.getIPLimiter(apiURL, clientIP)

		// Check rate limits
		if (limiter != nil && !limiter.Allow()) || (ipLimiter != nil && !ipLimiter.Allow()) {
			r.updateStats(apiURL, req.Method, clientIP, false, time.Since(startTime))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		// Add timeout if configured
		if config.Timeout > 0 {
			ctx, cancel := context.WithTimeout(req.Context(), config.Timeout)
			defer cancel()
			req = req.WithContext(ctx)
		}

		// Serve the request
		next.ServeHTTP(w, req)
		r.updateStats(apiURL, req.Method, clientIP, true, time.Since(startTime))
	})
}

// updateStats updates the statistics for an API request
func (r *APIRegistry) updateStats(apiURL, method, clientIP string, success bool, latency time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := r.stats[apiURL]
	if stats == nil {
		return
	}

	// Update method stats
	methodStats, exists := stats.Methods[method]
	if !exists {
		methodStats = &MethodStats{}
		stats.Methods[method] = methodStats
	}

	methodStats.TotalRequests++
	if success {
		methodStats.SuccessRequests++
	} else {
		methodStats.FailedRequests++
		methodStats.RateLimitDenials++
	}
	methodStats.AverageLatency = (methodStats.AverageLatency + latency) / 2
	methodStats.LastAccess = time.Now()

	// Update IP stats if IP-based limiting is enabled
	if r.apiConfigs[apiURL].IPBasedLimit {
		ipStats, exists := stats.IPStats[clientIP]
		if !exists {
			ipStats = &IPStats{}
			stats.IPStats[clientIP] = ipStats
		}
		ipStats.TotalRequests++
		if success {
			ipStats.AllowedRequests++
		} else {
			ipStats.DeniedRequests++
		}
		ipStats.LastAccess = time.Now()
	}
}

// DisplayEnhancedStats displays detailed API statistics
func (r *APIRegistry) DisplayEnhancedStats() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		r.mu.RLock()
		t := table.NewWriter()
		t.SetOutputMirror(os.Stdout)
		t.SetStyle(table.StyleDouble)

		// Create headers
		t.AppendHeader(table.Row{
			"API URL",
			"Method",
			"Total Requests",
			"Success",
			"Failed",
			"Rate Limited",
			"Avg Latency",
			"Last Access",
		})

		for apiURL, stats := range r.stats {
			for method, methodStats := range stats.Methods {
				t.AppendRow(table.Row{
					apiURL,
					method,
					methodStats.TotalRequests,
					methodStats.SuccessRequests,
					methodStats.FailedRequests,
					methodStats.RateLimitDenials,
					methodStats.AverageLatency.Round(time.Millisecond),
					methodStats.LastAccess.Format("15:04:05"),
				})
			}
		}

		fmt.Print("\033[H\033[2J") // Clear screen
		t.Render()
		r.mu.RUnlock()
	}
}

func (r *APIRegistry) getLimiter(apiURL, method string) *RateLimiter {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if methodLimiter, exists := r.methodLimiters[apiURL][strings.ToUpper(method)]; exists {
		return methodLimiter
	}
	return r.globalLimiters[apiURL]
}

func (r *APIRegistry) getIPLimiter(apiURL, clientIP string) *RateLimiter {
	if !r.apiConfigs[apiURL].IPBasedLimit {
		return nil
	}
	return r.ipLimiters[apiURL][clientIP]
}
