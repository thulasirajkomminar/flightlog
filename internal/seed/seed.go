// Package seed provides functions to seed the database with dummy flight data for development/testing.
package seed

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/thulasirajkomminar/flightlog/internal/domain"
)

const seedPassword = "flightlog"

func Run(db *gorm.DB) {
	var count int64

	db.Model(&domain.Flight{}).Count(&count)

	if count > 0 {
		log.Println("seed: skipping — data already exists")

		return
	}

	log.Println("seed: seeding 100 flights...")

	r := rand.New(rand.NewSource(42)) //nolint:gosec // test data

	user := seedUser(db)
	seedFlights(db, r, user.ID)

	log.Println("seed: done")
}

func Reset(db *gorm.DB) {
	log.Println("seed: resetting...")
	db.Exec("DELETE FROM user_flights")
	db.Exec("DELETE FROM flights")

	r := rand.New(rand.NewSource(42)) //nolint:gosec // test data

	var user domain.User

	db.First(&user)

	if user.ID == "" {
		user = seedUser(db)
	}

	seedFlights(db, r, user.ID)
	log.Println("seed: reset done")
}

const seedUserID = "0f9d3701-28cd-4cbe-bfe8-1bb8c21c32e8"

func seedUser(db *gorm.DB) domain.User {
	hash, err := bcrypt.GenerateFromPassword([]byte(seedPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("seed: hash password: %v", err)
	}

	u := domain.User{
		ID:           seedUserID,
		Email:        "thulasiraj@flightlog.app",
		PasswordHash: string(hash),
		Name:         "Thulasiraj Komminar",
	}

	err = db.FirstOrCreate(&u, "id = ?", u.ID).Error
	if err != nil {
		log.Fatalf("seed: user: %v", err)
	}

	log.Printf("seed: user created (email: %s, password: %s)", u.Email, seedPassword)

	return u
}

type airportData struct {
	ICAO, IATA, Name, ShortName, Municipality string
	Lat, Lon                                  float64
	Country, TZ                               string
}

type airlineData struct {
	Name, IATA, ICAO string
}

type aircraftData struct {
	Reg, ModeS, Model string
}

var airportsList = []airportData{
	{"EHAM", "AMS", "Amsterdam Schiphol", "Schiphol", "Amsterdam", 52.3086, 4.7639, "NL", "Europe/Amsterdam"},
	{"EGLL", "LHR", "London Heathrow", "Heathrow", "London", 51.4700, -0.4543, "GB", "Europe/London"},
	{"KJFK", "JFK", "John F Kennedy Intl", "JFK", "New York", 40.6413, -73.7781, "US", "America/New_York"},
	{"KLAX", "LAX", "Los Angeles Intl", "LAX", "Los Angeles", 33.9425, -118.4081, "US", "America/Los_Angeles"},
	{"OMDB", "DXB", "Dubai Intl", "Dubai", "Dubai", 25.2532, 55.3657, "AE", "Asia/Dubai"},
	{"WSSS", "SIN", "Singapore Changi", "Changi", "Singapore", 1.3502, 103.9944, "SG", "Asia/Singapore"},
	{"RJTT", "HND", "Tokyo Haneda", "Haneda", "Tokyo", 35.5494, 139.7798, "JP", "Asia/Tokyo"},
	{"YSSY", "SYD", "Sydney Kingsford Smith", "Sydney", "Sydney", -33.9461, 151.1772, "AU", "Australia/Sydney"},
	{"LFPG", "CDG", "Paris Charles de Gaulle", "CDG", "Paris", 49.0097, 2.5479, "FR", "Europe/Paris"},
	{"EDDF", "FRA", "Frankfurt Airport", "Frankfurt", "Frankfurt", 50.0333, 8.5706, "DE", "Europe/Berlin"},
	{"ZBAA", "PEK", "Beijing Capital Intl", "Beijing Capital", "Beijing", 40.0799, 116.6031, "CN", "Asia/Shanghai"},
	{"VHHH", "HKG", "Hong Kong Intl", "Hong Kong", "Hong Kong", 22.3080, 113.9185, "HK", "Asia/Hong_Kong"},
	{"RKSI", "ICN", "Incheon Intl", "Incheon", "Seoul", 37.4602, 126.4407, "KR", "Asia/Seoul"},
	{"KSFO", "SFO", "San Francisco Intl", "SFO", "San Francisco", 37.6213, -122.3790, "US", "America/Los_Angeles"},
	{"PHNL", "HNL", "Daniel K Inouye Intl", "Honolulu", "Honolulu", 21.3187, -157.9224, "US", "Pacific/Honolulu"},
	{"KMIA", "MIA", "Miami Intl", "Miami", "Miami", 25.7959, -80.2870, "US", "America/New_York"},
	{"SBGR", "GRU", "Sao Paulo Guarulhos", "Guarulhos", "Sao Paulo", -23.4356, -46.4731, "BR", "America/Sao_Paulo"},
	{"SAEZ", "EZE", "Buenos Aires Ezeiza", "Ezeiza", "Buenos Aires", -34.8222, -58.5358, "AR", "America/Argentina/Buenos_Aires"},
	{"SCEL", "SCL", "Santiago Arturo Merino", "Santiago", "Santiago", -33.3930, -70.7858, "CL", "America/Santiago"},
	{"FACT", "CPT", "Cape Town Intl", "Cape Town", "Cape Town", -33.9649, 18.6017, "ZA", "Africa/Johannesburg"},
	{"FAOR", "JNB", "O R Tambo Intl", "Johannesburg", "Johannesburg", -26.1392, 28.2460, "ZA", "Africa/Johannesburg"},
	{"LTFM", "IST", "Istanbul Airport", "Istanbul", "Istanbul", 41.2753, 28.7519, "TR", "Europe/Istanbul"},
	{"HECA", "CAI", "Cairo Intl", "Cairo", "Cairo", 30.1219, 31.4056, "EG", "Africa/Cairo"},
	{"OTHH", "DOH", "Hamad Intl", "Hamad", "Doha", 25.2731, 51.6081, "QA", "Asia/Qatar"},
	{"VTBS", "BKK", "Suvarnabhumi Airport", "Suvarnabhumi", "Bangkok", 13.6900, 100.7501, "TH", "Asia/Bangkok"},
	{"YMML", "MEL", "Melbourne Airport", "Melbourne", "Melbourne", -37.6733, 144.8433, "AU", "Australia/Melbourne"},
	{"NZAA", "AKL", "Auckland Airport", "Auckland", "Auckland", -37.0082, 174.7917, "NZ", "Pacific/Auckland"},
	{"MMMX", "MEX", "Mexico City Intl", "Mexico City", "Mexico City", 19.4363, -99.0721, "MX", "America/Mexico_City"},
	{"SKBO", "BOG", "El Dorado Intl", "El Dorado", "Bogota", 4.7016, -74.1469, "CO", "America/Bogota"},
	{"LIRF", "FCO", "Rome Fiumicino", "Fiumicino", "Rome", 41.8003, 12.2389, "IT", "Europe/Rome"},
	{"LGAV", "ATH", "Athens Eleftherios", "Athens", "Athens", 37.9364, 23.9445, "GR", "Europe/Athens"},
	{"EDDM", "MUC", "Munich Airport", "Munich", "Munich", 48.3538, 11.7861, "DE", "Europe/Berlin"},
	{"LOWW", "VIE", "Vienna Intl", "Vienna", "Vienna", 48.1103, 16.5697, "AT", "Europe/Vienna"},
	{"LSZH", "ZRH", "Zurich Airport", "Zurich", "Zurich", 47.4647, 8.5492, "CH", "Europe/Zurich"},
	{"LEBL", "BCN", "Barcelona El Prat", "El Prat", "Barcelona", 41.2971, 2.0785, "ES", "Europe/Madrid"},
	{"ENGM", "OSL", "Oslo Gardermoen", "Gardermoen", "Oslo", 60.1939, 11.1004, "NO", "Europe/Oslo"},
	{"EKCH", "CPH", "Copenhagen Kastrup", "Kastrup", "Copenhagen", 55.6180, 12.6508, "DK", "Europe/Copenhagen"},
	{"EFHK", "HEL", "Helsinki Vantaa", "Vantaa", "Helsinki", 60.3172, 24.9633, "FI", "Europe/Helsinki"},
	{"EIDW", "DUB", "Dublin Airport", "Dublin", "Dublin", 53.4213, -6.2701, "IE", "Europe/Dublin"},
	{"LPPT", "LIS", "Lisbon Portela", "Portela", "Lisbon", 38.7813, -9.1359, "PT", "Europe/Lisbon"},
	{"GCTS", "TFS", "Tenerife South", "Tenerife South", "Tenerife Island", 28.0445, -16.5725, "ES", "Atlantic/Canary"},
	{"KORD", "ORD", "Chicago O Hare Intl", "O Hare", "Chicago", 41.9742, -87.9073, "US", "America/Chicago"},
	{"KDEN", "DEN", "Denver Intl", "Denver", "Denver", 39.8561, -104.6737, "US", "America/Denver"},
	{"PANC", "ANC", "Ted Stevens Anchorage", "Anchorage", "Anchorage", 61.1743, -149.9962, "US", "America/Anchorage"},
	{"CYYZ", "YYZ", "Toronto Pearson Intl", "Pearson", "Toronto", 43.6777, -79.6248, "CA", "America/Toronto"},
	{"CYVR", "YVR", "Vancouver Intl", "Vancouver", "Vancouver", 49.1967, -123.1815, "CA", "America/Vancouver"},
	{"VIDP", "DEL", "Indira Gandhi Intl", "Delhi", "Delhi", 28.5562, 77.1000, "IN", "Asia/Kolkata"},
	{"VABB", "BOM", "Chhatrapati Shivaji", "Mumbai", "Mumbai", 19.0896, 72.8656, "IN", "Asia/Kolkata"},
	{"WMKK", "KUL", "Kuala Lumpur Intl", "KLIA", "Kuala Lumpur", 2.7456, 101.7099, "MY", "Asia/Kuala_Lumpur"},
	{"WIII", "CGK", "Soekarno Hatta Intl", "Soekarno Hatta", "Jakarta", -6.1256, 106.6558, "ID", "Asia/Jakarta"},
	{"HAAB", "ADD", "Addis Ababa Bole", "Bole", "Addis Ababa", 8.9779, 38.7993, "ET", "Africa/Addis_Ababa"},
	{"JKIA", "NBO", "Jomo Kenyatta Intl", "Jomo Kenyatta", "Nairobi", -1.3192, 36.9278, "KE", "Africa/Nairobi"},
	{"LEMD", "MAD", "Madrid Barajas", "Barajas", "Madrid", 40.4983, -3.5676, "ES", "Europe/Madrid"},
	{"ESSA", "ARN", "Stockholm Arlanda", "Arlanda", "Stockholm", 59.6519, 17.9186, "SE", "Europe/Stockholm"},
	{"EPWA", "WAW", "Warsaw Chopin", "Chopin", "Warsaw", 52.1657, 20.9671, "PL", "Europe/Warsaw"},
	{"LKPR", "PRG", "Prague Vaclav Havel", "Vaclav Havel", "Prague", 50.1008, 14.2600, "CZ", "Europe/Prague"},
	{"LHBP", "BUD", "Budapest Liszt Ferenc", "Budapest", "Budapest", 47.4369, 19.2556, "HU", "Europe/Budapest"},
	{"LLBG", "TLV", "Ben Gurion Intl", "Ben Gurion", "Tel Aviv", 32.0114, 34.8867, "IL", "Asia/Jerusalem"},
	{"WADD", "DPS", "Ngurah Rai Intl", "Bali", "Bali", -8.7482, 115.1672, "ID", "Asia/Jakarta"},
	{"RPLL", "MNL", "Ninoy Aquino Intl", "Manila", "Manila", 14.5086, 121.0198, "PH", "Asia/Manila"},
	{"RCTP", "TPE", "Taiwan Taoyuan Intl", "Taoyuan", "Taipei", 25.0777, 121.2325, "TW", "Asia/Taipei"},
	{"VVNB", "HAN", "Noi Bai Intl", "Hanoi", "Hanoi", 21.2212, 105.8070, "VN", "Asia/Ho_Chi_Minh"},
	{"SPJC", "LIM", "Jorge Chavez Intl", "Lima", "Lima", -12.0219, -77.1143, "PE", "America/Lima"},
	{"SEQM", "UIO", "Quito Mariscal Sucre", "Quito", "Quito", -0.1292, -78.3575, "EC", "America/Guayaquil"},
	{"KATL", "ATL", "Hartsfield Jackson Atlanta", "Atlanta", "Atlanta", 33.6407, -84.4277, "US", "America/New_York"},
	{"KBOS", "BOS", "Boston Logan Intl", "Boston", "Boston", 42.3656, -71.0096, "US", "America/New_York"},
	{"BIKF", "KEF", "Keflavik Intl", "Keflavik", "Reykjavik", 63.9850, -22.6056, "IS", "Atlantic/Reykjavik"},
	{"EETN", "TLL", "Tallinn Lennart Meri", "Tallinn", "Tallinn", 59.4133, 24.8328, "EE", "Europe/Tallinn"},
	{"EVRA", "RIX", "Riga Intl", "Riga", "Riga", 56.9236, 23.9711, "LV", "Europe/Riga"},
	{"UGTB", "TBS", "Tbilisi Intl", "Tbilisi", "Tbilisi", 41.6692, 44.9547, "GE", "Asia/Tbilisi"},
}

var airlinesList = []airlineData{
	{"KLM Royal Dutch Airlines", "KL", "KLM"},
	{"British Airways", "BA", "BAW"},
	{"Lufthansa", "LH", "DLH"},
	{"Air France", "AF", "AFR"},
	{"Emirates", "EK", "UAE"},
	{"Singapore Airlines", "SQ", "SIA"},
	{"Qatar Airways", "QR", "QTR"},
	{"Turkish Airlines", "TK", "THY"},
	{"Delta Air Lines", "DL", "DAL"},
	{"United Airlines", "UA", "UAL"},
	{"American Airlines", "AA", "AAL"},
	{"Qantas", "QF", "QFA"},
	{"Air New Zealand", "NZ", "ANZ"},
	{"LATAM Airlines", "LA", "LAN"},
	{"Ethiopian Airlines", "ET", "ETH"},
	{"Cathay Pacific", "CX", "CPA"},
	{"Korean Air", "KE", "KAL"},
	{"Japan Airlines", "JL", "JAL"},
	{"SAS Scandinavian", "SK", "SAS"},
	{"Swiss Intl Air Lines", "LX", "SWR"},
	{"Iberia", "IB", "IBE"},
	{"Finnair", "AY", "FIN"},
	{"Aer Lingus", "EI", "EIN"},
	{"TAP Air Portugal", "TP", "TAP"},
	{"easyJet", "U2", "EZY"},
	{"Ryanair", "FR", "RYR"},
	{"Norwegian", "DY", "NAX"},
	{"IndiGo", "6E", "IGO"},
	{"ANA", "NH", "ANA"},
	{"Garuda Indonesia", "GA", "GIA"},
}

var aircraftList = []aircraftData{
	{"OE-LSV", "440075", "Airbus A321-200 (Sharklets)"},
	{"PH-BHA", "484125", "Boeing 787-9 Dreamliner"},
	{"G-XWBA", "406A01", "Airbus A350-1041"},
	{"D-AIMC", "3C6587", "Airbus A380-841"},
	{"A6-ENA", "896452", "Boeing 777-31H(ER)"},
	{"9V-SMA", "76CC12", "Airbus A350-941"},
	{"A7-BEA", "06A1B5", "Boeing 777-3DZ(ER)"},
	{"TC-JJE", "4BA863", "Boeing 777-3F2(ER)"},
	{"N501DN", "A62EC1", "Airbus A350-941"},
	{"N78511", "ABF5A1", "Boeing 737 MAX 9"},
	{"N795AN", "AC1DAD", "Boeing 777-223(ER)"},
	{"VH-ZNA", "7C822A", "Boeing 787-9 Dreamliner"},
	{"ZK-NZE", "C8200E", "Boeing 787-9 Dreamliner"},
	{"CC-BGA", "E48DA5", "Boeing 787-9 Dreamliner"},
	{"ET-AVJ", "040B9E", "Airbus A350-941"},
	{"B-LRA", "780D28", "Airbus A350-941"},
	{"HL8226", "71C482", "Boeing 787-9 Dreamliner"},
	{"JA873J", "86E7FD", "Boeing 787-9 Dreamliner"},
	{"SE-RSA", "4AC812", "Airbus A350-941"},
	{"HB-JNA", "4B1813", "Boeing 777-3DE(ER)"},
	{"EC-MYX", "34169A", "Airbus A350-941"},
	{"OH-LWA", "461E8A", "Airbus A350-941"},
	{"EI-DAA", "4CA201", "Airbus A320-214"},
	{"CS-TJR", "49405B", "Airbus A321neo"},
	{"OE-IVA", "440101", "Airbus A320-214"},
	{"EI-DCL", "4CA2AC", "Boeing 737-8AS"},
	{"LN-DYG", "478C67", "Boeing 737-8JP"},
	{"VT-ITA", "800C01", "Airbus A320neo"},
	{"JA812A", "86D2E8", "Boeing 787-8"},
	{"PK-GIA", "8A0021", "Boeing 777-3U3(ER)"},
}

var routes = [][2]int{
	{0, 1},
	{1, 2},
	{2, 3},
	{3, 13},
	{13, 14},
	{4, 5},
	{5, 6},
	{6, 7},
	{7, 25},
	{25, 26},
	{8, 2},
	{9, 10},
	{10, 11},
	{11, 12},
	{12, 6},
	{1, 4},
	{4, 23},
	{23, 24},
	{24, 48},
	{48, 49},
	{15, 16},
	{16, 17},
	{17, 18},
	{18, 62},
	{62, 63},
	{19, 20},
	{20, 51},
	{51, 50},
	{50, 22},
	{22, 21},
	{21, 30},
	{30, 29},
	{29, 31},
	{31, 32},
	{32, 55},
	{33, 34},
	{34, 52},
	{52, 39},
	{39, 40},
	{40, 0},
	{35, 36},
	{36, 53},
	{53, 37},
	{37, 67},
	{67, 68},
	{38, 1},
	{0, 9},
	{9, 54},
	{54, 56},
	{56, 32},
	{8, 0},
	{0, 35},
	{1, 38},
	{33, 8},
	{9, 31},
	{3, 27},
	{27, 28},
	{28, 16},
	{41, 42},
	{42, 3},
	{2, 64},
	{64, 15},
	{65, 2},
	{13, 43},
	{43, 6},
	{44, 45},
	{45, 13},
	{46, 47},
	{47, 5},
	{5, 58},
	{58, 49},
	{49, 48},
	{48, 5},
	{5, 59},
	{59, 60},
	{60, 11},
	{11, 61},
	{61, 24},
	{24, 46},
	{46, 4},
	{4, 20},
	{57, 21},
	{21, 8},
	{8, 66},
	{66, 65},
	{65, 41},
	{41, 44},
	{44, 0},
	{0, 21},
	{21, 69},
	{69, 57},
	{52, 28},
	{63, 27},
	{27, 3},
	{3, 7},
	{7, 26},
	{26, 14},
	{14, 3},
	{10, 12},
	{12, 11},
	{20, 19},
}

func seedFlights(db *gorm.DB, r *rand.Rand, userID string) {
	statuses := weightedStatuses()
	terminals := []string{"1", "2", "3", "4", "A", "B", "C", ""}
	gateLetters := []string{"A", "B", "C", "D", "E", "F", "G"}

	yearStarts := []time.Time{
		time.Date(2022, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2023, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC),
		time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC),
	}

	flights := make([]domain.Flight, 0, len(routes))
	userFlights := make([]domain.UserFlight, 0, len(routes))

	for i, route := range routes {
		dep := airportsList[route[0]]
		arr := airportsList[route[1]]
		al := airlinesList[i%len(airlinesList)]
		ac := aircraftList[i%len(aircraftList)]
		flightNum := fmt.Sprintf("%s%d", al.IATA, 100+r.Intn(9900))
		yearIdx := i / 20

		if yearIdx >= len(yearStarts) {
			yearIdx = len(yearStarts) - 1
		}

		baseDate := yearStarts[yearIdx]
		flightDate := baseDate.AddDate(0, 0, (i%20)*17+r.Intn(11))
		depHour := 5 + r.Intn(18)
		depMin := r.Intn(12) * 5
		depTime := time.Date(flightDate.Year(), flightDate.Month(), flightDate.Day(),
			depHour, depMin, 0, 0, time.UTC)
		distM := haversine(dep.Lat, dep.Lon, arr.Lat, arr.Lon)
		distKM := distM / 1000
		durationMin := max(int(distKM/800*60)+r.Intn(31)-10, 45)
		arrTime := depTime.Add(time.Duration(durationMin) * time.Minute)
		delay := time.Duration(r.Intn(36)-5) * time.Minute
		depRevised := depTime.Add(delay)
		arrRevised := arrTime.Add(delay)
		status := statuses[r.Intn(len(statuses))]

		gate := func() string {
			return fmt.Sprintf("%s%d", gateLetters[r.Intn(len(gateLetters))], 1+r.Intn(60))
		}

		flightID := uuid.NewSHA1(uuid.NameSpaceDNS, fmt.Appendf(nil, "flight-seed-%d", i)).String()
		ufID := uuid.NewSHA1(uuid.NameSpaceDNS, fmt.Appendf(nil, "uf-seed-%d", i)).String()
		now := time.Now()

		f := domain.Flight{
			ID:              flightID,
			Number:          flightNum,
			FlightDate:      flightDate.Format("2006-01-02"),
			Status:          domain.FlightStatus(status),
			CallSign:        fmt.Sprintf("%s%d", al.ICAO, 10+r.Intn(90)),
			CodeshareStatus: "IsOperator",
			IsCargo:         false,
			Airline: domain.FlightAirline{
				Name: al.Name,
				IATA: al.IATA,
				ICAO: al.ICAO,
			},
			Aircraft: domain.FlightAircraft{
				Reg:   ac.Reg,
				ModeS: ac.ModeS,
				Model: ac.Model,
			},
			Departure: domain.Movement{
				Airport: domain.Airport{
					ICAO:             dep.ICAO,
					IATA:             dep.IATA,
					Name:             dep.Name,
					ShortName:        dep.ShortName,
					MunicipalityName: dep.Municipality,
					Location:         domain.Location{Lat: dep.Lat, Lon: dep.Lon},
					CountryCode:      dep.Country,
					TimeZone:         dep.TZ,
				},
				ScheduledTime: domain.TimeInfo{UTC: &depTime, Local: depTime.Format(time.RFC3339)},
				RevisedTime:   domain.TimeInfo{UTC: &depRevised, Local: depRevised.Format(time.RFC3339)},
				RunwayTime:    domain.TimeInfo{UTC: &depRevised, Local: depRevised.Format(time.RFC3339)},
				Terminal:      terminals[r.Intn(len(terminals))],
				Gate:          gate(),
			},
			Arrival: domain.Movement{
				Airport: domain.Airport{
					ICAO:             arr.ICAO,
					IATA:             arr.IATA,
					Name:             arr.Name,
					ShortName:        arr.ShortName,
					MunicipalityName: arr.Municipality,
					Location:         domain.Location{Lat: arr.Lat, Lon: arr.Lon},
					CountryCode:      arr.Country,
					TimeZone:         arr.TZ,
				},
				ScheduledTime: domain.TimeInfo{UTC: &arrTime, Local: arrTime.Format(time.RFC3339)},
				RevisedTime:   domain.TimeInfo{UTC: &arrRevised, Local: arrRevised.Format(time.RFC3339)},
				RunwayTime:    domain.TimeInfo{UTC: &arrRevised, Local: arrRevised.Format(time.RFC3339)},
				Terminal:      terminals[r.Intn(len(terminals))],
				Gate:          gate(),
			},
			GreatCircleDistance: domain.GreatCircleDistance{
				Meter: distM,
				Km:    distKM,
				Mile:  distKM * 0.621371,
				Nm:    distKM * 0.539957,
				Feet:  distM * 3.28084,
			},
			LastUpdatedUtc: &arrRevised,
			Provider:       "seed",
			CreatedAt:      now,
			UpdatedAt:      now,
		}

		flights = append(flights, f)
		userFlights = append(userFlights, domain.UserFlight{
			ID:        ufID,
			UserID:    userID,
			FlightID:  flightID,
			CreatedAt: now,
		})
	}

	err := db.Transaction(func(tx *gorm.DB) error {
		err := tx.CreateInBatches(flights, 50).Error
		if err != nil {
			return fmt.Errorf("flights: %w", err)
		}

		err = tx.CreateInBatches(userFlights, 50).Error
		if err != nil {
			return fmt.Errorf("user_flights: %w", err)
		}

		return nil
	})
	if err != nil {
		log.Fatalf("seed: %v", err)
	}

	log.Printf("seed: inserted %d flights for user %s", len(flights), userID)
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const r = 6371000

	p1 := lat1 * math.Pi / 180
	p2 := lat2 * math.Pi / 180
	dp := (lat2 - lat1) * math.Pi / 180
	dl := (lon2 - lon1) * math.Pi / 180
	a := math.Sin(dp/2)*math.Sin(dp/2) +
		math.Cos(p1)*math.Cos(p2)*math.Sin(dl/2)*math.Sin(dl/2)

	return r * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func weightedStatuses() []string {
	weights := []struct {
		status string
		count  int
	}{
		{"landed", 70},
		{"scheduled", 10},
		{"cancelled", 8},
		{"diverted", 5},
		{"active", 5},
		{"incident", 2},
	}

	var s []string

	for _, w := range weights {
		for range w.count {
			s = append(s, w.status)
		}
	}

	return s
}
