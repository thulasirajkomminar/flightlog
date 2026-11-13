package domain

import (
	"math"
	"testing"
)

func TestDistanceBetween(t *testing.T) {
	t.Parallel()

	ams := Location{Lat: 52.3086, Lon: 4.7639}
	jfk := Location{Lat: 40.6413, Lon: -73.7781}
	syd := Location{Lat: -33.9399, Lon: 151.1753}
	lax := Location{Lat: 33.9416, Lon: -118.4085}

	tests := []struct {
		name     string
		from, to Location
		wantKm   float64
	}{
		{"AMS-JFK", ams, jfk, 5848},
		{"JFK-AMS is symmetric", jfk, ams, 5848},
		{"SYD-LAX across the date line", syd, lax, 12051},
		{"same point", ams, ams, 0},
		{"unknown origin", Location{}, jfk, 0},
		{"unknown destination", ams, Location{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := DistanceBetween(tt.from, tt.to)
			if math.Abs(got.Km-tt.wantKm) > 15 {
				t.Errorf("Km = %.1f, want about %.0f", got.Km, tt.wantKm)
			}

			if math.Abs(got.Meter-got.Km*1000) > 0.001 {
				t.Errorf("Meter = %v, inconsistent with Km = %v", got.Meter, got.Km)
			}
		})
	}
}
