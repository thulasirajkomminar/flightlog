package importer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

const (
	providerDaysLimit   = 365
	searchThrottleDelay = 1 * time.Second
)

var (
	// ErrUnknownSource is returned for an unregistered import source.
	ErrUnknownSource = errors.New("unknown import source")
	// ErrNoProviderFlights is returned when the provider has no data for a flight.
	ErrNoProviderFlights = errors.New("no flights found from provider")
	// ErrNoMatchingRoute is returned when provider results don't match the expected route.
	ErrNoMatchingRoute = errors.New("multiple flights found, none matching route")
)

// FlightSearcher finds flights by number and date.
type FlightSearcher interface {
	SearchFlights(ctx context.Context, flightNumber, date string) ([]*domain.Flight, error)
}

// FlightLinker links a flight to a user.
type FlightLinker interface {
	AddFlight(ctx context.Context, userID, flightID string) error
}

// FlightCacher reads and writes the flight cache.
type FlightCacher interface {
	Create(ctx context.Context, flight *domain.Flight) error
	FindByRoute(ctx context.Context, flightNumber, flightDate, depIATA, arrIATA string) (*domain.Flight, error)
	FindByDateAndRoute(ctx context.Context, flightDate, depIATA, arrIATA string) (*domain.Flight, error)
}

// AirportLookup resolves airport details and route distances.
type AirportLookup interface {
	GetAirportByIATA(ctx context.Context, iata string) (*domain.Airport, error)
	GetDistanceBetweenAirports(ctx context.Context, fromIATA, toIATA string) (*domain.GreatCircleDistance, error)
}

// ImportResult summarises an import run.
type ImportResult struct {
	Total    int           `json:"total"`
	Imported int           `json:"imported"`
	Skipped  int           `json:"skipped"`
	Failed   int           `json:"failed"`
	Errors   []ImportError `json:"errors,omitempty"`
}

// ImportError records one failed entry.
type ImportError struct {
	FlightNumber string `json:"flightNumber"`
	Date         string `json:"date"`
	Reason       string `json:"reason"`
}

// PreviewResult is returned by Preview.
type PreviewResult struct {
	Total      int `json:"total"`
	Enrichable int `json:"enrichable"`
}

// Service orchestrates file-based flight imports.
type Service struct {
	adapters map[string]Adapter
	searcher FlightSearcher
	linker   FlightLinker
	cacher   FlightCacher
	airports AirportLookup
	log      *logger.ComponentLogger
}

// NewService returns a new import service.
func NewService(
	searcher FlightSearcher,
	linker FlightLinker,
	cacher FlightCacher,
	airports AirportLookup,
) *Service {
	s := &Service{
		adapters: make(map[string]Adapter),
		searcher: searcher,
		linker:   linker,
		cacher:   cacher,
		airports: airports,
		log:      logger.NewComponentLogger("importer"),
	}

	s.RegisterAdapter(&FlightyAdapter{})

	return s
}

// RegisterAdapter adds a source adapter.
func (s *Service) RegisterAdapter(a Adapter) {
	s.adapters[a.Name()] = a
}

// Sources lists registered adapter names.
func (s *Service) Sources() []string {
	sources := make([]string, 0, len(s.adapters))
	for name := range s.adapters {
		sources = append(sources, name)
	}

	return sources
}

// Preview returns import stats without persisting anything.
func (s *Service) Preview(entries []ImportEntry) *PreviewResult {
	cutoff := time.Now().AddDate(0, 0, -providerDaysLimit).Format("2006-01-02")

	enrichable := 0

	for _, e := range entries {
		if e.Date >= cutoff {
			enrichable++
		}
	}

	return &PreviewResult{
		Total:      len(entries),
		Enrichable: enrichable,
	}
}

// Parse resolves the source adapter and parses entries from the reader.
func (s *Service) Parse(source string, r io.Reader) ([]ImportEntry, error) {
	adapter, ok := s.adapters[source]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownSource, source)
	}

	entries, err := adapter.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", source, err)
	}

	return entries, nil
}

type importOptions struct {
	userID string
	enrich bool
	cutoff string
}

// Import processes parsed entries: deduplicates, optionally enriches from the provider, and links to the user.
func (s *Service) Import(ctx context.Context, userID string, entries []ImportEntry, enrich bool) *ImportResult {
	result := &ImportResult{Total: len(entries)}

	opts := importOptions{
		userID: userID,
		enrich: enrich,
		cutoff: time.Now().AddDate(0, 0, -providerDaysLimit).Format("2006-01-02"),
	}

	for i := range entries {
		err := s.importEntry(ctx, &entries[i], opts)
		if err != nil {
			if errors.Is(err, domain.ErrFlightAlreadyAdded) {
				result.Skipped++
			} else {
				result.Failed++
				result.Errors = append(result.Errors, ImportError{
					FlightNumber: entries[i].FlightNumber,
					Date:         entries[i].Date,
					Reason:       err.Error(),
				})
			}

			continue
		}

		result.Imported++
	}

	s.log.Info("import completed",
		zap.Int("total", result.Total),
		zap.Int("imported", result.Imported),
		zap.Int("skipped", result.Skipped),
		zap.Int("failed", result.Failed),
	)

	return result
}

