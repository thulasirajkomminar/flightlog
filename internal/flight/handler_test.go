package flight

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/thulasirajkomminar/flightlog/internal/api"
	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

func searchRequest(t *testing.T, h *Handler, query string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/flights/search?"+query, http.NoBody)
	req = req.WithContext(api.SetUser(req.Context(), &api.UserClaims{UserID: "user-1"}))
	rec := httptest.NewRecorder()

	h.SearchFlights(rec, req)

	return rec
}

func TestSearchFlightsErrorStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		provider   *fakeProvider
		wantStatus int
		wantBody   string
	}{
		{"provider knows no such flight", &fakeProvider{}, http.StatusNotFound, "no flights found"},
		{"provider failure is not reported as not found", &fakeProvider{err: errProviderDown}, http.StatusBadGateway, "provider is unavailable"},
		{"exhausted quota is named", &fakeProvider{err: domain.ErrProviderQuota}, http.StatusServiceUnavailable, "quota is used up"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			svc, _ := newTestService(t, tt.provider)
			rec := searchRequest(t, NewHandler(svc), "flight_number=KL1001&date=2026-01-01")

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}

			if !strings.Contains(rec.Body.String(), tt.wantBody) {
				t.Errorf("body = %s, want it to contain %q", rec.Body.String(), tt.wantBody)
			}
		})
	}
}
