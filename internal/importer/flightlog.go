package importer

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// ErrInvalidFlightLogCSV is returned when required columns are missing.
var ErrInvalidFlightLogCSV = errors.New("invalid FlightLog CSV: missing required columns")

var flightlogRequiredColumns = []string{ //nolint:gochecknoglobals // column schema
	"date", "flight_number", "departure_iata", "arrival_iata",
}

const floatBitSize = 64

// FlightLogAdapter parses CSV exports from FlightLog itself.
type FlightLogAdapter struct{}

// Name returns "flightlog".
func (a *FlightLogAdapter) Name() string {
	return "flightlog"
}

// Parse reads a FlightLog CSV and returns import entries.
func (a *FlightLogAdapter) Parse(r io.Reader) ([]ImportEntry, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIdx := buildColumnIndex(header)

	for _, col := range flightlogRequiredColumns {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidFlightLogCSV, col)
		}
	}

	return parseFlightLogRows(reader, colIdx)
}

func parseFlightLogRows(reader *csv.Reader, colIdx map[string]int) ([]ImportEntry, error) {
	var entries []ImportEntry

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		entry, err := parseFlightLogRow(row, colIdx)
		if err != nil {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func parseFlightLogRow(row []string, colIdx map[string]int) (ImportEntry, error) {
	getCol := func(name string) string {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}

		return ""
	}

	date := getCol("date")
	flightNumber := getCol("flight_number")
	depIATA := getCol("departure_iata")
	arrIATA := getCol("arrival_iata")

	if date == "" || flightNumber == "" || depIATA == "" || arrIATA == "" {
		return ImportEntry{}, ErrMissingFields
	}

	entry := ImportEntry{
		FlightNumber: flightNumber,
		Date:         date,
		DepIATA:      depIATA,
		ArrIATA:      arrIATA,
		Airline:      getCol("airline_iata"),
		Aircraft:     getCol("aircraft_model"),
		AirlineName:  getCol("airline"),
		AirlineICAO:  getCol("airline_icao"),
		DepICAO:      getCol("departure_icao"),
		DepName:      getCol("departure_airport"),
		DepCity:      getCol("departure_city"),
		DepCountry:   getCol("departure_country"),
		DepLat:       parseFloat(getCol("departure_lat")),
		DepLon:       parseFloat(getCol("departure_lon")),
		DepTimeLocal: getCol("departure_time_local"),
		DepTerminal:  getCol("departure_terminal"),
		DepGate:      getCol("departure_gate"),
		ArrICAO:      getCol("arrival_icao"),
		ArrName:      getCol("arrival_airport"),
		ArrCity:      getCol("arrival_city"),
		ArrCountry:   getCol("arrival_country"),
		ArrLat:       parseFloat(getCol("arrival_lat")),
		ArrLon:       parseFloat(getCol("arrival_lon")),
		ArrTimeLocal: getCol("arrival_time_local"),
		ArrTerminal:  getCol("arrival_terminal"),
		ArrGate:      getCol("arrival_gate"),
		AircraftReg:  getCol("aircraft_registration"),
	}

	entry.DistanceKm = parseFloat(getCol("distance_km"))

	setFlightLogTimes(&entry, getCol)

	return entry, nil
}

func parseFloat(s string) float64 {
	km, err := strconv.ParseFloat(s, floatBitSize)
	if err != nil {
		return 0
	}

	return km
}

func setFlightLogTimes(entry *ImportEntry, getCol func(string) string) {
	if depTimeStr := getCol("departure_time_utc"); depTimeStr != "" {
		t, err := time.Parse(time.RFC3339, depTimeStr)
		if err == nil {
			entry.DepTime = &t
		}
	}

	if arrTimeStr := getCol("arrival_time_utc"); arrTimeStr != "" {
		t, err := time.Parse(time.RFC3339, arrTimeStr)
		if err == nil {
			entry.ArrTime = &t
		}
	}
}
