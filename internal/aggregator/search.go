package aggregator

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// FlightOffer represents a flight deal found by a provider
type FlightOffer struct {
	Provider    string  `json:"provider"`
	FlightNo    string  `json:"flight_no"`
	Origin      string  `json:"origin"`
	Destination string  `json:"destination"`
	PriceUSD    float64 `json:"price_usd"`
	Duration    string  `json:"duration"` // Human-readable e.g. "4h 0m"
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
	latencyRange := int64(m.maxDelay - m.minDelay)
	var sleepTime time.Duration
	if latencyRange > 0 {
		sleepTime = m.minDelay + time.Duration(rand.Int63n(latencyRange))
	} else {
		sleepTime = m.minDelay
	}

	select {
	case <-time.After(sleepTime):
		return []FlightOffer{
			{
				Provider:    m.name,
				FlightNo:    m.name[:2] + "-101",
				Origin:      origin,
				Destination: dest,
				PriceUSD:    m.basePrice + float64(rand.Intn(40)),
				Duration:    "4h 0m",
			},
		}, nil
	case <-ctx.Done():
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
	ctx, cancel := context.WithTimeout(ctx, a.timeout)
	defer cancel()

	var wg sync.WaitGroup
	var mu sync.Mutex
	allOffers := make([]FlightOffer, 0)

	for _, p := range a.providers {
		wg.Add(1)
		go func(provider Provider) {
			defer wg.Done()

			offers, err := provider.FetchOffers(ctx, origin, dest)
			if err == nil {
				mu.Lock()
				allOffers = append(allOffers, offers...)
				mu.Unlock()
			}
		}(p)
	}

	wg.Wait()

	return allOffers
}
