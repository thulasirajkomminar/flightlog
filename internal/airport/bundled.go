package airport

import (
	"bytes"
	_ "embed" // embeds the airport database
	"encoding/csv"
	"strconv"
	"strings"
	"sync"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// airportsCSV is the bundled airport database: every airport with an IATA
// code from OurAirports (public domain). Regenerate it with `task airports`.
//
//go:embed data/airports.csv
var airportsCSV []byte

const (
	bundledColumns = 7
	floatBits      = 64
)

// loadBundled parses the embedded database once, on first use.
//
//nolint:gochecknoglobals // lazily built lookup table for the embedded data
var loadBundled = sync.OnceValue(func() map[string]domain.Airport {
	records, err := csv.NewReader(bytes.NewReader(airportsCSV)).ReadAll()
	if err != nil || len(records) == 0 {
		return nil
	}

	airports := make(map[string]domain.Airport, len(records))

	// The first record is the header.
	for _, r := range records[1:] {
		if len(r) != bundledColumns {
			continue
		}

		lat, errLat := strconv.ParseFloat(r[5], floatBits)
		lon, errLon := strconv.ParseFloat(r[6], floatBits)

		if errLat != nil || errLon != nil {
			continue
		}

		airports[r[0]] = domain.Airport{
			IATA:             r[0],
			ICAO:             r[1],
			Name:             r[2],
			MunicipalityName: r[3],
			CountryCode:      r[4],
			Location:         domain.Location{Lat: lat, Lon: lon},
		}
	}

	return airports
})

// Bundled looks an airport up in the database shipped with the application,
// which costs no API call. It knows airports currently in service; a closed
// airport that lost its IATA code is not in it.
func Bundled(iata string) (*domain.Airport, bool) {
	a, ok := loadBundled()[strings.ToUpper(strings.TrimSpace(iata))]
	if !ok {
		return nil, false
	}

	return &a, true
}
