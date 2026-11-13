// Package airport provides persistent caching for airport lookups.
package airport

import (
	"context"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

// Record is a cached airport entry.
type Record struct {
	IATA             string  `gorm:"primaryKey;column:iata"`
	ICAO             string  `gorm:"column:icao"`
	Name             string  `gorm:"column:name"`
	ShortName        string  `gorm:"column:short_name"`
	MunicipalityName string  `gorm:"column:municipality_name"`
	Lat              float64 `gorm:"column:lat"`
	Lon              float64 `gorm:"column:lon"`
	CountryCode      string  `gorm:"column:country_code"`
	TimeZone         string  `gorm:"column:time_zone"`
}

// TableName overrides the GORM table name.
func (*Record) TableName() string {
	return "airport_cache"
}

func (r *Record) toDomain() *domain.Airport {
	return &domain.Airport{
		IATA:             r.IATA,
		ICAO:             r.ICAO,
		Name:             r.Name,
		ShortName:        r.ShortName,
		MunicipalityName: r.MunicipalityName,
		Location:         domain.Location{Lat: r.Lat, Lon: r.Lon},
		CountryCode:      r.CountryCode,
		TimeZone:         r.TimeZone,
	}
}

func recordFromDomain(a *domain.Airport) *Record {
	return &Record{
		IATA:             a.IATA,
		ICAO:             a.ICAO,
		Name:             a.Name,
		ShortName:        a.ShortName,
		MunicipalityName: a.MunicipalityName,
		Lat:              a.Location.Lat,
		Lon:              a.Location.Lon,
		CountryCode:      a.CountryCode,
		TimeZone:         a.TimeZone,
	}
}

// Store persists airport data in SQLite.
type Store struct {
	db *gorm.DB
}

// NewStore returns a new store backed by db.
func NewStore(db *gorm.DB) *Store {
	return &Store{db: db}
}

// FindAirport looks up a cached airport by IATA code.
func (s *Store) FindAirport(ctx context.Context, iata string) (*domain.Airport, error) {
	var rec Record

	err := s.db.WithContext(ctx).Where("iata = ?", iata).First(&rec).Error
	if err != nil {
		return nil, fmt.Errorf("find airport %s: %w", iata, err)
	}

	return rec.toDomain(), nil
}

// SaveAirport inserts or updates an airport.
func (s *Store) SaveAirport(ctx context.Context, a *domain.Airport) error {
	rec := recordFromDomain(a)

	return s.db.WithContext(ctx).Clauses(clause.OnConflict{UpdateAll: true}).Create(rec).Error
}
