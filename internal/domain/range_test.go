package domain

import (
	"errors"
	"testing"
	"time"
)

func TestResolveRange(t *testing.T) {
	now := time.Date(2026, 3, 4, 12, 34, 56, 0, time.UTC).Unix()
	cases := map[RangeKey]Resolution{
		Range1h: Minute, Range6h: Minute, Range24h: Minute, Range48h: Minute,
		Range7d: Hour, Range30d: Hour, Range90d: Hour,
		Range1y: Day, RangeAll: Day,
	}
	for key, want := range cases {
		r, err := key.Resolve(now)
		if err != nil {
			t.Fatalf("%s: %v", key, err)
		}
		if r.Resolution != want || r.To != now {
			t.Errorf("%s: resolution=%v to=%d", key, r.Resolution, r.To)
		}
		if key == RangeAll {
			if r.From != 0 {
				t.Errorf("all: from = %d, want 0", r.From)
			}
			continue
		}
		if r.From > r.To || r.From%want.Seconds() != 0 {
			t.Errorf("%s: bad from %d (resolution %v)", key, r.From, want)
		}
	}
}

func TestParseRangeRejectsUnknown(t *testing.T) {
	for _, raw := range []string{"", "14d", "2h", "1H"} {
		if _, err := ParseRange(raw); !errors.Is(err, ErrUnknownRange) {
			t.Errorf("ParseRange(%q) = %v, want ErrUnknownRange", raw, err)
		}
	}
	if _, err := ParseRange("24h"); err != nil {
		t.Errorf("ParseRange(24h) = %v", err)
	}
}
