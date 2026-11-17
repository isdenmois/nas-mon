package monitor

import (
	"errors"
	"fmt"
	"time"

	"mon/internal/collect"
	"mon/internal/domain"
)

// recordWear applies the wear policy to every freshly read disk: a row is
// appended only when domain.Decide finds a reason worth recording.
func (m *Monitor) recordWear(at time.Time, disks []collect.DiskSmart) error {
	var errs []error
	for _, d := range disks {
		prev, err := m.store.LastWear(d.Disk)
		if err != nil {
			errs = append(errs, fmt.Errorf("last wear %s: %w", d.Disk, err))
			continue
		}
		cur := observation(d, at)
		reasons := domain.Decide(prev, cur, domain.TBWStep)
		if len(reasons) == 0 {
			continue
		}
		cur.Reasons = reasons
		if err := m.store.AppendWear(cur); err != nil {
			errs = append(errs, fmt.Errorf("append wear %s: %w", d.Disk, err))
		}
	}
	return errors.Join(errs...)
}

// observation turns a SMART reading into the domain's wear observation.
func observation(d collect.DiskSmart, at time.Time) domain.WearObservation {
	return domain.WearObservation{
		Disk: d.Disk, TS: at.Unix(), Model: d.Model, Serial: d.Serial,
		PercentageUsed: d.PercentageUsed, TBWBytes: d.TBWBytes,
		PowerOnHours: d.PowerOnHours, MediaErrors: d.MediaErrors,
		Reallocated: d.Reallocated, TempC: d.TempC,
	}
}
