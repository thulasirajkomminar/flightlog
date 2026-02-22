// Package provider defines the flight provider interface and HTTP handler.
package provider

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// FlightProvider defines the contract for flight data providers.
type FlightProvider interface {
	SearchFlights(ctx context.Context, criteria map[string]string) ([]*domain.Flight, error)
	GetProviderName() string
}

// Handler handles provider HTTP endpoints.
type Handler struct {
	providers map[string]FlightProvider
}

// NewHandler creates a Handler.
func NewHandler(providers map[string]FlightProvider) *Handler {
	return &Handler{
		providers: providers,
	}
}

// SearchFlights searches flights directly from an external provider.
func (h *Handler) SearchFlights(w http.ResponseWriter, r *http.Request) {
	providerName := chi.URLParam(r, "provider")
	if providerName == "" {
		providerName = "aerodatabox"
	}

	provider, ok := h.providers[providerName]
	if !ok {
		api.RespondError(w, http.StatusNotFound, "unknown provider: "+providerName)

		return
	}

	params, ok := parseSearchParams(w, r)
	if !ok {
		return
	}

	criteria := map[string]string{
		"flight_iata": params.flightNumber,
	}

	if params.date != "" {
		criteria["flight_date"] = params.date
	}

	flights, err := provider.SearchFlights(r.Context(), criteria)
	if err != nil {
		api.RespondError(w, http.StatusInternalServerError, "failed to fetch flights from provider")

		return
	}

	api.RespondJSON(w, http.StatusOK, map[string]any{
		"flights": flights,
		"count":   len(flights),
	})
}

type searchParams struct {
	flightNumber string
	date         string
}

func parseSearchParams(w http.ResponseWriter, r *http.Request) (searchParams, bool) {
	flightNumber := r.URL.Query().Get("flight_number")
	if flightNumber == "" {
		api.RespondError(w, http.StatusBadRequest, "flight_number parameter is required")

		return searchParams{}, false
	}

	err := domain.ValidateFlight(flightNumber)
	if err != nil {
		api.RespondError(w, http.StatusBadRequest, err.Error())

		return searchParams{}, false
	}

	date := r.URL.Query().Get("date")
	if date != "" {
		_, err := time.Parse(time.DateOnly, date)
		if err != nil {
			api.RespondError(w, http.StatusBadRequest, "invalid date format, expected YYYY-MM-DD")

			return searchParams{}, false
		}
	}

	return searchParams{flightNumber: domain.NormalizeFlightNumber(flightNumber), date: date}, true
}
