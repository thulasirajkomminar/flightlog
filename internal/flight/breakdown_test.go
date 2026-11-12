package flight

import (
	"context"
	"testing"
	"time"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// collection gives testUser a small history: two years, a round trip, two
// airlines, three countries, and one cancelled flight that must not count.
func collection(t *testing.T) *Service {
	t.Helper()

	svc, db := newTestService(t, &fakeProvider{})

	type leg struct {
		number, date, from, fromCountry, to, toCountry, airline string
		status                                                  domain.FlightStatus
	}

	legs := []leg{
		{"KL641", "2024-03-01", ams, "NL", jfk, "US", klm, domain.FlightStatusLanded},
		{"KL642", "2024-03-09", jfk, "US", ams, "NL", klm, domain.FlightStatusLanded},
		{"BA431", "2025-06-02", ams, "NL", lhr, "GB", "British Airways", domain.FlightStatusLanded},
		{"BA432", "2025-06-05", lhr, "GB", ams, "NL", "British Airways", domain.FlightStatusCancelled},
		{"KL643", "2025-07-01", ams, "NL", jfk, "US", klm, domain.FlightStatusLanded},
	}

	for _, l := range legs {
		f := testFlight(l.number, l.date, l.status)
		f.Departure.Airport = domain.Airport{IATA: l.from, Name: l.from + " airport", MunicipalityName: "City of " + l.from, CountryCode: l.fromCountry}
		f.Arrival.Airport = domain.Airport{IATA: l.to, Name: l.to + " airport", MunicipalityName: "City of " + l.to, CountryCode: l.toCountry}
		f.Airline = domain.FlightAirline{Name: l.airline, IATA: l.number[:2]}
		f.GreatCircleDistance.Km = 1000
		insertFlight(t, db, f, time.Now())

		err := svc.AddFlight(context.Background(), testUser, f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	return svc
}

func TestGetBreakdown(t *testing.T) {
	t.Parallel()

	got, err := collection(t).GetBreakdown(context.Background(), testUser)
	if err != nil {
		t.Fatalf("GetBreakdown: %v", err)
	}

	wantYears := []domain.YearStat{{Year: 2025, Flights: 2, Distance: 2000}, {Year: 2024, Flights: 2, Distance: 2000}}
	if len(got.Years) != len(wantYears) || got.Years[0] != wantYears[0] || got.Years[1] != wantYears[1] {
		t.Errorf("years = %+v, want %+v", got.Years, wantYears)
	}

	// The round trip and the later one-way are one route flown three times.
	if top := got.Routes[0]; top != (domain.RouteStat{From: ams, To: jfk, Flights: 3}) {
		t.Errorf("top route = %+v, want AMS-JFK three times", top)
	}

	if top := got.Airlines[0]; top != (domain.AirlineStat{Name: klm, Flights: 3}) {
		t.Errorf("top airline = %+v, want KLM with 3", top)
	}

	if got.Countries != 3 {
		t.Errorf("countries = %d, want 3 (NL, US, GB)", got.Countries)
	}
}

func TestListFlightsTextSearch(t *testing.T) {
	t.Parallel()

	svc := collection(t)

	tests := []struct {
		query string
		want  int
	}{
		{"kl 641", 1},
		{"KL64", 3},
		{"british", 2},
		{"lhr", 2},
		{"city of jfk", 3},
		{"100%", 0},
		{"no such thing", 0},
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			t.Parallel()

			criteria := &domain.FlightSearchCriteria{UserID: testUser, Query: tt.query, Limit: 50}

			flights, total, err := svc.ListFlights(context.Background(), criteria)
			if err != nil || len(flights) != tt.want || int(total) != tt.want {
				t.Errorf("search %q = %d flights (total %d), err %v; want %d", tt.query, len(flights), total, err, tt.want)
			}
		})
	}
}
