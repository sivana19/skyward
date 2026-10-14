package main

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"skyward/internal/aggregator"

	"github.com/gin-gonic/gin"
)

// RequestIDMiddleware assigns a unique tracking ID to every HTTP request
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Check if client passed an X-Request-ID; generate one if missing
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			b := make([]byte, 4)
			rand.Read(b)
			reqID = hex.EncodeToString(b)
		}

		// 2. Attach the tracking ID to response headers
		c.Header("X-Request-ID", reqID)

		// 3. Log the incoming request
		slog.Info("incoming request",
			"request_id", reqID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"client_ip", c.ClientIP(),
		)

		start := time.Now()
		c.Next()
		latency := time.Since(start)

		// 4. Log request completion with duration and response code
		slog.Info("request completed",
			"request_id", reqID,
			"status", c.Writer.Status(),
			"duration_ms", latency.Milliseconds(),
		)
	}
}

func main() {
	// Configure structured JSON logging as global default
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Set up mock upstream providers
	p1 := aggregator.NewMockProvider("EthiopianAirlines", 350.0, 50*time.Millisecond, 150*time.Millisecond)
	p2 := aggregator.NewMockProvider("Emirates", 420.0, 80*time.Millisecond, 200*time.Millisecond)
	p3 := aggregator.NewMockProvider("FlyDubai", 280.0, 100*time.Millisecond, 300*time.Millisecond)

	// Aggregator with 400ms timeout budget across all providers
	agg := aggregator.NewAggregator(400*time.Millisecond, p1, p2, p3)

	router := gin.Default()

	// Apply structured logging middleware
	router.Use(RequestIDMiddleware())

	router.GET("/flights/search", func(c *gin.Context) {
		origin := c.Query("origin")
		dest := c.Query("dest")

		if origin == "" || dest == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "both 'origin' and 'dest' query parameters are required",
			})
			return
		}

		// Aggregate offers passing the HTTP request context for cancellation
		offers := agg.SearchAll(c.Request.Context(), origin, dest)

		// Optional max_price query filter
		maxPriceStr := c.Query("max_price")
		if maxPriceStr != "" {
			maxPrice, err := strconv.ParseFloat(maxPriceStr, 64)
			if err == nil {
				filtered := make([]aggregator.FlightOffer, 0)
				for _, o := range offers {
					if o.PriceUSD <= maxPrice {
						filtered = append(filtered, o)
					}
				}
				offers = filtered
			}
		}

		c.JSON(http.StatusOK, gin.H{
			"count":  len(offers),
			"offers": offers,
		})
	})

	slog.Info("server starting on port :8080")
	if err := router.Run(":8080"); err != nil {
		slog.Error("server failed to start", "error", err)
	}
}
