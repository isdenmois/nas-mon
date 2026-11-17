package collect

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// HostSnapshot is the current host-level resource usage.
type HostSnapshot struct {
	CPUPct    float64
	MemUsed   uint64
	MemTotal  uint64
	MemPct    float64
	Load1     float64
	Load5     float64
	Load15    float64
	NetRxBps  float64
	NetTxBps  float64
	UpdatedAt time.Time
}

// Host reads CPU, memory, load and network counters from /proc. It keeps the
// previous counters to turn monotonic values into rates.
type Host struct {
	ProcRoot string

	haveCPU   bool
	prevTotal uint64
	prevIdle  uint64

	haveNet bool
	prevRx  uint64
	prevTx  uint64
	prevAt  time.Time
}

// Read samples the host. The first call returns zero rates because there is no
// previous counter to compare with.
func (h *Host) Read(now time.Time) (HostSnapshot, error) {
	snap := HostSnapshot{UpdatedAt: now}
	root := h.ProcRoot
	if root == "" {
		root = "/proc"
	}

	total, idle, err := readCPUStat(filepath.Join(root, "stat"))
	if err != nil {
		return snap, err
	}
	if h.haveCPU && total > h.prevTotal {
		dTotal := total - h.prevTotal
		dIdle := idle - h.prevIdle
		snap.CPUPct = float64(dTotal-dIdle) / float64(dTotal) * 100
	}
	h.prevTotal, h.prevIdle, h.haveCPU = total, idle, true

	memTotal, memAvail, err := readMemInfo(filepath.Join(root, "meminfo"))
	if err != nil {
		return snap, err
	}
	if memTotal > 0 {
		snap.MemTotal = memTotal
		snap.MemUsed = memTotal - memAvail
		snap.MemPct = float64(snap.MemUsed) / float64(memTotal) * 100
	}

	if l1, l5, l15, err := readLoadAvg(filepath.Join(root, "loadavg")); err == nil {
		snap.Load1, snap.Load5, snap.Load15 = l1, l5, l15
	}

	rx, tx, err := readNetDev(filepath.Join(root, "net", "dev"))
	if err != nil {
		return snap, err
	}
	if h.haveNet {
		if dt := now.Sub(h.prevAt).Seconds(); dt > 0 {
			if rx >= h.prevRx {
				snap.NetRxBps = float64(rx-h.prevRx) / dt
			}
			if tx >= h.prevTx {
				snap.NetTxBps = float64(tx-h.prevTx) / dt
			}
		}
	}
	h.prevRx, h.prevTx, h.prevAt, h.haveNet = rx, tx, now, true

	return snap, nil
}

func readCPUStat(path string) (total, idle uint64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, fmt.Errorf("%s: empty", path)
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, fmt.Errorf("%s: unexpected first line", path)
	}
	// user nice system idle iowait irq softirq steal
	for i := 1; i < len(fields) && i <= 8; i++ {
		v, err := strconv.ParseUint(fields[i], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("%s: bad counter %q: %w", path, fields[i], err)
		}
		total += v
		if i == 4 || i == 5 { // idle + iowait
			idle += v
		}
	}
	return total, idle, nil
}

func readMemInfo(path string) (total, available uint64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var free, buffers, cached uint64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		v, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "MemTotal":
			total = v * 1024
		case "MemAvailable":
			available = v * 1024
		case "MemFree":
			free = v * 1024
		case "Buffers":
			buffers = v * 1024
		case "Cached":
			cached = v * 1024
		}
	}
	if available == 0 {
		available = free + buffers + cached
	}
	if total > 0 && available > total {
		available = total
	}
	return total, available, sc.Err()
}

func readLoadAvg(path string) (l1, l5, l15 float64, err error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, 0, err
	}
	fields := strings.Fields(string(b))
	if len(fields) < 3 {
		return 0, 0, 0, fmt.Errorf("%s: unexpected content", path)
	}
	for i, dst := range []*float64{&l1, &l5, &l15} {
		v, err := strconv.ParseFloat(fields[i], 64)
		if err != nil {
			return 0, 0, 0, err
		}
		*dst = v
	}
	return l1, l5, l15, nil
}

// virtualPrefixes are interfaces that double-count traffic already seen on a
// physical interface.
var virtualPrefixes = []string{
	"lo", "veth", "docker", "br-", "virbr", "tap", "tun", "vmnet",
	"cni", "flannel", "kube", "cali", "wg", "tailscale",
}

func readNetDev(path string) (rx, tx uint64, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		name, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		name = strings.TrimSpace(name)
		if skipInterface(name) {
			continue
		}
		fields := strings.Fields(rest)
		if len(fields) < 9 {
			continue
		}
		r, err1 := strconv.ParseUint(fields[0], 10, 64)
		t, err2 := strconv.ParseUint(fields[8], 10, 64)
		if err1 != nil || err2 != nil {
			continue
		}
		rx += r
		tx += t
	}
	return rx, tx, sc.Err()
}

func skipInterface(name string) bool {
	for _, p := range virtualPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}
