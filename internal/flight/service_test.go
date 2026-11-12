package flight

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

var errProviderDown = errors.New("provider down")

func TestMain(m *testing.M) {
	// Initialise the lazy global logger once, before parallel tests use it.
	logger.GetLogger()
	m.Run()
}

// fakeProvider returns canned flights and counts calls.
type fakeProvider struct {
	flights []*domain.Flight
	err     error
	calls   int
}

func (p *fakeProvider) SearchFlights(_ context.Context, _ map[string]string) ([]*domain.Flight, error) {
	p.calls++

	if p.err != nil {
		return nil, p.err
	}

	out := make([]*domain.Flight, 0, len(p.flights))

	for _, f := range p.flights {
		c := *f
		out = append(out, &c)
	}

	return out, nil
}

func (p *fakeProvider) GetProviderName() string { return "fake" }

type noopBackfiller struct{}

func (noopBackfiller) BackfillFromFlight(context.Context, *domain.Flight) {}

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{
		Logger: gormlogger.Discard,
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	err = db.AutoMigrate(&domain.Flight{}, &domain.UserFlight{}, &domain.User{})
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}

	return db
}

func newTestService(t *testing.T, p Provider) (*Service, *gorm.DB) {
	t.Helper()

	db := newTestDB(t)

	return NewService(NewCacheStore(db), NewUserFlightStore(db), p, noopBackfiller{}), db
}

func testFlight(number, date string, status domain.FlightStatus) *domain.Flight {
	f := &domain.Flight{Number: number, FlightDate: date, Status: status, Provider: "fake"}
	f.Departure.Airport = domain.Airport{IATA: "AMS", Name: "Amsterdam Schiphol", Location: domain.Location{Lat: 52.31, Lon: 4.76}}
	f.Arrival.Airport = domain.Airport{IATA: "LHR", Name: "London Heathrow", Location: domain.Location{Lat: 51.47, Lon: -0.45}}
	f.Airline = domain.FlightAirline{Name: "KLM", IATA: "KL"}

	return f
}

// insertFlight stores a flight with an explicit last-checked time.
func insertFlight(t *testing.T, db *gorm.DB, f *domain.Flight, updatedAt time.Time) {
	t.Helper()

	f.GenerateID()

	err := db.Create(f).Error
	if err != nil {
		t.Fatalf("insert flight: %v", err)
	}

	err = db.Model(&domain.Flight{}).Where("id = ?", f.ID).UpdateColumn("updated_at", updatedAt).Error
	if err != nil {
		t.Fatalf("set updated_at: %v", err)
	}
}

func TestSearchFlightsRefreshesStaleNonFinalFlight(t *testing.T) {
	t.Parallel()

	today := time.Now().UTC().Format(time.DateOnly)
	provider := &fakeProvider{flights: []*domain.Flight{testFlight("KL1001", today, domain.FlightStatusLanded)}}
	svc, db := newTestService(t, provider)

	cached := testFlight("KL1001", today, domain.FlightStatusScheduled)
	insertFlight(t, db, cached, time.Now().Add(-time.Hour))

	got, err := svc.SearchFlights(context.Background(), "KL1001", today)
	if err != nil {
		t.Fatalf("SearchFlights: %v", err)
	}

	if provider.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", provider.calls)
	}

	if len(got) != 1 || got[0].Status != domain.FlightStatusLanded {
		t.Fatalf("got %+v, want one landed flight", got)
	}

	if got[0].ID != cached.ID {
		t.Errorf("flight ID changed from %s to %s; user links would break", cached.ID, got[0].ID)
	}

	var count int64

	db.Model(&domain.Flight{}).Count(&count)

	if count != 1 {
		t.Errorf("flights in cache = %d, want 1", count)
	}
}

func TestSearchFlightsUsesCacheWhenFresh(t *testing.T) {
	t.Parallel()

	today := time.Now().UTC().Format(time.DateOnly)

	tests := []struct {
		name      string
		status    domain.FlightStatus
		updatedAt time.Time
	}{
		{"final status is never refreshed", domain.FlightStatusLanded, time.Now().Add(-48 * time.Hour)},
		{"recently checked flight is not refreshed", domain.FlightStatusScheduled, time.Now().Add(-time.Minute)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			provider := &fakeProvider{}
			svc, db := newTestService(t, provider)
			insertFlight(t, db, testFlight("KL1001", today, tt.status), tt.updatedAt)

			got, err := svc.SearchFlights(context.Background(), "KL1001", today)
			if err != nil {
				t.Fatalf("SearchFlights: %v", err)
			}

			if provider.calls != 0 {
				t.Errorf("provider calls = %d, want 0", provider.calls)
			}

			if len(got) != 1 || got[0].Status != tt.status {
				t.Errorf("got %+v, want cached flight", got)
			}
		})
	}
}

