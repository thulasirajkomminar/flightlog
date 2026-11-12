package flight

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/logger"
)

const testUser = "user-1"

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
	// noBackground makes the provider report a low quota.
	noBackground bool
	// gate, when set, blocks each search until it is closed.
	gate chan struct{}
	mu   sync.Mutex
}

func (p *fakeProvider) BackgroundAllowed() bool { return !p.noBackground }

func (p *fakeProvider) SearchFlights(_ context.Context, _, _ string) ([]*domain.Flight, error) {
	p.mu.Lock()
	p.calls++
	p.mu.Unlock()

	if p.gate != nil {
		<-p.gate
	}

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

	err = db.AutoMigrate(&domain.Flight{}, &domain.FlightMiss{}, &domain.UserFlight{}, &domain.User{})
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

	got, err := svc.SearchFlights(context.Background(), testUser, "KL1001", today)
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

			got, err := svc.SearchFlights(context.Background(), testUser, "KL1001", today)
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

	got, err := svc.SearchFlights(context.Background(), testUser, "KL1001", today)
	if err != nil {
		t.Fatalf("SearchFlights: %v", err)
	}

	if len(got) != 1 || got[0].Status != domain.FlightStatusScheduled {
		t.Errorf("got %+v, want cached scheduled flight", got)
	}
}

// arrivedFlight is a scheduled flight in user-1's collection that should
// have landed two hours ago.
func arrivedFlight(t *testing.T, svc *Service, db *gorm.DB, now time.Time, number string) *domain.Flight {
	t.Helper()

	f := testFlight(number, now.Format(time.DateOnly), domain.FlightStatusScheduled)
	f.Departure.Airport.ICAO, f.Arrival.Airport.ICAO = "EHAM", "EGLL"
	arrived := now.Add(-2 * time.Hour)
	f.Arrival.ScheduledTime.UTC = &arrived
	insertFlight(t, db, f, now.Add(-12*time.Hour))

	err := svc.AddFlight(context.Background(), testUser, f.ID)
	if err != nil {
		t.Fatalf("AddFlight: %v", err)
	}

	return f
}

func TestRefreshDueUpdatesArrivedFlight(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	today := now.Format(time.DateOnly)
	provider := &fakeProvider{flights: []*domain.Flight{testFlight("KL1001", today, domain.FlightStatusLanded)}}
	svc, db := newTestService(t, provider)

	linked := arrivedFlight(t, svc, db, now, "KL1001")

	// Not in anyone's collection: must not cost a provider call.
	unlinked := testFlight("KL2002", today, domain.FlightStatusScheduled)
	unlinked.Arrival.ScheduledTime.UTC = linked.Arrival.ScheduledTime.UTC
	insertFlight(t, db, unlinked, now.Add(-12*time.Hour))

	if updated := svc.RefreshDue(context.Background(), now); updated != 1 {
		t.Fatalf("RefreshDue updated = %d, want 1", updated)
	}

	got, err := svc.GetFlight(context.Background(), testUser, linked.ID)
	if err != nil {
		t.Fatalf("GetFlight: %v", err)
	}

	if got.Status != domain.FlightStatusLanded || provider.calls != 1 {
		t.Errorf("status = %s after %d provider calls, want landed after 1", got.Status, provider.calls)
	}

	// Landed is final: later runs spend nothing.
	svc.RefreshDue(context.Background(), now.Add(48*time.Hour))

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want still 1", provider.calls)
	}
}

func TestRefreshDueWaitsForArrival(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)

	f := arrivedFlight(t, svc, db, now, "KL1001")
	landsLater := now.Add(3 * time.Hour)

	err := db.Model(f).Update("arr_scheduled_utc", landsLater).Error
	if err != nil {
		t.Fatalf("set arrival: %v", err)
	}

	svc.RefreshDue(context.Background(), now)

	if provider.calls != 0 {
		t.Errorf("provider calls before arrival = %d, want 0", provider.calls)
	}
}

