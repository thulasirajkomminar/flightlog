// Package flight implements flight search, caching, and user flight management.
package flight

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	defaultListLimit    = 100
	maxListLimit        = 200
	floatBitSize        = 64
	errFlightIDRequired = "flight id is required"
	errInvalidDate      = "invalid date format, expected YYYY-MM-DD"
	errInternal         = "internal server error"
	paramID             = "id"

	errProviderUnavailable = "flight data provider is unavailable, please try again later"
	errProviderQuota       = "the flight data API quota is used up for now, please try again later"
)

// ErrInvalidStatus is returned when an unknown flight status is provided.
var ErrInvalidStatus = errors.New("invalid status")

// Handler handles flight HTTP endpoints.
type Handler struct {
	flightService *Service
}

// NewHandler creates a Handler.
func NewHandler(flightService *Service) *Handler {
	return &Handler{
		flightService: flightService,
	}
}

// SearchFlights handles flight search by number and optional date.
func (h *Handler) SearchFlights(w http.ResponseWriter, r *http.Request) {
	flightNumber := r.URL.Query().Get("flight_number")
	if flightNumber == "" {
		api.RespondError(w, http.StatusBadRequest, "flight_number parameter is required")

		return
	}

	err := domain.ValidateFlight(flightNumber)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, err.Error())

		return
	}

	flightNumber = domain.NormalizeFlightNumber(flightNumber)

	date := r.URL.Query().Get("date")
	if date != "" {
		_, err := time.Parse(time.DateOnly, date)
		if err != nil {
			api.RespondError(w, http.StatusBadRequest, errInvalidDate)

			return
		}
	}

	flights, err := h.flightService.SearchFlights(r.Context(), api.GetUser(r).UserID, flightNumber, date)
	if err != nil {
		respondSearchError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"flights": flights,
		"count":   len(flights),
	})
}

// respondSearchError tells "the flight does not exist" apart from "the flight
// data provider could not be reached", so quota or outage problems are visible.
func respondSearchError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrNoFlightsFound) {
		api.RespondError(w, http.StatusNotFound, "no flights found")

		return
	}

	if errors.Is(err, domain.ErrProviderQuota) {
		api.RespondError(w, http.StatusServiceUnavailable, errProviderQuota)

		return
	}

	api.RespondError(w, http.StatusBadGateway, errProviderUnavailable)
}

// AddFlight links a cached flight to the authenticated user.
func (h *Handler) AddFlight(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	id := chi.URLParam(r, paramID)
	if id == "" {
		api.RespondError(w, http.StatusBadRequest, errFlightIDRequired)

		return
	}

	err := h.flightService.AddFlight(r.Context(), user.UserID, id)
	if err != nil {
		respondCollectionError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Flight added successfully"})
}

// CreateFlight adds a flight the user typed in by hand.
func (h *Handler) CreateFlight(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeManual(w, r)
	if !ok {
		return
	}

	flight, err := h.flightService.CreateManual(r.Context(), api.GetUser(r).UserID, in)
	if err != nil {
		respondCollectionError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusCreated, flight)
}

// UpdateFlight changes a flight the user imported or added by hand.
func (h *Handler) UpdateFlight(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeManual(w, r)
	if !ok {
		return
	}

	flight, err := h.flightService.UpdateManual(r.Context(), api.GetUser(r).UserID, chi.URLParam(r, paramID), in)
	if err != nil {
		respondCollectionError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusOK, flight)
}

func decodeManual(w http.ResponseWriter, r *http.Request) (*ManualFlight, bool) {
	var in ManualFlight

	r.Body = http.MaxBytesReader(w, r.Body, api.MaxBodySize)

	err := json.NewDecoder(r.Body).Decode(&in)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, "invalid request body")

		return nil, false
	}

	return &in, true
}

// respondCollectionError maps errors from changing a user's collection.
func respondCollectionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrFlightAlreadyAdded):
		api.RespondError(w, http.StatusConflict, domain.ErrFlightAlreadyAdded.Error())
	case errors.Is(err, domain.ErrValidation):
		api.RespondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrNotEditable):
		api.RespondError(w, http.StatusForbidden, ErrNotEditable.Error())
	case errors.Is(err, domain.ErrFlightNotFound):
		api.RespondError(w, http.StatusNotFound, domain.ErrFlightNotFound.Error())
	default:
		api.RespondError(w, http.StatusInternalServerError, errInternal)
	}
}

func parseIntParam(r *http.Request, key string) (int, bool) {
	s := r.URL.Query().Get(key)
	if s == "" {
		return 0, false
	}

	v, err := strconv.Atoi(s)
	if err != nil {
		return 0, false
	}

	return v, true
}

// maxSearchText bounds the free-text filter of the flight list.
const maxSearchText = 50

func searchText(r *http.Request) string {
	text := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(text) > maxSearchText {
		text = text[:maxSearchText]
	}

	return text
}

