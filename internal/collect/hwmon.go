// Package collect contains the host, temperature, SMART, volume and Docker
// collectors. Each collector returns plain snapshots; nothing here talks to the
// database or the network server directly.
package collect

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// TempReading is one temperature sensor value.
type TempReading struct {
	Key     string
	Chip    string
	Label   string
	Celsius float64
	Warn    bool
}

// Hwmon reads temperatures from /sys/class/hwmon. By default it shows the CPU
// package temperature and the composite sensor of every NVMe drive; Include and
// Exclude select any other sensor by its key.
type Hwmon struct {
	Root    string
	Include *regexp.Regexp
	Exclude *regexp.Regexp
	WarnC   float64
}

var cpuChips = map[string]bool{
	"k10temp":     true, // AMD
	"coretemp":    true, // Intel
	"zenpower":    true, // out-of-tree AMD
	"cpu_thermal": true, // ARM
}

const (
	nvmeChip = "nvme"
)

type sensor struct {
	index int
	key   string
	label string
	value float64
}

// Read returns the selected temperature readings sorted by key.
func (h Hwmon) Read() ([]TempReading, error) {
	dirs, err := os.ReadDir(h.Root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []TempReading
	for _, de := range dirs {
		// Entries in /sys/class/hwmon are symlinks, so IsDir is false here.
		if !strings.HasPrefix(de.Name(), "hwmon") {
			continue
		}
		dir := filepath.Join(h.Root, de.Name())
		chip := readTrim(filepath.Join(dir, "name"))
		if chip == "" {
			continue
		}
		disk := ""
		if chip == nvmeChip {
			disk = nvmeDiskName(dir)
		}
		sensors := readSensors(dir, chip, disk)
		selected := h.selectSensors(chip, disk, sensors)
		for _, s := range selected {
			if h.Exclude != nil && h.Exclude.MatchString(s.key) {
				continue
			}
			out = append(out, TempReading{
				Key:     s.key,
				Chip:    chip,
				Label:   s.label,
				Celsius: s.value,
				Warn:    h.WarnC > 0 && s.value >= h.WarnC,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return dedupeTemps(out), nil
}

// dedupeTemps keeps the first reading per key; duplicate keys would merge two
// different sensors into one history series.
func dedupeTemps(in []TempReading) []TempReading {
	seen := make(map[string]bool, len(in))
	out := in[:0]
	for _, t := range in {
		if seen[t.Key] {
			continue
		}
		seen[t.Key] = true
		out = append(out, t)
	}
	return out
}

// selectSensors applies the include override or the default selection: one CPU
// package sensor and the composite sensor of each NVMe drive.
func (h Hwmon) selectSensors(chip, disk string, sensors []sensor) []sensor {
	if h.Include != nil {
		var out []sensor
		for _, s := range sensors {
			if h.Include.MatchString(s.key) {
				out = append(out, s)
			}
		}
		return out
	}
	switch {
	case chip == nvmeChip:
		for _, s := range sensors {
			if s.index == 1 {
				return []sensor{s}
			}
		}
		if len(sensors) > 0 {
			return sensors[:1]
		}
	case cpuChips[chip]:
		// Prefer Tctl (AMD) then Tdie, else the lowest-index sensor.
		for _, want := range []string{"Tctl", "Tdie"} {
			for _, s := range sensors {
				if s.label == want {
					return []sensor{s}
				}
			}
		}
		if len(sensors) > 0 {
			return sensors[:1]
		}
	}
	return nil
}

func readSensors(dir, chip, disk string) []sensor {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []sensor
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "temp") || !strings.HasSuffix(name, "_input") {
			continue
		}
		idx, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "temp"), "_input"))
		if err != nil {
			continue
		}
		raw := readTrim(filepath.Join(dir, name))
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		celsius := v / 1000
		if celsius < -50 || celsius > 200 {
			continue // bogus sensor value
		}
		label := readTrim(filepath.Join(dir, "temp"+strconv.Itoa(idx)+"_label"))
		if label == "" {
			label = "temp" + strconv.Itoa(idx)
		}
		key := chip + ":" + label
		if chip == nvmeChip {
			key = "nvme:" + disk
			if idx != 1 {
				key += ":" + label
			}
		}
		out = append(out, sensor{index: idx, key: key, label: label, value: celsius})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].index < out[j].index })
	return out
}

// nvmeDiskName resolves the hwmon device symlink to a name such as "nvme0".
func nvmeDiskName(dir string) string {
	link, err := filepath.EvalSymlinks(filepath.Join(dir, "device"))
	if err == nil {
		if base := filepath.Base(link); base != "" && base != "." {
			return base
		}
	}
	return filepath.Base(dir)
}

func readTrim(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}
