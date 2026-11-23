package profiles

import (
	"fmt"
	"math"
)

// cellSizeDeg is the latitude height of a location cell, about 500 m. Profiles
// only ever store the center of the cell they fall in, so the exact position a
// client reports is discarded on write and cannot leak from the database,
// exports, backups taken after migration 000048, or distance arithmetic.
const cellSizeDeg = 0.0045

// minCosLat keeps the longitude step finite at the poles.
const minCosLat = 0.01

// SnapToCell returns the center of the ~500 m cell containing (lat, lng).
// Cell width in longitude grows with latitude so cells stay roughly square.
// The function is idempotent: snapping a cell center returns the same center.
// The SQL backfill in migration 000048 mirrors this formula.
func SnapToCell(lat, lng float64) (float64, float64) {
	row := math.Floor(lat / cellSizeDeg)
	cLat := clamp((row+0.5)*cellSizeDeg, 90)
	lngStep := cellSizeDeg / max(math.Cos(cLat*math.Pi/180), minCosLat)
	col := math.Floor(lng / lngStep)
	cLng := clamp((col+0.5)*lngStep, 180)
	return round6(cLat), round6(cLng)
}

func round6(v float64) float64 { return math.Round(v*1e6) / 1e6 }

// clamp limits v to [-limit, limit]; edge cells can otherwise center past it.
func clamp(v, limit float64) float64 { return max(min(v, limit), -limit) }

// validateCoordinates enforces that coordinates arrive as a pair and in range.
func validateCoordinates(lat, lng *float64) error {
	if (lat == nil) != (lng == nil) {
		return fmt.Errorf("%w: latitude and longitude must be set together", ErrInvalidInput)
	}
	if lat == nil {
		return nil
	}
	if math.IsNaN(*lat) || *lat < -90 || *lat > 90 {
		return fmt.Errorf("%w: latitude must be between -90 and 90", ErrInvalidInput)
	}
	if math.IsNaN(*lng) || *lng < -180 || *lng > 180 {
		return fmt.Errorf("%w: longitude must be between -180 and 180", ErrInvalidInput)
	}
	return nil
}
