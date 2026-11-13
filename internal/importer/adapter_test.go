package importer

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const flightyCSV = `Date,Airline,Flight,From,To,Dep Terminal,Dep Gate,Arr Terminal,Arr Gate,Canceled,Gate Departure (Scheduled),Gate Arrival (Scheduled),Aircraft Type Name,Tail Number
2025-03-14,ek,565,blr,DXB,2,B12,3,A4,false,2025-03-14T04:30,2025-03-14T07:05,Boeing 777-300ER,A6-EGO
03/15/2025,KL,1001,AMS,LHR,,,,,,,,,
2025-03-16,BA,117,LHR,JFK,,,,,true,,,,
2025-03-17,,,AMS,,,,,,,,,,
not-a-date,LH,400,FRA,JFK,,,,,,,,,
`

func TestFlightyAdapterParse(t *testing.T) {
	t.Parallel()

	entries, err := (&FlightyAdapter{}).Parse(strings.NewReader(flightyCSV))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// The cancelled row, the row missing fields and the bad date are skipped.
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2: %+v", len(entries), entries)
	}

	dep := time.Date(2025, 3, 14, 4, 30, 0, 0, time.UTC)
	first := entries[0]

	want := ImportEntry{
		FlightNumber: "EK565", Date: "2025-03-14", DepIATA: "BLR", ArrIATA: "DXB", Airline: "EK",
		Aircraft: "Boeing 777-300ER", AircraftReg: "A6-EGO",
		DepTerminal: "2", DepGate: "B12", ArrTerminal: "3", ArrGate: "A4",
	}

	if first.DepTime == nil || !first.DepTime.Equal(dep) || first.ArrTime == nil {
		t.Errorf("times = %v / %v, want departure %v and an arrival", first.DepTime, first.ArrTime, dep)
	}

	first.DepTime, first.ArrTime = nil, nil
	if first != want {
		t.Errorf("first entry = %+v\nwant %+v", first, want)
	}

	if entries[1].FlightNumber != "KL1001" || entries[1].Date != "2025-03-15" {
		t.Errorf("second entry = %+v, want KL1001 on 2025-03-15 (US date format)", entries[1])
	}
}

func TestAdaptersRejectMissingColumns(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		adapter Adapter
		csv     string
		want    error
	}{
		{"flighty without To", &FlightyAdapter{}, "Date,Airline,Flight,From\n", ErrInvalidFlightyCSV},
		{"flightlog without arrival", &FlightlogAdapter{}, "date,flight_number,departure_iata\n", ErrInvalidFlightlogCSV},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := tt.adapter.Parse(strings.NewReader(tt.csv))
			if !errors.Is(err, tt.want) {
				t.Errorf("err = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestImportComputesDistanceFromCoordinates(t *testing.T) {
	t.Parallel()

	cache := &fakeCache{}
	svc := NewService(nil, &fakeLinker{}, cache, nil)

	entry := importEntry("KL641")
	entry.ArrIATA, entry.ArrName = "JFK", "John F Kennedy"
	entry.DistanceKm = 0
	entry.DepLat, entry.DepLon = 52.3086, 4.7639
	entry.ArrLat, entry.ArrLon = 40.6413, -73.7781

	result := svc.Import(t.Context(), "user-1", []ImportEntry{entry}, "flightlog", false)
	if result.Imported != 1 || len(cache.flights) != 1 {
		t.Fatalf("result = %+v", result)
	}

	km := cache.flights[0].GreatCircleDistance.Km
	if km < 5800 || km > 5900 {
		t.Errorf("distance = %.0f km, want about 5848 computed from the coordinates", km)
	}
}
