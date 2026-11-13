package domain

import "math"

const (
	earthRadiusKm = 6371.0088
	degToRad      = math.Pi / 180
	halfAngle     = 2

	metersPerKm   = 1000
	milesPerKm    = 0.621371
	nauticalPerKm = 0.539957
	feetPerKm     = 3280.84
)

// DistanceBetween returns the great-circle distance between two points using
// the haversine formula. It returns a zero distance when either point is
// unknown (0, 0), which is how missing coordinates are stored.
func DistanceBetween(from, to Location) GreatCircleDistance {
	if from == (Location{}) || to == (Location{}) {
		return GreatCircleDistance{}
	}

	lat1 := from.Lat * degToRad
	lat2 := to.Lat * degToRad
	dLat := (to.Lat - from.Lat) * degToRad
	dLon := (to.Lon - from.Lon) * degToRad

	sinLat := math.Sin(dLat / halfAngle)
	sinLon := math.Sin(dLon / halfAngle)
	a := sinLat*sinLat + math.Cos(lat1)*math.Cos(lat2)*sinLon*sinLon

	return DistanceFromKm(earthRadiusKm * halfAngle * math.Atan2(math.Sqrt(a), math.Sqrt(1-a)))
}

// DistanceFromKm expresses a distance in kilometres in all supported units.
func DistanceFromKm(km float64) GreatCircleDistance {
	return GreatCircleDistance{
		Km:    km,
		Meter: km * metersPerKm,
		Mile:  km * milesPerKm,
		Nm:    km * nauticalPerKm,
		Feet:  km * feetPerKm,
	}
}
