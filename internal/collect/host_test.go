package collect

import (
	"path/filepath"
	"testing"
	"time"
)

func TestHostReadsCountersAndRates(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proc")
	writeFile(t, filepath.Join(root, "stat"), "cpu  100 0 50 850 0 0 0 0 0 0\n")
	writeFile(t, filepath.Join(root, "meminfo"), `MemTotal:       1000 kB
MemFree:         100 kB
MemAvailable:    400 kB
Buffers:          50 kB
Cached:          200 kB
`)
	writeFile(t, filepath.Join(root, "loadavg"), "0.50 0.40 0.30 1/100 1234\n")
	writeFile(t, filepath.Join(root, "net", "dev"), `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:   99999       0    0    0    0     0          0         0    99999       0    0    0    0     0       0          0
  eth0:    1000       0    0    0    0     0          0         0     2000       0    0    0    0     0       0          0
  eth1:     500       0    0    0    0     0          0         0      100       0    0    0    0     0       0          0
 veth12:  777777       0    0    0    0     0          0         0   777777       0    0    0    0     0       0          0
`)

	h := Host{ProcRoot: root}
	first := time.Unix(1000, 0)
	snap, err := h.Read(first)
	if err != nil {
		t.Fatal(err)
	}
	if snap.CPUPct != 0 || snap.NetRxBps != 0 || snap.NetTxBps != 0 {
		t.Errorf("first read should have zero rates: %+v", snap)
	}
	if snap.MemTotal != 1000*1024 || snap.MemUsed != 600*1024 || snap.MemPct != 60 {
		t.Errorf("memory = %+v", snap)
	}
	if snap.Load1 != 0.5 || snap.Load5 != 0.4 || snap.Load15 != 0.3 {
		t.Errorf("load = %+v", snap)
	}

	// 10s later: +300 total CPU, +150 idle => 50%; +6000 rx, +2000 tx.
	writeFile(t, filepath.Join(root, "stat"), "cpu  200 0 100 1000 0 0 0 0 0 0\n")
	writeFile(t, filepath.Join(root, "net", "dev"), `Inter-|   Receive |  Transmit
  eth0:    7000       0    0    0    0     0          0         0     4000       0    0    0    0     0       0          0
  eth1:     500       0    0    0    0     0          0         0      100       0    0    0    0     0       0          0
 veth12:  777777       0    0    0    0     0          0         0   777777       0    0    0    0     0       0          0
`)
	snap, err = h.Read(first.Add(10 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if snap.CPUPct != 50 {
		t.Errorf("cpu = %v, want 50", snap.CPUPct)
	}
	if snap.NetRxBps != 600 || snap.NetTxBps != 200 {
		t.Errorf("net rates = %v/%v, want 600/200", snap.NetRxBps, snap.NetTxBps)
	}
}

func TestHostCounterResetDoesNotGoNegative(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proc")
	writeFile(t, filepath.Join(root, "stat"), "cpu  100 0 50 850 0 0 0 0\n")
	writeFile(t, filepath.Join(root, "meminfo"), "MemTotal: 100 kB\nMemAvailable: 50 kB\n")
	writeFile(t, filepath.Join(root, "loadavg"), "0 0 0 1/1 1\n")
	writeFile(t, filepath.Join(root, "net", "dev"), "  eth0: 1000 0 0 0 0 0 0 0 2000 0 0 0 0 0 0 0\n")
	h := Host{ProcRoot: root}
	at := time.Unix(1000, 0)
	if _, err := h.Read(at); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "stat"), "cpu  1 0 1 2 0 0 0 0\n")
	writeFile(t, filepath.Join(root, "net", "dev"), "  eth0: 1 0 0 0 0 0 0 0 2 0 0 0 0 0 0 0\n")
	snap, err := h.Read(at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if snap.NetRxBps != 0 || snap.NetTxBps != 0 {
		t.Errorf("reset counters produced rates: %+v", snap)
	}
}

func TestHostMissingProcReturnsError(t *testing.T) {
	h := Host{ProcRoot: filepath.Join(t.TempDir(), "missing")}
	if _, err := h.Read(time.Now()); err == nil {
		t.Fatal("expected error for missing /proc/stat")
	}
}
