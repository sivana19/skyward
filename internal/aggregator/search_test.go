package aggregator

import (
	"testing"
	"time"
)

func TestAggregator_ConcurrentSearchSpeed(t *testing.T) {
	p1 := NewMockProvider("Ethiopian Airlines", 450.0, 80*time.Millisecond, 120*time.Millisecond)
	p2 := NewMockProvider("Emirates", 520.0, 90*time.Millisecond, 110*time.Millisecond)
	p3 := NewMockProvider("Flydubai", 380.0, 70*time.Millisecond, 100*time.Millisecond)

	agg := NewAggregator(500*time.Millisecond, p1, p2, p3)

	start := time.Now()
	offers := agg.SearchAll("ADD", "DXB")
	elapsed := time.Since(start)

	if len(offers) != 3 {
		t.Fatalf("expected 3 airline offers, got %d", len(offers))
	}

	if elapsed > 250*time.Millisecond {
		t.Errorf("search took too long (%v), goroutines might not be executing concurrently", elapsed)
	}
}

func TestAggregator_SlowProviderTimeout(t *testing.T) {
	fastProv := NewMockProvider("FastAir", 300.0, 40*time.Millisecond, 60*time.Millisecond)
	slowProv := NewMockProvider("LagAir", 250.0, 600*time.Millisecond, 700*time.Millisecond)

	agg := NewAggregator(150*time.Millisecond, fastProv, slowProv)

	offers := agg.SearchAll("ADD", "NBO")

	if len(offers) != 1 {
		t.Fatalf("expected exactly 1 offer from fast provider, got %d", len(offers))
	}
	if offers[0].Provider != "FastAir" {
		t.Errorf("expected FastAir offer, got %s", offers[0].Provider)
	}
}
