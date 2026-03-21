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

// ErrInvalidFlightlogCSV is returned when required columns are missing.
var ErrInvalidFlightlogCSV = errors.New("invalid Flightlog CSV: missing required columns")

var flightlogRequiredColumns = []string{ //nolint:gochecknoglobals // column schema
	"date", "flight_number", "departure_iata", "arrival_iata",
}

const floatBitSize = 64

// FlightlogAdapter parses CSV exports from Flightlog itself.
type FlightlogAdapter struct{}

// Name returns "flightlog".
func (a *FlightlogAdapter) Name() string {
	return "flightlog"
}

// Parse reads a Flightlog CSV and returns import entries.
func (a *FlightlogAdapter) Parse(r io.Reader) ([]ImportEntry, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIdx := buildColumnIndex(header)

	for _, col := range flightlogRequiredColumns {
		if _, ok := colIdx[col]; !ok {
			return nil, fmt.Errorf("%w: %s", ErrInvalidFlightlogCSV, col)
		}
	}

	return parseFlightlogRows(reader, colIdx)
}

func parseFlightlogRows(reader *csv.Reader, colIdx map[string]int) ([]ImportEntry, error) {
	var entries []ImportEntry

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		entry, err := parseFlightlogRow(row, colIdx)
		if err != nil {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func parseFlightlogRow(row []string, colIdx map[string]int) (ImportEntry, error) {
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
		Status:       getCol("status"),
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

	setFlightlogTimes(&entry, getCol)

	return entry, nil
}

func parseFloat(s string) float64 {
	km, err := strconv.ParseFloat(s, floatBitSize)
	if err != nil {
		return 0
	}

	return km
}

func setFlightlogTimes(entry *ImportEntry, getCol func(string) string) {
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
