# Skyward ✈️

A high-concurrency flight search aggregator and atomic seat reservation engine built with Go and Gin, featuring a real-time web dashboard.

## System Architecture

- **Parallel Fare Aggregation:** Queries multiple upstream airline providers concurrently using Goroutines and `sync.WaitGroup`. Bounded by `context.WithTimeout` to guarantee low-latency search results under 150ms without failing if an external provider lags.
- **Atomic Reservation Engine:** Prevents double-booking race conditions during high-demand flight sales using thread-safe `sync.Mutex` state locks.
- **Background Expiration Sweeper:** A background daemon goroutine continually sweeps abandoned holds, automatically returning inventory to the market after 30 seconds.
- **Self-Contained Full-Stack UI:** Single-binary deployment where Go directly serves the real-time seat map and fare aggregator web dashboard via Tailwind CSS.

## API Endpoints

- `GET /api/v1/flights/search?origin=ADD&dest=DXB` - Parallel airline aggregation.
- `GET /api/v1/flights/:flight_id/seats` - Real-time seat inventory state.
- `POST /api/v1/seats/hold` - Atomic temporary reservation with expiration.
- `POST /api/v1/seats/confirm` - Finalizes ticket purchase and issues confirmation.

## Quickstart

```bash
# Run tests
go test -v ./...

# Start server & dashboard
go run cmd/server/main.go
# Visit http://localhost:8083