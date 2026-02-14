// Package domain defines core business entities and validation rules.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// FlightStatus is an enum of possible flight lifecycle states.
type FlightStatus string

// Possible flight statuses.
const (
	FlightStatusScheduled FlightStatus = "scheduled"
	FlightStatusActive    FlightStatus = "active"
	FlightStatusLanded    FlightStatus = "landed"
	FlightStatusCancelled FlightStatus = "cancelled"
	FlightStatusIncident  FlightStatus = "incident"
	FlightStatusDiverted  FlightStatus = "diverted"
)

// Location holds a WGS84 latitude/longitude pair.
type Location struct {
	Lat float64 `json:"lat" gorm:"column:lat"`
	Lon float64 `json:"lon" gorm:"column:lon"`
}

// Airport holds IATA/ICAO codes, name, location, and timezone.
type Airport struct {
	ICAO             string   `json:"icao,omitempty"             example:"KSFO"`
	IATA             string   `json:"iata"                       example:"SFO"`
	Name             string   `json:"name"                       example:"San Francisco International"`
	ShortName        string   `json:"shortName,omitempty"        example:"San Francisco"`
	MunicipalityName string   `json:"municipalityName,omitempty" example:"San Francisco"`
	Location         Location `json:"location"                   gorm:"embedded"`
	CountryCode      string   `json:"countryCode,omitempty"      example:"US"`
	TimeZone         string   `json:"timeZone,omitempty"         example:"America/Los_Angeles"`
}

// TimeInfo pairs a UTC timestamp with its local-time string.
type TimeInfo struct {
	UTC   *time.Time `json:"utc,omitempty"   example:"2026-03-17T08:00:00Z"`
	Local string     `json:"local,omitempty" example:"2026-03-17T01:00:00-07:00"`
}

// Movement captures one end of a flight: airport, times, terminal, gate, etc.
type Movement struct {
	Airport       Airport  `json:"airport"               gorm:"embedded;embeddedPrefix:airport_"`
	ScheduledTime TimeInfo `json:"scheduledTime"         gorm:"embedded;embeddedPrefix:scheduled_"`
	RevisedTime   TimeInfo `json:"revisedTime"           gorm:"embedded;embeddedPrefix:revised_"`
	RunwayTime    TimeInfo `json:"runwayTime"            gorm:"embedded;embeddedPrefix:runway_"`
	Terminal      string   `json:"terminal,omitempty"    example:"1"`
	Gate          string   `json:"gate,omitempty"        example:"A12"`
	CheckInDesk   string   `json:"checkInDesk,omitempty" example:"221-228"`
	BaggageBelt   string   `json:"baggageBelt,omitempty" example:"3"`
}

// GreatCircleDistance holds the route distance in multiple units.
type GreatCircleDistance struct {
	Meter float64 `json:"meter" example:"3223194.53"`
	Km    float64 `json:"km"    example:"3223.19"`
	Mile  float64 `json:"mile"  example:"2002.8"`
	Nm    float64 `json:"nm"    example:"1740.39"`
	Feet  float64 `json:"feet"  example:"10574785.22"`
}

// FlightAircraft holds registration and model info for an aircraft.
type FlightAircraft struct {
	Reg   string `json:"reg,omitempty"   example:"OE-LSV"`
	ModeS string `json:"modeS,omitempty" example:"440075"`
	Model string `json:"model,omitempty" example:"Airbus A321-200"`
}

// FlightAirline holds IATA/ICAO codes and name for an airline.
type FlightAirline struct {
	Name string `json:"name"           example:"United Airlines"`
	IATA string `json:"iata,omitempty" example:"UA"`
	ICAO string `json:"icao,omitempty" example:"UAL"`
}

// Flight represents a cached flight entity.
type Flight struct {
	ID                  string              `json:"id"                        gorm:"primaryKey"                        example:"123e4567-e89b-12d3-a456-426614174000"`
	Number              string              `json:"number"                    gorm:"column:flight_number;index"        example:"U2 7898"`
	FlightDate          string              `json:"flightDate"                gorm:"column:flight_date;index"          example:"2026-03-17"`
	Status              FlightStatus        `json:"status"                    example:"scheduled"`
	CallSign            string              `json:"callSign,omitempty"        example:"EJU19LD"`
	CodeshareStatus     string              `json:"codeshareStatus,omitempty" example:"IsOperator"`
	IsCargo             bool                `json:"isCargo"`
	Airline             FlightAirline       `json:"airline"                   gorm:"embedded;embeddedPrefix:airline_"`
	Aircraft            FlightAircraft      `json:"aircraft"                  gorm:"embedded;embeddedPrefix:aircraft_"`
	Departure           Movement            `json:"departure"                 gorm:"embedded;embeddedPrefix:dep_"`
	Arrival             Movement            `json:"arrival"                   gorm:"embedded;embeddedPrefix:arr_"`
	GreatCircleDistance GreatCircleDistance `json:"greatCircleDistance"       gorm:"embedded;embeddedPrefix:gcd_"`
	LastUpdatedUtc      *time.Time          `json:"lastUpdatedUtc,omitempty"`
	Provider            string              `json:"provider"                  example:"aerodatabox"`
	RawData             string              `json:"-"                         gorm:"type:text"`
	CreatedAt           time.Time           `json:"createdAt"                 example:"2026-03-17T12:00:00Z"`
	UpdatedAt           time.Time           `json:"updatedAt"                 example:"2026-03-17T12:00:00Z"`
}

// UserFlight represents the link between a user and a cached flight.
type UserFlight struct {
	ID        string    `json:"id"        gorm:"primaryKey"                                            example:"uf-abc-123"`
	UserID    string    `json:"userId"    gorm:"column:user_id;uniqueIndex:idx_user_flight;not null"   example:"user-abc-123"`
	FlightID  string    `json:"flightId"  gorm:"column:flight_id;uniqueIndex:idx_user_flight;not null" example:"123e4567-e89b-12d3-a456-426614174000"`
	Flight    Flight    `json:"flight"    gorm:"foreignKey:FlightID"`
	CreatedAt time.Time `json:"createdAt"`
}

// FlightSearchCriteria for searching flights.
type FlightSearchCriteria struct {
	UserID string
	Year   int
	Limit  int
	Offset int
}

// FlightStats holds aggregated flight statistics for a user.
type FlightStats struct {
	Flights    int     `json:"flights"`
	Distance   float64 `json:"distance"`
	FlightTime float64 `json:"flightTime"`
	Airports   int     `json:"airports"`
	Airlines   int     `json:"airlines"`
}

// GenerateID creates a UUID for the flight if not already set.
func (f *Flight) GenerateID() {
	if f.ID == "" {
		f.ID = uuid.New().String()
	}
}

// GenerateID creates a UUID for the user-flight link if not already set.
func (uf *UserFlight) GenerateID() {
	if uf.ID == "" {
		uf.ID = uuid.New().String()
	}
}
