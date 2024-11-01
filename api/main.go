// main.go
package main

import (
	"api-rate-limiter/limiter"
	"fmt"
	"net/http"
	"time"
)

func main() {
	registry := limiter.NewAPIRegistry()

	// Register an API with enhanced configuration
	apiConfig := limiter.APIConfig{
		BaseURL:      "http://localhost:8080",
		RateLimit:    100,  // Global rate limit
		BurstLimit:   20,   // Global burst limit
		IPBasedLimit: true, // Enable IP-based rate limiting
		RequireAuth:  true,
		Timeout:      time.Second * 30,
		Headers: map[string]string{
			"X-API-Key": "required", // Required headers
		},
		Methods: map[string]limiter.MethodConfig{
			"GET": {
				RateLimit:    50,
				BurstLimit:   10,
				PathPatterns: []string{"/api/*"},
				Enabled:      true,
			},
			"POST": {
				RateLimit:    20,
				BurstLimit:   5,
				PathPatterns: []string{"/api/submit/*"},
				Enabled:      true,
			},
		},
	}

	err := registry.RegisterAPI(apiConfig)
	if err != nil {
		fmt.Printf("Failed to register API: %v\n", err)
		return
	}

	// Create routes
	mux := http.NewServeMux()

	mux.HandleFunc("/api/data", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 100) // Simulate processing time
		fmt.Fprintf(w, "Data retrieved successfully")
	})

	mux.HandleFunc("/api/submit", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(time.Millisecond * 200) // Simulate processing time
		fmt.Fprintf(w, "Data submitted successfully")
	})

	// Apply rate limiting middleware
	rateLimitedMux := registry.Middleware("http://localhost:8080", mux)

	// Start statistics display in a separate goroutine
	go registry.DisplayEnhancedStats()

	// Start server
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", rateLimitedMux); err != nil {
		fmt.Printf("Server failed: %v\n", err)
	}
}
