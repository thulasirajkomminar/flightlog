package flight

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	// manualProvider marks flights a user typed in.
	manualProvider = "manual"
	airportCodeLen = 3
	airlineCodeLen = 2
	maxTextLen     = 100
)

// ErrNotEditable is returned when a user tries to change a flight that came
// from the provider. Provider data is shared between users.
var ErrNotEditable = errors.New("only flights you imported or added by hand can be edited")

// ManualFlight is what a user enters to add or change a flight by hand: the
// way in for flights the provider does not know, such as old ones.
type ManualFlight struct {
	Number        string `json:"number"`
	Date          string `json:"date"`
	DepartureIATA string `json:"departureIata"`
	ArrivalIATA   string `json:"arrivalIata"`
	AirlineName   string `json:"airlineName"`
	AircraftModel string `json:"aircraftModel"`
	Status        string `json:"status"`
}

// AirportResolver looks up airport details by IATA code.
type AirportResolver interface {
	GetAirportByIATA(ctx context.Context, iata string) (*domain.Airport, error)
}

// WithAirports lets the service resolve airports for flights added by hand.
func (s *Service) WithAirports(airports AirportResolver) *Service {
	s.airports = airports

	return s
}

// CreateManual adds a flight the user typed in to their collection. The
// flight is private to them.
func (s *Service) CreateManual(ctx context.Context, userID string, in *ManualFlight) (*domain.Flight, error) {
	flight := &domain.Flight{Provider: manualProvider, ImportedBy: userID}

	err := s.applyManual(ctx, flight, in)
	if err != nil {
		return nil, err
	}

	existing, err := s.cache.FindByRoute(ctx, userID, flight.Number, flight.FlightDate,
		flight.Departure.Airport.IATA, flight.Arrival.Airport.IATA)
	if err == nil && existing.ImportedBy == userID {
		return nil, domain.ErrFlightAlreadyAdded
	}

	err = s.cache.Create(ctx, flight)
	if err != nil {
		return nil, fmt.Errorf("store manual flight: %w", err)
	}

	err = s.userRepo.Add(ctx, userID, flight.ID)
	if err != nil {
		return nil, fmt.Errorf("add manual flight for user %s: %w", userID, err)
	}

	s.log.Info("flight added by hand", zap.String("user_id", userID), zap.String("flight_id", flight.ID))

	return presentOwn(flight), nil
}

// UpdateManual changes a flight the user imported or added by hand.
func (s *Service) UpdateManual(ctx context.Context, userID, flightID string, in *ManualFlight) (*domain.Flight, error) {
	uf, err := s.userRepo.Get(ctx, userID, flightID)
	if err != nil {
		return nil, domain.ErrFlightNotFound
	}

	flight := &uf.Flight
	if flight.ImportedBy == "" || flight.ImportedBy != userID {
		return nil, ErrNotEditable
	}

	err = s.applyManual(ctx, flight, in)
	if err != nil {
		return nil, err
	}

	err = s.cache.Update(ctx, flight)
	if err != nil {
		return nil, fmt.Errorf("update manual flight %s: %w", flightID, err)
	}

	return presentOwn(flight), nil
}

// presentOwn prepares a user's own flight for the response.
func presentOwn(flight *domain.Flight) *domain.Flight {
	flight.Number = domain.FormatFlightNumber(flight.Number)
	flight.Editable = true

	return flight
}

// applyManual validates the input and writes it onto the flight, resolving
// both airports and working out the distance between them.
func (s *Service) applyManual(ctx context.Context, flight *domain.Flight, in *ManualFlight) error {
	err := validateManual(in)
	if err != nil {
		return err
	}

	dep, err := s.resolveAirport(ctx, in.DepartureIATA)
	if err != nil {
		return err
	}

	arr, err := s.resolveAirport(ctx, in.ArrivalIATA)
	if err != nil {
		return err
	}

	number := domain.NormalizeFlightNumber(in.Number)

	flight.Number = number
	flight.FlightDate = in.Date
	flight.Status = manualStatus(in.Status)
	flight.Airline = domain.FlightAirline{Name: strings.TrimSpace(in.AirlineName), IATA: number[:airlineCodeLen]}
	flight.Aircraft.Model = strings.TrimSpace(in.AircraftModel)
	flight.Departure.Airport = *dep
	flight.Arrival.Airport = *arr
	flight.GreatCircleDistance = domain.DistanceBetween(dep.Location, arr.Location)

	return nil
}

func validateManual(in *ManualFlight) error {
	err := domain.ValidateFlight(in.Number)
	if err != nil {
		return fmt.Errorf("%w: flight number: %w", domain.ErrValidation, err)
	}

	_, err = time.Parse(time.DateOnly, in.Date)
	if err != nil {
		return fmt.Errorf("%w: date must be YYYY-MM-DD", domain.ErrValidation)
	}

	if in.Status != "" && !domain.ValidFlightStatus(domain.FlightStatus(in.Status)) {
		return fmt.Errorf("%w: unknown status %q", domain.ErrValidation, in.Status)
	}

	return validateManualDetails(in)
}

func validateManualDetails(in *ManualFlight) error {
	if len(in.AirlineName) > maxTextLen || len(in.AircraftModel) > maxTextLen {
		return fmt.Errorf("%w: airline and aircraft may be at most %d characters", domain.ErrValidation, maxTextLen)
	}

	if strings.EqualFold(strings.TrimSpace(in.DepartureIATA), strings.TrimSpace(in.ArrivalIATA)) {
		return fmt.Errorf("%w: departure and arrival airport must differ", domain.ErrValidation)
	}

	return nil
}

// manualStatus defaults to landed: a flight typed in by hand is nearly always
// one that has already been flown.
func manualStatus(status string) domain.FlightStatus {
	if status == "" {
		return domain.FlightStatusLanded
	}

	return domain.FlightStatus(status)
}

func (s *Service) resolveAirport(ctx context.Context, code string) (*domain.Airport, error) {
	iata := strings.ToUpper(strings.TrimSpace(code))
	if len(iata) != airportCodeLen || s.airports == nil {
		return nil, fmt.Errorf("%w: %q is not a 3-letter airport code", domain.ErrValidation, code)
	}

	airport, err := s.airports.GetAirportByIATA(ctx, iata)
	if err != nil {
		return nil, fmt.Errorf("%w: unknown airport code %s", domain.ErrValidation, iata)
	}

	return airport, nil
}
