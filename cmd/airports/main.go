// Package main regenerates the bundled airport database from OurAirports.
//
// OurAirports (https://ourairports.com/data/) publishes its data in the public
// domain. Run with `task airports` to refresh internal/airport/data/airports.csv.
package main

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"
)

const (
	sourceURL      = "https://davidmegginson.github.io/ourairports-data/airports.csv"
	outputPath     = "internal/airport/data/airports.csv"
	requestTimeout = 2 * time.Minute
	outputPerm     = 0o644
	coordDecimals  = 5
	// coordinateFields is how many trailing fields of a row are coordinates.
	coordinateFields = 2
	floatBits        = 64
)

var (
	errDownload      = errors.New("download failed")
	errMissingColumn = errors.New("source is missing a column")
)

func main() {
	rows, err := download(context.Background())
	if err != nil {
		log.Fatalf("airports: %v", err)
	}

	err = write(rows)
	if err != nil {
		log.Fatalf("airports: %v", err)
	}

	log.Printf("airports: wrote %d airports to %s", len(rows), outputPath)
}

// download fetches the source and returns one output row per airport that has
// an IATA code: iata, icao, name, municipality, country, lat, lon.
func download(ctx context.Context) ([][]string, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", sourceURL, err)
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d", errDownload, resp.StatusCode)
	}

	return convert(csv.NewReader(resp.Body))
}

// wantedColumns are the source columns kept, in output order. The IATA code
// comes first and the coordinates last.
func wantedColumns() []string {
	return []string{"iata_code", "icao_code", "name", "municipality", "iso_country", "latitude_deg", "longitude_deg"}
}

func convert(reader *csv.Reader) ([][]string, error) {
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	indexes, err := columnIndexes(header)
	if err != nil {
		return nil, err
	}

	var rows [][]string

	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return nil, fmt.Errorf("read row: %w", err)
		}

		if row := pick(record, indexes); row[0] != "" {
			rows = append(rows, roundCoordinates(row))
		}
	}

	slices.SortFunc(rows, func(a, b []string) int { return strings.Compare(a[0], b[0]) })

	return rows, nil
}

// columnIndexes finds where each wanted column sits in the source header.
func columnIndexes(header []string) ([]int, error) {
	wanted := wantedColumns()
	indexes := make([]int, 0, len(wanted))

	for _, name := range wanted {
		i := slices.Index(header, name)
		if i < 0 {
			return nil, fmt.Errorf("%w: %s", errMissingColumn, name)
		}

		indexes = append(indexes, i)
	}

	return indexes, nil
}

func pick(record []string, indexes []int) []string {
	row := make([]string, 0, len(indexes))
	for _, i := range indexes {
		row = append(row, record[i])
	}

	return row
}

// roundCoordinates trims latitude and longitude (the last two fields) to about
// one metre of precision, which keeps the file small.
func roundCoordinates(row []string) []string {
	for i := len(row) - coordinateFields; i < len(row); i++ {
		value, err := strconv.ParseFloat(row[i], floatBits)
		if err == nil {
			row[i] = strconv.FormatFloat(value, 'f', coordDecimals, floatBits)
		}
	}

	return row
}

func write(rows [][]string) error {
	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, outputPerm)
	if err != nil {
		return fmt.Errorf("open %s: %w", outputPath, err)
	}

	defer func() { _ = file.Close() }()

	writer := csv.NewWriter(file)

	err = writer.Write([]string{"iata", "icao", "name", "municipality", "country", "lat", "lon"})
	if err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	err = writer.WriteAll(rows)
	if err != nil {
		return fmt.Errorf("write rows: %w", err)
	}

	return nil
}
