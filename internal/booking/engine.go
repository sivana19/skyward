package booking

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

var (
	ErrSoldOut         = errors.New("flight is completely booked")
	ErrSeatLocked      = errors.New("seat is currently held by another customer")
	ErrBookingNotFound = errors.New("reservation expired or invalid")
)

type SeatStatus string

const (
	StatusAvailable SeatStatus = "AVAILABLE"
	StatusHeld      SeatStatus = "HELD"
	StatusConfirmed SeatStatus = "CONFIRMED"
)

type Seat struct {
	SeatNumber string     `json:"seat_number"`
	Status     SeatStatus `json:"status"`
	HeldBy     string     `json:"held_by,omitempty"`
	HoldExpiry time.Time  `json:"hold_expiry,omitempty"`
}

type Flight struct {
	FlightID       string           `json:"flight_id"`
	Origin         string           `json:"origin"`
	Destination    string           `json:"destination"`
	AvailableSeats int              `json:"available_seats"`
	Seats          map[string]*Seat `json:"seats"`
}

type BookingEngine struct {
	mu      sync.Mutex
	flights map[string]*Flight
	timeout time.Duration // Time a seat is held before auto-release
}

func NewBookingEngine(holdTimeout time.Duration) *BookingEngine {
	engine := &BookingEngine{
		flights: make(map[string]*Flight),
		timeout: holdTimeout,
	}

	// Background sweeper: Auto-releases expired holds so seats go back on sale
	go engine.startExpirationDaemon()

	return engine
}

// RegisterFlight populates initial inventory
func (b *BookingEngine) RegisterFlight(flightID, origin, dest string, totalSeats int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	seats := make(map[string]*Seat)
	for i := 1; i <= totalSeats; i++ {
		seatNo := fmt.Sprintf("%dA", i)
		seats[seatNo] = &Seat{
			SeatNumber: seatNo,
			Status:     StatusAvailable,
		}
	}

	b.flights[flightID] = &Flight{
		FlightID:       flightID,
		Origin:         origin,
		Destination:    dest,
		AvailableSeats: totalSeats,
		Seats:          seats,
	}
}

// HoldSeat locks a seat for a specific customer for X seconds (Atomic & Thread-Safe)
func (b *BookingEngine) HoldSeat(flightID, seatNo, customerID string) (*Seat, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	flight, exists := b.flights[flightID]
	if !exists {
		return nil, errors.New("flight not found")
	}

	seat, exists := flight.Seats[seatNo]
	if !exists {
		return nil, errors.New("seat does not exist")
	}

	// Check if seat is already occupied or held
	if seat.Status == StatusConfirmed {
		return nil, ErrSoldOut
	}
	if seat.Status == StatusHeld && time.Now().Before(seat.HoldExpiry) {
		if seat.HeldBy != customerID {
			return nil, ErrSeatLocked
		}
	}

	// Reserve seat and lock down timer
	seat.Status = StatusHeld
	seat.HeldBy = customerID
	seat.HoldExpiry = time.Now().Add(b.timeout)
	flight.AvailableSeats--

	return seat, nil
}

// ConfirmBooking finalizes payment and permanently confirms the seat
func (b *BookingEngine) ConfirmBooking(flightID, seatNo, customerID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	flight, exists := b.flights[flightID]
	if !exists {
		return errors.New("flight not found")
	}

	seat, exists := flight.Seats[seatNo]
	if !exists {
		return errors.New("seat does not exist")
	}

	if seat.Status != StatusHeld || seat.HeldBy != customerID {
		return ErrBookingNotFound
	}

	seat.Status = StatusConfirmed
	seat.HeldBy = customerID
	seat.HoldExpiry = time.Time{} // Clear timer
	return nil
}

// Background worker: releases seats if customers abandon checkout
func (b *BookingEngine) startExpirationDaemon() {
	ticker := time.NewTicker(1 * time.Second)
	for range ticker.C {
		b.mu.Lock()
		now := time.Now()
		for _, flight := range b.flights {
			for _, seat := range flight.Seats {
				if seat.Status == StatusHeld && now.After(seat.HoldExpiry) {
					// Seat expired - release back to public inventory
					seat.Status = StatusAvailable
					seat.HeldBy = ""
					seat.HoldExpiry = time.Time{}
					flight.AvailableSeats++
				}
			}
		}
		b.mu.Unlock()
	}
}

// GetFlight safely retrieves flight details and seat states
func (b *BookingEngine) GetFlight(flightID string) (*Flight, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	flight, exists := b.flights[flightID]
	if !exists {
		return nil, false
	}
	return flight, true
}
