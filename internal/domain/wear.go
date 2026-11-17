package domain

import "strings"

// WearReason says why a wear row is worth writing.
type WearReason string

const (
	ReasonFirst    WearReason = "first"
	ReasonIdentity WearReason = "identity"
	ReasonErrors   WearReason = "errors"
	ReasonWear     WearReason = "wear"
	ReasonTBW      WearReason = "tbw"
	ReasonTBWReset WearReason = "tbw-reset"
)

// TBWStep is the minimum TBW growth (0.1 TB, decimal) that deserves a wear-log
// row. Smaller writes are accumulated between rows.
const TBWStep = int64(100_000_000_000)

// WearObservation is one SSD wear reading. Every counter except the identity is
// optional because not every drive reports everything. Reasons is empty for a
// freshly collected observation and says why a stored row was worth writing.
type WearObservation struct {
	Disk           string
	TS             int64
	Model          string
	Serial         string
	PercentageUsed *int64
	TBWBytes       *int64
	PowerOnHours   *int64
	MediaErrors    *int64
	Reallocated    *int64
	TempC          *int64
	Reasons        []WearReason
}

// Decide reports the reasons cur deserves a new wear row, comparing against the
// previous observation of the same disk.
//
// Only the first observation, a drive identity change, new errors, more wear or
// at least tbwStep of new writes qualify. Power-on hours and temperature do
// not: they change on every poll and would turn the log into noise.
func Decide(prev *WearObservation, cur WearObservation, tbwStep int64) []WearReason {
	if prev == nil {
		return []WearReason{ReasonFirst}
	}
	var reasons []WearReason
	if prev.Model != cur.Model || prev.Serial != cur.Serial {
		reasons = append(reasons, ReasonIdentity)
	}
	if Increased(prev.MediaErrors, cur.MediaErrors) || Increased(prev.Reallocated, cur.Reallocated) {
		reasons = append(reasons, ReasonErrors)
	}
	if Increased(prev.PercentageUsed, cur.PercentageUsed) {
		reasons = append(reasons, ReasonWear)
	}
	switch {
	case prev.TBWBytes == nil && cur.TBWBytes != nil:
		reasons = append(reasons, ReasonTBW)
	case prev.TBWBytes != nil && cur.TBWBytes != nil:
		delta := *cur.TBWBytes - *prev.TBWBytes
		if delta >= tbwStep {
			reasons = append(reasons, ReasonTBW)
		} else if delta < 0 {
			reasons = append(reasons, ReasonTBWReset)
		}
	}
	return reasons
}

// Increased reports a counter growing or appearing with a non-zero value. A
// drive that starts reporting zero errors is not a significant change.
func Increased(prev, cur *int64) bool {
	if cur == nil {
		return false
	}
	if prev == nil {
		return *cur > 0
	}
	return *cur > *prev
}

// FormatReasons renders reasons for the wear log's reason column.
func FormatReasons(reasons []WearReason) string {
	parts := make([]string, len(reasons))
	for i, r := range reasons {
		parts[i] = string(r)
	}
	return strings.Join(parts, "+")
}

// ParseReasons is the inverse of FormatReasons.
func ParseReasons(raw string) []WearReason {
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, "+")
	out := make([]WearReason, 0, len(parts))
	for _, p := range parts {
		out = append(out, WearReason(p))
	}
	return out
}
