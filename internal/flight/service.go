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
)

// CacheRepository manages cached flight data from providers.
type CacheRepository interface {
	Create(ctx context.Context, flight *domain.Flight) error
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
}

// Provider defines the contract for flight data providers.
type Provider interface {
	SearchFlights(ctx context.Context, criteria map[string]string) ([]*domain.Flight, error)
	GetProviderName() string
}

// Service implements the flight service.
type Service struct {
	cache    CacheRepository
	userRepo UserFlightRepository
	provider Provider
	log      *logger.ComponentLogger
}

// NewService creates a Service.
func NewService(cache CacheRepository, userRepo UserFlightRepository, provider Provider) *Service {
	return &Service{
		cache:    cache,
		userRepo: userRepo,
		provider: provider,
		log:      logger.NewComponentLogger("flight_service"),
	}
}

// SearchFlights checks the flight cache first for the flight_number+date combo.
// If not found, fetches from the provider and caches them.
func (s *Service) SearchFlights(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error) {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}

	flights, err := s.cache.FindByNumberAndDate(ctx, flightNumber, date)
	if err == nil && len(flights) > 0 {
		s.log.Debug("cache hit",
			zap.String(logKeyFlightNumber, flightNumber),
			zap.String(logKeyDate, date),
			zap.Int("count", len(flights)),
		)
		formatFlightNumbers(flights)

		return flights, nil
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

func (s *Service) fetchFromProvider(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error) {
	criteria := map[string]string{
		"flight_iata": flightNumber,
		"flight_date": date,
	}

	providerFlights, err := s.provider.SearchFlights(ctx, criteria)
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

		cached, err := s.cache.FindByRoute(ctx, flight.Number, flight.FlightDate,
			flight.Departure.Airport.IATA, flight.Arrival.Airport.IATA)
		if err == nil {
			result = append(result, cached)

			continue
		}

		err = s.cache.Create(ctx, flight)
		if err != nil {
			return nil, fmt.Errorf("cache flight %s: %w", flight.Number, err)
		}

		result = append(result, flight)
	}

	formatFlightNumbers(result)

	return result, nil
}

func formatFlightNumbers(flights []*domain.Flight) {
	for _, f := range flights {
		f.Number = domain.FormatFlightNumber(f.Number)
	}
}
