package collect

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseMountInfo(t *testing.T) {
	mi := `36 35 0:32 / /volumes/root rw,relatime - ext4 /dev/sda1 rw
37 35 0:33 /srv/media /volumes/media rw,relatime - ext4 /dev/sdb1 rw
38 35 0:34 /mnt/my\040disk /volumes/disk rw,relatime - ext4 /dev/sdc1 rw
`
	got, err := parseMountInfo(strings.NewReader(mi))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"/volumes/root":  "/",
		"/volumes/media": "/srv/media",
		"/volumes/disk":  "/mnt/my disk",
	}
	for point, root := range want {
		if got[point] != root {
			t.Errorf("mount %s = %q, want %q", point, got[point], root)
		}
	}
}

func TestVolumesReadsEveryMountedFilesystem(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "volumes")
	for _, name := range []string{"root", "media", ".hidden", "file.txt"} {
		path := filepath.Join(dir, name)
		if name == "file.txt" {
			os.MkdirAll(dir, 0o755)
			os.WriteFile(path, []byte("x"), 0o644)
			continue
		}
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mi := filepath.Join(dir, "mountinfo")
	os.WriteFile(mi, []byte(
		fmt.Sprintf("36 35 0:32 / %s rw - ext4 /dev/sda1 rw\n", filepath.Join(dir, "root"))+
			fmt.Sprintf("37 35 0:33 /srv/media %s rw - ext4 /dev/sdb1 rw\n", filepath.Join(dir, "media"))), 0o644)

	v := Volumes{Dir: dir, MountInfo: mi}
	got, err := v.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("volumes = %+v, want root and media", got)
	}
	if got[0].Name != "/" || got[1].Name != "/srv/media" {
		t.Errorf("names = %q %q", got[0].Name, got[1].Name)
	}
	for _, vol := range got {
		if vol.Total == 0 || vol.UsedPct < 0 || vol.UsedPct > 100 {
			t.Errorf("unexpected stats: %+v", vol)
		}
	}
}

func TestVolumesMissingDirIsEmpty(t *testing.T) {
	v := Volumes{Dir: filepath.Join(t.TempDir(), "nope"), MountInfo: "/nonexistent"}
	got, err := v.Read()
	if err != nil || len(got) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}
