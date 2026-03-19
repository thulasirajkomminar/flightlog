package flight

import (
	"context"
	"errors"

	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// CacheStore persists provider responses in SQLite for deduplication.
type CacheStore struct {
	db *gorm.DB
}

// NewCacheStore returns a new store backed by db.
func NewCacheStore(db *gorm.DB) *CacheStore {
	return &CacheStore{db: db}
}

// Create inserts a new flight into the cache.
func (s *CacheStore) Create(ctx context.Context, flight *domain.Flight) error {
	flight.GenerateID()

	return s.db.WithContext(ctx).Create(flight).Error
}

// FindByRoute retrieves a cached flight by number, date, and route.
func (s *CacheStore) FindByRoute(ctx context.Context, flightNumber, flightDate, depIATA, arrIATA string) (*domain.Flight, error) {
	var flight domain.Flight

	err := s.db.WithContext(ctx).
		Where("flight_number = ? AND flight_date = ? AND dep_airport_iata = ? AND arr_airport_iata = ?",
			flightNumber, flightDate, depIATA, arrIATA).
		First(&flight).Error
	if err != nil {
		return nil, err
	}

	return &flight, nil
}

// FindByNumberAndDate retrieves cached flights by number and date.
func (s *CacheStore) FindByNumberAndDate(ctx context.Context, flightNumber, flightDate string) ([]*domain.Flight, error) {
	var flights []*domain.Flight

	err := s.db.WithContext(ctx).
		Where("flight_number = ? AND flight_date = ?", flightNumber, flightDate).
		Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// UserFlightStore manages the user-flight join table.
type UserFlightStore struct {
	db *gorm.DB
}

// NewUserFlightStore returns a new store backed by db.
func NewUserFlightStore(db *gorm.DB) *UserFlightStore {
	return &UserFlightStore{db: db}
}

// Add links a flight to a user.
func (s *UserFlightStore) Add(ctx context.Context, userID, flightID string) error {
	var existing domain.UserFlight

	err := s.db.WithContext(ctx).
		Where("user_id = ? AND flight_id = ?", userID, flightID).
		First(&existing).Error
	if err == nil {
		return domain.ErrFlightAlreadyAdded
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	uf := domain.UserFlight{
		UserID:   userID,
		FlightID: flightID,
	}
	uf.GenerateID()

	return s.db.WithContext(ctx).Create(&uf).Error
}

// Remove unlinks a flight from a user.
func (s *UserFlightStore) Remove(ctx context.Context, userID, flightID string) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND flight_id = ?", userID, flightID).
		Delete(&domain.UserFlight{}).Error
}

// Get retrieves a user's flight with full flight data.
func (s *UserFlightStore) Get(ctx context.Context, userID, flightID string) (*domain.UserFlight, error) {
	var uf domain.UserFlight

	err := s.db.WithContext(ctx).
		Preload("Flight").
		Where("user_id = ? AND flight_id = ?", userID, flightID).
		First(&uf).Error
	if err != nil {
		return nil, err
	}

	return &uf, nil
}

// List returns a user's flights matching the criteria.
func (s *UserFlightStore) List(ctx context.Context, criteria *domain.FlightSearchCriteria) ([]*domain.Flight, error) {
	query := s.db.WithContext(ctx).Model(&domain.Flight{}).
		Joins("JOIN user_flights ON user_flights.flight_id = flights.id").
		Where("user_flights.user_id = ?", criteria.UserID)

	query = applyFilters(query, criteria)
	query = applyPagination(query, criteria)
	query = query.Order("flights.flight_date DESC")

	var flights []*domain.Flight

	err := query.Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// ListAll returns all flights for a user without pagination.
func (s *UserFlightStore) ListAll(ctx context.Context, userID string) ([]*domain.Flight, error) {
	var flights []*domain.Flight

	err := s.db.WithContext(ctx).Model(&domain.Flight{}).
		Joins("JOIN user_flights ON user_flights.flight_id = flights.id").
		Where("user_flights.user_id = ?", userID).
		Order("flights.flight_date DESC").
		Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// Count returns total flights matching the criteria.
func (s *UserFlightStore) Count(ctx context.Context, criteria *domain.FlightSearchCriteria) (int64, error) {
	var count int64

	query := s.db.WithContext(ctx).Model(&domain.Flight{}).
		Joins("JOIN user_flights ON user_flights.flight_id = flights.id").
		Where("user_flights.user_id = ?", criteria.UserID)

	query = applyFilters(query, criteria)

	err := query.Count(&count).Error
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetYears returns distinct flight years for a user.
func (s *UserFlightStore) GetYears(ctx context.Context, userID string) ([]int, error) {
	var years []int

	err := s.db.WithContext(ctx).Raw(`
		SELECT DISTINCT CAST(strftime('%Y', f.flight_date) AS INTEGER) AS year
		FROM flights f
		JOIN user_flights uf ON uf.flight_id = f.id
		WHERE uf.user_id = ?
		ORDER BY year DESC
	`, userID).Scan(&years).Error
	if err != nil {
		return nil, err
	}

	return years, nil
}

// GetStats computes flight statistics for a user.
func (s *UserFlightStore) GetStats(ctx context.Context, userID string) (*domain.FlightStats, error) {
	var stats domain.FlightStats

	err := s.db.WithContext(ctx).Raw(`
		SELECT
			COUNT(*) AS flights,
			COALESCE(SUM(f.gcd_km), 0) AS distance,
			COALESCE(SUM(
				CASE WHEN f.dep_scheduled_utc IS NOT NULL AND f.arr_scheduled_utc IS NOT NULL
				THEN (julianday(f.arr_scheduled_utc) - julianday(f.dep_scheduled_utc)) * 24
				ELSE 0 END
			), 0) AS flight_time,
			COUNT(DISTINCT f.airline_iata) AS airlines
		FROM flights f
		JOIN user_flights uf ON uf.flight_id = f.id
		WHERE uf.user_id = ?
	`, userID).Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	// Count unique airports (union of departure and arrival)
	var airportCount int64

	err = s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			SELECT dep_airport_iata AS iata FROM flights f
			JOIN user_flights uf ON uf.flight_id = f.id WHERE uf.user_id = ?
			UNION
			SELECT arr_airport_iata AS iata FROM flights f
			JOIN user_flights uf ON uf.flight_id = f.id WHERE uf.user_id = ?
		)
	`, userID, userID).Scan(&airportCount).Error
	if err != nil {
		return nil, err
	}

	stats.Airports = int(airportCount)

	return &stats, nil
}

func applyFilters(query *gorm.DB, criteria *domain.FlightSearchCriteria) *gorm.DB {
	if criteria.Year > 0 {
		query = query.Where("CAST(strftime('%Y', flight_date) AS INTEGER) = ?", criteria.Year)
	}

	return query
}

func applyPagination(query *gorm.DB, criteria *domain.FlightSearchCriteria) *gorm.DB {
	if criteria.Limit > 0 {
		query = query.Limit(criteria.Limit)
	}

	if criteria.Offset > 0 {
		query = query.Offset(criteria.Offset)
	}

	return query
}
