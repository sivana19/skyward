package booking

import (
	"sync"
	"testing"
	"time"
)

func TestBookingEngine_PreventOverbooking(t *testing.T) {
	// Initialize a flight with only 1 seat
	engine := NewBookingEngine(5 * time.Second)
	engine.RegisterFlight("ET-701", "ADD", "DXB", 1)

	var wg sync.WaitGroup
	successfulHolds := 0
	var mu sync.Mutex

	// 50 users try to grab the same seat at the exact same millisecond
	for i := 1; i <= 50; i++ {
		wg.Add(1)
		custID := string(rune('A' + i))
		go func(id string) {
			defer wg.Done()
			_, err := engine.HoldSeat("ET-701", "1A", id)
			if err == nil {
				mu.Lock()
				successfulHolds++
				mu.Unlock()
			}
		}(custID)
	}

	wg.Wait()

	// Exactly 1 user must get the seat; 49 must be rejected cleanly
	if successfulHolds != 1 {
		t.Fatalf("critical race condition: expected 1 successful hold, got %d", successfulHolds)
	}
}

func TestBookingEngine_HoldAutoExpiration(t *testing.T) {
	// 500ms temporary hold
	engine := NewBookingEngine(500 * time.Millisecond)
	engine.RegisterFlight("ET-302", "ADD", "NBO", 1)

	// User 1 locks seat 1A
	_, err := engine.HoldSeat("ET-302", "1A", "USER_1")
	if err != nil {
		t.Fatalf("first hold should succeed: %v", err)
	}

	// User 2 tries immediately -> must fail
	_, err = engine.HoldSeat("ET-302", "1A", "USER_2")
	if err != ErrSeatLocked {
		t.Fatalf("expected ErrSeatLocked, got %v", err)
	}

	// Wait 700ms for expiration daemon to release the hold
	time.Sleep(700 * time.Millisecond)

	// User 2 tries again -> should now succeed because USER_1 abandoned it
	_, err = engine.HoldSeat("ET-302", "1A", "USER_2")
	if err != nil {
		t.Fatalf("seat should have been re-opened after expiration: %v", err)
	}
}
