package domain

import "math"

// Sanitize reports whether v may be recorded, rejecting NaN and infinities.
// Values are checked where they enter the system, not inside SQL.
func Sanitize(v float64) (float64, bool) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0, false
	}
	return v, true
}
