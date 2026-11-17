// Package app is the composition root: it builds the adapters, wires them into
// the monitor and the HTTP server, and owns the collector loops, startup and
// shutdown.
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
	"mon/internal/monitor"
	"mon/internal/httpapi"
	"mon/internal/store"
)

// App is the running monitor process.
type App struct {
	cfg config.Config
	log *slog.Logger
	db  *store.DB
	mon *monitor.Monitor

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
	mon := monitor.New(db, log)

	a := &App{
		cfg: cfg,
		log: log,
		db:  db,
		mon: mon,
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
		Handler:           httpapi.New(mon, log).Handler(),
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
		a.mon.SetCollector(name, started, err, time.Since(started))
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
	a.mon.IngestTemps(time.Now(), temps)
	return nil
}

func (a *App) collectHost(ctx context.Context) error {
	now := time.Now()
	snap, err := a.host.Read(now)
	if err != nil {
		return err
	}
	a.mon.IngestHost(now, snap)
	return nil
}

func (a *App) collectDocker(ctx context.Context) error {
	now := time.Now()
	containers, err := a.docker.Read(ctx, now)
	a.mon.IngestContainers(now, containers)
	return err
}

func (a *App) collectVolumes(ctx context.Context) error {
	now := time.Now()
	volumes, err := a.volumes.Read()
	a.mon.IngestVolumes(now, volumes)
	return err
}

func (a *App) collectSmart(ctx context.Context) error {
	now := time.Now()
	disks, readErr := a.smart.Read(ctx)
	var ingestErr error
	if disks != nil {
		ingestErr = a.mon.IngestSmart(now, disks)
	}
	return errors.Join(readErr, ingestErr)
}

// flush persists every fully closed minute bucket.
func (a *App) flush(now time.Time) error {
	return a.mon.Flush(now)
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
