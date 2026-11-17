package collect

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const nvmeJSON = `{
  "device": {"name": "/dev/nvme0n1", "type": "nvme", "protocol": "NVMe"},
  "model_name": "Samsung SSD 990 PRO 2TB",
  "serial_number": "S6Z1NJ0T123456",
  "smart_status": {"passed": true},
  "temperature": {"current": 38},
  "nvme_smart_health_information_log": {
    "critical_warning": 0,
    "temperature": 38,
    "percentage_used": 3,
    "data_units_written": 100000,
    "power_on_hours": 1234,
    "media_errors": 0
  }
}`

const sataJSON = `{
  "device": {"name": "/dev/sda", "type": "sat", "protocol": "ATA"},
  "model_name": "Samsung SSD 870 EVO 1TB",
  "serial_number": "S5Y1NG0R123456",
  "rotation_rate": 0,
  "smart_status": {"passed": true},
  "temperature": {"current": 31},
  "ata_smart_attributes": {"table": [
    {"id": 5, "name": "Reallocated_Sector_Ct", "value": 100, "raw": {"value": 0}},
    {"id": 9, "name": "Power_On_Hours", "value": 99, "raw": {"value": 5678}},
    {"id": 187, "name": "Reported_Uncorrect", "value": 100, "raw": {"value": 2}},
    {"id": 194, "name": "Temperature_Celsius", "value": 69, "raw": {"value": 31}},
    {"id": 231, "name": "SSD_Life_Left", "value": 97, "raw": {"value": 0}},
    {"id": 241, "name": "Total_LBAs_Written", "value": 99, "raw": {"value": 2000000}}
  ]}
}`

const hddJSON = `{
  "device": {"name": "/dev/sdb", "type": "sat", "protocol": "ATA"},
  "model_name": "WDC WD40EFRX",
  "rotation_rate": 5400,
  "smart_status": {"passed": true},
  "temperature": {"current": 33}
}`

func fakeBlock(t *testing.T, names ...string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "block")
	for _, n := range names {
		if err := os.MkdirAll(filepath.Join(root, n), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestSmartNVMe(t *testing.T) {
	s := Smart{
		SysBlock: fakeBlock(t, "nvme0n1"),
		Runner: func(_ context.Context, dev string) ([]byte, error) {
			if dev != "/dev/nvme0n1" {
				t.Errorf("dev = %q", dev)
			}
			return []byte(nvmeJSON), nil
		},
	}
	got, err := s.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("disks = %+v", got)
	}
	d := got[0]
	if d.Disk != "nvme0n1" || d.Model != "Samsung SSD 990 PRO 2TB" || d.Serial != "S6Z1NJ0T123456" {
		t.Errorf("identity = %+v", d)
	}
	if d.PercentageUsed == nil || *d.PercentageUsed != 3 {
		t.Errorf("percentage used = %v", d.PercentageUsed)
	}
	if d.TBWBytes == nil || *d.TBWBytes != 100000*512000 {
		t.Errorf("tbw = %v", d.TBWBytes)
	}
	if d.PowerOnHours == nil || *d.PowerOnHours != 1234 {
		t.Errorf("hours = %v", d.PowerOnHours)
	}
	if d.TempC == nil || *d.TempC != 38 {
		t.Errorf("temp = %v", d.TempC)
	}
	if d.MediaErrors == nil || *d.MediaErrors != 0 {
		t.Errorf("media errors = %v", d.MediaErrors)
	}
	if d.CollectedAt.IsZero() {
		t.Error("collected at not set")
	}
}

func TestSmartSATA(t *testing.T) {
	s := Smart{
		SysBlock: fakeBlock(t, "sda"),
		Runner:   func(context.Context, string) ([]byte, error) { return []byte(sataJSON), nil },
	}
	got, err := s.Read(context.Background())
	if err != nil || len(got) != 1 {
		t.Fatalf("got %+v, %v", got, err)
	}
	d := got[0]
	if d.PercentageUsed == nil || *d.PercentageUsed != 3 {
		t.Errorf("percentage used = %v, want 3 (100 - SSD_Life_Left)", d.PercentageUsed)
	}
	if d.TBWBytes == nil || *d.TBWBytes != 2000000*512 {
		t.Errorf("tbw = %v", d.TBWBytes)
	}
	if d.PowerOnHours == nil || *d.PowerOnHours != 5678 {
		t.Errorf("hours = %v", d.PowerOnHours)
	}
	if d.Reallocated == nil || *d.Reallocated != 0 {
		t.Errorf("reallocated = %v", d.Reallocated)
	}
	if d.MediaErrors == nil || *d.MediaErrors != 2 {
		t.Errorf("media errors = %v", d.MediaErrors)
	}
	if d.TempC == nil || *d.TempC != 31 {
		t.Errorf("temp = %v", d.TempC)
	}
}

func TestSmartAutoDetectsDisksAndSkipsVirtual(t *testing.T) {
	s := Smart{
		SysBlock: fakeBlock(t, "nvme0n1", "nvme0n1p1", "sda", "sda1", "loop0", "dm-0", "sr0", "sdb"),
		Runner: func(_ context.Context, dev string) ([]byte, error) {
			switch dev {
			case "/dev/nvme0n1", "/dev/sda":
				return []byte(nvmeJSON), nil
			case "/dev/sdb":
				return []byte(hddJSON), nil
			}
			t.Errorf("unexpected device %q", dev)
			return nil, errors.New("unexpected")
		},
	}
	got, err := s.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var disks []string
	for _, d := range got {
		disks = append(disks, d.Disk)
	}
	if strings.Join(disks, ",") != "nvme0n1,sda" {
		t.Errorf("disks = %v, want nvme0n1,sda (partition, loop, dm, optical and HDD skipped)", disks)
	}
}

func TestSmartFailuresAreReportedButPartialResultsKept(t *testing.T) {
	s := Smart{
		SysBlock: fakeBlock(t, "nvme0n1", "sda"),
		Runner: func(_ context.Context, dev string) ([]byte, error) {
			if dev == "/dev/sda" {
				return []byte("not json"), errors.New("exit status 2")
			}
			return []byte(nvmeJSON), nil
		},
	}
	got, err := s.Read(context.Background())
	if err == nil || !strings.Contains(err.Error(), "sda") {
		t.Errorf("err = %v, want sda failure", err)
	}
	if len(got) != 1 || got[0].Disk != "nvme0n1" {
		t.Errorf("partial results = %+v", got)
	}
}

func TestSmartNoDevicesIsNotAnError(t *testing.T) {
	s := Smart{SysBlock: fakeBlock(t), Runner: func(context.Context, string) ([]byte, error) {
		t.Fatal("runner called without devices")
		return nil, nil
	}}
	got, err := s.Read(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestSmartTimeoutIsHandled(t *testing.T) {
	s := Smart{
		SysBlock: fakeBlock(t, "nvme0n1"),
		Timeout:  30 * time.Millisecond,
		Runner: func(ctx context.Context, dev string) ([]byte, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Second):
				return nil, nil
			}
		},
	}
	if _, err := s.Read(context.Background()); err == nil {
		t.Fatal("expected timeout error")
	}
}
