package domain

import (
	"math"
	"testing"
)

func TestResolutionTable(t *testing.T) {
	cases := []struct {
		res   Resolution
		name  string
		table string
	}{
		{Minute, "minute", "points_1m"},
		{Hour, "hour", "points_1h"},
		{Day, "day", "points_1d"},
	}
	for _, tc := range cases {
		if got := tc.res.String(); got != tc.name {
			t.Errorf("%v.String() = %q, want %q", tc.res, got, tc.name)
		}
		if got := tc.res.Table(); got != tc.table {
			t.Errorf("%v.Table() = %q, want %q", tc.res, got, tc.table)
		}
		parsed, ok := ParseResolution(tc.name)
		if !ok || parsed != tc.res {
			t.Errorf("ParseResolution(%q) = %v, %v", tc.name, parsed, ok)
		}
	}
	if Resolution(0).Table() != "" || Resolution(0).String() != "" {
		t.Error("unknown resolution has a table or name")
	}
	if _, ok := ParseResolution("fortnight"); ok {
		t.Error("ParseResolution(fortnight) accepted")
	}
}

func TestResolutionBucket(t *testing.T) {
	ts := int64(1_767_261_296) // 2026-01-01T10:04:56Z
	if got := Minute.Bucket(ts); got != ts-56 {
		t.Errorf("Minute.Bucket = %d", got)
	}
	if got := Hour.Bucket(ts); got%3600 != 0 || got > ts {
		t.Errorf("Hour.Bucket = %d", got)
	}
	if got := Day.Bucket(ts); got%86400 != 0 || got > ts {
		t.Errorf("Day.Bucket = %d", got)
	}
}

func TestSanitize(t *testing.T) {
	if v, ok := Sanitize(41.5); !ok || v != 41.5 {
		t.Errorf("Sanitize(41.5) = %v, %v", v, ok)
	}
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if _, ok := Sanitize(bad); ok {
			t.Errorf("Sanitize(%v) accepted", bad)
		}
	}
}