func (s *Service) importEntry(
	ctx context.Context,
	entry *ImportEntry,
	opts importOptions,
) error {
	if id, ok := s.findCached(ctx, entry); ok {
		return s.linkFlight(ctx, opts.userID, id)
	}

	if opts.enrich && entry.Date >= opts.cutoff {
		flightID, found := s.tryEnrich(ctx, entry)
		if found {
			return s.linkFlight(ctx, opts.userID, flightID)
		}
	}

	flight := s.createFromImport(ctx, entry)

	err := s.cacher.Create(ctx, flight)
	if err != nil {
		return fmt.Errorf("cache imported flight: %w", err)
	}

	return s.linkFlight(ctx, opts.userID, flight.ID)
}

func (s *Service) findCached(ctx context.Context, entry *ImportEntry) (string, bool) {
	cached, err := s.cacher.FindByRoute(ctx, entry.FlightNumber, entry.Date, entry.DepIATA, entry.ArrIATA)
	if err == nil && cached != nil {
		return cached.ID, true
	}

	// Fallback: match by date+route only (handles ICAO/IATA flight number differences).
	cached, err = s.cacher.FindByDateAndRoute(ctx, entry.Date, entry.DepIATA, entry.ArrIATA)
	if err == nil && cached != nil {
		return cached.ID, true
	}

	return "", false
}

func (s *Service) tryEnrich(
	ctx context.Context,
	entry *ImportEntry,
) (string, bool) {
	time.Sleep(searchThrottleDelay)

	flight, enrichErr := s.enrichFromProvider(ctx, entry)
	if enrichErr == nil && flight != nil {
		return flight.ID, true
	}

	s.log.Debug("provider enrichment failed, falling back to import data",
		zap.String("flight", entry.FlightNumber),
		zap.String("date", entry.Date),
		zap.Error(enrichErr),
	)

	return "", false
}

func (s *Service) enrichFromProvider(ctx context.Context, entry *ImportEntry) (*domain.Flight, error) {
	flights, err := s.searcher.SearchFlights(ctx, entry.FlightNumber, entry.Date)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	for _, f := range flights {
		if f.Departure.Airport.IATA == entry.DepIATA && f.Arrival.Airport.IATA == entry.ArrIATA {
			return f, nil
		}
	}

	if len(flights) == 1 {
		return flights[0], nil
	}

	if len(flights) == 0 {
		return nil, ErrNoProviderFlights
	}

	return nil, ErrNoMatchingRoute
}

func (s *Service) createFromImport(
	ctx context.Context,
	entry *ImportEntry,
) *domain.Flight {
	flight := &domain.Flight{
		Number:     entry.FlightNumber,
		FlightDate: entry.Date,
		Status:     domain.FlightStatusLanded,
		Provider:   "import",
		Airline: domain.FlightAirline{
			IATA: entry.Airline,
		},
	}
	flight.GenerateID()

	if entry.Aircraft != "" {
		flight.Aircraft.Model = entry.Aircraft
	}

	flight.Departure.Airport = s.lookupAirport(ctx, entry.DepIATA)
	flight.Arrival.Airport = s.lookupAirport(ctx, entry.ArrIATA)

	if entry.DepTime != nil {
		flight.Departure.ScheduledTime = domain.TimeInfo{UTC: entry.DepTime}
	}

	if entry.ArrTime != nil {
		flight.Arrival.ScheduledTime = domain.TimeInfo{UTC: entry.ArrTime}
	}

	dist, err := s.airports.GetDistanceBetweenAirports(ctx, entry.DepIATA, entry.ArrIATA)
	if err != nil {
		s.log.Warn("distance lookup failed",
			zap.String("route", entry.DepIATA+"-"+entry.ArrIATA),
			zap.Error(err),
		)
	} else {
		flight.GreatCircleDistance = *dist
	}

	return flight
}

func (s *Service) lookupAirport(ctx context.Context, iata string) domain.Airport {
	airport, err := s.airports.GetAirportByIATA(ctx, iata)
	if err != nil {
		s.log.Warn("airport lookup failed",
			zap.String("iata", iata),
			zap.Error(err),
		)

		return domain.Airport{IATA: iata}
	}

	return *airport
}

func (s *Service) linkFlight(ctx context.Context, userID, flightID string) error {
	err := s.linker.AddFlight(ctx, userID, flightID)
	if err != nil {
		return fmt.Errorf("link flight: %w", err)
	}

	return nil
}