func TestSearchFlightsFallsBackToCacheWhenRefreshFails(t *testing.T) {
	t.Parallel()

	today := time.Now().UTC().Format(time.DateOnly)
	provider := &fakeProvider{err: errProviderDown}
	svc, db := newTestService(t, provider)
	insertFlight(t, db, testFlight("KL1001", today, domain.FlightStatusScheduled), time.Now().Add(-time.Hour))

	got, err := svc.SearchFlights(context.Background(), "KL1001", today)
	if err != nil {
		t.Fatalf("SearchFlights: %v", err)
	}

	if len(got) != 1 || got[0].Status != domain.FlightStatusScheduled {
		t.Errorf("got %+v, want cached scheduled flight", got)
	}
}

func TestRefreshDue(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	today := now.Format(time.DateOnly)
	provider := &fakeProvider{flights: []*domain.Flight{testFlight("KL1001", today, domain.FlightStatusLanded)}}
	svc, db := newTestService(t, provider)

	linked := testFlight("KL1001", today, domain.FlightStatusScheduled)
	insertFlight(t, db, linked, now.Add(-12*time.Hour))

	// Not in anyone's collection: must not cost a provider call.
	unlinked := testFlight("KL2002", today, domain.FlightStatusScheduled)
	insertFlight(t, db, unlinked, now.Add(-12*time.Hour))

	err := svc.AddFlight(context.Background(), "user-1", linked.ID)
	if err != nil {
		t.Fatalf("AddFlight: %v", err)
	}

	if updated := svc.RefreshDue(context.Background(), now); updated != 1 {
		t.Fatalf("RefreshDue updated = %d, want 1", updated)
	}

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want 1", provider.calls)
	}

	got, err := svc.GetFlight(context.Background(), "user-1", linked.ID)
	if err != nil {
		t.Fatalf("GetFlight: %v", err)
	}

	if got.Status != domain.FlightStatusLanded {
		t.Errorf("status = %s, want landed", got.Status)
	}

	// A second run finds nothing due.
	if updated := svc.RefreshDue(context.Background(), now); updated != 0 {
		t.Errorf("second RefreshDue updated = %d, want 0", updated)
	}
}

func TestRefreshDueMarksCheckedWhenProviderHasNoAnswer(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	today := now.Format(time.DateOnly)
	provider := &fakeProvider{err: errProviderDown}
	svc, db := newTestService(t, provider)

	f := testFlight("KL1001", today, domain.FlightStatusScheduled)
	insertFlight(t, db, f, now.Add(-12*time.Hour))

	err := svc.AddFlight(context.Background(), "user-1", f.ID)
	if err != nil {
		t.Fatalf("AddFlight: %v", err)
	}

	svc.RefreshDue(context.Background(), now)
	svc.RefreshDue(context.Background(), now)

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want 1 (failed check must not be retried immediately)", provider.calls)
	}
}

