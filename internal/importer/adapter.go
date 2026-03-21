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
	Status       string
	DepTime      *time.Time
	ArrTime      *time.Time

	// Optional fields populated by adapters with complete data (e.g. Flightlog export).
	DepICAO      string
	DepName      string
	DepCity      string
	DepCountry   string
	DepLat       float64
	DepLon       float64
	DepTimeLocal string
	DepTerminal  string
	DepGate      string
	ArrICAO      string
	ArrName      string
	ArrCity      string
	ArrCountry   string
	ArrLat       float64
	ArrLon       float64
	ArrTimeLocal string
	ArrTerminal  string
	ArrGate      string
	AircraftReg  string
	AirlineName  string
	AirlineICAO  string
	DistanceKm   float64
}

// Adapter parses a file from a specific app into import entries.
type Adapter interface {
	Name() string
	Parse(r io.Reader) ([]ImportEntry, error)
}
