package collect

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// ContainerSnapshot is one running container's current resource usage.
type ContainerSnapshot struct {
	Name     string
	ID       string
	Image    string
	CPUPct   float64
	MemUsed  uint64
	MemLimit uint64
	MemPct   float64
	NetRxBps float64
	NetTxBps float64

	netRx uint64
	netTx uint64
}

type containerSummary struct {
	ID    string   `json:"Id"`
	Names []string `json:"Names"`
	Image string   `json:"Image"`
	State string   `json:"State"`
}

type dockerStats struct {
	CPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
		OnlineCPUs     uint32 `json:"online_cpus"`
	} `json:"cpu_stats"`
	PreCPUStats struct {
		CPUUsage struct {
			TotalUsage  uint64   `json:"total_usage"`
			PercpuUsage []uint64 `json:"percpu_usage"`
		} `json:"cpu_usage"`
		SystemCPUUsage uint64 `json:"system_cpu_usage"`
	} `json:"precpu_stats"`
	MemoryStats struct {
		Usage uint64            `json:"usage"`
		Limit uint64            `json:"limit"`
		Stats map[string]uint64 `json:"stats"`
	} `json:"memory_stats"`
	Networks map[string]struct {
		RxBytes uint64 `json:"rx_bytes"`
		TxBytes uint64 `json:"tx_bytes"`
	} `json:"networks"`
}

type dockerAPI interface {
	Containers(ctx context.Context) ([]containerSummary, error)
	Stats(ctx context.Context, id string) (dockerStats, error)
}

// Docker polls the Docker Engine API. Network rates are derived from the
// cumulative counters, so the first poll reports zero for every container.
type Docker struct {
	api     dockerAPI
	exclude *regexp.Regexp
	prev    map[string]netSample
}

type netSample struct {
	rx uint64
	tx uint64
	at time.Time
}

// NewDocker wires the collector to an API client.
func NewDocker(api dockerAPI, exclude *regexp.Regexp) *Docker {
	return &Docker{api: api, exclude: exclude, prev: make(map[string]netSample)}
}

