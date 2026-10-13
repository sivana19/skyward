package main

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"skyward/internal/aggregator"
	"skyward/internal/booking"

	"github.com/gin-gonic/gin"
)

type HoldRequest struct {
	FlightID   string `json:"flight_id" binding:"required"`
	SeatNumber string `json:"seat_number" binding:"required"`
	CustomerID string `json:"customer_id" binding:"required"`
}

type ConfirmRequest struct {
	FlightID   string `json:"flight_id" binding:"required"`
	SeatNumber string `json:"seat_number" binding:"required"`
	CustomerID string `json:"customer_id" binding:"required"`
}

func main() {
	router := gin.Default()

	// 1. Initialize Booking Engine (Holds expire after 30 seconds if abandoned)
	engine := booking.NewBookingEngine(30 * time.Second)

	// Seed sample flights with seats
	engine.RegisterFlight("ET-701", "ADD", "DXB", 5) // 5 seats on ET-701
	engine.RegisterFlight("ET-302", "ADD", "NBO", 3) // 3 seats on ET-302

	// 2. Initialize Multi-Airline Aggregator with 3 providers
	p1 := aggregator.NewMockProvider("Ethiopian Airlines", 420.0, 50*time.Millisecond, 100*time.Millisecond)
	p2 := aggregator.NewMockProvider("Emirates", 510.0, 70*time.Millisecond, 120*time.Millisecond)
	p3 := aggregator.NewMockProvider("Flydubai", 360.0, 40*time.Millisecond, 90*time.Millisecond)
	agg := aggregator.NewAggregator(200*time.Millisecond, p1, p2, p3)

	// Serve the web UI directly on root "/"
	router.StaticFile("/", "./web/index.html")

	api := router.Group("/api/v1")
	{
		// Fetch Flight and Seat Map State for UI
		api.GET("/flights/:flight_id/seats", func(c *gin.Context) {
			flightID := c.Param("flight_id")
			flight, exists := engine.GetFlight(flightID)
			if !exists {
				c.JSON(http.StatusNotFound, gin.H{"error": "flight not found"})
				return
			}
			c.JSON(http.StatusOK, flight)
		})

		// 1. Concurrent Flight Search with Optional Price Filtering
		// GET /api/v1/flights/search?origin=ADD&dest=DXB&max_price=400
		api.GET("/flights/search", func(c *gin.Context) {
			origin := c.DefaultQuery("origin", "ADD")
			dest := c.DefaultQuery("dest", "DXB")

			offers := agg.SearchAll(c.Request.Context(), origin, dest)

			maxPriceStr := c.Query("max_price")
			if maxPriceStr != "" {
				maxPrice, err := strconv.ParseFloat(maxPriceStr, 64)
				if err == nil {
					var filtered []aggregator.FlightOffer
					for _, offer := range offers {
						if offer.PriceUSD <= maxPrice {
							filtered = append(filtered, offer)
						}
					}
					offers = filtered
				}
			}

			c.JSON(http.StatusOK, gin.H{
				"origin":      origin,
				"destination": dest,
				"count":       len(offers),
				"offers":      offers,
			})
		})

		// 2. Hold Seat (Atomic & Thread-safe temporary hold)
		// POST /api/v1/seats/hold
		api.POST("/seats/hold", func(c *gin.Context) {
			var req HoldRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			seat, err := engine.HoldSeat(req.FlightID, req.SeatNumber, req.CustomerID)
			if err != nil {
				c.JSON(http.StatusConflict, gin.H{
					"error":   err.Error(),
					"message": "Seat could not be held",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":     "Seat reserved successfully. Complete payment within 30 seconds.",
				"flight_id":   req.FlightID,
				"seat_number": seat.SeatNumber,
				"status":      seat.Status,
				"expires_at":  seat.HoldExpiry,
			})
		})

		// 3. Confirm Seat (Payment finalized)
		// POST /api/v1/seats/confirm
		api.POST("/seats/confirm", func(c *gin.Context) {
			var req ConfirmRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			err := engine.ConfirmBooking(req.FlightID, req.SeatNumber, req.CustomerID)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   err.Error(),
					"message": "Hold expired or invalid booking parameters",
				})
				return
			}

			c.JSON(http.StatusOK, gin.H{
				"message":     "Ticket issued! Seat confirmed.",
				"flight_id":   req.FlightID,
				"seat_number": req.SeatNumber,
				"customer_id": req.CustomerID,
			})
		})
	}

	log.Println("Skyward Travel Engine listening on :8083")
	if err := router.Run(":8083"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
