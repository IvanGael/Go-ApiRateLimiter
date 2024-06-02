<h1>Api rate limiter built in Go</h1>

This project demonstrates a simple API rate limiter implemented in Go using a token bucket algorithm. The rate limiter can be configured for both global limits and specific HTTP methods (GET, POST, etc.). It also includes real-time statistics displayed in the terminal and scripts to test the rate limiter by making requests to the endpoints.


<h2>Project Structure</h2>
- limiter/ratelimiter.go: Contains the implementation of the rate limiter and statistics display.
- api/main.go: Sets up the API server with rate-limited endpoints.
- get_requests.sh: Script to send 10 GET requests per second to the /data endpoint.
- post_requests.sh: Script to send 5 POST requests per second to the /submit endpoint.


<h2>Implementation Details</h2>

- Rate Limiter :
  The rate limiter is implemented using a token bucket algorithm. It allows configuring both global rate limits and method-specific rate limits. The RateLimiter struct holds the configuration and state for the rate limiter. The APIRegistry struct manages multiple RateLimiter instances for different APIs and methods.

- Displaying Statistics :
  The DisplayStats method of APIRegistry displays real-time statistics about incoming requests in a terminal table. This includes the total number of requests, allowed requests, denied requests, and the last update time for each API and method.

- Setting Up the API Server :
  The api/main.go file sets up an HTTP server with two endpoints: /data for GET requests and /submit for POST requests. The server uses the APIRegistry middleware to enforce rate limits on these endpoints.

- Testing the Rate Limiter :
  To test the rate limiter, two scripts (get_requests.sh and post_requests.sh) are provided. These scripts send requests to the server at specified rates, allowing you to observe the rate limiting in action and monitor the real-time statistics.


<h2>Usage</h2>
- <h3>Prerequisites</h3>
- - Go 1.16 or later
- - curl command-line tool

- <h3>Setup</h3>
1. After cloning the repository, install dependencies
````
go mod tidy
go get
````
2. Run the API server
````
go run api/main.go
````
3. Run the test scripts
````
get_requests.sh
post_requests.sh
````