func TestGetRoutesGroupsLandedFlights(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()
	old := time.Now().Add(-time.Hour)

	noCoords := testFlight("KL1004", "2026-01-04", domain.FlightStatusLanded)
	noCoords.Arrival.Airport = domain.Airport{IATA: "XXX"}

	flights := []*domain.Flight{
		testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded),
		testFlight("KL1002", "2026-01-02", domain.FlightStatusLanded),
		testFlight("KL1003", "2026-01-03", domain.FlightStatusCancelled),
		noCoords,
	}

	for _, f := range flights {
		insertFlight(t, db, f, old)

		err := svc.AddFlight(ctx, "user-1", f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	routes, err := svc.GetRoutes(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetRoutes: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1 (cancelled and coordinate-less flights excluded)", len(routes))
	}

	want := domain.Route{
		Departure: domain.RouteAirport{IATA: "AMS", Name: "Amsterdam Schiphol", Location: domain.Location{Lat: 52.31, Lon: 4.76}},
		Arrival:   domain.RouteAirport{IATA: "LHR", Name: "London Heathrow", Location: domain.Location{Lat: 51.47, Lon: -0.45}},
		Flights:   2,
	}
	if *routes[0] != want {
		t.Errorf("route = %+v, want %+v", *routes[0], want)
	}

	other, err := svc.GetRoutes(ctx, "user-2")
	if err != nil {
		t.Fatalf("GetRoutes: %v", err)
	}

	if len(other) != 0 {
		t.Errorf("other user's routes = %d, want 0", len(other))
	}
}

func TestGetStatsCountsOnlyLandedFlights(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()
	dep := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	arr := dep.Add(90 * time.Minute)

	cancelled := testFlight("BA2002", "2026-01-02", domain.FlightStatusCancelled)
	cancelled.Airline.IATA = "BA"
	cancelled.Arrival.Airport.IATA = "CDG"

	for _, f := range []*domain.Flight{
		testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded),
		cancelled,
		testFlight("KL1003", "2026-01-03", domain.FlightStatusScheduled),
	} {
		f.GreatCircleDistance.Km = 370
		f.Departure.ScheduledTime.UTC = &dep
		f.Arrival.ScheduledTime.UTC = &arr
		insertFlight(t, db, f, time.Now())

		err := svc.AddFlight(ctx, "user-1", f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	got, err := svc.GetStats(ctx, "user-1")
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	want := domain.FlightStats{Flights: 1, Distance: 370, FlightTime: 1.5, Airports: 2, Airlines: 1}
	if got.Flights != want.Flights || got.Distance != want.Distance || got.Airports != want.Airports || got.Airlines != want.Airlines {
		t.Errorf("stats = %+v, want %+v", *got, want)
	}

	if diff := got.FlightTime - want.FlightTime; diff > 0.001 || diff < -0.001 {
		t.Errorf("flight time = %v, want %v", got.FlightTime, want.FlightTime)
	}
}

func TestListFlightsPagesAreStableForSameDayFlights(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()
	morning := time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC)

	for i, number := range []string{"KL1001", "KL1003", "KL1005", "KL1007"} {
		f := testFlight(number, "2026-01-01", domain.FlightStatusLanded)
		dep := morning.Add(time.Duration(i) * time.Hour)
		f.Departure.ScheduledTime.UTC = &dep
		insertFlight(t, db, f, time.Now())

		err := svc.AddFlight(ctx, "user-1", f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	var got []string

	for _, offset := range []int{0, 2} {
		page, total, err := svc.ListFlights(ctx, &domain.FlightSearchCriteria{UserID: "user-1", Limit: 2, Offset: offset})
		if err != nil || total != 4 {
			t.Fatalf("ListFlights: total = %d, err = %v", total, err)
		}

		for _, f := range page {
			got = append(got, f.Number)
		}
	}

	want := "KL 1007,KL 1005,KL 1003,KL 1001"
	if strings.Join(got, ",") != want {
		t.Errorf("paged order = %v, want %s (latest departure first, no repeats)", got, want)
	}
}

func TestAddAndDeleteFlightErrors(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()

	f := testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded)
	insertFlight(t, db, f, time.Now())

	steps := []struct {
		name string
		run  func() error
		want error
	}{
		{"add unknown flight", func() error { return svc.AddFlight(ctx, "user-1", "no-such-id") }, domain.ErrFlightNotFound},
		{"add flight", func() error { return svc.AddFlight(ctx, "user-1", f.ID) }, nil},
		{"add it again", func() error { return svc.AddFlight(ctx, "user-1", f.ID) }, domain.ErrFlightAlreadyAdded},
		{"delete flight", func() error { return svc.DeleteFlight(ctx, "user-1", f.ID) }, nil},
		{"delete it again", func() error { return svc.DeleteFlight(ctx, "user-1", f.ID) }, domain.ErrFlightNotFound},
	}

	for _, step := range steps {
		err := step.run()
		if !errors.Is(err, step.want) {
			t.Errorf("%s: err = %v, want %v", step.name, err, step.want)
		}
	}

	var links int64

	db.Model(&domain.UserFlight{}).Count(&links)

	if links != 0 {
		t.Errorf("user_flights rows = %d, want 0", links)
	}
}
