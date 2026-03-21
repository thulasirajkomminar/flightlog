package importer

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var (
	// ErrInvalidFlightyCSV is returned when required columns are missing.
	ErrInvalidFlightyCSV = errors.New("invalid Flighty CSV: missing required columns")
	// ErrMissingFields is returned for rows missing required fields.
	ErrMissingFields = errors.New("missing required fields")
	// ErrCanceledFlight is returned for canceled flight rows.
	ErrCanceledFlight = errors.New("canceled flight")
	// ErrUnsupportedDateFormat is returned for unparseable dates.
	ErrUnsupportedDateFormat = errors.New("unsupported date format")
	// ErrUnsupportedTimestampFormat is returned for unparseable timestamps.
	ErrUnsupportedTimestampFormat = errors.New("unsupported timestamp format")
)

var flightyRequiredColumns = []string{ //nolint:gochecknoglobals // column schema
	"Date", "Airline", "Flight", "From", "To",
}

// FlightyAdapter parses CSV exports from the Flighty app.
type FlightyAdapter struct{}

// Name returns "flighty".
func (a *FlightyAdapter) Name() string {
	return "flighty"
}

// Parse reads a Flighty CSV and returns import entries.
func (a *FlightyAdapter) Parse(r io.Reader) ([]ImportEntry, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true

	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV header: %w", err)
	}

	colIdx := buildColumnIndex(header)

	err = validateRequiredColumns(colIdx)
	if err != nil {
		return nil, err
	}

	return parseFlightyRows(reader, colIdx)
}

func buildColumnIndex(header []string) map[string]int {
	idx := make(map[string]int, len(header))
	for i, col := range header {
		idx[strings.TrimSpace(col)] = i
	}

	return idx
}

func validateRequiredColumns(colIdx map[string]int) error {
	for _, col := range flightyRequiredColumns {
		if _, ok := colIdx[col]; !ok {
			return fmt.Errorf("%w: %s", ErrInvalidFlightyCSV, col)
		}
	}

	return nil
}

func parseFlightyRows(reader *csv.Reader, colIdx map[string]int) ([]ImportEntry, error) {
	var entries []ImportEntry

	for {
		row, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("failed to read CSV row: %w", err)
		}

		entry, err := parseFlightyRow(row, colIdx)
		if err != nil {
			continue
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

func parseFlightyRow(row []string, colIdx map[string]int) (ImportEntry, error) {
	getCol := func(name string) string {
		if idx, ok := colIdx[name]; ok && idx < len(row) {
			return strings.TrimSpace(row[idx])
		}

		return ""
	}

	entry, err := buildFlightyEntry(getCol)
	if err != nil {
		return ImportEntry{}, err
	}

	setFlightyTimes(&entry, getCol)

	return entry, nil
}

func buildFlightyEntry(getCol func(string) string) (ImportEntry, error) {
	dateStr := getCol("Date")
	airline := getCol("Airline")
	flightNum := getCol("Flight")
	from := getCol("From")
	to := getCol("To")

	if dateStr == "" || flightNum == "" || from == "" || to == "" {
		return ImportEntry{}, ErrMissingFields
	}

	if isCanceled(getCol("Canceled")) {
		return ImportEntry{}, ErrCanceledFlight
	}

	date, err := parseFlightyDate(dateStr)
	if err != nil {
		return ImportEntry{}, fmt.Errorf("invalid date %q: %w", dateStr, err)
	}

	// Build IATA flight number: "EK" + "565" → "EK565".
	normalizedFlight := strings.ToUpper(strings.ReplaceAll(airline+flightNum, " ", ""))

	return ImportEntry{
		FlightNumber: normalizedFlight,
		Date:         date,
		DepIATA:      strings.ToUpper(from),
		ArrIATA:      strings.ToUpper(to),
		Airline:      strings.ToUpper(airline),
		Aircraft:     getCol("Aircraft Type Name"),
		AircraftReg:  getCol("Tail Number"),
		DepTerminal:  getCol("Dep Terminal"),
		DepGate:      getCol("Dep Gate"),
		ArrTerminal:  getCol("Arr Terminal"),
		ArrGate:      getCol("Arr Gate"),
	}, nil
}

func isCanceled(value string) bool {
	return strings.EqualFold(value, "true") || strings.EqualFold(value, "1")
}

func setFlightyTimes(entry *ImportEntry, getCol func(string) string) {
	if depTimeStr := getCol("Gate Departure (Scheduled)"); depTimeStr != "" {
		t, err := parseFlightyTimestamp(depTimeStr)
		if err == nil {
			entry.DepTime = &t
		}
	}

	if arrTimeStr := getCol("Gate Arrival (Scheduled)"); arrTimeStr != "" {
		t, err := parseFlightyTimestamp(arrTimeStr)
		if err == nil {
			entry.ArrTime = &t
		}
	}
}

func parseFlightyDate(s string) (string, error) {
	for _, layout := range []string{
		"2006-01-02",
		"01/02/2006",
		"1/2/2006",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.Format("2006-01-02"), nil
		}
	}

	return "", fmt.Errorf("%w: %s", ErrUnsupportedDateFormat, s)
}

func parseFlightyTimestamp(s string) (time.Time, error) {
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04",
		"1/2/2006 3:04 PM",
		"01/02/2006 15:04",
	} {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: %s", ErrUnsupportedTimestampFormat, s)
}