// TestRefreshDueGivesUpAfterThreeChecks covers a flight the provider never
// confirms: it must cost at most MaxStatusChecks calls, spaced out.
func TestRefreshDueGivesUpAfterThreeChecks(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	provider := &fakeProvider{err: errProviderDown}
	svc, db := newTestService(t, provider)
	f := arrivedFlight(t, svc, db, now, "KL1001")

	// lastCheck moves the stored last-check time, as if that long had passed.
	lastCheck := func(ago time.Duration) {
		err := db.Model(&domain.Flight{}).Where("id = ?", f.ID).UpdateColumn("updated_at", now.Add(-ago)).Error
		if err != nil {
			t.Fatalf("set updated_at: %v", err)
		}
	}

	steps := []struct {
		name      string
		sinceLast time.Duration
		wantCalls int
	}{
		{"first check runs once the flight should have arrived", 0, 1},
		{"no second check within six hours", time.Hour, 1},
		{"second check after six hours", 7 * time.Hour, 2},
		{"no third check within a day", 7 * time.Hour, 2},
		{"third check after a day", 25 * time.Hour, 3},
		{"then it gives up", 72 * time.Hour, 3},
	}

	for i, step := range steps {
		if i > 0 {
			lastCheck(step.sinceLast)
		}

		svc.RefreshDue(context.Background(), now)

		if provider.calls != step.wantCalls {
			t.Fatalf("%s: provider calls = %d, want %d", step.name, provider.calls, step.wantCalls)
		}
	}
}

type fakeCoverage map[string]struct{}

func (c fakeCoverage) LiveAirports(context.Context) (map[string]struct{}, error) { return c, nil }

func TestRefreshDueSkipsAirportsWithoutLiveCoverage(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)
	svc.WithLiveCoverage(fakeCoverage{"KJFK": {}})

	untracked := arrivedFlight(t, svc, db, now, "KL1001")

	svc.RefreshDue(context.Background(), now)
	svc.RefreshDue(context.Background(), now.Add(48*time.Hour))

	if provider.calls != 0 {
		t.Errorf("provider calls = %d, want 0 for a flight between untracked airports", provider.calls)
	}

	var stored domain.Flight

	db.First(&stored, "id = ?", untracked.ID)

	if stored.StatusChecks != MaxStatusChecks {
		t.Errorf("status checks = %d, want %d (given up without a call)", stored.StatusChecks, MaxStatusChecks)
	}

	// One tracked end is enough for the provider to see the flight.
	svc.WithLiveCoverage(fakeCoverage{"EGLL": {}})
	arrivedFlight(t, svc, db, now, "KL1003")
	svc.RefreshDue(context.Background(), now)

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want 1 for a flight into a tracked airport", provider.calls)
	}
}

