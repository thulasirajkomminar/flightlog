// Package importer handles flight data import from external apps.
package importer

import (
	"io"
	"time"
)

// ImportEntry is a flight parsed from an import file.
type ImportEntry struct {
	FlightNumber string     `json:"flightNumber,omitempty"`
	Date         string     `json:"date,omitempty"`
	DepIATA      string     `json:"depIata,omitempty"`
	ArrIATA      string     `json:"arrIata,omitempty"`
	Airline      string     `json:"airline,omitempty"`
	Aircraft     string     `json:"aircraft,omitempty"`
	Status       string     `json:"status,omitempty"`
	DepTime      *time.Time `json:"depTime,omitempty"`
	ArrTime      *time.Time `json:"arrTime,omitempty"`

	// Optional fields populated by adapters with complete data (e.g. Flightlog export).
	DepICAO      string  `json:"depIcao,omitempty"`
	DepName      string  `json:"depName,omitempty"`
	DepCity      string  `json:"depCity,omitempty"`
	DepCountry   string  `json:"depCountry,omitempty"`
	DepLat       float64 `json:"depLat,omitempty"`
	DepLon       float64 `json:"depLon,omitempty"`
	DepTimeLocal string  `json:"depTimeLocal,omitempty"`
	DepTerminal  string  `json:"depTerminal,omitempty"`
	DepGate      string  `json:"depGate,omitempty"`
	ArrICAO      string  `json:"arrIcao,omitempty"`
	ArrName      string  `json:"arrName,omitempty"`
	ArrCity      string  `json:"arrCity,omitempty"`
	ArrCountry   string  `json:"arrCountry,omitempty"`
	ArrLat       float64 `json:"arrLat,omitempty"`
	ArrLon       float64 `json:"arrLon,omitempty"`
	ArrTimeLocal string  `json:"arrTimeLocal,omitempty"`
	ArrTerminal  string  `json:"arrTerminal,omitempty"`
	ArrGate      string  `json:"arrGate,omitempty"`
	AircraftReg  string  `json:"aircraftReg,omitempty"`
	AirlineName  string  `json:"airlineName,omitempty"`
	AirlineICAO  string  `json:"airlineIcao,omitempty"`
	DistanceKm   float64 `json:"distanceKm,omitempty"`
}

// Adapter parses a file from a specific app into import entries.
type Adapter interface {
	Name() string
	Parse(r io.Reader) ([]ImportEntry, error)
}