// Read lists running containers and fetches stats concurrently. A failure on
// one container does not discard the others; the errors are joined.
func (d *Docker) Read(ctx context.Context, now time.Time) ([]ContainerSnapshot, error) {
	containers, err := d.api.Containers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	var errs []error
	kept := make([]containerSummary, 0, len(containers))
	for _, c := range containers {
		name := containerName(c)
		if d.exclude != nil && d.exclude.MatchString(name) {
			continue
		}
		kept = append(kept, c)
	}

	type result struct {
		index int
		snap  ContainerSnapshot
		err   error
	}
	results := make(chan result, len(kept))
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for i, c := range kept {
		wg.Add(1)
		go func(i int, c containerSummary) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			sctx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			st, err := d.api.Stats(sctx, c.ID)
			if err != nil {
				results <- result{index: i, err: fmt.Errorf("%s: %w", containerName(c), err)}
				return
			}
			results <- result{index: i, snap: snapshotFromStats(c, st)}
		}(i, c)
	}
	wg.Wait()
	close(results)

	var out []ContainerSnapshot
	stats := make([]*ContainerSnapshot, len(kept))
	for r := range results {
		if r.err != nil {
			errs = append(errs, r.err)
			continue
		}
		s := r.snap
		stats[r.index] = &s
	}

	alive := make(map[string]bool, len(kept))
	for i := range kept {
		s := stats[i]
		if s == nil {
			continue
		}
		alive[s.ID] = true
		s.NetRxBps, s.NetTxBps = d.rates(*s, now)
		out = append(out, *s)
	}
	for id := range d.prev {
		if !alive[id] {
			delete(d.prev, id)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, joinErrs(errs)
}

// rates converts cumulative rx/tx counters into bytes per second.
func (d *Docker) rates(s ContainerSnapshot, now time.Time) (rx, tx float64) {
	cur := netSample{rx: s.netRx, tx: s.netTx, at: now}
	prev, ok := d.prev[s.ID]
	d.prev[s.ID] = cur
	if !ok {
		return 0, 0
	}
	dt := now.Sub(prev.at).Seconds()
	if dt <= 0 {
		return 0, 0
	}
	if cur.rx >= prev.rx {
		rx = float64(cur.rx-prev.rx) / dt
	}
	if cur.tx >= prev.tx {
		tx = float64(cur.tx-prev.tx) / dt
	}
	return rx, tx
}

func containerName(c containerSummary) string {
	if len(c.Names) > 0 {
		return strings.TrimPrefix(c.Names[0], "/")
	}
	if len(c.ID) > 12 {
		return c.ID[:12]
	}
	return c.ID
}

func snapshotFromStats(c containerSummary, st dockerStats) ContainerSnapshot {
	used := memUsed(st)
	s := ContainerSnapshot{
		Name:     containerName(c),
		ID:       c.ID,
		Image:    c.Image,
		CPUPct:   cpuPercent(st),
		MemUsed:  used,
		MemLimit: st.MemoryStats.Limit,
	}
	if s.MemLimit > 0 {
		s.MemPct = float64(used) / float64(s.MemLimit) * 100
	}
	for _, n := range st.Networks {
		s.netRx += n.RxBytes
		s.netTx += n.TxBytes
	}
	return s
}

// cpuPercent mirrors docker stats: container CPU time over system CPU time,
// multiplied by the number of online CPUs, so it may exceed 100%.
func cpuPercent(st dockerStats) float64 {
	if st.PreCPUStats.SystemCPUUsage == 0 ||
		st.CPUStats.SystemCPUUsage <= st.PreCPUStats.SystemCPUUsage {
		return 0
	}
	cpuDelta := st.CPUStats.CPUUsage.TotalUsage
	if cpuDelta <= st.PreCPUStats.CPUUsage.TotalUsage {
		return 0
	}
	cpuDelta -= st.PreCPUStats.CPUUsage.TotalUsage
	sysDelta := st.CPUStats.SystemCPUUsage - st.PreCPUStats.SystemCPUUsage
	cpus := st.CPUStats.OnlineCPUs
	if cpus == 0 {
		cpus = uint32(len(st.CPUStats.CPUUsage.PercpuUsage))
	}
	if cpus == 0 {
		cpus = 1
	}
	return float64(cpuDelta) / float64(sysDelta) * float64(cpus) * 100
}

// memUsed subtracts the page cache, matching docker stats.
func memUsed(st dockerStats) uint64 {
	usage := st.MemoryStats.Usage
	cache := st.MemoryStats.Stats["inactive_file"]
	if cache == 0 {
		cache = st.MemoryStats.Stats["total_inactive_file"]
	}
	if cache == 0 {
		cache = st.MemoryStats.Stats["cache"]
	}
	if cache >= usage {
		return 0
	}
	return usage - cache
}

// DockerClient talks to the Docker Engine over its unix socket. Only the two
// endpoints used by the collector are implemented.
type DockerClient struct {
	base string
	hc   *http.Client

	mu         sync.Mutex
	version    string
	negotiated bool
}

// NewDockerClient returns a client for the socket at path.
func NewDockerClient(socketPath string) *DockerClient {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			d := &net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, "unix", socketPath)
		},
	}
	return &DockerClient{
		base: "http://docker",
		hc:   &http.Client{Transport: tr, Timeout: 15 * time.Second},
	}
}

func newDockerClientForTest(base string, hc *http.Client) *DockerClient {
	return &DockerClient{base: base, hc: hc}
}

// Containers lists running containers.
func (c *DockerClient) Containers(ctx context.Context) ([]containerSummary, error) {
	var out []containerSummary
	if err := c.get(ctx, "/containers/json", &out); err != nil {
		return nil, err
	}
	return out, nil
}

// Stats returns one non-streaming stats sample for a container.
func (c *DockerClient) Stats(ctx context.Context, id string) (dockerStats, error) {
	var out dockerStats
	err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/stats?stream=false", &out)
	return out, err
}

func (c *DockerClient) get(ctx context.Context, path string, dst any) error {
	c.ensureVersion(ctx)
	c.mu.Lock()
	version := c.version
	c.mu.Unlock()

	base := c.base
	if version != "" {
		base += "/v" + version
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("docker API %s: %s: %s", path, resp.Status, strings.TrimSpace(string(body)))
	}
	dec := json.NewDecoder(io.LimitReader(resp.Body, 8<<20))
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("docker API %s: decode: %w", path, err)
	}
	return nil
}

// ensureVersion negotiates the API version once. If the ping fails the client
// proceeds unversioned and the real request reports the actual problem.
func (c *DockerClient) ensureVersion(ctx context.Context) {
	c.mu.Lock()
	done := c.negotiated
	c.mu.Unlock()
	if done {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/_ping", nil)
	version := ""
	if err == nil {
		if resp, err := c.hc.Do(req); err == nil {
			version = resp.Header.Get("Api-Version")
			io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
			resp.Body.Close()
		}
	}
	c.mu.Lock()
	c.version = version
	c.negotiated = true
	c.mu.Unlock()
}

func joinErrs(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	}
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error()
	}
	return fmt.Errorf("%s", strings.Join(msgs, "; "))
}
