package flight

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	// refreshInterval is how often the background refresher looks for flights
	// whose status may have changed.
	refreshInterval = 30 * time.Minute
	// refreshRetryAge is the minimum time between two status checks of one flight.
	refreshRetryAge = 6 * time.Hour
	// refreshWindowDays is how long after its date a flight is still checked.
	// Flights that never reach a final status are given up on after this.
	refreshWindowDays = 3
	// refreshBatchSize caps provider calls per run to protect the API quota.
	refreshBatchSize = 20
	// refreshThrottle spaces out provider calls within one run.
	refreshThrottle = 1 * time.Second
)

// RunRefresher periodically updates the status of flights in users'
// collections that were added before they landed. It blocks until ctx is done.
func (s *Service) RunRefresher(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		s.RefreshDue(ctx, time.Now().UTC())

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// RefreshDue re-fetches flights that are due for a status check and returns
// how many were updated.
func (s *Service) RefreshDue(ctx context.Context, now time.Time) int {
	from := now.AddDate(0, 0, -refreshWindowDays).Format(time.DateOnly)
	to := now.Format(time.DateOnly)

	due, err := s.cache.FindRefreshDue(ctx, from, to, now.Add(-refreshRetryAge), refreshBatchSize)
	if err != nil {
		s.log.Error("failed to find flights due for refresh", err)

		return 0
	}

	updated := 0

	for i, f := range due {
		if i > 0 && !sleep(ctx, refreshThrottle) {
			break
		}

		if s.refreshFlight(ctx, f) {
			updated++
		}
	}

	if len(due) > 0 {
		s.log.Info("flight status refresh completed", zap.Int("checked", len(due)), zap.Int("updated", updated))
	}

	return updated
}

// refreshFlight re-fetches one cached flight. When the provider has no usable
// answer the flight is only marked as checked, so it is retried later.
func (s *Service) refreshFlight(ctx context.Context, cached *domain.Flight) bool {
	fresh := s.fetchSameRoute(ctx, cached)
	if fresh == nil {
		err := s.cache.Touch(ctx, cached.ID)
		if err != nil {
			s.log.Error("failed to mark flight as checked", err, zap.String("flight_id", cached.ID))
		}

		return false
	}

	fresh.Number = domain.NormalizeFlightNumber(fresh.Number)
	fresh.ID = cached.ID
	fresh.CreatedAt = cached.CreatedAt

	err := s.cache.Update(ctx, fresh)
	if err != nil {
		s.log.Error("failed to update refreshed flight", err, zap.String("flight_id", cached.ID))

		return false
	}

	return true
}

func (s *Service) fetchSameRoute(ctx context.Context, cached *domain.Flight) *domain.Flight {
	flights, err := s.provider.SearchFlights(ctx, domain.NormalizeFlightNumber(cached.Number), cached.FlightDate)
	if err != nil {
		s.log.Warn("flight refresh failed", zap.String("flight_id", cached.ID), zap.Error(err))

		return nil
	}

	for _, f := range flights {
		if f.Departure.Airport.IATA == cached.Departure.Airport.IATA &&
			f.Arrival.Airport.IATA == cached.Arrival.Airport.IATA {
			return f
		}
	}

	return nil
}

// sleep waits for d and reports false if ctx ended first.
func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