func TestSearchFlightsDoesNotRefreshFarFutureFlight(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)
	nextMonth := time.Now().UTC().AddDate(0, 1, 0).Format(time.DateOnly)
	insertFlight(t, db, testFlight("KL1001", nextMonth, domain.FlightStatusScheduled), time.Now().Add(-72*time.Hour))

	got, err := svc.SearchFlights(context.Background(), testUser, "KL1001", nextMonth)
	if err != nil || len(got) != 1 {
		t.Fatalf("SearchFlights = %d flights, err %v", len(got), err)
	}

	if provider.calls != 0 {
		t.Errorf("provider calls = %d, want 0: a flight a month away is served from the cache", provider.calls)
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
		// Never confirmed by the provider, but long past: counts as flown.
		testFlight("KL1002", "2026-01-02", domain.FlightStatusScheduled),
		testFlight("KL1003", "2026-01-03", domain.FlightStatusCancelled),
		testFlight("KL1005", time.Now().AddDate(0, 0, 30).Format(time.DateOnly), domain.FlightStatusScheduled),
		noCoords,
	}

	for _, f := range flights {
		insertFlight(t, db, f, old)

		err := svc.AddFlight(ctx, testUser, f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	routes, err := svc.GetRoutes(ctx, testUser)
	if err != nil {
		t.Fatalf("GetRoutes: %v", err)
	}

	if len(routes) != 1 {
		t.Fatalf("routes = %d, want 1 (cancelled, upcoming and coordinate-less flights excluded)", len(routes))
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

func TestGetStatsCountsOnlyFlownFlights(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()
	dep := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	arr := dep.Add(90 * time.Minute)

	cancelled := testFlight("BA2002", "2026-01-02", domain.FlightStatusCancelled)
	cancelled.Airline.IATA = "BA"
	cancelled.Arrival.Airport.IATA = "CDG"

	today := time.Now().UTC().Format(time.DateOnly)

	for _, f := range []*domain.Flight{
		testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded),
		// Never confirmed by the provider, but long past: counts as flown.
		testFlight("KL1002", "2026-01-02", domain.FlightStatusScheduled),
		cancelled,
		// Today's flight may still be refreshed, so it does not count yet.
		testFlight("KL1003", today, domain.FlightStatusScheduled),
	} {
		f.GreatCircleDistance.Km = 370
		f.Departure.ScheduledTime.UTC = &dep
		f.Arrival.ScheduledTime.UTC = &arr
		insertFlight(t, db, f, time.Now())

		err := svc.AddFlight(ctx, testUser, f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	got, err := svc.GetStats(ctx, testUser)
	if err != nil {
		t.Fatalf("GetStats: %v", err)
	}

	want := domain.FlightStats{Flights: 2, Distance: 740, FlightTime: 3, Airports: 2, Airlines: 1}
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

		err := svc.AddFlight(ctx, testUser, f.ID)
		if err != nil {
			t.Fatalf("AddFlight: %v", err)
		}
	}

	var got []string

	for _, offset := range []int{0, 2} {
		page, total, err := svc.ListFlights(ctx, &domain.FlightSearchCriteria{UserID: testUser, Limit: 2, Offset: offset})
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
		{"add unknown flight", func() error { return svc.AddFlight(ctx, testUser, "no-such-id") }, domain.ErrFlightNotFound},
		{"add flight", func() error { return svc.AddFlight(ctx, testUser, f.ID) }, nil},
		{"add it again", func() error { return svc.AddFlight(ctx, testUser, f.ID) }, domain.ErrFlightAlreadyAdded},
		{"delete flight", func() error { return svc.DeleteFlight(ctx, testUser, f.ID) }, nil},
		{"delete it again", func() error { return svc.DeleteFlight(ctx, testUser, f.ID) }, domain.ErrFlightNotFound},
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

func TestSearchFlightsRemembersMisses(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)
	ctx := context.Background()
	lastYear := time.Now().UTC().AddDate(-1, 0, 0).Format(time.DateOnly)

	for range 3 {
		_, err := svc.SearchFlights(ctx, testUser, "KL1001", lastYear)
		if !errors.Is(err, ErrNoFlightsFound) {
			t.Fatalf("err = %v, want ErrNoFlightsFound", err)
		}
	}

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want 1: the miss must be remembered", provider.calls)
	}

	// Another flight number is a different question.
	_, _ = svc.SearchFlights(ctx, testUser, "KL1003", lastYear)

	if provider.calls != 2 {
		t.Errorf("provider calls = %d, want 2", provider.calls)
	}

	// Once the remembered answer is too old, the provider is asked again.
	err := db.Model(&domain.FlightMiss{}).Where("flight_number = ?", "KL1001").
		Update("checked_at", time.Now().Add(-pastMissTTL-time.Hour)).Error
	if err != nil {
		t.Fatalf("age miss: %v", err)
	}

	provider.flights = []*domain.Flight{testFlight("KL1001", lastYear, domain.FlightStatusLanded)}

	got, err := svc.SearchFlights(ctx, testUser, "KL1001", lastYear)
	if err != nil || len(got) != 1 || provider.calls != 3 {
		t.Errorf("after expiry: %d flights, err %v, %d calls; want 1 flight from a third call", len(got), err, provider.calls)
	}
}

func TestSearchFlightsSharesConcurrentLookups(t *testing.T) {
	t.Parallel()

	today := time.Now().UTC().Format(time.DateOnly)
	provider := &fakeProvider{
		flights: []*domain.Flight{testFlight("KL1001", today, domain.FlightStatusScheduled)},
		gate:    make(chan struct{}),
	}
	svc, _ := newTestService(t, provider)

	const searches = 5

	var wg sync.WaitGroup

	results := make([]int, searches)

	for i := range searches {
		wg.Go(func() {
			flights, _ := svc.SearchFlights(context.Background(), testUser, "KL1001", today)
			results[i] = len(flights)
		})
	}

	// Let every search reach the provider gate, then open it.
	time.Sleep(100 * time.Millisecond)
	close(provider.gate)
	wg.Wait()

	if provider.calls != 1 {
		t.Errorf("provider calls = %d, want 1 shared by %d identical searches", provider.calls, searches)
	}

	for i, n := range results {
		if n != 1 {
			t.Errorf("search %d got %d flights, want 1", i, n)
		}
	}
}

func TestRefreshDueStopsWhenQuotaIsLow(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	provider := &fakeProvider{noBackground: true}
	svc, db := newTestService(t, provider)
	arrivedFlight(t, svc, db, now, "KL1001")

	svc.RefreshDue(context.Background(), now)

	if provider.calls != 0 {
		t.Errorf("provider calls = %d, want 0 while the quota is low", provider.calls)
	}
}

func TestDistanceUnitsAreDerivedFromKilometres(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()

	f := testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded)
	f.GreatCircleDistance = domain.GreatCircleDistance{Km: 370}
	insertFlight(t, db, f, time.Now())

	err := svc.AddFlight(ctx, testUser, f.ID)
	if err != nil {
		t.Fatalf("AddFlight: %v", err)
	}

	want := domain.DistanceFromKm(370)

	single, err := svc.GetFlight(ctx, testUser, f.ID)
	if err != nil || single.GreatCircleDistance != want {
		t.Errorf("GetFlight distance = %+v, err %v; want %+v", single.GreatCircleDistance, err, want)
	}

	list, _, err := svc.ListFlights(ctx, &domain.FlightSearchCriteria{UserID: testUser, Limit: 10})
	if err != nil || len(list) != 1 || list[0].GreatCircleDistance != want {
		t.Errorf("ListFlights = %+v, err %v; want one flight with distance %+v", list, err, want)
	}

	if db.Migrator().HasColumn(&domain.Flight{}, "gcd_mile") {
		t.Error("miles are stored; only kilometres should be")
	}
}

func TestPruneMissesKeepsOnlyUsableAnswers(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	ctx := context.Background()
	now := time.Now().UTC()

	misses := []domain.FlightMiss{
		{FlightNumber: "KL1001", FlightDate: "2025-01-01", CheckedAt: now.Add(-pastMissTTL - time.Hour)},
		{FlightNumber: "KL2003", FlightDate: "2025-01-01", CheckedAt: now.Add(-time.Hour)},
	}

	err := db.Create(&misses).Error
	if err != nil {
		t.Fatalf("create misses: %v", err)
	}

	svc.pruneMisses(ctx, now)

	var left []domain.FlightMiss

	db.Find(&left)

	if len(left) != 1 || left[0].FlightNumber != "KL2003" {
		t.Errorf("misses left = %+v, want only the recent one", left)
	}
}

func TestRefreshDueCanBeSwitchedOff(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)
	svc.WithStatusRefresh(false)
	arrivedFlight(t, svc, db, now, "KL1001")

	svc.RefreshDue(context.Background(), now)

	if provider.calls != 0 {
		t.Errorf("provider calls = %d, want 0 with status refresh switched off", provider.calls)
	}
}

func TestSearchDoesNotShowAnotherUsersImportedFlight(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{}
	svc, db := newTestService(t, provider)
	ctx := context.Background()

	imported := testFlight("KL1001", "2025-01-01", domain.FlightStatusLanded)
	imported.ImportedBy = testUser
	insertFlight(t, db, imported, time.Now())

	own, err := svc.SearchFlights(ctx, testUser, "KL1001", "2025-01-01")
	if err != nil || len(own) != 1 || provider.calls != 0 {
		t.Errorf("owner's search = %d flights, err %v, %d provider calls; want their flight from the cache", len(own), err, provider.calls)
	}

	_, err = svc.SearchFlights(ctx, "someone-else", "KL1001", "2025-01-01")
	if !errors.Is(err, ErrNoFlightsFound) || provider.calls != 1 {
		t.Errorf("other user's search: err = %v, provider calls = %d; want the provider asked and nothing found", err, provider.calls)
	}
}

func TestProviderDataDoesNotOverwriteAnImportedFlight(t *testing.T) {
	t.Parallel()

	today := time.Now().UTC().Format(time.DateOnly)
	provider := &fakeProvider{flights: []*domain.Flight{testFlight("KL1001", today, domain.FlightStatusLanded)}}
	svc, db := newTestService(t, provider)

	imported := testFlight("KL1001", today, domain.FlightStatusScheduled)
	imported.ImportedBy = testUser
	imported.Aircraft.Model = "typed in by hand"
	insertFlight(t, db, imported, time.Now())

	// Another user's search fetches provider data for the same flight.
	got, err := svc.SearchFlights(context.Background(), "someone-else", "KL1001", today)
	if err != nil || len(got) != 1 || got[0].ID == imported.ID {
		t.Fatalf("search = %+v, err %v; want a separate shared flight", got, err)
	}

	var kept domain.Flight

	db.First(&kept, "id = ?", imported.ID)

	if kept.Aircraft.Model != "typed in by hand" || kept.ImportedBy != testUser {
		t.Errorf("imported flight after another user's search = %+v; want it untouched", kept)
	}
}
