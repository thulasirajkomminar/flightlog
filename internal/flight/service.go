package flight

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

// ErrNoFlightsFound is returned when no flights match.
var ErrNoFlightsFound = errors.New("no flights found")

const (
	logKeyFlightNumber = "flight_number"
	logKeyDate         = "date"

	// searchRefreshAge is how old a cached flight near its travel day may be
	// before a search re-fetches it from the provider.
	searchRefreshAge = 30 * time.Minute
	// A search only re-fetches flights dated within this window around today.
	searchRefreshDaysBefore = 3
	searchRefreshDaysAhead  = 1
)

// CacheRepository manages cached flight data from providers.
type CacheRepository interface {
	Create(ctx context.Context, flight *domain.Flight) error
	Update(ctx context.Context, flight *domain.Flight) error
	RecordStatusCheck(ctx context.Context, flightID string, checks int) error
	FindRefreshDue(ctx context.Context, due *RefreshDue) ([]*domain.Flight, error)
	FindByRoute(ctx context.Context, flightNumber, flightDate, depIATA, arrIATA string) (*domain.Flight, error)
	FindByNumberAndDate(ctx context.Context, flightNumber, flightDate string) ([]*domain.Flight, error)
}

// UserFlightRepository manages user-to-flight associations.
type UserFlightRepository interface {
	Add(ctx context.Context, userID, flightID string) error
	Remove(ctx context.Context, userID, flightID string) error
	Get(ctx context.Context, userID, flightID string) (*domain.UserFlight, error)
	List(ctx context.Context, criteria *domain.FlightSearchCriteria) ([]*domain.Flight, error)
	ListAll(ctx context.Context, userID string) ([]*domain.Flight, error)
	Count(ctx context.Context, criteria *domain.FlightSearchCriteria) (int64, error)
	GetYears(ctx context.Context, userID string) ([]int, error)
	GetStats(ctx context.Context, userID string) (*domain.FlightStats, error)
	GetRoutes(ctx context.Context, userID string) ([]*domain.Route, error)
}

// Provider defines the contract for flight data providers.
type Provider interface {
	// SearchFlights returns the flights with the given IATA flight number
	// departing on date (YYYY-MM-DD).
	SearchFlights(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error)
	GetProviderName() string
}

// CacheBackfiller populates lookup caches from provider flight data.
type CacheBackfiller interface {
	BackfillFromFlight(ctx context.Context, flight *domain.Flight)
}

// Service implements the flight service.
type Service struct {
	cache      CacheRepository
	userRepo   UserFlightRepository
	provider   Provider
	backfiller CacheBackfiller
	coverage   LiveCoverage
	log        *logger.ComponentLogger
}

// NewService creates a Service.
func NewService(cache CacheRepository, userRepo UserFlightRepository, provider Provider, backfiller CacheBackfiller) *Service {
	return &Service{
		cache:      cache,
		userRepo:   userRepo,
		provider:   provider,
		backfiller: backfiller,
		log:        logger.NewComponentLogger("flight_service"),
	}
}

// SearchFlights checks the flight cache first for the flight_number+date combo.
// If not found, or if a cached flight may have changed status since it was
// cached, fetches from the provider and updates the cache.
func (s *Service) SearchFlights(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error) {
	now := time.Now().UTC()

	if date == "" {
		date = now.Format(time.DateOnly)
	}

	flights, err := s.cache.FindByNumberAndDate(ctx, flightNumber, date)
	if err == nil && len(flights) > 0 {
		return s.searchCached(ctx, flights, flightNumber, date, now), nil
	}

	s.log.Debug("cache miss, fetching from provider",
		zap.String(logKeyFlightNumber, flightNumber),
		zap.String(logKeyDate, date),
	)

	return s.fetchFromProvider(ctx, flightNumber, date)
}

// ListFlights returns a user's flights matching criteria.
func (s *Service) ListFlights(ctx context.Context, criteria *domain.FlightSearchCriteria) ([]*domain.Flight, int64, error) {
	total, err := s.userRepo.Count(ctx, criteria)
	if err != nil {
		return nil, 0, fmt.Errorf("count: %w", err)
	}

	flights, err := s.userRepo.List(ctx, criteria)
	if err != nil {
		return nil, 0, fmt.Errorf("list: %w", err)
	}

	formatFlightNumbers(flights)

	return flights, total, nil
}

// GetYears returns the distinct years from a user's flights.
func (s *Service) GetYears(ctx context.Context, userID string) ([]int, error) {
	years, err := s.userRepo.GetYears(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("get years for user %s: %w", userID, err)
	}

	return years, nil
}

// AddFlight links an existing cached flight to a user.
func (s *Service) AddFlight(ctx context.Context, userID, flightID string) error {
	err := s.userRepo.Add(ctx, userID, flightID)
	if err != nil {
		return fmt.Errorf("add flight %s for user %s: %w", flightID, userID, err)
	}

	s.log.Info("flight added to collection",
		zap.String("user_id", userID),
		zap.String("flight_id", flightID),
	)

	return nil
}

