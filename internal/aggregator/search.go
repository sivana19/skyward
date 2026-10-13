package aggregator

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// FlightOffer represents a flight deal found by a provider
type FlightOffer struct {
	Provider    string        `json:"provider"`
	FlightNo    string        `json:"flight_no"`
	Origin      string        `json:"origin"`
	Destination string        `json:"destination"`
	PriceUSD    float64       `json:"price_usd"`
	Duration    time.Duration `json:"duration"`
}

// Provider defines the interface any airline or GDS API must satisfy
type Provider interface {
	Name() string
	FetchOffers(ctx context.Context, origin, dest string) ([]FlightOffer, error)
}

// MockProvider simulates an upstream airline API with realistic network latency
type MockProvider struct {
	name      string
	basePrice float64
	minDelay  time.Duration
	maxDelay  time.Duration
}

func NewMockProvider(name string, basePrice float64, minDelay, maxDelay time.Duration) *MockProvider {
	return &MockProvider{
		name:      name,
		basePrice: basePrice,
		minDelay:  minDelay,
		maxDelay:  maxDelay,
	}
}

func (m *MockProvider) Name() string {
	return m.name
}

func (m *MockProvider) FetchOffers(ctx context.Context, origin, dest string) ([]FlightOffer, error) {
	// Simulate variable upstream API network latency
	latencyRange := int64(m.maxDelay - m.minDelay)
	var sleepTime time.Duration
	if latencyRange > 0 {
		sleepTime = m.minDelay + time.Duration(rand.Int63n(latencyRange))
	} else {
		sleepTime = m.minDelay
	}

	select {
	case <-time.After(sleepTime):
		// Upstream returned successfully
		return []FlightOffer{
			{
				Provider:    m.name,
				FlightNo:    m.name[:2] + "-101",
				Origin:      origin,
				Destination: dest,
				PriceUSD:    m.basePrice + float64(rand.Intn(40)),
				Duration:    4 * time.Hour,
			},
		}, nil
	case <-ctx.Done():
		// Request timed out or cancelled before airline responded
		return nil, ctx.Err()
	}
}

// Aggregator coordinates concurrent searches across all registered providers
type Aggregator struct {
	providers []Provider
	timeout   time.Duration
}

func NewAggregator(timeout time.Duration, providers ...Provider) *Aggregator {
	return &Aggregator{
		providers: providers,
		timeout:   timeout,
	}
}

// SearchAll queries all airlines in parallel and merges the results
func (a *Aggregator) SearchAll(ctx context.Context, origin, dest string) []FlightOffer {
	// Apply aggregation timeout to incoming context
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	allOffers := make([]FlightOffer, 0)

	// Launch parallel worker for every airline provider
	for _, p := range a.providers {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()

			// Fetch directly passing ctx; provider handles cancellation & timeout
			offers, err := provider.FetchOffers(ctx, origin, dest)
			if err == nil {
				mu.Lock()
				allOffers = append(allOffers, offers...)
				mu.Unlock()
			}
		}(p)
	}

	// Wait until all airline requests either complete or time out
	wg.Wait()

	return allOffers
}
