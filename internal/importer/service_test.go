package importer

import (
	"context"
	"errors"
	"testing"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

const klm1001 = "KL1001"

var errNotFound = errors.New("not found")

func TestMain(m *testing.M) {
	// Initialise the lazy global logger once, before parallel tests use it.
	logger.GetLogger()
	m.Run()
}

func TestSameFlightNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		a, b string
		want bool
	}{
		{klm1001, klm1001, true},
		{klm1001, "kl 1001", true},
		{klm1001, "KLM1001", true},
		{"EZY789", "U2789", true},
		{klm1001, "BA1001", false},
		{klm1001, "KL1002", false},
		// "KL1" is not an ICAO prefix, so KL1234 must not match BA234.
		{"KL1234", "BA234", false},
		{klm1001, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.a+"/"+tt.b, func(t *testing.T) {
			t.Parallel()

			if got := sameFlightNumber(tt.a, tt.b); got != tt.want {
				t.Errorf("sameFlightNumber(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}

			if got := sameFlightNumber(tt.b, tt.a); got != tt.want {
				t.Errorf("sameFlightNumber(%q, %q) = %v, want %v", tt.b, tt.a, got, tt.want)
			}
		})
	}
}

// fakeCache is an in-memory flight cache.
type fakeCache struct {
	flights []*domain.Flight
}

func (c *fakeCache) Create(_ context.Context, f *domain.Flight) error {
	f.GenerateID()
	c.flights = append(c.flights, f)

	return nil
}

func (c *fakeCache) FindByRoute(_ context.Context, number, date, dep, arr string) (*domain.Flight, error) {
	for _, f := range c.flights {
		if f.Number == number && f.FlightDate == date && f.Departure.Airport.IATA == dep && f.Arrival.Airport.IATA == arr {
			return f, nil
		}
	}

	return nil, errNotFound
}

func (c *fakeCache) FindByDateAndRoute(_ context.Context, date, dep, arr string) ([]*domain.Flight, error) {
	var out []*domain.Flight

	for _, f := range c.flights {
		if f.FlightDate == date && f.Departure.Airport.IATA == dep && f.Arrival.Airport.IATA == arr {
			out = append(out, f)
		}
	}

	return out, nil
}

// fakeLinker records which flight IDs were linked to the user.
type fakeLinker struct {
	linked []string
}

func (l *fakeLinker) AddFlight(_ context.Context, _, flightID string) error {
	l.linked = append(l.linked, flightID)

	return nil
}

func cachedFlight(id, number string) *domain.Flight {
	f := &domain.Flight{ID: id, Number: number, FlightDate: "2026-01-01"}
	f.Departure.Airport.IATA = "AMS"
	f.Arrival.Airport.IATA = "LHR"

	return f
}

func importEntry(number string) ImportEntry {
	return ImportEntry{
		FlightNumber: number, Date: "2026-01-01", DepIATA: "AMS", ArrIATA: "LHR",
		DepName: "Amsterdam Schiphol", ArrName: "London Heathrow", DistanceKm: 370,
	}
}

func TestImportMatchesCachedFlights(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		entry      string
		wantLinked string // empty means a new flight must be created
	}{
		{"exact flight number", "BA431", "ba-flight"},
		{"ICAO number in cache, IATA in file", klm1001, "klm-flight"},
		{"other airline on the same route and day is not reused", "U2789", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cache := &fakeCache{flights: []*domain.Flight{
				cachedFlight("ba-flight", "BA431"),
				cachedFlight("klm-flight", "KLM1001"),
			}}
			linker := &fakeLinker{}
			svc := NewService(nil, linker, cache, nil)

			result := svc.Import(context.Background(), "user-1", []ImportEntry{importEntry(tt.entry)}, "flightlog", false)
			if result.Imported != 1 || len(linker.linked) != 1 {
				t.Fatalf("result = %+v, linked = %v", result, linker.linked)
			}

			if tt.wantLinked != "" {
				if linker.linked[0] != tt.wantLinked {
					t.Errorf("linked %q, want %q", linker.linked[0], tt.wantLinked)
				}

				return
			}

			if len(cache.flights) != 3 || linker.linked[0] != cache.flights[2].ID {
				t.Errorf("expected a new cached flight to be created and linked, cache has %d flights", len(cache.flights))
			}
		})
	}
}
