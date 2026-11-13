package airport

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

var errNoSuchAirport = errors.New("no such airport")

func TestMain(m *testing.M) {
	// Initialise the lazy global logger once, before parallel tests use it.
	logger.GetLogger()
	m.Run()
}

func TestBundledKnowsMajorAirports(t *testing.T) {
	t.Parallel()

	tests := []struct {
		iata, icao, country string
		lat, lon            float64
	}{
		{"AMS", "EHAM", "NL", 52.31, 4.76},
		{"jfk", "KJFK", "US", 40.64, -73.78},
		{"SYD", "YSSY", "AU", -33.95, 151.18},
		{" HND ", "RJTT", "JP", 35.55, 139.78},
	}

	for _, tt := range tests {
		t.Run(tt.iata, func(t *testing.T) {
			t.Parallel()

			a, ok := Bundled(tt.iata)
			if !ok {
				t.Fatalf("Bundled(%q) not found", tt.iata)
			}

			if a.ICAO != tt.icao || a.CountryCode != tt.country || a.Name == "" {
				t.Errorf("airport = %+v, want ICAO %s in %s with a name", a, tt.icao, tt.country)
			}

			if math.Abs(a.Location.Lat-tt.lat) > 0.05 || math.Abs(a.Location.Lon-tt.lon) > 0.05 {
				t.Errorf("location = %+v, want about %.2f, %.2f", a.Location, tt.lat, tt.lon)
			}
		})
	}

	if _, ok := Bundled("ZZZZ"); ok {
		t.Error("Bundled found a non-existent code")
	}

	if n := len(loadBundled()); n < 8000 {
		t.Errorf("bundled database has %d airports, want several thousand", n)
	}
}

// countingProvider counts airport API calls.
type countingProvider struct {
	calls int
}

func (p *countingProvider) GetAirportByIATA(_ context.Context, iata string) (*domain.Airport, error) {
	p.calls++

	if iata != "TXL" {
		return nil, errNoSuchAirport
	}

	return &domain.Airport{IATA: iata, Name: "Berlin Tegel", Location: domain.Location{Lat: 52.56, Lon: 13.29}}, nil
}

func newTestLookup(t *testing.T, provider Provider) *CachedLookup {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{Logger: gormlogger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&Record{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	lookup := NewCachedLookup(NewStore(db), provider)
	lookup.throttle = 0

	return lookup
}

func TestLookupUsesBundledDataBeforeTheAPI(t *testing.T) {
	t.Parallel()

	provider := &countingProvider{}
	lookup := newTestLookup(t, provider)
	ctx := context.Background()

	a, err := lookup.GetAirportByIATA(ctx, "AMS")
	if err != nil || a.ICAO != "EHAM" {
		t.Fatalf("AMS = %+v, err %v", a, err)
	}

	if provider.calls != 0 {
		t.Errorf("API calls for a bundled airport = %d, want 0", provider.calls)
	}

	// A closed airport is not bundled: the API is asked once, then cached.
	for range 2 {
		a, err = lookup.GetAirportByIATA(ctx, "TXL")
		if err != nil || a.Name != "Berlin Tegel" {
			t.Fatalf("TXL = %+v, err %v", a, err)
		}
	}

	if provider.calls != 1 {
		t.Errorf("API calls for an unbundled airport looked up twice = %d, want 1", provider.calls)
	}
}
