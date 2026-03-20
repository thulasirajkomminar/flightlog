// Package importer handles flight data import from external apps.
package importer

import (
	"io"
	"time"
)

// ImportEntry is a flight parsed from an import file.
type ImportEntry struct {
	FlightNumber string
	Date         string
	DepIATA      string
	ArrIATA      string
	Airline      string
	Aircraft     string
	DepTime      *time.Time
	ArrTime      *time.Time
}

// Adapter parses a file from a specific app into import entries.
type Adapter interface {
	Name() string
	Parse(r io.Reader) ([]ImportEntry, error)
}
