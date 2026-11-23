package profiles

import (
	"errors"
	"math"
	"testing"
)

// haversineKm is the same great-circle formula the browse query uses.
func haversineKm(aLat, aLng, bLat, bLng float64) float64 {
	rad := math.Pi / 180
	dLat := (bLat - aLat) * rad
	dLng := (bLng - aLng) * rad
	h := math.Pow(math.Sin(dLat/2), 2) + math.Cos(aLat*rad)*math.Cos(bLat*rad)*math.Pow(math.Sin(dLng/2), 2)
	return 2 * 6371 * math.Asin(math.Sqrt(h))
}

func TestSnapToCell_StaysWithinHalfACellDiagonal(t *testing.T) {
	points := [][2]float64{
		{-34.603722, -58.381592}, // Buenos Aires
		{-22.906847, -43.172897}, // Rio de Janeiro
		{40.416775, -3.703790},   // Madrid
		{64.146582, -21.942635},  // Reykjavik: wide longitude step
		{0.000123, 0.000456},
	}
	for _, p := range points {
		lat, lng := SnapToCell(p[0], p[1])
		// Cells are ~500 m squares, so the center is at most ~354 m away.
		if d := haversineKm(p[0], p[1], lat, lng); d > 0.36 {
			t.Errorf("SnapToCell(%v, %v) moved the point %.3f km, want <= 0.36", p[0], p[1], d)
		}
	}
}

func TestSnapToCell_IsIdempotent(t *testing.T) {
	lat, lng := SnapToCell(-34.603722, -58.381592)
	lat2, lng2 := SnapToCell(lat, lng)
	if lat != lat2 || lng != lng2 {
		t.Errorf("snapping a center moved it: (%v, %v) -> (%v, %v)", lat, lng, lat2, lng2)
	}
}

func TestSnapToCell_NearbyPointsShareACell(t *testing.T) {
	// Two points ~20 m apart inside the same cell must be indistinguishable.
	aLat, aLng := SnapToCell(-34.60100, -58.38100)
	bLat, bLng := SnapToCell(-34.60110, -58.38115)
	if aLat != bLat || aLng != bLng {
		t.Errorf("nearby points landed in different cells: (%v, %v) vs (%v, %v)", aLat, aLng, bLat, bLng)
	}
}

func TestSnapToCell_StaysInRangeAtTheEdges(t *testing.T) {
	for _, p := range [][2]float64{{90, 180}, {-90, -180}, {89.9999, 179.9999}, {-89.9999, -179.9999}} {
		lat, lng := SnapToCell(p[0], p[1])
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			t.Errorf("SnapToCell(%v, %v) = (%v, %v), out of range", p[0], p[1], lat, lng)
		}
	}
}

func TestValidateCoordinates(t *testing.T) {
	cases := []struct {
		name     string
		lat, lng *float64
		wantErr  bool
	}{
		{"both nil", nil, nil, false},
		{"valid pair", new(-34.6), new(-58.38), false},
		{"bounds inclusive", new(90.0), new(-180.0), false},
		{"latitude only", new(-34.6), nil, true},
		{"longitude only", nil, new(-58.38), true},
		{"latitude too high", new(90.1), new(0.0), true},
		{"longitude too low", new(0.0), new(-180.1), true},
		{"NaN latitude", new(math.NaN()), new(0.0), true},
		{"NaN longitude", new(0.0), new(math.NaN()), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCoordinates(tc.lat, tc.lng)
			if tc.wantErr != (err != nil) {
				t.Fatalf("validateCoordinates() error = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !errors.Is(err, ErrInvalidInput) {
				t.Errorf("error %v does not wrap ErrInvalidInput", err)
			}
		})
	}
}
