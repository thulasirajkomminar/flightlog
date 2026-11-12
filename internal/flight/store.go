package flight

import (
	"context"
	"database/sql"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

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

// Update overwrites a cached flight with fresh provider data.
func (s *CacheStore) Update(ctx context.Context, flight *domain.Flight) error {
	return s.db.WithContext(ctx).Save(flight).Error
}

// RecordStatusCheck stores how many status checks a flight has had and when
// the last one was, without changing its data.
func (s *CacheStore) RecordStatusCheck(ctx context.Context, flightID string, checks int) error {
	return s.db.WithContext(ctx).Model(&domain.Flight{}).
		Where("id = ?", flightID).
		Updates(map[string]any{"status_checks": checks, "updated_at": time.Now()}).Error
}

// FindRefreshDue returns flights in a user's collection that should have
// arrived, still have a non-final status, and are due for their next check.
func (s *CacheStore) FindRefreshDue(ctx context.Context, due *RefreshDue) ([]*domain.Flight, error) {
	var flights []*domain.Flight

	err := s.db.WithContext(ctx).
		Where("status IN ?", []domain.FlightStatus{domain.FlightStatusScheduled, domain.FlightStatusActive}).
		Where("flight_date BETWEEN ? AND ?", due.FromDate, due.ToDate).
		Where("id IN (?)", s.db.Table("user_flights").Select("flight_id")).
		// Should have arrived: by its scheduled arrival, or by date when no time is known.
		Where("(arr_scheduled_utc IS NOT NULL AND arr_scheduled_utc < ?) OR (arr_scheduled_utc IS NULL AND flight_date < ?)",
			due.ArrivedBefore, due.ToDate).
		// First check at once, later checks after growing waits, then give up.
		Where("status_checks = 0 OR (status_checks = 1 AND updated_at < ?) OR (status_checks = 2 AND updated_at < ?)",
			due.SecondCheckBefore, due.ThirdCheckBefore).
		Order("updated_at").
		Limit(due.Limit).
		Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// FindMiss returns when the provider last had no flight for a number and
// date, and whether there is such a record.
func (s *CacheStore) FindMiss(ctx context.Context, flightNumber, flightDate string) (time.Time, bool) {
	var miss domain.FlightMiss

	err := s.db.WithContext(ctx).
		Where("flight_number = ? AND flight_date = ?", flightNumber, flightDate).
		First(&miss).Error
	if err != nil {
		return time.Time{}, false
	}

	return miss.CheckedAt, true
}

// SaveMiss records that the provider had no flight for a number and date.
func (s *CacheStore) SaveMiss(ctx context.Context, flightNumber, flightDate string, checkedAt time.Time) error {
	miss := domain.FlightMiss{FlightNumber: flightNumber, FlightDate: flightDate, CheckedAt: checkedAt}

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(&miss).Error
}

// PruneMisses forgets "no such flight" answers checked before the given time
// and returns how many were removed.
func (s *CacheStore) PruneMisses(ctx context.Context, before time.Time) (int64, error) {
	result := s.db.WithContext(ctx).Where("checked_at < ?", before).Delete(&domain.FlightMiss{})

	return result.RowsAffected, result.Error
}

// visibleTo limits a query to flights a user may be matched with: provider
// data, which is shared, and flights that user imported or added themselves.
// An empty ownerID matches shared flights only.
func visibleTo(db *gorm.DB, ownerID string) *gorm.DB {
	return db.Where("imported_by = '' OR imported_by = ?", ownerID)
}

// FindByRoute retrieves a flight visible to ownerID by number, date, and
// route, preferring shared provider data over the owner's own entry.
func (s *CacheStore) FindByRoute(ctx context.Context, ownerID, flightNumber, flightDate, depIATA, arrIATA string) (*domain.Flight, error) {
	var flight domain.Flight

	err := visibleTo(s.db.WithContext(ctx), ownerID).
		Where("flight_number = ? AND flight_date = ? AND dep_airport_iata = ? AND arr_airport_iata = ?",
			flightNumber, flightDate, depIATA, arrIATA).
		Order("imported_by").
		First(&flight).Error
	if err != nil {
		return nil, err
	}

	return &flight, nil
}

// FindByDateAndRoute retrieves all flights visible to ownerID on a date and
// route, whatever their flight number.
func (s *CacheStore) FindByDateAndRoute(ctx context.Context, ownerID, flightDate, depIATA, arrIATA string) ([]*domain.Flight, error) {
	var flights []*domain.Flight

	err := visibleTo(s.db.WithContext(ctx), ownerID).
		Where("flight_date = ? AND dep_airport_iata = ? AND arr_airport_iata = ?",
			flightDate, depIATA, arrIATA).
		Order("imported_by").
		Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// FindByNumberAndDate retrieves flights visible to ownerID by number and date.
func (s *CacheStore) FindByNumberAndDate(ctx context.Context, ownerID, flightNumber, flightDate string) ([]*domain.Flight, error) {
	var flights []*domain.Flight

	err := visibleTo(s.db.WithContext(ctx), ownerID).
		Where("flight_number = ? AND flight_date = ?", flightNumber, flightDate).
		Find(&flights).Error
	if err != nil {
		return nil, err
	}

	return flights, nil
}

// newestFirst orders flights by date, then departure time. The ID tie-breaker
// keeps the order stable so pages never overlap or skip same-day flights.
const newestFirst = "flights.flight_date DESC, flights.dep_scheduled_utc DESC, flights.id DESC"

// UserFlightStore manages the user-flight join table.
type UserFlightStore struct {
	db *gorm.DB
}

// NewUserFlightStore returns a new store backed by db.
func NewUserFlightStore(db *gorm.DB) *UserFlightStore {
	return &UserFlightStore{db: db}
}

// Add links a flight to a user. It returns ErrFlightNotFound for an unknown
// flight and ErrFlightAlreadyAdded when the link already exists.
func (s *UserFlightStore) Add(ctx context.Context, userID, flightID string) error {
	var known int64

	err := s.db.WithContext(ctx).Model(&domain.Flight{}).Where("id = ?", flightID).Count(&known).Error
	if err != nil {
		return err
	}

	if known == 0 {
		return domain.ErrFlightNotFound
	}

	uf := domain.UserFlight{
		UserID:   userID,
		FlightID: flightID,
	}
	uf.GenerateID()

	// The unique index decides, so two concurrent requests cannot both insert.
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&uf)
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrFlightAlreadyAdded
	}

	return nil
}

// Remove unlinks a flight from a user. It returns ErrFlightNotFound when the
// flight is not in the user's collection.
func (s *UserFlightStore) Remove(ctx context.Context, userID, flightID string) error {
	result := s.db.WithContext(ctx).
		Where("user_id = ? AND flight_id = ?", userID, flightID).
		Delete(&domain.UserFlight{})
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return domain.ErrFlightNotFound
	}

	return nil
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
	query = query.Order(newestFirst)

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
		Order(newestFirst).
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

// flownSQL selects the flights (alias f) a user has actually flown: landed
// ones, plus past flights the provider never confirmed. Outside its live
// coverage the provider keeps reporting a flight as scheduled, so a
// not-cancelled flight dated before the cutoff is taken as flown. Cancelled,
// diverted and incident flights never count.
const flownSQL = `(f.status = @landed OR (f.status IN (@scheduled, @active) AND f.flight_date < @cutoff))`

// flownGraceDays is how long after its date an unconfirmed flight is still
// given the chance to be refreshed before it counts as flown.
const flownGraceDays = 1

// flownArgs returns the named arguments for a query using flownSQL and @user.
func flownArgs(userID string, now time.Time) []any {
	return []any{
		sql.Named("user", userID),
		sql.Named("landed", domain.FlightStatusLanded),
		sql.Named("scheduled", domain.FlightStatusScheduled),
		sql.Named("active", domain.FlightStatusActive),
		sql.Named("cutoff", now.AddDate(0, 0, -flownGraceDays).Format(time.DateOnly)),
	}
}

// GetStats computes flight statistics for a user over the flights they have
// flown (see flownSQL), matching what the map shows.
func (s *UserFlightStore) GetStats(ctx context.Context, userID string) (*domain.FlightStats, error) {
	var stats domain.FlightStats

	args := flownArgs(userID, time.Now().UTC())

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
		WHERE uf.user_id = @user AND `+flownSQL+`
	`, args...).Scan(&stats).Error
	if err != nil {
		return nil, err
	}

	// Count unique airports (union of departure and arrival)
	var airportCount int64

	err = s.db.WithContext(ctx).Raw(`
		SELECT COUNT(*) FROM (
			SELECT dep_airport_iata AS iata FROM flights f
			JOIN user_flights uf ON uf.flight_id = f.id WHERE uf.user_id = @user AND `+flownSQL+`
			UNION
			SELECT arr_airport_iata AS iata FROM flights f
			JOIN user_flights uf ON uf.flight_id = f.id WHERE uf.user_id = @user AND `+flownSQL+`
		)
	`, args...).Scan(&airportCount).Error
	if err != nil {
		return nil, err
	}

	stats.Airports = int(airportCount)

	return &stats, nil
}

// routeRow is the flat result of the routes query.
type routeRow struct {
	DepIATA string
	DepName string
	DepLat  float64
	DepLon  float64
	ArrIATA string
	ArrName string
	ArrLat  float64
	ArrLon  float64
	Flights int
}

// GetRoutes returns the distinct routes a user has flown (see flownSQL), with
// the number of flights on each. Flights without airport coordinates are left
// out because they cannot be drawn.
func (s *UserFlightStore) GetRoutes(ctx context.Context, userID string) ([]*domain.Route, error) {
	var rows []routeRow

	err := s.db.WithContext(ctx).Raw(`
		SELECT
			f.dep_airport_iata AS dep_iata, MAX(f.dep_airport_name) AS dep_name,
			MAX(f.dep_airport_lat) AS dep_lat, MAX(f.dep_airport_lon) AS dep_lon,
			f.arr_airport_iata AS arr_iata, MAX(f.arr_airport_name) AS arr_name,
			MAX(f.arr_airport_lat) AS arr_lat, MAX(f.arr_airport_lon) AS arr_lon,
			COUNT(*) AS flights
		FROM flights f
		JOIN user_flights uf ON uf.flight_id = f.id
		WHERE uf.user_id = @user AND `+flownSQL+`
			AND NOT (f.dep_airport_lat = 0 AND f.dep_airport_lon = 0)
			AND NOT (f.arr_airport_lat = 0 AND f.arr_airport_lon = 0)
		GROUP BY f.dep_airport_iata, f.arr_airport_iata
		ORDER BY f.dep_airport_iata, f.arr_airport_iata
	`, flownArgs(userID, time.Now().UTC())...).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	routes := make([]*domain.Route, 0, len(rows))
	for i := range rows {
		routes = append(routes, rows[i].toDomain())
	}

	return routes, nil
}

func (r *routeRow) toDomain() *domain.Route {
	return &domain.Route{
		Departure: domain.RouteAirport{IATA: r.DepIATA, Name: r.DepName, Location: domain.Location{Lat: r.DepLat, Lon: r.DepLon}},
		Arrival:   domain.RouteAirport{IATA: r.ArrIATA, Name: r.ArrName, Location: domain.Location{Lat: r.ArrLat, Lon: r.ArrLon}},
		Flights:   r.Flights,
	}
}

func applyFilters(query *gorm.DB, criteria *domain.FlightSearchCriteria) *gorm.DB {
	if criteria.Year > 0 {
		query = query.Where("CAST(strftime('%Y', flight_date) AS INTEGER) = ?", criteria.Year)
	}

	if criteria.Status != "" {
		query = query.Where("flights.status = ?", criteria.Status)
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