// GetFlight retrieves a flight linked to a user.
func (s *Service) GetFlight(ctx context.Context, userID, flightID string) (*domain.Flight, error) {
	uf, err := s.userRepo.Get(ctx, userID, flightID)
	if err != nil {
		return nil, fmt.Errorf("get flight %s for user %s: %w", flightID, userID, err)
	}

	uf.Flight.Number = domain.FormatFlightNumber(uf.Flight.Number)

	return &uf.Flight, nil
}

// DeleteFlight removes a flight from a user's collection.
func (s *Service) DeleteFlight(ctx context.Context, userID, flightID string) error {
	err := s.userRepo.Remove(ctx, userID, flightID)
	if err != nil {
		return fmt.Errorf("remove flight %s for user %s: %w", flightID, userID, err)
	}

	s.log.Info("flight removed from collection",
		zap.String("user_id", userID),
		zap.String("flight_id", flightID),
	)

	return nil
}

// ExportFlights returns all flights for a user without pagination.
func (s *Service) ExportFlights(ctx context.Context, userID string) ([]*domain.Flight, error) {
	flights, err := s.userRepo.ListAll(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("export flights for user %s: %w", userID, err)
	}

	formatFlightNumbers(flights)

	return flights, nil
}

// GetStats returns aggregated flight statistics for a user.
func (s *Service) GetStats(ctx context.Context, userID string) (*domain.FlightStats, error) {
	stats, err := s.userRepo.GetStats(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("stats for user %s: %w", userID, err)
	}

	return stats, nil
}

// GetRoutes returns the distinct routes a user has flown.
func (s *Service) GetRoutes(ctx context.Context, userID string) ([]*domain.Route, error) {
	routes, err := s.userRepo.GetRoutes(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("routes for user %s: %w", userID, err)
	}

	return routes, nil
}

// searchCached returns cached flights, refreshing them from the provider first
// when their status may be out of date. A failed refresh falls back to the cache.
func (s *Service) searchCached(ctx context.Context, cached []*domain.Flight, flightNumber, date string, now time.Time) []*domain.Flight {
	if anyStale(cached, now) {
		fresh, err := s.fetchFromProvider(ctx, flightNumber, date)
		if err == nil {
			return fresh
		}
	}

	s.log.Debug("cache hit",
		zap.String(logKeyFlightNumber, flightNumber),
		zap.String(logKeyDate, date),
		zap.Int("count", len(cached)),
	)
	formatFlightNumbers(cached)

	return cached
}

// anyStale reports whether a search should re-fetch cached flights. That is
// only worth a provider call around the day of travel, when gates, times and
// status actually change: a flight weeks away, or long past, is served from
// the cache however old the entry is.
func anyStale(flights []*domain.Flight, now time.Time) bool {
	from := now.AddDate(0, 0, -searchRefreshDaysBefore).Format(time.DateOnly)
	to := now.AddDate(0, 0, searchRefreshDaysAhead).Format(time.DateOnly)

	for _, f := range flights {
		changing := !f.Status.IsFinal() && f.FlightDate >= from && f.FlightDate <= to
		if changing && now.Sub(f.UpdatedAt) > searchRefreshAge {
			return true
		}
	}

	return false
}

func (s *Service) fetchFromProvider(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error) {
	providerFlights, err := s.provider.SearchFlights(ctx, flightNumber, date)
	if err != nil {
		s.log.Error("provider search failed", err,
			zap.String(logKeyFlightNumber, flightNumber),
			zap.String(logKeyDate, date),
		)

		return nil, fmt.Errorf("provider search failed: %w", err)
	}

	if len(providerFlights) == 0 {
		return nil, fmt.Errorf("%w for %s on %s", ErrNoFlightsFound, flightNumber, date)
	}

	s.log.Debug("provider returned flights",
		zap.String(logKeyFlightNumber, flightNumber),
		zap.String(logKeyDate, date),
		zap.Int("count", len(providerFlights)),
	)

	result := make([]*domain.Flight, 0, len(providerFlights))

	for _, flight := range providerFlights {
		flight.Number = domain.NormalizeFlightNumber(flight.Number)

		err := s.storeProviderFlight(ctx, flight)
		if err != nil {
			return nil, err
		}

		result = append(result, flight)
	}

	formatFlightNumbers(result)

	return result, nil
}

// storeProviderFlight caches a provider flight, overwriting the existing cache
// entry for the same flight and route so its ID (and user links) are kept.
func (s *Service) storeProviderFlight(ctx context.Context, flight *domain.Flight) error {
	cached, err := s.cache.FindByRoute(ctx, flight.Number, flight.FlightDate,
		flight.Departure.Airport.IATA, flight.Arrival.Airport.IATA)
	if err == nil {
		flight.ID = cached.ID
		flight.CreatedAt = cached.CreatedAt
		flight.StatusChecks = cached.StatusChecks

		err = s.cache.Update(ctx, flight)
		if err != nil {
			return fmt.Errorf("update cached flight %s: %w", flight.Number, err)
		}

		return nil
	}

	err = s.cache.Create(ctx, flight)
	if err != nil {
		return fmt.Errorf("cache flight %s: %w", flight.Number, err)
	}

	s.backfiller.BackfillFromFlight(ctx, flight)

	return nil
}

func formatFlightNumbers(flights []*domain.Flight) {
	for _, f := range flights {
		f.Number = domain.FormatFlightNumber(f.Number)
	}
}
