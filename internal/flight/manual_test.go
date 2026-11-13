package flight

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const (
	ams       = "AMS"
	jfk       = "JFK"
	mayFourth = "2019-05-04"
	kl641     = "KL641"
	klm       = "KLM"
	lhr       = "LHR"
)

// fakeAirports knows two airports.
type fakeAirports struct{}

func (fakeAirports) GetAirportByIATA(_ context.Context, iata string) (*domain.Airport, error) {
	known := map[string]domain.Airport{
		ams: {IATA: ams, Name: "Schiphol", Location: domain.Location{Lat: 52.31, Lon: 4.76}},
		jfk: {IATA: jfk, Name: "John F Kennedy", Location: domain.Location{Lat: 40.64, Lon: -73.78}},
	}

	a, ok := known[iata]
	if !ok {
		return nil, errProviderDown
	}

	return &a, nil
}

func TestCreateManualAddsAPrivateFlight(t *testing.T) {
	t.Parallel()

	provider := &fakeProvider{}
	svc, _ := newTestService(t, provider)
	svc.WithAirports(fakeAirports{})

	ctx := context.Background()
	in := &ManualFlight{Number: "kl 641", Date: mayFourth, DepartureIATA: "ams ", ArrivalIATA: jfk, AirlineName: klm}

	created, err := svc.CreateManual(ctx, testUser, in)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	if created.Number != "KL 641" || created.Status != domain.FlightStatusLanded || !created.Editable {
		t.Errorf("created = %+v, want KL 641, landed and editable", created)
	}

	if km := created.GreatCircleDistance.Km; km < 5800 || km > 5900 {
		t.Errorf("distance = %.0f km, want about 5848 from the airport coordinates", km)
	}

	if provider.calls != 0 {
		t.Errorf("provider calls = %d, want 0: adding by hand costs no API quota", provider.calls)
	}
}

func TestCreateManualAddsToTheCollectionOnce(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, &fakeProvider{})
	svc.WithAirports(fakeAirports{})

	ctx := context.Background()
	in := &ManualFlight{Number: kl641, Date: mayFourth, DepartureIATA: ams, ArrivalIATA: jfk}

	_, err := svc.CreateManual(ctx, testUser, in)
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	list, total, err := svc.ListFlights(ctx, &domain.FlightSearchCriteria{UserID: testUser, Limit: 10})
	if err != nil || total != 1 || !list[0].Editable {
		t.Errorf("owner's list = %d flights, err %v; want the new flight, editable", total, err)
	}

	_, err = svc.CreateManual(ctx, testUser, in)
	if !errors.Is(err, domain.ErrFlightAlreadyAdded) {
		t.Errorf("adding it twice: err = %v, want already added", err)
	}
}

func TestCreateManualRejectsBadInput(t *testing.T) {
	t.Parallel()

	svc, _ := newTestService(t, &fakeProvider{})
	svc.WithAirports(fakeAirports{})

	valid := ManualFlight{Number: kl641, Date: mayFourth, DepartureIATA: ams, ArrivalIATA: jfk}

	tests := []struct {
		name   string
		change func(*ManualFlight)
	}{
		{"flight number", func(m *ManualFlight) { m.Number = "not a flight" }},
		{"date", func(m *ManualFlight) { m.Date = "4 May 2019" }},
		{"unknown airport", func(m *ManualFlight) { m.ArrivalIATA = "ZZZ" }},
		{"airport code length", func(m *ManualFlight) { m.DepartureIATA = "AMST" }},
		{"same airports", func(m *ManualFlight) { m.ArrivalIATA = "ams " }},
		{"status", func(m *ManualFlight) { m.Status = "teleported" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := valid
			tt.change(&in)

			_, err := svc.CreateManual(context.Background(), testUser, &in)
			if !errors.Is(err, domain.ErrValidation) {
				t.Errorf("err = %v, want a validation error", err)
			}
		})
	}
}

func TestUpdateManualOnlyChangesOwnFlights(t *testing.T) {
	t.Parallel()

	svc, db := newTestService(t, &fakeProvider{})
	svc.WithAirports(fakeAirports{})

	ctx := context.Background()
	edit := &ManualFlight{Number: "KL642", Date: "2019-05-11", DepartureIATA: jfk, ArrivalIATA: ams, AircraftModel: "Boeing 777"}

	own, err := svc.CreateManual(ctx, testUser, &ManualFlight{Number: kl641, Date: mayFourth, DepartureIATA: ams, ArrivalIATA: jfk})
	if err != nil {
		t.Fatalf("CreateManual: %v", err)
	}

	updated, err := svc.UpdateManual(ctx, testUser, own.ID, edit)
	if err != nil || updated.Number != "KL 642" || updated.Departure.Airport.IATA != jfk || updated.Aircraft.Model != "Boeing 777" {
		t.Errorf("UpdateManual = %+v, err %v; want the edited flight", updated, err)
	}

	// Provider data in the same collection cannot be edited.
	shared := testFlight("KL1001", "2026-01-01", domain.FlightStatusLanded)
	insertFlight(t, db, shared, time.Now())

	err = svc.AddFlight(ctx, testUser, shared.ID)
	if err != nil {
		t.Fatalf("AddFlight: %v", err)
	}

	_, err = svc.UpdateManual(ctx, testUser, shared.ID, edit)
	if !errors.Is(err, ErrNotEditable) {
		t.Errorf("editing provider data: err = %v, want not editable", err)
	}

	// Someone else's flight looks like it does not exist.
	_, err = svc.UpdateManual(ctx, "someone-else", own.ID, edit)
	if !errors.Is(err, domain.ErrFlightNotFound) {
		t.Errorf("editing another user's flight: err = %v, want not found", err)
	}
}
