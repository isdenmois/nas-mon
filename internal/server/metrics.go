package server

import (
	"embed"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// distFS carries the Vite build. A placeholder index.html is committed so a
// plain `go build` works before the first frontend build.
//
//go:embed all:dist
var distFS embed.FS

func (s *Server) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(s.metrics()))
}

// metrics renders the current snapshot in Prometheus text exposition format.
func (s *Server) metrics() string {
	snap := s.state.Snapshot()
	m := &metricBuf{seen: map[string]bool{}}

	temps := snap.Temps
	sort.Slice(temps, func(i, j int) bool { return temps[i].Key < temps[j].Key })
	for _, t := range temps {
		m.metric("mon_temperature_celsius", "Current temperature of a selected sensor.",
			labels{"chip": t.Chip, "label": t.Label, "sensor": t.Key}, t.Celsius)
	}

	if h := snap.Host; h != nil {
		m.metric("mon_host_cpu_percent", "Host CPU utilisation.", nil, h.CPUPct)
		m.metric("mon_host_memory_used_bytes", "Host memory used.", nil, float64(h.MemUsed))
		m.metric("mon_host_memory_total_bytes", "Host memory total.", nil, float64(h.MemTotal))
		m.metric("mon_host_memory_percent", "Host memory used percentage.", nil, h.MemPct)
		m.metric("mon_host_load1", "Host 1-minute load average.", nil, h.Load1)
		m.metric("mon_host_load5", "Host 5-minute load average.", nil, h.Load5)
		m.metric("mon_host_load15", "Host 15-minute load average.", nil, h.Load15)
		m.metric("mon_host_network_receive_bytes_per_second", "Host receive rate.", nil, h.NetRxBps)
		m.metric("mon_host_network_transmit_bytes_per_second", "Host transmit rate.", nil, h.NetTxBps)
	}

	containers := snap.Containers
	sort.Slice(containers, func(i, j int) bool { return containers[i].Name < containers[j].Name })
	for _, c := range containers {
		l := labels{"name": c.Name, "image": c.Image, "id": c.ID}
		m.metric("mon_container_cpu_percent", "Container CPU usage; 100 is one full core.", l, c.CPUPct)
		m.metric("mon_container_memory_used_bytes", "Container memory used.", l, float64(c.MemUsed))
		m.metric("mon_container_memory_limit_bytes", "Container memory limit.", l, float64(c.MemLimit))
		m.metric("mon_container_memory_percent", "Container memory usage percentage.", l, c.MemPct)
		m.metric("mon_container_network_receive_bytes_per_second", "Container receive rate.", l, c.NetRxBps)
		m.metric("mon_container_network_transmit_bytes_per_second", "Container transmit rate.", l, c.NetTxBps)
	}

	vols := snap.Volumes
	sort.Slice(vols, func(i, j int) bool { return vols[i].Name < vols[j].Name })
	for _, v := range vols {
		l := labels{"volume": v.Name}
		m.metric("mon_volume_used_bytes", "Filesystem bytes used.", l, float64(v.Used))
		m.metric("mon_volume_total_bytes", "Filesystem size.", l, float64(v.Total))
		m.metric("mon_volume_used_percent", "Filesystem usage percentage.", l, v.UsedPct)
	}

	smart := snap.Smart
	sort.Slice(smart, func(i, j int) bool { return smart[i].Disk < smart[j].Disk })
	for _, d := range smart {
		l := labels{"disk": d.Disk, "model": d.Model, "serial": d.Serial}
		if d.PercentageUsed != nil {
			m.metric("mon_smart_percentage_used", "SSD wear used percentage.", l, float64(*d.PercentageUsed))
		}
		if d.TBWBytes != nil {
			m.metric("mon_smart_data_written_bytes", "Lifetime bytes written according to SMART.", l, float64(*d.TBWBytes))
		}
		if d.PowerOnHours != nil {
			m.metric("mon_smart_power_on_hours", "Drive power-on hours.", l, float64(*d.PowerOnHours))
		}
		if d.MediaErrors != nil {
			m.metric("mon_smart_media_errors", "Drive media errors.", l, float64(*d.MediaErrors))
		}
		if d.Reallocated != nil {
			m.metric("mon_smart_reallocated_sectors", "Drive reallocated sectors.", l, float64(*d.Reallocated))
		}
		if d.TempC != nil {
			m.metric("mon_smart_temperature_celsius", "Drive temperature from SMART.", l, float64(*d.TempC))
		}
	}

	names := make([]string, 0, len(snap.Collectors))
	for name := range snap.Collectors {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		st := snap.Collectors[name]
		l := labels{"collector": name}
		up := 0.0
		if st.LastError == "" && !st.LastSuccess.IsZero() {
			up = 1
		}
		m.metric("mon_collector_up", "Whether the last collector run succeeded.", l, up)
		if !st.LastSuccess.IsZero() {
			m.metric("mon_collector_last_success_timestamp_seconds", "Unix time of the last successful run.", l,
				float64(st.LastSuccess.Unix()))
		}
		m.metric("mon_collector_duration_seconds", "Duration of the last collector run.", l,
			float64(st.DurationMS)/1000)
	}
	return m.b.String()
}

type labels map[string]string

type metricBuf struct {
	b    strings.Builder
	seen map[string]bool
}

func (m *metricBuf) metric(name, help string, l labels, v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return
	}
	if !m.seen[name] {
		m.seen[name] = true
		m.b.WriteString("# HELP " + name + " " + help + "\n")
		m.b.WriteString("# TYPE " + name + " gauge\n")
	}
	m.b.WriteString(name)
	if len(l) > 0 {
		keys := make([]string, 0, len(l))
		for k := range l {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		m.b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				m.b.WriteByte(',')
			}
			m.b.WriteString(k + `="` + escapeLabel(l[k]) + `"`)
		}
		m.b.WriteByte('}')
	}
	m.b.WriteByte(' ')
	m.b.WriteString(formatFloat(v))
	m.b.WriteByte('\n')
}

func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func escapeLabel(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return strings.ReplaceAll(s, `"`, `\"`)
}
