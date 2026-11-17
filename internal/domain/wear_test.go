package domain

import "testing"

func i64(v int64) *int64 { return &v }

func TestDecide(t *testing.T) {
	const tbw = int64(28_000_000_000_000)
	base := WearObservation{
		Disk: "nvme0n1", Model: "Samsung", Serial: "S1",
		PercentageUsed: i64(3), TBWBytes: i64(tbw), PowerOnHours: i64(100),
		MediaErrors: i64(0), Reallocated: i64(0), TempC: i64(40),
	}
	prev := base

	cases := []struct {
		name   string
		prev   *WearObservation
		mutate func(*WearObservation)
		want   string
	}{
		{"first ever", nil, func(*WearObservation) {}, "first"},
		{"idle", &prev, func(*WearObservation) {}, ""},
		{"temperature only", &prev, func(d *WearObservation) { d.TempC = i64(55) }, ""},
		{"power-on only", &prev, func(d *WearObservation) { d.PowerOnHours = i64(101) }, ""},
		{"wear percentage up", &prev, func(d *WearObservation) { d.PercentageUsed = i64(4) }, "wear"},
		{"tbw below step", &prev, func(d *WearObservation) { d.TBWBytes = i64(tbw + 50_000_000_000) }, ""},
		{"tbw at step", &prev, func(d *WearObservation) { d.TBWBytes = i64(tbw + TBWStep) }, "tbw"},
		{"tbw above step", &prev, func(d *WearObservation) { d.TBWBytes = i64(tbw + 2*TBWStep) }, "tbw"},
		{"tbw reset", &prev, func(d *WearObservation) { d.TBWBytes = i64(tbw - 1) }, "tbw-reset"},
		{"tbw missing now", &prev, func(d *WearObservation) { d.TBWBytes = nil }, ""},
		{"errors appear as nil", &prev, func(d *WearObservation) { d.MediaErrors = nil; d.Reallocated = nil }, ""},
		{"wear appears as nil", &prev, func(d *WearObservation) { d.PercentageUsed = nil }, ""},
		{"media errors", &prev, func(d *WearObservation) { d.MediaErrors = i64(1) }, "errors"},
		{"reallocated", &prev, func(d *WearObservation) { d.Reallocated = i64(1) }, "errors"},
		{"errors with same count", &prev, func(d *WearObservation) { d.MediaErrors = i64(0) }, ""},
		{"serial changed", &prev, func(d *WearObservation) { d.Serial = "S2" }, "identity"},
		{"model changed", &prev, func(d *WearObservation) { d.Model = "WD" }, "identity"},
		{"wear and tbw", &prev, func(d *WearObservation) {
			d.PercentageUsed = i64(4)
			d.TBWBytes = i64(tbw + TBWStep)
		}, "wear+tbw"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cur := base
			tc.mutate(&cur)
			if got := FormatReasons(Decide(tc.prev, cur, TBWStep)); got != tc.want {
				t.Errorf("reasons = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestDecideErrorsAppearNonZero(t *testing.T) {
	prev := &WearObservation{Disk: "sda", Model: "Samsung"}
	cur := WearObservation{Disk: "sda", Model: "Samsung", MediaErrors: i64(2)}
	if got := FormatReasons(Decide(prev, cur, TBWStep)); got != "errors" {
		t.Errorf("reasons = %q, want errors", got)
	}
}

func TestDecideFirstTBWValue(t *testing.T) {
	prev := &WearObservation{Disk: "sda", Model: "Samsung"}
	cur := WearObservation{Disk: "sda", Model: "Samsung", TBWBytes: i64(100)}
	if got := FormatReasons(Decide(prev, cur, TBWStep)); got != "tbw" {
		t.Errorf("reasons = %q, want tbw", got)
	}
}
