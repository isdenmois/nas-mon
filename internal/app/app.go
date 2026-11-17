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
	"sync"
	"time"

	"mon/internal/collect"
	"mon/internal/config"
	"mon/internal/domain"
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
		a.rec.Add(domain.Temp, t.Key, t.Celsius)
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
	a.rec.Add(domain.HostCPU, "host", snap.CPUPct)
	a.rec.Add(domain.HostMem, "host", snap.MemPct)
	a.rec.Add(domain.HostLoad1, "host", snap.Load1)
	a.rec.Add(domain.HostLoad5, "host", snap.Load5)
	a.rec.Add(domain.HostLoad15, "host", snap.Load15)
	a.rec.Add(domain.HostNetRx, "host", snap.NetRxBps)
	a.rec.Add(domain.HostNetTx, "host", snap.NetTxBps)
	return nil
}

func (a *App) collectDocker(ctx context.Context) error {
	now := time.Now()
	containers, err := a.docker.Read(ctx, now)
	a.st.SetContainers(now, containers)
	for _, c := range containers {
		a.rec.Add(domain.ContainerCPU, c.Name, c.CPUPct)
		a.rec.Add(domain.ContainerMem, c.Name, float64(c.MemUsed))
		a.rec.Add(domain.ContainerMemPct, c.Name, c.MemPct)
		a.rec.Add(domain.ContainerNetRx, c.Name, c.NetRxBps)
		a.rec.Add(domain.ContainerNetTx, c.Name, c.NetTxBps)
	}
	return err
}

func (a *App) collectVolumes(ctx context.Context) error {
	now := time.Now()
	volumes, err := a.volumes.Read()
	a.st.SetVolumes(now, volumes)
	for _, v := range volumes {
		a.rec.Add(domain.VolumeUsedPct, v.Name, v.UsedPct)
	}
	return err
}

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
		cur := diskObservation(d, now)
		reasons := domain.Decide(previousObservation(prev), cur, domain.TBWStep)
		if len(reasons) == 0 {
			continue
		}
		w := store.Wear{
			Disk: cur.Disk, TS: cur.TS, Model: cur.Model, Serial: cur.Serial,
			PercentageUsed: cur.PercentageUsed, TBWBytes: cur.TBWBytes,
			PowerOnHours: cur.PowerOnHours, MediaErrors: cur.MediaErrors,
			Reallocated: cur.Reallocated, TempC: cur.TempC,
			Reason: domain.FormatReasons(reasons),
		}
		if err := a.db.AppendWear(w); err != nil {
			errs = append(errs, fmt.Errorf("append wear %s: %w", d.Disk, err))
		}
	}
	return errors.Join(errs...)
}

// diskObservation turns a SMART reading into the domain's wear observation.
func diskObservation(d collect.DiskSmart, at time.Time) domain.WearObservation {
	return domain.WearObservation{
		Disk: d.Disk, TS: at.Unix(), Model: d.Model, Serial: d.Serial,
		PercentageUsed: d.PercentageUsed, TBWBytes: d.TBWBytes,
		PowerOnHours: d.PowerOnHours, MediaErrors: d.MediaErrors,
		Reallocated: d.Reallocated, TempC: d.TempC,
	}
}

// previousObservation turns the stored wear row into a domain observation.
func previousObservation(prev *store.Wear) *domain.WearObservation {
	if prev == nil {
		return nil
	}
	return &domain.WearObservation{
		Disk: prev.Disk, TS: prev.TS, Model: prev.Model, Serial: prev.Serial,
		PercentageUsed: prev.PercentageUsed, TBWBytes: prev.TBWBytes,
		PowerOnHours: prev.PowerOnHours, MediaErrors: prev.MediaErrors,
		Reallocated: prev.Reallocated, TempC: prev.TempC,
	}
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
