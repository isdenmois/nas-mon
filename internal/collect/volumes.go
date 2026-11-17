package collect

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
)

// VolumeSnapshot is the usage of one monitored filesystem.
type VolumeSnapshot struct {
	Name          string
	ContainerPath string
	Used          uint64
	Total         uint64
	UsedPct       float64
}

// Volumes reports usage for every subdirectory of Dir, which compose maps to
// host filesystems. MountInfo is consulted to show the host path instead of the
// container path.
type Volumes struct {
	Dir       string
	MountInfo string
}

// Read returns one snapshot per mounted filesystem, sorted by name.
func (v Volumes) Read() ([]VolumeSnapshot, error) {
	dir := v.Dir
	if dir == "" {
		dir = "/volumes"
	}
	mounts := map[string]string{}
	if mi := v.mountInfoPath(); mi != "" {
		if m, err := readMountInfo(mi); err == nil {
			mounts = m
		}
	}

	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []VolumeSnapshot
	var errs []error
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		var st syscall.Statfs_t
		if err := syscall.Statfs(path, &st); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		bsize := uint64(st.Bsize)
		total := st.Blocks * bsize
		free := st.Bfree * bsize
		avail := st.Bavail * bsize
		used := total - free
		if total == 0 {
			continue
		}
		name := mounts[filepath.Clean(path)]
		if name == "" {
			name = e.Name()
		}
		pct := float64(used) / float64(used+avail) * 100
		if used+avail == 0 {
			pct = 0
		}
		out = append(out, VolumeSnapshot{
			Name:          name,
			ContainerPath: path,
			Used:          used,
			Total:         total,
			UsedPct:       pct,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if len(errs) > 0 {
		return out, joinErrs(errs)
	}
	return out, nil
}

func (v Volumes) mountInfoPath() string {
	if v.MountInfo != "" {
		return v.MountInfo
	}
	return "/proc/self/mountinfo"
}

// readMountInfo maps a mount point to its root inside the original filesystem.
// For bind mounts that root is the host path, e.g. "/srv/media".
func readMountInfo(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseMountInfo(f)
}

func parseMountInfo(r io.Reader) (map[string]string, error) {
	out := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 5 {
			continue
		}
		root := unescapeMount(fields[3])
		point := unescapeMount(fields[4])
		out[point] = root
	}
	return out, sc.Err()
}

var mountEscaper = strings.NewReplacer(
	`\040`, " ",
	`\011`, "\t",
	`\012`, "\n",
	`\134`, `\`,
)

func unescapeMount(s string) string { return mountEscaper.Replace(s) }
