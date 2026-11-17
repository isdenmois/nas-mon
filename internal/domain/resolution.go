package domain

// Resolution is the bucket size of an aggregation table. Its value is the
// bucket width in seconds, so arithmetic on it is always correct.
type Resolution int64

const (
	Minute Resolution = 60
	Hour   Resolution = 3600
	Day    Resolution = 86400
)

// String returns the wire name of the resolution ("minute", "hour", "day") or
// "" for an unknown value.
func (r Resolution) String() string {
	switch r {
	case Minute:
		return "minute"
	case Hour:
		return "hour"
	case Day:
		return "day"
	default:
		return ""
	}
}

// ParseResolution is the inverse of String.
func ParseResolution(s string) (Resolution, bool) {
	switch s {
	case "minute":
		return Minute, true
	case "hour":
		return Hour, true
	case "day":
		return Day, true
	default:
		return 0, false
	}
}

// Table returns the SQL table holding this resolution, or "" when unknown. The
// table name is never built from request input.
func (r Resolution) Table() string {
	switch r {
	case Minute:
		return "points_minute"
	case Hour:
		return "points_hour"
	case Day:
		return "points_day"
	default:
		return ""
	}
}

// Seconds is the bucket width.
func (r Resolution) Seconds() int64 { return int64(r) }

// Bucket aligns a unix second to the start of its bucket.
func (r Resolution) Bucket(unix int64) int64 {
	return unix - mod(unix, r.Seconds())
}

func mod(a, b int64) int64 { return ((a % b) + b) % b }
