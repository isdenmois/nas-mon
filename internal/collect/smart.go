package collect

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DiskSmart is one SSD SMART snapshot. Pointer fields are nil when the drive
// does not report the attribute.
type DiskSmart struct {
	Disk           string
	Model          string
	Serial         string
	PercentageUsed *int64
	TBWBytes       *int64
	PowerOnHours   *int64
	MediaErrors    *int64
	Reallocated    *int64
	TempC          *int64
	CollectedAt    time.Time
}

// Smart reads SSD SMART data with smartctl -j. The runner is injectable so the
// parser and device discovery can be tested without hardware or root.
type Smart struct {
	SysBlock string
	Timeout  time.Duration
	Runner   func(ctx context.Context, dev string) ([]byte, error)
}

var (
	nvmeDevRe = regexp.MustCompile(`^nvme\d+n\d+$`)
	sdDevRe   = regexp.MustCompile(`^sd[a-z]+$`)
)

const nvmeBytePerUnit = 1000 * 512 // NVMe Data Units Written = 1000 * 512 bytes

// Read probes every NVMe namespace and SCSI disk in /sys/block. Drives that
// report a non-zero rotation rate (HDDs) and devices without SMART data are
// skipped. Per-disk failures are joined into the returned error while the
// successful disks are still returned.
func (s Smart) Read(ctx context.Context) ([]DiskSmart, error) {
	devices, err := s.devices()
	if err != nil {
		return nil, err
	}
	runner := s.Runner
	if runner == nil {
		runner = smartctlRunner
	}
	timeout := s.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	var out []DiskSmart
	var errs []error
	for _, dev := range devices {
		dctx, cancel := context.WithTimeout(ctx, timeout)
		raw, runErr := runner(dctx, dev)
		cancel()

		if len(raw) == 0 {
			if runErr == nil {
				runErr = errors.New("empty smartctl output")
			}
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(dev), runErr))
			continue
		}
		d, skip, parseErr := parseSmart(raw, filepath.Base(dev))
		if parseErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(dev), parseErr))
			continue
		}
		if skip {
			continue
		}
		if runErr != nil {
			// smartctl exit bits 4/8 still carry valid JSON; remember the warning
			// but keep what was parsed.
			errs = append(errs, fmt.Errorf("%s: %w", filepath.Base(dev), runErr))
		}
		d.CollectedAt = time.Now().UTC()
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Disk < out[j].Disk })
	return out, errors.Join(errs...)
}

func (s Smart) devices() ([]string, error) {
	root := s.SysBlock
	if root == "" {
		root = "/sys/block"
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		name := e.Name()
		if !nvmeDevRe.MatchString(name) && !sdDevRe.MatchString(name) {
			continue
		}
		out = append(out, "/dev/"+name)
	}
	sort.Strings(out)
	return out, nil
}

func smartctlRunner(ctx context.Context, dev string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "-j", "-a", dev)
	out, err := cmd.Output()
	var exitErr *exec.ExitError
	if err != nil && len(out) == 0 {
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("smartctl: %s", strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, err
	}
	return out, nil
}

type smartJSON struct {
	Device struct {
		Name string `json:"name"`
	} `json:"device"`
	ModelName    string `json:"model_name"`
	SerialNumber string `json:"serial_number"`
	RotationRate *int   `json:"rotation_rate"`
	Temperature  struct {
		Current *int64 `json:"current"`
	} `json:"temperature"`
	NVMe *struct {
		PercentageUsed   *int64 `json:"percentage_used"`
		DataUnitsWritten *int64 `json:"data_units_written"`
		PowerOnHours     *int64 `json:"power_on_hours"`
		MediaErrors      *int64 `json:"media_errors"`
		Temperature      *int64 `json:"temperature"`
	} `json:"nvme_smart_health_information_log"`
	Ata *struct {
		Table []ataAttrJSON `json:"table"`
	} `json:"ata_smart_attributes"`
}

type ataAttrJSON struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Value int64  `json:"value"`
	Raw   struct {
		Value int64 `json:"value"`
	} `json:"raw"`
}

func parseSmart(raw []byte, disk string) (DeviceSmart DiskSmart, skip bool, err error) {
	var j smartJSON
	if err := json.Unmarshal(raw, &j); err != nil {
		return DiskSmart{}, false, fmt.Errorf("parse smartctl JSON: %w", err)
	}
	d := DiskSmart{
		Disk:   disk,
		Model:  j.ModelName,
		Serial: j.SerialNumber,
	}

	if j.RotationRate != nil && *j.RotationRate > 0 {
		return d, true, nil // HDD, not an SSD wear target
	}
	if j.Temperature.Current != nil {
		d.TempC = intPtr(*j.Temperature.Current)
	}

	if j.NVMe != nil {
		d.PercentageUsed = j.NVMe.PercentageUsed
		if j.NVMe.DataUnitsWritten != nil {
			d.TBWBytes = intPtr(saturatingMul(*j.NVMe.DataUnitsWritten, nvmeBytePerUnit))
		}
		d.PowerOnHours = j.NVMe.PowerOnHours
		d.MediaErrors = j.NVMe.MediaErrors
		if d.TempC == nil {
			d.TempC = j.NVMe.Temperature
		}
	}

	if j.Ata != nil {
		attrs := j.Ata.Table
		if v := attrRaw(attrs, 5); v != nil {
			d.Reallocated = v
		}
		if v := attrRaw(attrs, 9); v != nil {
			d.PowerOnHours = v
		}
		if d.MediaErrors == nil {
			d.MediaErrors = attrRaw(attrs, 187)
		}
		if d.TempC == nil {
			d.TempC = attrRaw(attrs, 194)
		}
		for _, id := range []int{231, 202, 233, 177} { // SSD life remaining indicators
			if v := attrByID(attrs, id); v != nil {
				used := 100 - v.Value
				if used < 0 {
					used = 0
				}
				if used > 100 {
					used = 100
				}
				d.PercentageUsed = intPtr(used)
				break
			}
		}
		if v := attrRaw(attrs, 241); v != nil { // Total_LBAs_Written, 512-byte sectors
			d.TBWBytes = intPtr(saturatingMul(*v, 512))
		}
	}

	useful := d.Model != "" || d.PercentageUsed != nil || d.TBWBytes != nil ||
		d.PowerOnHours != nil || d.TempC != nil || d.Reallocated != nil || d.MediaErrors != nil
	if !useful {
		return d, true, nil
	}
	return d, false, nil
}

// DeviceSmart is an alias kept for readability in parser signatures.
type DeviceSmart = DiskSmart

type ataAttr struct {
	ID    int
	Name  string
	Value int64
	Raw   int64
}

func attrByID(table []ataAttrJSON, id int) *ataAttr {
	for _, a := range table {
		if a.ID == id {
			return &ataAttr{ID: a.ID, Name: a.Name, Value: a.Value, Raw: a.Raw.Value}
		}
	}
	return nil
}

func attrRaw(table []ataAttrJSON, id int) *int64 {
	a := attrByID(table, id)
	if a == nil {
		return nil
	}
	return intPtr(a.Raw)
}

func intPtr(v int64) *int64 { return &v }

func saturatingMul(a, b int64) int64 {
	if a <= 0 || b <= 0 {
		return 0
	}
	if a > (1<<63-1)/b {
		return 1<<63 - 1
	}
	return a * b
}
