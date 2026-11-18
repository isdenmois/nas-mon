// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Defaults for every tunable. A zero interval disables its collector.
const (
	DefaultPort              = 8090
	DefaultDBPath            = "/data/mon.db"
	DefaultDockerSocket      = "/var/run/docker.sock"
	DefaultVolumesDir        = "/volumes"
	DefaultTempInterval      = 30 * time.Second
	DefaultDockerInterval    = time.Minute
	DefaultHostInterval      = 30 * time.Second
	DefaultVolumeInterval    = 5 * time.Minute
	DefaultSmartInterval     = 12 * time.Hour
	DefaultAggregateInterval = 5 * time.Minute
	DefaultRetentionMinute   = 48 * time.Hour
	DefaultRetentionHour     = 90 * 24 * time.Hour
	DefaultRetentionDay      = 5 * 365 * 24 * time.Hour
	DefaultRetentionWear     = 2 * 365 * 24 * time.Hour
	DefaultTempWarnC         = 80.0
	DefaultLogLevel          = "info"

	// DefaultPprofAddr is empty: the profiler is opt-in and never on the public port.
	DefaultPprofAddr = ""
)

// Config is the validated runtime configuration.
type Config struct {
	Port              int
	DBPath            string
	DockerSocket      string
	VolumesDir        string
	TempInterval      time.Duration
	DockerInterval    time.Duration
	HostInterval      time.Duration
	VolumeInterval    time.Duration
	SmartInterval     time.Duration
	AggregateInterval time.Duration
	RetentionMinute   time.Duration
	RetentionHour     time.Duration
	RetentionDay      time.Duration
	RetentionWear     time.Duration
	TempWarnC         float64
	TempInclude       *regexp.Regexp
	TempExclude       *regexp.Regexp
	DockerExclude     *regexp.Regexp
	LogLevel          string
	PprofAddr         string
}

// Addr returns the listen address for the HTTP server.
func (c Config) Addr() string { return fmt.Sprintf(":%d", c.Port) }

// FromEnv parses configuration using getenv. It returns an error naming the
// offending variable so startup fails loudly instead of silently using defaults.
func FromEnv(getenv func(string) string) (Config, error) {
	c := Config{
		Port:              DefaultPort,
		DBPath:            DefaultDBPath,
		DockerSocket:      DefaultDockerSocket,
		VolumesDir:        DefaultVolumesDir,
		TempInterval:      DefaultTempInterval,
		DockerInterval:    DefaultDockerInterval,
		HostInterval:      DefaultHostInterval,
		VolumeInterval:    DefaultVolumeInterval,
		SmartInterval:     DefaultSmartInterval,
		AggregateInterval: DefaultAggregateInterval,
		RetentionMinute:   DefaultRetentionMinute,
		RetentionHour:     DefaultRetentionHour,
		RetentionDay:      DefaultRetentionDay,
		RetentionWear:     DefaultRetentionWear,
		TempWarnC:         DefaultTempWarnC,
		LogLevel:          DefaultLogLevel,
		PprofAddr:         DefaultPprofAddr,
	}

	var errs []error
	setInt := func(env string, dst *int) {
		raw := strings.TrimSpace(getenv(env))
		if raw == "" {
			return
		}
		v, err := strconv.Atoi(raw)
		if err != nil || v < 1 || v > 65535 {
			errs = append(errs, fmt.Errorf("%s: want port 1..65535, got %q", env, raw))
			return
		}
		*dst = v
	}
	setDur := func(env string, dst *time.Duration) {
		raw := strings.TrimSpace(getenv(env))
		if raw == "" {
			return
		}
		d, err := parseDuration(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", env, err))
			return
		}
		*dst = d
	}
	setStr := func(env string, dst *string) {
		if v := strings.TrimSpace(getenv(env)); v != "" {
			*dst = v
		}
	}
	setRegexp := func(env string, dst **regexp.Regexp) {
		raw := strings.TrimSpace(getenv(env))
		if raw == "" {
			return
		}
		re, err := regexp.Compile(raw)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %v", env, err))
			return
		}
		*dst = re
	}

	setInt("MON_PORT", &c.Port)
	setStr("MON_DB_PATH", &c.DBPath)
	setStr("MON_DOCKER_SOCKET", &c.DockerSocket)
	setStr("MON_VOLUMES_DIR", &c.VolumesDir)
	setDur("MON_TEMP_INTERVAL", &c.TempInterval)
	setDur("MON_DOCKER_INTERVAL", &c.DockerInterval)
	setDur("MON_HOST_INTERVAL", &c.HostInterval)
	setDur("MON_VOLUME_INTERVAL", &c.VolumeInterval)
	setDur("MON_SMART_INTERVAL", &c.SmartInterval)
	setDur("MON_AGGREGATE_INTERVAL", &c.AggregateInterval)
	setDur("MON_RETENTION_MINUTE", &c.RetentionMinute)
	setDur("MON_RETENTION_HOUR", &c.RetentionHour)
	setDur("MON_RETENTION_DAY", &c.RetentionDay)
	setDur("MON_RETENTION_WEAR", &c.RetentionWear)
	setRegexp("MON_TEMP_INCLUDE", &c.TempInclude)
	setRegexp("MON_TEMP_EXCLUDE", &c.TempExclude)
	setRegexp("MON_DOCKER_EXCLUDE", &c.DockerExclude)
	setStr("MON_LOG_LEVEL", &c.LogLevel)
	setStr("MON_PPROF_ADDR", &c.PprofAddr)

	if raw := strings.TrimSpace(getenv("MON_TEMP_WARN_C")); raw != "" {
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil || v <= 0 {
			errs = append(errs, fmt.Errorf("MON_TEMP_WARN_C: want positive number, got %q", raw))
		} else {
			c.TempWarnC = v
		}
	}
	if c.AggregateInterval <= 0 {
		errs = append(errs, errors.New("MON_AGGREGATE_INTERVAL: must be positive"))
	}
	if c.RetentionMinute < 0 || c.RetentionHour < 0 || c.RetentionDay < 0 || c.RetentionWear < 0 {
		errs = append(errs, errors.New("MON_RETENTION_MINUTE, MON_RETENTION_HOUR, MON_RETENTION_DAY and MON_RETENTION_WEAR must not be negative (0 keeps data forever)"))
	}
	if c.PprofAddr != "" && !isLoopbackAddr(c.PprofAddr) {
		errs = append(errs, fmt.Errorf("MON_PPROF_ADDR: want a loopback address such as 127.0.0.1:6061, got %q", c.PprofAddr))
	}
	if len(errs) > 0 {
		return Config{}, errors.Join(errs...)
	}
	return c, nil
}

// parseDuration accepts Go duration strings ("10s", "12h") and bare seconds.
func parseDuration(raw string) (time.Duration, error) {
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if n < 0 {
			return 0, fmt.Errorf("duration must not be negative: %q", raw)
		}
		return time.Duration(n) * time.Second, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q (use 10s, 15m, 12h or seconds)", raw)
	}
	if d < 0 {
		return 0, fmt.Errorf("duration must not be negative: %q", raw)
	}
	return d, nil
}

// isLoopbackAddr reports whether addr listens only on the loopback interface.
// The profiler exposes goroutine, heap and CPU internals, so it must never be
// reachable from the LAN.
func isLoopbackAddr(addr string) bool {
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
