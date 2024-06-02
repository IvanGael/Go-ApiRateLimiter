package main

import (
	"fmt"
	"net/http"

	"api-rate-limiter/limiter"
)

func main() {
	registry := limiter.NewAPIRegistry()
	registry.RegisterAPI("exampleAPI", 5, 10)              // Global rate: 5 requests per second, burst capacity: 10
	registry.RegisterAPIMethod("exampleAPI", "GET", 2, 5)  // GET method: 2 requests per second, burst capacity: 5
	registry.RegisterAPIMethod("exampleAPI", "POST", 1, 2) // POST method: 1 request per second, burst capacity: 2

	mux := http.NewServeMux()
	mux.HandleFunc("/data", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "Here is some data.")
		if err != nil {
			return
		}
	})

	mux.HandleFunc("/submit", func(w http.ResponseWriter, r *http.Request) {
		_, err := fmt.Fprintf(w, "Data submitted.")
		if err != nil {
			return
		}
	})

	rateLimitedMux := registry.Middleware("exampleAPI", mux)

	go registry.DisplayStats() // Start the statistics display in a separate goroutine

	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", rateLimitedMux); err != nil {
		fmt.Println("Server failed:", err)
	}
}
