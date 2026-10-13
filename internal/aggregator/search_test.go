package aggregator

import (
	"context"
	"testing"
	"time"
)

func TestAggregator_ConcurrentSearchSpeed(t *testing.T) {
	p1 := NewMockProvider("AirlineA", 300.0, 80*time.Millisecond, 80*time.Millisecond)
	p2 := NewMockProvider("AirlineB", 400.0, 90*time.Millisecond, 90*time.Millisecond)

	agg := NewAggregator(500*time.Millisecond, p1, p2)

	start := time.Now()
	offers := agg.SearchAll(context.Background(), "ADD", "DXB")
	duration := time.Since(start)

	if len(offers) != 2 {
		t.Fatalf("expected 2 offers, got %d", len(offers))
	}

	// Because searches run in parallel, total duration must stay around ~100ms, not sequential ~170ms+
	if duration > 150*time.Millisecond {
		t.Fatalf("concurrent search took too long: %v", duration)
	}
}

func TestAggregator_SlowProviderTimeout(t *testing.T) {
	fastProvider := NewMockProvider("FastAir", 250.0, 50*time.Millisecond, 50*time.Millisecond)
	slowProvider := NewMockProvider("LaggyAir", 500.0, 500*time.Millisecond, 500*time.Millisecond)

	// Set timeout strictly at 120ms so slowProvider gets dropped cleanly
	agg := NewAggregator(120*time.Millisecond, fastProvider, slowProvider)

	offers := agg.SearchAll(context.Background(), "ADD", "DXB")

	if len(offers) != 1 {
		t.Fatalf("expected 1 offer (fast only), got %d", len(offers))
	}

	if offers[0].Provider != "FastAir" {
		t.Fatalf("expected FastAir, got %s", offers[0].Provider)
	}
}

func TestAggregator_ContextCancellation(t *testing.T) {
	p := NewMockProvider("SlowAirline", 300.0, 200*time.Millisecond, 200*time.Millisecond)
	agg := NewAggregator(500*time.Millisecond, p)

	// Simulating immediate client abort / cancelled request
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	offers := agg.SearchAll(ctx, "ADD", "DXB")
	duration := time.Since(start)

	if len(offers) != 0 {
		t.Fatalf("expected 0 offers on cancelled context, got %d", len(offers))
	}

	// Should abort immediately without waiting for the 200ms delay
	if duration >= 100*time.Millisecond {
		t.Fatalf("expected instant abort, took %v", duration)
	}
}
