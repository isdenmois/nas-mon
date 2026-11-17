// Package app wires collectors, storage, the recorder and the HTTP server.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"mon/internal/collect"
	"mon/internal/config"
	"mon/internal/model"
	"mon/internal/recorder"
	"mon/internal/server"
	"mon/internal/state"
	"mon/internal/store"
)

// App is the running monitor process.
type App struct {
	cfg config.Config
	log *slog.Logger
	db  *store.DB
	rec *recorder.Recorder
	st  *state.State

	hwmon   collect.Hwmon
	host    *collect.Host
	docker  *collect.Docker
	volumes collect.Volumes
	smart   collect.Smart

	http *http.Server
}

// New prepares storage, collectors and the HTTP server. It does not start
// anything; call Run.
func New(cfg config.Config, log *slog.Logger) (*App, error) {
	if log == nil {
		log = slog.Default()
	}
	if dir := filepath.Dir(cfg.DBPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	db, err := store.Open(cfg.DBPath, cfg.RetentionMinute, cfg.RetentionHour)
	if err != nil {
		return nil, err
	}

	a := &App{
		cfg: cfg,
		log: log,
		db:  db,
		rec: recorder.New(),
		st:  state.New(),
		hwmon: collect.Hwmon{
			Root:    "/sys/class/hwmon",
			Include: cfg.TempInclude,
			Exclude: cfg.TempExclude,
			WarnC:   cfg.TempWarnC,
		},
		host:    &collect.Host{ProcRoot: "/proc"},
		volumes: collect.Volumes{Dir: cfg.VolumesDir},
		smart:   collect.Smart{},
	}
	a.docker = collect.NewDocker(collect.NewDockerClient(cfg.DockerSocket), cfg.DockerExclude)

	a.http = &http.Server{
		Addr:              cfg.Addr(),
		Handler:           server.New(a.st, db, log).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	return a, nil
}

// Run starts the collectors and the HTTP server and blocks until ctx is
// cancelled. The last partial minute is intentionally not flushed.
func (a *App) Run(ctx context.Context) error {
	defer a.db.Close()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	start := func(fn func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			fn()
		}()
	}

	start(func() { a.aggregateLoop(ctx) })
	if a.cfg.TempInterval > 0 {
		start(func() { a.loop(ctx, "temp", a.cfg.TempInterval, a.collectTemps) })
	}
	if a.cfg.HostInterval > 0 {
		start(func() { a.loop(ctx, "host", a.cfg.HostInterval, a.collectHost) })
	}
	if a.cfg.DockerInterval > 0 {
		start(func() { a.loop(ctx, "docker", a.cfg.DockerInterval, a.collectDocker) })
	}
	if a.cfg.VolumeInterval > 0 {
		start(func() { a.loop(ctx, "volumes", a.cfg.VolumeInterval, a.collectVolumes) })
	}
	if a.cfg.SmartInterval > 0 {
		start(func() { a.loop(ctx, "smart", a.cfg.SmartInterval, a.collectSmart) })
	}

	errCh := make(chan error, 1)
	go func() {
		a.log.Info("listening", "addr", a.cfg.Addr(), "db", a.cfg.DBPath)
		err := a.http.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		return err
	}

	shutdownCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	shutdownErr := a.http.Shutdown(shutdownCtx)
	cancel()
	wg.Wait()
	if shutdownErr != nil {
		return fmt.Errorf("http shutdown: %w", shutdownErr)
	}
	a.log.Info("stopped")
	return nil
}

func (a *App) loop(ctx context.Context, name string, interval time.Duration, fn func(context.Context) error) {
	var lastErr string
	run := func() {
		started := time.Now()
		timeout := interval
		if timeout > 2*time.Minute {
			timeout = 2 * time.Minute
		}
		runCtx, cancel := context.WithTimeout(ctx, timeout+15*time.Second)
		err := fn(runCtx)
		cancel()
		a.st.SetCollector(name, started, err, time.Since(started))
		if err != nil {
			msg := err.Error()
			if msg != lastErr {
				a.log.Warn("collector failed", "collector", name, "err", err)
			}
			lastErr = msg
		} else if lastErr != "" {
			a.log.Info("collector recovered", "collector", name)
			lastErr = ""
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func (a *App) collectTemps(ctx context.Context) error {
	temps, err := a.hwmon.Read()
	if err != nil {
		return err
	}
	now := time.Now()
	a.st.SetTemps(now, temps)
	for _, t := range temps {
		a.rec.Add(model.KindTemp, t.Key, t.Celsius)
	}
	return nil
}

func (a *App) collectHost(ctx context.Context) error {
	now := time.Now()
	snap, err := a.host.Read(now)
	if err != nil {
		return err
	}
	a.st.SetHost(now, snap)
	a.rec.Add(model.KindHostCPU, "host", snap.CPUPct)
	a.rec.Add(model.KindHostMem, "host", snap.MemPct)
	a.rec.Add(model.KindHostLoad1, "host", snap.Load1)
	a.rec.Add(model.KindHostLoad5, "host", snap.Load5)
	a.rec.Add(model.KindHostLoad15, "host", snap.Load15)
	a.rec.Add(model.KindHostNetRx, "host", snap.NetRxBps)
	a.rec.Add(model.KindHostNetTx, "host", snap.NetTxBps)
	return nil
}

func (a *App) collectDocker(ctx context.Context) error {
	now := time.Now()
	containers, err := a.docker.Read(ctx, now)
	a.st.SetContainers(now, containers)
	for _, c := range containers {
		a.rec.Add(model.KindContainerCPU, c.Name, c.CPUPct)
		a.rec.Add(model.KindContainerMem, c.Name, float64(c.MemUsed))
		a.rec.Add(model.KindContainerMemPct, c.Name, c.MemPct)
		a.rec.Add(model.KindContainerNetRx, c.Name, c.NetRxBps)
		a.rec.Add(model.KindContainerNetTx, c.Name, c.NetTxBps)
	}
	return err
}

func (a *App) collectVolumes(ctx context.Context) error {
	now := time.Now()
	volumes, err := a.volumes.Read()
	a.st.SetVolumes(now, volumes)
	for _, v := range volumes {
		a.rec.Add(model.KindVolumeUsedPct, v.Name, v.UsedPct)
	}
	return err
}

// wearLogTBWStep is the minimum TBW growth (0.1 TB, decimal) that deserves a
// wear-log row. Smaller writes are accumulated between rows.
const wearLogTBWStep = int64(100_000_000_000)

func (a *App) collectSmart(ctx context.Context) error {
	now := time.Now()
	disks, readErr := a.smart.Read(ctx)
	if disks != nil {
		a.st.SetSmart(now, disks)
	}
	var errs []error
	if readErr != nil {
		errs = append(errs, readErr)
	}
	for _, d := range disks {
		prev, err := a.db.LastWear(d.Disk)
		if err != nil {
			errs = append(errs, fmt.Errorf("last wear %s: %w", d.Disk, err))
			continue
		}
		appendRow, reason := shouldAppendWear(prev, d, wearLogTBWStep)
		if !appendRow {
			continue
		}
		w := store.Wear{
			Disk: d.Disk, TS: now.Unix(), Model: d.Model, Serial: d.Serial,
			PercentageUsed: d.PercentageUsed, TBWBytes: d.TBWBytes,
			PowerOnHours: d.PowerOnHours, MediaErrors: d.MediaErrors,
			Reallocated: d.Reallocated, TempC: d.TempC, Reason: reason,
		}
		if err := a.db.AppendWear(w); err != nil {
			errs = append(errs, fmt.Errorf("append wear %s: %w", d.Disk, err))
		}
	}
	return errors.Join(errs...)
}

// shouldAppendWear reports whether a wear row is worth writing. Only the first
// observation, a drive identity change, new errors, more wear or at least
// tbwStep of new writes qualify. Power-on hours and temperature do not: they
// change on every poll and would turn the log into noise.
func shouldAppendWear(prev *store.Wear, d collect.DiskSmart, tbwStep int64) (bool, string) {
	if prev == nil {
		return true, "first"
	}
	var reasons []string
	if prev.Model != d.Model || prev.Serial != d.Serial {
		reasons = append(reasons, "identity")
	}
	if increased(prev.MediaErrors, d.MediaErrors) || increased(prev.Reallocated, d.Reallocated) {
		reasons = append(reasons, "errors")
	}
	if increased(prev.PercentageUsed, d.PercentageUsed) {
		reasons = append(reasons, "wear")
	}
	switch {
	case prev.TBWBytes == nil && d.TBWBytes != nil:
		reasons = append(reasons, "tbw")
	case prev.TBWBytes != nil && d.TBWBytes != nil:
		delta := *d.TBWBytes - *prev.TBWBytes
		if delta >= tbwStep {
			reasons = append(reasons, "tbw")
		} else if delta < 0 {
			reasons = append(reasons, "tbw-reset")
		}
	}
	if len(reasons) == 0 {
		return false, ""
	}
	return true, strings.Join(reasons, "+")
}

// increased reports a counter growing or appearing with a non-zero value.
// A drive that starts reporting zero errors is not a significant change.
func increased(prev, cur *int64) bool {
	if cur == nil {
		return false
	}
	if prev == nil {
		return *cur > 0
	}
	return *cur > *prev
}

// flush persists every fully closed minute bucket.
func (a *App) flush(now time.Time) error {
	points := a.rec.CollectCompleted(now)
	if len(points) == 0 {
		return nil
	}
	return a.db.InsertMinute(points)
}

func (a *App) aggregateLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.AggregateInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := a.flush(time.Now()); err != nil {
				a.log.Error("store aggregates", "err", err)
			}
		}
	}
}
