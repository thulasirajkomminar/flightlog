package flight

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// The refresher exists to turn "scheduled" into "landed" for flights that were
// added before they flew. Every check costs provider quota, so it is frugal:
// it waits until a flight should have arrived, checks at most MaxStatusChecks
// times with growing gaps, and skips flights the provider cannot track live.
const (
	// refreshInterval is how often the refresher looks for flights that are due.
	refreshInterval = 30 * time.Minute
	// arrivalGrace is how long after the scheduled arrival the first check waits,
	// so the provider has had time to record the landing.
	arrivalGrace = 30 * time.Minute
	// secondCheckAfter and thirdCheckAfter are the waits before the later checks.
	secondCheckAfter = 6 * time.Hour
	thirdCheckAfter  = 24 * time.Hour
	// refreshWindowDays is how long after its date a flight is still checked.
	refreshWindowDays = 3
	// refreshBatchSize caps provider calls per run.
	refreshBatchSize = 20
	// refreshThrottle spaces out provider calls within one run.
	refreshThrottle = 1 * time.Second
)

// MaxStatusChecks is how many times the refresher asks the provider about one
// flight before accepting that its status will never be confirmed.
const MaxStatusChecks = 3

// RefreshDue describes which flights are due for a status check at Now.
type RefreshDue struct {
	Now time.Time
	// FromDate and ToDate bound the flight date (YYYY-MM-DD).
	FromDate, ToDate string
	// ArrivedBefore is the latest scheduled arrival that counts as arrived.
	ArrivedBefore time.Time
	// SecondCheckBefore and ThirdCheckBefore are the latest last-check times
	// at which the second and third checks may run.
	SecondCheckBefore, ThirdCheckBefore time.Time
	Limit                               int
}

func newRefreshDue(now time.Time) *RefreshDue {
	return &RefreshDue{
		Now:               now,
		FromDate:          now.AddDate(0, 0, -refreshWindowDays).Format(time.DateOnly),
		ToDate:            now.Format(time.DateOnly),
		ArrivedBefore:     now.Add(-arrivalGrace),
		SecondCheckBefore: now.Add(-secondCheckAfter),
		ThirdCheckBefore:  now.Add(-thirdCheckAfter),
		Limit:             refreshBatchSize,
	}
}

// LiveCoverage tells which airports the provider tracks live. Outside them a
// flight's status is never updated, so checking it again is wasted quota.
type LiveCoverage interface {
	// LiveAirports returns the ICAO codes of live-tracked airports.
	LiveAirports(ctx context.Context) (map[string]struct{}, error)
}

// WithLiveCoverage makes the refresher skip flights between airports the
// provider does not track live.
func (s *Service) WithLiveCoverage(coverage LiveCoverage) *Service {
	s.coverage = coverage

	return s
}

// WithStatusRefresh turns background status checks on or off. They are on by
// default; turning them off means no API call is ever made unprompted.
func (s *Service) WithStatusRefresh(enabled bool) *Service {
	s.refreshOff = !enabled

	return s
}

// RunRefresher periodically updates the status of flights in users'
// collections that were added before they landed. It blocks until ctx is done.
func (s *Service) RunRefresher(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval)
	defer ticker.Stop()

	for {
		now := time.Now().UTC()
		s.pruneMisses(ctx, now)
		s.RefreshDue(ctx, now)

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// pruneMisses drops remembered "no such flight" answers that are too old to
// be used any more, so the table does not grow without bound.
func (s *Service) pruneMisses(ctx context.Context, now time.Time) {
	_, err := s.cache.PruneMisses(ctx, now.Add(-pastMissTTL))
	if err != nil {
		s.log.Warn("failed to prune remembered misses", zap.Error(err))
	}
}

// RefreshDue re-fetches flights that are due for a status check and returns
// how many were updated with provider data.
func (s *Service) RefreshDue(ctx context.Context, now time.Time) int {
	if s.refreshOff {
		return 0
	}

	// Keep what is left of a low quota for searches people start themselves.
	if !s.provider.BackgroundAllowed() {
		s.log.Debug("skipping status refresh: provider quota is low or exhausted")

		return 0
	}

	due, err := s.cache.FindRefreshDue(ctx, newRefreshDue(now))
	if err != nil {
		s.log.Error("failed to find flights due for refresh", err)

		return 0
	}

	result := s.refreshBatch(ctx, due)

	if len(due) > 0 {
		s.log.Info("flight status refresh completed",
			zap.Int("due", len(due)), zap.Int("provider_calls", result.calls), zap.Int("updated", result.updated))
	}

	return result.updated
}

// batchResult counts what one refresher run did.
type batchResult struct {
	updated int // flights updated with provider data
	calls   int // provider calls spent
}

// refreshBatch checks each due flight.
func (s *Service) refreshBatch(ctx context.Context, due []*domain.Flight) batchResult {
	var result batchResult

	for _, f := range due {
		if !s.trackedLive(ctx, f) {
			// No call spent: the provider will never report this flight's landing.
			s.recordCheck(ctx, f.ID, MaxStatusChecks)

			continue
		}

		if result.calls > 0 && !sleep(ctx, refreshThrottle) {
			break
		}

		result.calls++

		if s.refreshFlight(ctx, f) {
			result.updated++
		}
	}

	return result
}

// trackedLive reports whether the provider can know this flight's live status.
// When coverage is unknown the flight is assumed to be tracked.
func (s *Service) trackedLive(ctx context.Context, f *domain.Flight) bool {
	dep, arr := f.Departure.Airport.ICAO, f.Arrival.Airport.ICAO
	if s.coverage == nil || dep == "" || arr == "" {
		return true
	}

	live, err := s.coverage.LiveAirports(ctx)
	if err != nil || len(live) == 0 {
		return true
	}

	_, depLive := live[dep]
	_, arrLive := live[arr]

	return depLive || arrLive
}

// refreshFlight re-fetches one cached flight and counts the check. It reports
// whether the flight was updated with provider data.
func (s *Service) refreshFlight(ctx context.Context, cached *domain.Flight) bool {
	fresh := s.fetchSameRoute(ctx, cached)
	if fresh == nil {
		s.recordCheck(ctx, cached.ID, cached.StatusChecks+1)

		return false
	}

	fresh.Number = domain.NormalizeFlightNumber(fresh.Number)
	fresh.ID = cached.ID
	fresh.CreatedAt = cached.CreatedAt
	fresh.StatusChecks = cached.StatusChecks + 1

	err := s.cache.Update(ctx, fresh)
	if err != nil {
		s.log.Error("failed to update refreshed flight", err, zap.String("flight_id", cached.ID))

		return false
	}

	return true
}

func (s *Service) recordCheck(ctx context.Context, flightID string, checks int) {
	err := s.cache.RecordStatusCheck(ctx, flightID, checks)
	if err != nil {
		s.log.Error("failed to record status check", err, zap.String("flight_id", flightID))
	}
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