func parseCriteria(r *http.Request, userID string) (domain.FlightSearchCriteria, error) {
	criteria := domain.FlightSearchCriteria{
		UserID: userID,
		Limit:  defaultListLimit,
	}

	if v, ok := parseIntParam(r, "limit"); ok {
		criteria.Limit = min(v, maxListLimit)
	}

	if v, ok := parseIntParam(r, "offset"); ok {
		criteria.Offset = v
	}

	if v, ok := parseIntParam(r, "year"); ok {
		criteria.Year = v
	}

	criteria.Query = searchText(r)

	if s := r.URL.Query().Get("status"); s != "" {
		if !domain.ValidFlightStatus(domain.FlightStatus(s)) {
			return criteria, fmt.Errorf("%w: %s", ErrInvalidStatus, s)
		}

		criteria.Status = domain.FlightStatus(s)
	}

	return criteria, nil
}

// ListFlights lists the authenticated user's flights.
func (h *Handler) ListFlights(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	criteria, err := parseCriteria(r, user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, err.Error())

		return
	}

	flights, total, err := h.flightService.ListFlights(r.Context(), &criteria)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	years, err := h.flightService.GetYears(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"flights": flights,
		"count":   len(flights),
		"total":   total,
		"years":   years,
	})
}

// GetFlight returns a single flight by ID.
func (h *Handler) GetFlight(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	id := chi.URLParam(r, paramID)
	if id == "" {
		api.RespondError(w, http.StatusBadRequest, errFlightIDRequired)

		return
	}

	flight, err := h.flightService.GetFlight(r.Context(), user.UserID, id)
	if err != nil {
		api.RespondError(w, http.StatusNotFound, "Flight not found")

		return
	}

	api.RespondJSON(w, http.StatusOK, flight)
}

// GetStats returns aggregated flight statistics.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	stats, err := h.flightService.GetStats(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, stats)
}

// GetBreakdown returns per-year totals and the most flown routes and airlines.
func (h *Handler) GetBreakdown(w http.ResponseWriter, r *http.Request) {
	breakdown, err := h.flightService.GetBreakdown(r.Context(), api.GetUser(r).UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, breakdown)
}

// GetQuota returns the flight data provider's API quota as last reported.
func (h *Handler) GetQuota(w http.ResponseWriter, _ *http.Request) {
	api.RespondJSON(w, http.StatusOK, h.flightService.ProviderQuota())
}

// GetRoutes returns the distinct routes the user has flown, for the map.
func (h *Handler) GetRoutes(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	routes, err := h.flightService.GetRoutes(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{"routes": routes})
}

// ExportFlights returns all user flights as a CSV file download.
func (h *Handler) ExportFlights(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	flights, err := h.flightService.ExportFlights(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="flightlog-export.csv"`)

	cw := csv.NewWriter(w)

	err = cw.Write(csvHeader())
	if err != nil {
		return
	}

	for _, f := range flights {
		err = cw.Write(flightToRow(f))
		if err != nil {
			return
		}
	}

	cw.Flush()
}

func csvHeader() []string {
	return []string{
		"date", "flight_number", "status",
		"airline", "airline_iata", "airline_icao",
		"departure_iata", "departure_icao", "departure_airport", "departure_city", "departure_country",
		"departure_lat", "departure_lon",
		"departure_time_utc", "departure_time_local", "departure_terminal", "departure_gate",
		"arrival_iata", "arrival_icao", "arrival_airport", "arrival_city", "arrival_country",
		"arrival_lat", "arrival_lon",
		"arrival_time_utc", "arrival_time_local", "arrival_terminal", "arrival_gate",
		"aircraft_model", "aircraft_registration",
		"distance_km",
	}
}

func flightToRow(f *domain.Flight) []string {
	return []string{
		f.FlightDate, f.Number, string(f.Status),
		f.Airline.Name, f.Airline.IATA, f.Airline.ICAO,
		f.Departure.Airport.IATA, f.Departure.Airport.ICAO, f.Departure.Airport.Name,
		f.Departure.Airport.MunicipalityName, f.Departure.Airport.CountryCode,
		formatCoord(f.Departure.Airport.Location.Lat), formatCoord(f.Departure.Airport.Location.Lon),
		formatTimePtr(f.Departure.ScheduledTime.UTC), f.Departure.ScheduledTime.Local,
		f.Departure.Terminal, f.Departure.Gate,
		f.Arrival.Airport.IATA, f.Arrival.Airport.ICAO, f.Arrival.Airport.Name,
		f.Arrival.Airport.MunicipalityName, f.Arrival.Airport.CountryCode,
		formatCoord(f.Arrival.Airport.Location.Lat), formatCoord(f.Arrival.Airport.Location.Lon),
		formatTimePtr(f.Arrival.ScheduledTime.UTC), f.Arrival.ScheduledTime.Local,
		f.Arrival.Terminal, f.Arrival.Gate,
		f.Aircraft.Model, f.Aircraft.Reg,
		fmt.Sprintf("%.2f", f.GreatCircleDistance.Km),
	}
}

func formatCoord(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, floatBitSize)
}

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}

	return t.Format(time.RFC3339)
}

// DeleteFlight removes a flight from the user's collection.
func (h *Handler) DeleteFlight(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)

	id := chi.URLParam(r, paramID)
	if id == "" {
		api.RespondError(w, http.StatusBadRequest, errFlightIDRequired)

		return
	}

	err := h.flightService.DeleteFlight(r.Context(), user.UserID, id)
	if err != nil {
		respondCollectionError(w, err)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Flight deleted successfully"})
}
