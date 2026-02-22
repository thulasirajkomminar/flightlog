// Package aerodatabox implements the AeroDataBox flight data provider.
package aerodatabox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
	"github.com/thulasirajkomminar/flightlog/internal/provider/aerodatabox/api"
)

// ErrUnexpectedStatus indicates a non-OK API response.
var (
	ErrUnexpectedStatus = errors.New("aerodatabox API returned unexpected status")
	ErrTimeParseFailure = errors.New("cannot parse time")
)

const maxResponseSize = 10 << 20 // 10 MB

// Provider implements flight.Provider.
type Provider struct {
	client *api.Client
	log    *logger.ComponentLogger
}

// NewProvider creates a Provider.
func NewProvider(apiKey, baseURL string, timeout time.Duration) (*Provider, error) {
	if baseURL == "" {
		baseURL = "https://aerodatabox.p.rapidapi.com"
	}

	client, err := api.NewClient(baseURL,
		api.WithHTTPClient(&http.Client{Timeout: timeout}),
		api.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
			req.Header.Set("X-Rapidapi-Key", apiKey)
			req.Header.Set("X-Rapidapi-Host", "aerodatabox.p.rapidapi.com")

			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create aerodatabox client: %w", err)
	}

	return &Provider{client: client, log: logger.NewComponentLogger("aerodatabox")}, nil
}

// GetProviderName returns the provider name.
func (p *Provider) GetProviderName() string {
	return "aerodatabox"
}

// SearchFlights searches for flights.
// Criteria keys: "flight_iata", "flight_date" (YYYY-MM-DD).
func (p *Provider) SearchFlights(ctx context.Context, criteria map[string]string) ([]*domain.Flight, error) {
	flightNumber := criteria["flight_iata"]
	dateStr := criteria["flight_date"]

	if dateStr == "" {
		dateStr = time.Now().UTC().Format("2006-01-02")
	}

	dateLocal, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format %q: %w", dateStr, err)
	}

	p.log.Debug("searching flights",
		zap.String("flight_number", flightNumber),
		zap.String("date", dateStr),
	)

	body, err := p.fetchFlightData(ctx, flightNumber, dateLocal)
	if err != nil {
		p.log.Error("API request failed", err,
			zap.String("flight_number", flightNumber),
			zap.String("date", dateStr),
		)

		return nil, err
	}

	if body == nil {
		p.log.Debug("no flights returned from API",
			zap.String("flight_number", flightNumber),
			zap.String("date", dateStr),
		)

		return nil, nil
	}

	return parseFlightResults(body, dateStr)
}

func (p *Provider) fetchFlightData(ctx context.Context, flightNumber string, date time.Time) ([]byte, error) {
	resp, err := p.client.GetFlightFlightOnSpecificDate(ctx,
		api.FlightSearchByEnumNumber,
		flightNumber,
		date,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("aerodatabox API request failed: %w", err)
	}

	defer func() { _, _ = io.Copy(io.Discard, resp.Body); _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNoContent {
		return nil, nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		p.log.Warn("unexpected API status",
			zap.Int("status", resp.StatusCode),
			zap.String("body", string(body)),
		)

		return nil, fmt.Errorf("%w: %d", ErrUnexpectedStatus, resp.StatusCode)
	}

	return body, nil
}

func parseFlightResults(body []byte, dateStr string) ([]*domain.Flight, error) {
	var results []flightResponse

	err := json.Unmarshal(body, &results)
	if err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	flights := make([]*domain.Flight, 0, len(results))
	for i := range results {
		flights = append(flights, convertToFlight(&results[i], dateStr, body))
	}

	return flights, nil
}

// flexTime handles AeroDataBox timestamps which may be RFC3339 or "YYYY-MM-DD HH:MMZ".
type flexTime struct {
	time.Time
}

func (ft *flexTime) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}

	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04Z07:00",
		"2006-01-02 15:04Z07:00",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05Z07:00",
		"2006-01-02T15:04Z",
		"2006-01-02 15:04Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05Z",
		"2006-01-02T15:04-07:00",
		"2006-01-02 15:04-07:00",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05-07:00",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			ft.Time = t

			return nil
		}
	}

	return fmt.Errorf("%w %q", ErrTimeParseFailure, s)
}

type flightResponse struct {
	Number              string                       `json:"number"`
	Status              api.FlightStatus             `json:"status"`
	CallSign            string                       `json:"callSign,omitempty"`
	CodeshareStatus     string                       `json:"codeshareStatus,omitempty"`
	IsCargo             bool                         `json:"isCargo"`
	Airline             *airlineResponse             `json:"airline,omitempty"`
	Aircraft            *aircraftResponse            `json:"aircraft,omitempty"`
	Departure           movementResponse             `json:"departure"`
	Arrival             movementResponse             `json:"arrival"`
	GreatCircleDistance *greatCircleDistanceResponse `json:"greatCircleDistance,omitempty"`
	LastUpdatedUtc      flexTime                     `json:"lastUpdatedUtc"`
}

type airlineResponse struct {
	Name string  `json:"name"`
	Iata *string `json:"iata,omitempty"`
	Icao *string `json:"icao,omitempty"`
}

type aircraftResponse struct {
	Reg   string `json:"reg,omitempty"`
	ModeS string `json:"modeS,omitempty"`
	Model string `json:"model,omitempty"`
}

type greatCircleDistanceResponse struct {
	Meter float64 `json:"meter"`
	Km    float64 `json:"km"`
	Mile  float64 `json:"mile"`
	Nm    float64 `json:"nm"`
	Feet  float64 `json:"feet"`
}

