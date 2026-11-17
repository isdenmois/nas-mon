package collect

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// fakeSysfs builds a hwmon tree and returns its root.
func fakeSysfs(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "hwmon")
	devices := filepath.Join(t.TempDir(), "devices")

	writeFile(t, filepath.Join(root, "hwmon0", "name"), "nvme\n")
	writeFile(t, filepath.Join(root, "hwmon0", "temp1_input"), "35250\n")
	writeFile(t, filepath.Join(root, "hwmon0", "temp1_label"), "Composite\n")
	writeFile(t, filepath.Join(devices, "nvme0", "model"), "x\n")
	os.MkdirAll(filepath.Join(root, "hwmon0"), 0o755)
	if err := os.Symlink(filepath.Join(devices, "nvme0"), filepath.Join(root, "hwmon0", "device")); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(root, "hwmon1", "name"), "k10temp\n")
	writeFile(t, filepath.Join(root, "hwmon1", "temp1_input"), "42125\n")
	writeFile(t, filepath.Join(root, "hwmon1", "temp1_label"), "Tctl\n")
	writeFile(t, filepath.Join(root, "hwmon1", "temp3_input"), "40125\n")
	writeFile(t, filepath.Join(root, "hwmon1", "temp3_label"), "Tccd1\n")

	writeFile(t, filepath.Join(root, "hwmon2", "name"), "r8169_0_e00:00\n")
	writeFile(t, filepath.Join(root, "hwmon2", "temp1_input"), "30000\n")

	writeFile(t, filepath.Join(root, "hwmon3", "name"), "nvme\n")
	writeFile(t, filepath.Join(root, "hwmon3", "temp1_input"), "40000\n")
	writeFile(t, filepath.Join(root, "hwmon3", "temp1_label"), "Composite\n")
	os.MkdirAll(filepath.Join(devices, "nvme2"), 0o755)
	os.Symlink(filepath.Join(devices, "nvme2"), filepath.Join(root, "hwmon3", "device"))

	return root
}

func TestHwmonDefaultsCPUAndNVMeOnly(t *testing.T) {
	h := Hwmon{Root: fakeSysfs(t), WarnC: 80}
	got, err := h.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("readings = %+v, want CPU + 2 NVMe", got)
	}
	byKey := map[string]TempReading{}
	for _, r := range got {
		byKey[r.Key] = r
	}
	cpu, ok := byKey["k10temp:Tctl"]
	if !ok || cpu.Celsius != 42.125 || cpu.Warn {
		t.Errorf("cpu = %+v", cpu)
	}
	if _, ok := byKey["k10temp:Tccd1"]; ok {
		t.Error("secondary CCD temp included by default")
	}
	if _, ok := byKey["r8169_0_e00:00:temp1"]; ok {
		t.Error("network chip included by default")
	}
	// Symlinked device names must map to distinct disks, not collide on "nvme".
	nvme0, ok0 := byKey["nvme:nvme0"]
	nvme2, ok2 := byKey["nvme:nvme2"]
	if !ok0 || !ok2 {
		t.Fatalf("nvme keys missing: %+v", got)
	}
	if nvme0.Celsius != 35.25 || nvme2.Celsius != 40 {
		t.Errorf("nvme temps = %v %v", nvme0.Celsius, nvme2.Celsius)
	}
}

func TestHwmonIncludeReplaceDefaultsAndWarn(t *testing.T) {
	h := Hwmon{
		Root:    fakeSysfs(t),
		Include: regexp.MustCompile(`k10temp:Tccd1|r8169_0_e00:00:temp1`),
		WarnC:   25,
	}
	got, err := h.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("readings = %+v", got)
	}
	for _, r := range got {
		if !r.Warn {
			t.Errorf("%s should be over 25C: %+v", r.Key, r)
		}
	}
}

func TestHwmonExclude(t *testing.T) {
	h := Hwmon{Root: fakeSysfs(t), Exclude: regexp.MustCompile(`Tctl`)}
	got, err := h.Read()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range got {
		if strings.Contains(r.Key, "Tctl") {
			t.Errorf("excluded sensor returned: %+v", r)
		}
	}
	if len(got) != 2 {
		t.Errorf("readings = %+v, want only the two NVMe disks", got)
	}
}

func TestHwmonFollowsClassSymlinks(t *testing.T) {
	real := filepath.Join(t.TempDir(), "devices", "hwmon", "hwmon0")
	writeFile(t, filepath.Join(real, "name"), "k10temp\n")
	writeFile(t, filepath.Join(real, "temp1_input"), "50000\n")
	writeFile(t, filepath.Join(real, "temp1_label"), "Tctl\n")
	root := t.TempDir()
	if err := os.Symlink(real, filepath.Join(root, "hwmon0")); err != nil {
		t.Fatal(err)
	}
	got, err := Hwmon{Root: root}.Read()
	if err != nil || len(got) != 1 || got[0].Celsius != 50 {
		t.Fatalf("got %+v, %v; want one CPU reading", got, err)
	}
}

func TestHwmonMissingRootIsEmpty(t *testing.T) {
	h := Hwmon{Root: filepath.Join(t.TempDir(), "nope")}
	got, err := h.Read()
	if err != nil || len(got) != 0 {
		t.Fatalf("got %v, %v; want empty, nil", got, err)
	}
}

func TestHwmonIgnoresGarbageValues(t *testing.T) {
	root := filepath.Join(t.TempDir(), "hwmon")
	writeFile(t, filepath.Join(root, "hwmon0", "name"), "k10temp\n")
	writeFile(t, filepath.Join(root, "hwmon0", "temp1_input"), "not-a-number\n")
	writeFile(t, filepath.Join(root, "hwmon0", "temp1_label"), "Tctl\n")
	writeFile(t, filepath.Join(root, "hwmon0", "temp2_input"), "45000\n")
	got, err := Hwmon{Root: root}.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Key != "k10temp:temp2" {
		t.Errorf("got %+v", got)
	}
}
