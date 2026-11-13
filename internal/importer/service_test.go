package importer

import (
	"context"
	"errors"
	"testing"
	"time"

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

func (c *fakeCache) FindByRoute(_ context.Context, owner, number, date, dep, arr string) (*domain.Flight, error) {
	for _, f := range c.flights {
		if visible(f, owner) && f.Number == number && f.FlightDate == date && f.Departure.Airport.IATA == dep && f.Arrival.Airport.IATA == arr {
			return f, nil
		}
	}

	return nil, errNotFound
}

func (c *fakeCache) FindByDateAndRoute(_ context.Context, owner, date, dep, arr string) ([]*domain.Flight, error) {
	var out []*domain.Flight

	for _, f := range c.flights {
		if visible(f, owner) && f.FlightDate == date && f.Departure.Airport.IATA == dep && f.Arrival.Airport.IATA == arr {
			out = append(out, f)
		}
	}

	return out, nil
}

// visible mirrors the store rule: shared flights and the owner's own.
func visible(f *domain.Flight, owner string) bool {
	return f.ImportedBy == "" || f.ImportedBy == owner
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

// slowLinker blocks each link until released, to observe a running job.
type slowLinker struct {
	release chan struct{}
}

func (l *slowLinker) AddFlight(context.Context, string, string) error {
	<-l.release

	return nil
}

func waitForJob(t *testing.T, svc *Service, userID string, done func(Job) bool) Job {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, ok := svc.ImportStatus(t.Context(), userID)
		if ok && done(job) {
			return job
		}

		time.Sleep(5 * time.Millisecond)
	}

	t.Fatal("timed out waiting for import job")

	return Job{}
}

func TestStartImportRunsInBackgroundAndReportsProgress(t *testing.T) {
	t.Parallel()

	linker := &slowLinker{release: make(chan struct{})}
	svc := NewService(nil, linker, &fakeCache{}, nil)
	entries := []ImportEntry{importEntry("KL1001"), importEntry("KL1003"), importEntry("KL1005")}

	job, err := svc.StartImport(context.Background(), "user-1", entries, "flightlog", false)
	if err != nil || job.State != JobRunning || job.Total != 3 {
		t.Fatalf("StartImport = %+v, %v; want a running job of 3", job, err)
	}

	linker.release <- struct{}{}

	job = waitForJob(t, svc, "user-1", func(j Job) bool { return j.Processed == 1 })
	if job.State != JobRunning || job.Result != nil {
		t.Errorf("after one flight: %+v, want still running without a result", job)
	}

	_, err = svc.StartImport(context.Background(), "user-1", entries, "flightlog", false)
	if !errors.Is(err, ErrImportRunning) {
		t.Errorf("StartImport while running: err = %v, want ErrImportRunning", err)
	}

	close(linker.release)

	job = waitForJob(t, svc, "user-1", func(j Job) bool { return j.State == JobCompleted })
	if job.Processed != 3 || job.Result == nil || job.Result.Imported != 3 {
		t.Errorf("finished job = %+v (result %+v), want 3 imported", job, job.Result)
	}
}

func TestImportJobsArePerUserAndRestartable(t *testing.T) {
	t.Parallel()

	svc := NewService(nil, &fakeLinker{}, &fakeCache{}, nil)
	entries := []ImportEntry{importEntry("KL1001")}

	_, ok := svc.ImportStatus(t.Context(), "user-1")
	if ok {
		t.Fatal("status before any import: want none")
	}

	_, err := svc.StartImport(context.Background(), "user-1", entries, "flightlog", false)
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}

	waitForJob(t, svc, "user-1", func(j Job) bool { return j.State == JobCompleted })

	_, ok = svc.ImportStatus(t.Context(), "user-2")
	if ok {
		t.Error("user-2 sees user-1's job")
	}

	_, err = svc.StartImport(context.Background(), "user-1", entries, "flightlog", false)
	if err != nil {
		t.Errorf("StartImport after completion: %v", err)
	}
}

func TestImportedFlightsStayPrivateToTheirOwner(t *testing.T) {
	t.Parallel()

	const alice, bob = "alice", "bob"

	cache := &fakeCache{}
	linker := &fakeLinker{}
	svc := NewService(nil, linker, cache, nil)
	entry := []ImportEntry{importEntry(klm1001)}

	for _, userID := range []string{alice, bob, alice} {
		result := svc.Import(t.Context(), userID, entry, "flightlog", false)
		if result.Imported != 1 {
			t.Fatalf("import for %s: %+v", userID, result)
		}
	}

	// Each user gets a flight built from their own file; importing the same
	// file again reuses that user's flight instead of adding a third.
	if len(cache.flights) != 2 {
		t.Fatalf("cached flights = %d, want one per user", len(cache.flights))
	}

	if cache.flights[0].ImportedBy != alice || cache.flights[1].ImportedBy != bob {
		t.Errorf("owners = %q, %q; want alice and bob", cache.flights[0].ImportedBy, cache.flights[1].ImportedBy)
	}

	if linker.linked[0] == linker.linked[1] || linker.linked[0] != linker.linked[2] {
		t.Errorf("linked flights = %v; want bob on a separate flight and alice reusing her own", linker.linked)
	}
}

func TestCancelImportStopsAfterTheCurrentEntry(t *testing.T) {
	t.Parallel()

	const user = "user-9"

	linker := &slowLinker{release: make(chan struct{})}
	svc := NewService(nil, linker, &fakeCache{}, nil)
	entries := []ImportEntry{importEntry("KL1001"), importEntry("KL1003"), importEntry("KL1005")}

	if svc.CancelImport(user) {
		t.Error("CancelImport with nothing running reported true")
	}

	_, err := svc.StartImport(context.Background(), user, entries, "flightlog", false)
	if err != nil {
		t.Fatalf("StartImport: %v", err)
	}

	// Let the first entry through, then cancel while the second is in flight.
	linker.release <- struct{}{}

	waitForJob(t, svc, user, func(j Job) bool { return j.Processed == 1 })

	if !svc.CancelImport(user) {
		t.Fatal("CancelImport reported nothing running")
	}

	close(linker.release)

	job := waitForJob(t, svc, user, func(j Job) bool { return j.State == JobCompleted })
	if job.Result == nil || !job.Result.Cancelled || job.Result.Imported != 2 {
		t.Errorf("cancelled job result = %+v, want cancelled with the 2 entries already under way imported", job.Result)
	}
}