type movementResponse struct {
	Airport       airportResponse   `json:"airport"`
	ScheduledTime *dateTimeResponse `json:"scheduledTime,omitempty"`
	RevisedTime   *dateTimeResponse `json:"revisedTime,omitempty"`
	RunwayTime    *dateTimeResponse `json:"runwayTime,omitempty"`
	Terminal      *string           `json:"terminal,omitempty"`
	Gate          *string           `json:"gate,omitempty"`
	CheckInDesk   *string           `json:"checkInDesk,omitempty"`
	BaggageBelt   *string           `json:"baggageBelt,omitempty"`
}

type airportResponse struct {
	ICAO             string            `json:"icao,omitempty"`
	Name             string            `json:"name"`
	Iata             *string           `json:"iata,omitempty"`
	ShortName        string            `json:"shortName,omitempty"`
	MunicipalityName string            `json:"municipalityName,omitempty"`
	Location         *locationResponse `json:"location,omitempty"`
	CountryCode      string            `json:"countryCode,omitempty"`
	TimeZone         string            `json:"timeZone,omitempty"`
}

type locationResponse struct {
	Lat float64 `json:"lat"`
	Lon float64 `json:"lon"`
}

type dateTimeResponse struct {
	Utc   flexTime `json:"utc"`
	Local flexTime `json:"local"`
}

func convertToFlight(fc *flightResponse, flightDate string, rawBody []byte) *domain.Flight {
	flight := &domain.Flight{
		Number:          fc.Number,
		FlightDate:      flightDate,
		Status:          convertStatus(fc.Status),
		CallSign:        fc.CallSign,
		CodeshareStatus: fc.CodeshareStatus,
		IsCargo:         fc.IsCargo,
		Provider:        "aerodatabox",
	}

	if fc.Airline != nil {
		flight.Airline.Name = fc.Airline.Name
		if fc.Airline.Iata != nil {
			flight.Airline.IATA = *fc.Airline.Iata
		}

		if fc.Airline.Icao != nil {
			flight.Airline.ICAO = *fc.Airline.Icao
		}
	}

	if fc.Aircraft != nil {
		flight.Aircraft = domain.FlightAircraft{
			Reg:   fc.Aircraft.Reg,
			ModeS: fc.Aircraft.ModeS,
			Model: fc.Aircraft.Model,
		}
	}

	if fc.GreatCircleDistance != nil {
		flight.GreatCircleDistance = domain.GreatCircleDistance{
			Meter: fc.GreatCircleDistance.Meter,
			Km:    fc.GreatCircleDistance.Km,
			Mile:  fc.GreatCircleDistance.Mile,
			Nm:    fc.GreatCircleDistance.Nm,
			Feet:  fc.GreatCircleDistance.Feet,
		}
	}

	if !fc.LastUpdatedUtc.IsZero() {
		t := fc.LastUpdatedUtc.Time
		flight.LastUpdatedUtc = &t
	}

	flight.Departure = mapMovement(&fc.Departure)
	flight.Arrival = mapMovement(&fc.Arrival)
	flight.RawData = string(rawBody)

	return flight
}

func mapMovement(m *movementResponse) domain.Movement {
	mv := domain.Movement{
		Airport: domain.Airport{
			ICAO:             m.Airport.ICAO,
			Name:             m.Airport.Name,
			ShortName:        m.Airport.ShortName,
			MunicipalityName: m.Airport.MunicipalityName,
			CountryCode:      m.Airport.CountryCode,
			TimeZone:         m.Airport.TimeZone,
		},
	}
	if m.Airport.Iata != nil {
		mv.Airport.IATA = *m.Airport.Iata
	}

	if m.Airport.Location != nil {
		mv.Airport.Location = domain.Location{
			Lat: m.Airport.Location.Lat,
			Lon: m.Airport.Location.Lon,
		}
	}

	if m.Terminal != nil {
		mv.Terminal = *m.Terminal
	}

	if m.Gate != nil {
		mv.Gate = *m.Gate
	}

	if m.CheckInDesk != nil {
		mv.CheckInDesk = *m.CheckInDesk
	}

	if m.BaggageBelt != nil {
		mv.BaggageBelt = *m.BaggageBelt
	}

	mv.ScheduledTime = mapTimeInfo(m.ScheduledTime)
	mv.RevisedTime = mapTimeInfo(m.RevisedTime)
	mv.RunwayTime = mapTimeInfo(m.RunwayTime)

	return mv
}

func mapTimeInfo(dt *dateTimeResponse) domain.TimeInfo {
	if dt == nil {
		return domain.TimeInfo{}
	}

	var ti domain.TimeInfo

	if !dt.Utc.IsZero() {
		t := dt.Utc.Time
		ti.UTC = &t
	}

	if !dt.Local.IsZero() {
		ti.Local = dt.Local.Format(time.RFC3339)
	}

	return ti
}

func convertStatus(status api.FlightStatus) domain.FlightStatus {
	switch status {
	case api.FlightStatusExpected, api.FlightStatusCheckIn,
		api.FlightStatusBoarding, api.FlightStatusGateClosed,
		api.FlightStatusDelayed, api.FlightStatusUnknown:
		return domain.FlightStatusScheduled
	case api.FlightStatusEnRoute, api.FlightStatusApproaching,
		api.FlightStatusDeparted:
		return domain.FlightStatusActive
	case api.FlightStatusArrived:
		return domain.FlightStatusLanded
	case api.FlightStatusCanceled, api.FlightStatusCanceledUncertain:
		return domain.FlightStatusCancelled
	case api.FlightStatusDiverted:
		return domain.FlightStatusDiverted
	}

	return domain.FlightStatusScheduled
}
