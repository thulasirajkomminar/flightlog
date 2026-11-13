package flight

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
	"github.com/thulasirajkomminar/flightlog/internal/importer"
)

// TestExportRoundTripsThroughImport guards the contract between the CSV
// export and the Flightlog import adapter: what is exported must come back.
func TestExportRoundTripsThroughImport(t *testing.T) {
	t.Parallel()

	dep := time.Date(2026, 1, 1, 8, 15, 0, 0, time.UTC)
	arr := dep.Add(75 * time.Minute)

	f := testFlight("KL 1001", "2026-01-01", domain.FlightStatusLanded)
	f.Airline.ICAO = "KLC"
	f.Aircraft = domain.FlightAircraft{Model: "Boeing 737-800", Reg: "PH-BXA"}
	f.Departure.ScheduledTime = domain.TimeInfo{UTC: &dep, Local: "2026-01-01T09:15:00+01:00"}
	f.Departure.Terminal, f.Departure.Gate = "1", "D7"
	f.Arrival.ScheduledTime = domain.TimeInfo{UTC: &arr, Local: "2026-01-01T09:30:00Z"}
	f.Arrival.Airport.CountryCode = "GB"
	f.GreatCircleDistance = domain.DistanceFromKm(371.25)

	var buf bytes.Buffer

	err := csv.NewWriter(&buf).WriteAll([][]string{csvHeader(), flightToRow(f)})
	if err != nil {
		t.Fatalf("write csv: %v", err)
	}

	entries, err := (&importer.FlightlogAdapter{}).Parse(&buf)
	if err != nil || len(entries) != 1 {
		t.Fatalf("Parse = %d entries, err %v; want 1", len(entries), err)
	}

	got := entries[0]
	if got.DepTime == nil || !got.DepTime.Equal(dep) || got.ArrTime == nil || !got.ArrTime.Equal(arr) {
		t.Errorf("times = %v / %v, want %v / %v", got.DepTime, got.ArrTime, dep, arr)
	}

	got.DepTime, got.ArrTime = nil, nil

	depAirport, arrAirport := f.Departure.Airport, f.Arrival.Airport

	want := importer.ImportEntry{
		FlightNumber: "KL 1001", Date: "2026-01-01", Status: "landed",
		Airline: "KL", AirlineName: f.Airline.Name, AirlineICAO: "KLC",
		Aircraft: "Boeing 737-800", AircraftReg: "PH-BXA",
		DepIATA: depAirport.IATA, DepName: depAirport.Name, DepLat: depAirport.Location.Lat, DepLon: depAirport.Location.Lon,
		DepTimeLocal: "2026-01-01T09:15:00+01:00", DepTerminal: "1", DepGate: "D7",
		ArrIATA: arrAirport.IATA, ArrName: arrAirport.Name, ArrCountry: "GB", ArrLat: arrAirport.Location.Lat, ArrLon: arrAirport.Location.Lon,
		ArrTimeLocal: "2026-01-01T09:30:00Z",
		DistanceKm:   371.25,
	}
	if got != want {
		t.Errorf("round trip mismatch\n got %+v\nwant %+v", got, want)
	}
}
