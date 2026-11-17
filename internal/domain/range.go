package domain

import (
	"errors"
	"time"
)

// RangeKey is a UI range preset.
type RangeKey string

const (
	Range1h  RangeKey = "1h"
	Range6h  RangeKey = "6h"
	Range24h RangeKey = "24h"
	Range48h RangeKey = "48h"
	Range7d  RangeKey = "7d"
	Range30d RangeKey = "30d"
	Range90d RangeKey = "90d"
	Range1y  RangeKey = "1y"
	RangeAll RangeKey = "all"
)

var rangeWindows = map[RangeKey]time.Duration{
	Range1h:  time.Hour,
	Range6h:  6 * time.Hour,
	Range24h: 24 * time.Hour,
	Range48h: 48 * time.Hour,
	Range7d:  7 * 24 * time.Hour,
	Range30d: 30 * 24 * time.Hour,
	Range90d: 90 * 24 * time.Hour,
	Range1y:  365 * 24 * time.Hour,
}

var rangeResolutions = map[RangeKey]Resolution{
	Range1h:  Minute,
	Range6h:  Minute,
	Range24h: Minute,
	Range48h: Minute,
	Range7d:  Hour,
	Range30d: Hour,
	Range90d: Hour,
	Range1y:  Day,
	RangeAll: Day,
}

// ErrUnknownRange is returned for anything that is not a UI preset.
var ErrUnknownRange = errors.New("range must be one of 1h, 6h, 24h, 48h, 7d, 30d, 90d, 1y, all")

// ParseRange validates a preset name.
func ParseRange(raw string) (RangeKey, error) {
	k := RangeKey(raw)
	if _, ok := rangeResolutions[k]; !ok {
		return "", ErrUnknownRange
	}
	return k, nil
}

// Range is a resolved query window: buckets of Resolutions from From to To.
type Range struct {
	Key        RangeKey
	From       int64
	To         int64
	Resolution Resolution
}

// Resolve turns a preset into a window ending at now (unix seconds). From is
// aligned down to the resolution so the first bucket is complete; "all" starts
// at the beginning of time.
func (k RangeKey) Resolve(now int64) (Range, error) {
	res, ok := rangeResolutions[k]
	if !ok {
		return Range{}, ErrUnknownRange
	}
	r := Range{Key: k, To: now, Resolution: res}
	if k == RangeAll {
		return r, nil
	}
	r.From = res.Bucket(now - int64(rangeWindows[k].Seconds()))
	return r, nil
}
