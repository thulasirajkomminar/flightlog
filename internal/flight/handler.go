// Package flight implements flight search, caching, and user flight management.
package flight

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	defaultListLimit    = 100
	maxListLimit        = 200
	errFlightIDRequired = "flight id is required"
	errUnauthorized     = "unauthorized"
	errInvalidDate      = "invalid date format, expected YYYY-MM-DD"
	errInternal         = "internal server error"
)

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
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

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

	flights, err := h.flightService.SearchFlights(r.Context(), flightNumber, date)
	if err != nil {
		api.RespondError(w, http.StatusNotFound, "no flights found")

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"flights": flights,
		"count":   len(flights),
	})
}

// AddFlight links a cached flight to the authenticated user.
func (h *Handler) AddFlight(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.RespondError(w, http.StatusBadRequest, errFlightIDRequired)

		return
	}

	err := h.flightService.AddFlight(r.Context(), user.UserID, id)
	if err != nil {
		if errors.Is(err, domain.ErrFlightAlreadyAdded) {
			api.RespondError(w, http.StatusConflict, err.Error())
		} else {
			api.RespondError(w, http.StatusInternalServerError, errInternal)
		}

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Flight added successfully"})
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

func parseCriteria(r *http.Request, userID string) domain.FlightSearchCriteria {
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

	return criteria
}

// ListFlights lists the authenticated user's flights.
func (h *Handler) ListFlights(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	criteria := parseCriteria(r, user.UserID)

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
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	id := chi.URLParam(r, "id")
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
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	stats, err := h.flightService.GetStats(r.Context(), user.UserID)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, stats)
}

// DeleteFlight removes a flight from the user's collection.
func (h *Handler) DeleteFlight(w http.ResponseWriter, r *http.Request) {
	user := api.GetUser(r)
	if user == nil {
		api.RespondError(w, http.StatusUnauthorized, errUnauthorized)

		return
	}

	id := chi.URLParam(r, "id")
	if id == "" {
		api.RespondError(w, http.StatusBadRequest, errFlightIDRequired)

		return
	}

	err := h.flightService.DeleteFlight(r.Context(), user.UserID, id)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, errInternal)

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]string{"message": "Flight deleted successfully"})
}
