package config

import (
	"strings"
	"testing"
	"time"
)

func env(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestDefaults(t *testing.T) {
	c, err := FromEnv(env(nil))
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if c.Port != 8090 || c.Addr() != ":8090" {
		t.Errorf("port = %d addr = %q", c.Port, c.Addr())
	}
	if c.TempInterval != 10*time.Second || c.DockerInterval != 15*time.Second ||
		c.HostInterval != 15*time.Second || c.VolumeInterval != time.Minute ||
		c.SmartInterval != 12*time.Hour || c.AggregateInterval != time.Minute {
		t.Errorf("unexpected intervals: %+v", c)
	}
	if c.RetentionMinute != 48*time.Hour || c.RetentionHour != 90*24*time.Hour {
		t.Errorf("unexpected retention: %v %v", c.RetentionMinute, c.RetentionHour)
	}
	if c.TempWarnC != 80 || c.TempInclude != nil || c.DockerExclude != nil {
		t.Errorf("unexpected defaults: warn=%v include=%v", c.TempWarnC, c.TempInclude)
	}
}

func TestOverridesAndBareSeconds(t *testing.T) {
	c, err := FromEnv(env(map[string]string{
		"MON_PORT":               "9090",
		"MON_DB_PATH":            "/tmp/x.db",
		"MON_TEMP_INTERVAL":      "5", // bare seconds
		"MON_DOCKER_INTERVAL":    "0", // disabled
		"MON_HOST_INTERVAL":      "1m",
		"MON_VOLUME_INTERVAL":    "2m",
		"MON_SMART_INTERVAL":     "6h",
		"MON_AGGREGATE_INTERVAL": "30s",
		"MON_RETENTION_MINUTE":   "24h",
		"MON_RETENTION_HOUR":     "720h",
		"MON_TEMP_WARN_C":        "85.5",
		"MON_TEMP_INCLUDE":       "^k10temp:",
		"MON_TEMP_EXCLUDE":       "Tccd",
		"MON_DOCKER_EXCLUDE":     "^ignore-",
		"MON_LOG_LEVEL":          "debug",
	}))
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if c.Port != 9090 || c.Addr() != ":9090" || c.DBPath != "/tmp/x.db" {
		t.Errorf("unexpected: %+v", c)
	}
	if c.TempInterval != 5*time.Second || c.DockerInterval != 0 ||
		c.HostInterval != time.Minute || c.VolumeInterval != 2*time.Minute ||
		c.SmartInterval != 6*time.Hour || c.AggregateInterval != 30*time.Second {
		t.Errorf("intervals: %v %v %v %v %v %v", c.TempInterval, c.DockerInterval,
			c.HostInterval, c.VolumeInterval, c.SmartInterval, c.AggregateInterval)
	}
	if c.RetentionMinute != 24*time.Hour || c.RetentionHour != 720*time.Hour {
		t.Errorf("retention: %v %v", c.RetentionMinute, c.RetentionHour)
	}
	if c.TempWarnC != 85.5 {
		t.Errorf("warn = %v", c.TempWarnC)
	}
	if !c.TempInclude.MatchString("k10temp:Tctl") || !c.TempExclude.MatchString("Tccd1") {
		t.Errorf("regexps not compiled: %v %v", c.TempInclude, c.TempExclude)
	}
	if c.DockerExclude.MatchString("keep") || !c.DockerExclude.MatchString("ignore-me") {
		t.Errorf("docker exclude: %v", c.DockerExclude)
	}
	if c.LogLevel != "debug" {
		t.Errorf("log level = %q", c.LogLevel)
	}
}

func TestInvalidValuesReportVariable(t *testing.T) {
	cases := map[string]map[string]string{
		"MON_PORT":          {"MON_PORT": "70000"},
		"MON_TEMP_INTERVAL": {"MON_TEMP_INTERVAL": "soon"},
		"MON_TEMP_INCLUDE":  {"MON_TEMP_INCLUDE": "("},
		"MON_TEMP_WARN_C":   {"MON_TEMP_WARN_C": "-1"},
	}
	for want, e := range cases {
		_, err := FromEnv(env(e))
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("env %s: err = %v, want mention of %s", want, err, want)
		}
	}
}

func TestZeroDisablesButNegativeRejected(t *testing.T) {
	c, err := FromEnv(env(map[string]string{"MON_SMART_INTERVAL": "0"}))
	if err != nil || c.SmartInterval != 0 {
		t.Fatalf("zero smart: %v %v", c.SmartInterval, err)
	}
	if _, err := FromEnv(env(map[string]string{"MON_TEMP_INTERVAL": "-5s"})); err == nil {
		t.Fatal("negative duration accepted")
	}
}
