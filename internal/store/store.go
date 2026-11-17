// Package store persists aggregated metric buckets and the SSD wear log in
// SQLite. Minute buckets are rolled up into hours and days; old buckets are
// pruned. Every write is an idempotent upsert so a crash or retry cannot create
// duplicate history.
package store

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"

	"mon/internal/api"
	"mon/internal/domain"
)

//go:embed migrations/*.sql
var migrations embed.FS

// DB is the SQLite-backed history store.
type DB struct {
	sql             *sql.DB
	retentionMinute time.Duration
	retentionHour   time.Duration
	retentionDay    time.Duration
}

// Wear is one SSD wear snapshot.

// Open opens the database and applies the embedded migrations. A zero
// retention keeps that resolution forever.
func Open(path string, retentionMinute, retentionHour, retentionDay time.Duration) (*DB, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	sqlDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	// A single connection keeps writes serialized and avoids SQLITE_BUSY under
	// our small write load, while WAL still allows the HTTP readers to work.
	sqlDB.SetMaxOpenConns(1)
	if err := sqlDB.Ping(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("ping sqlite %s: %w", path, err)
	}
	if err := migrate(sqlDB); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate sqlite %s: %w", path, err)
	}
	return &DB{
		sql:             sqlDB,
		retentionMinute: retentionMinute,
		retentionHour:   retentionHour,
		retentionDay:    retentionDay,
	}, nil
}

// migrate applies every embedded migration that has not run yet.
func migrate(db *sql.DB) error {
	goose.SetBaseFS(migrations)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	return goose.Up(db, "migrations")
}

// Close closes the underlying database.
func (d *DB) Close() error { return d.sql.Close() }

// InsertMinute stores completed minute buckets, then refreshes the hour and day
// aggregates for the hours and days those buckets touch.
func (d *DB) InsertMinute(points []domain.Point) error {
	clean := make([]domain.Point, 0, len(points))
	for _, p := range points {
		if p.N <= 0 || !p.Kind.Valid() || p.Name == "" {
			continue
		}
		avg, okAvg := domain.Sanitize(p.Avg)
		mn, okMin := domain.Sanitize(p.Min)
		mx, okMax := domain.Sanitize(p.Max)
		if !okAvg || !okMin || !okMax {
			continue
		}
		p.Avg, p.Min, p.Max = avg, mn, mx
		p.TS = domain.Minute.Bucket(p.TS)
		clean = append(clean, p)
	}
	if len(clean) == 0 {
		return nil
	}

	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, p := range clean {
		id, err := seriesTx(tx, p.Kind, p.Name)
		if err != nil {
			return err
		}
		_, err = tx.Exec(fmt.Sprintf(`
			INSERT INTO %s (series_id, ts, avg, min, max, n)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (series_id, ts) DO UPDATE SET
				avg = (%[1]s.avg * %[1]s.n + excluded.avg * excluded.n) /
				      (%[1]s.n + excluded.n),
				min = MIN(%[1]s.min, excluded.min),
				max = MAX(%[1]s.max, excluded.max),
				n   = %[1]s.n + excluded.n`, domain.Minute.Table()),
			id, p.TS, p.Avg, p.Min, p.Max, p.N)
		if err != nil {
			return fmt.Errorf("insert minute %s:%s: %w", p.Kind, p.Name, err)
		}
	}

	minHour, maxHour := int64(math.MaxInt64), int64(math.MinInt64)
	for _, p := range clean {
		h := domain.Hour.Bucket(p.TS)
		minHour = min(minHour, h)
		maxHour = max(maxHour, h)
	}
	for h := minHour; h <= maxHour; h += domain.Hour.Seconds() {
		if err := rollup(tx, domain.Hour.Table(), domain.Minute.Table(), h, h+domain.Hour.Seconds()); err != nil {
			return err
		}
	}
	for dd := domain.Day.Bucket(minHour); dd <= maxHour; dd += domain.Day.Seconds() {
		if err := rollup(tx, domain.Day.Table(), domain.Hour.Table(), dd, dd+domain.Day.Seconds()); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return d.Prune(time.Now())
}

// rollup recomputes one parent bucket [from, to) from its child table.
func rollup(tx *sql.Tx, parent, child string, from, to int64) error {
	q := fmt.Sprintf(`
		INSERT INTO %s (series_id, ts, avg, min, max, n)
		SELECT series_id, ?, SUM(avg * n) / SUM(n), MIN(min), MAX(max), SUM(n)
		FROM %s
		WHERE ts >= ? AND ts < ?
		GROUP BY series_id
		ON CONFLICT (series_id, ts) DO UPDATE SET
			avg = excluded.avg, min = excluded.min,
			max = excluded.max, n = excluded.n`, parent, child)
	if _, err := tx.Exec(q, from, from, to); err != nil {
		return fmt.Errorf("rollup %s [%d,%d): %w", parent, from, to, err)
	}
	return nil
}

// Prune deletes minute and hour buckets older than their retention.
func (d *DB) Prune(now time.Time) error {
	if d.retentionMinute > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Minute.Table()+` WHERE ts < ?`,
			now.Add(-d.retentionMinute).Unix()); err != nil {
			return fmt.Errorf("prune minutes: %w", err)
		}
	}
	if d.retentionHour > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Hour.Table()+` WHERE ts < ?`,
			now.Add(-d.retentionHour).Unix()); err != nil {
			return fmt.Errorf("prune hours: %w", err)
		}
	}
	if d.retentionDay > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Day.Table()+` WHERE ts < ?`,
			now.Add(-d.retentionDay).Unix()); err != nil {
			return fmt.Errorf("prune days: %w", err)
		}
	}
	return nil
}

// History queries one aggregation table for the given series names. An empty
// names slice returns every series of that kind.
func (d *DB) History(kind domain.Kind, names []string, from, to int64, res domain.Resolution) ([]api.SeriesHistory, error) {
	table := res.Table()
	if table == "" {
		return nil, fmt.Errorf("unknown history resolution %q", res)
	}
	args := []any{string(kind)}
	query := fmt.Sprintf(`
		SELECT s.kind, s.name, p.ts, p.avg, p.min, p.max, p.n
		FROM %s p
		JOIN series s ON s.id = p.series_id
		WHERE s.kind = ? AND p.ts >= ? AND p.ts < ?`, table)
	args = append(args, from, to)
	if len(names) > 0 {
		query += ` AND s.name IN (` + placeholders(len(names)) + `)`
		for _, n := range names {
			args = append(args, n)
		}
	}
	query += ` ORDER BY s.name, p.ts`

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []api.SeriesHistory
	byName := map[string]int{}
	for rows.Next() {
		var kind, name string
		var p api.SeriesPoint
		if err := rows.Scan(&kind, &name, &p.TS, &p.Avg, &p.Min, &p.Max, &p.N); err != nil {
			return nil, err
		}
		i, ok := byName[name]
		if !ok {
			i = len(out)
			byName[name] = i
			out = append(out, api.SeriesHistory{Kind: kind, Name: name})
		}
		out[i].Points = append(out[i].Points, p)
	}
	return out, rows.Err()
}

// LastWear returns the newest wear row for a device, or nil when there is one.
func (d *DB) LastWear(device string) (*domain.WearObservation, error) {
	row := d.sql.QueryRow(wearSelect+` WHERE w.device = ? ORDER BY w.ts DESC LIMIT 1`, device)
	w, err := scanWear(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

// AppendWear stores a wear row. Writing the same device and timestamp twice is
// an update, so a retried poll cannot duplicate history.
func (d *DB) AppendWear(w domain.WearObservation) error {
	_, err := d.sql.Exec(`
		INSERT INTO wear_log
			(device, ts, model, serial, percentage_used, tbw_bytes,
			 power_on_hours, media_errors, reallocated, temp_c, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT (device, ts) DO UPDATE SET
			model = excluded.model, serial = excluded.serial,
			percentage_used = excluded.percentage_used, tbw_bytes = excluded.tbw_bytes,
			power_on_hours = excluded.power_on_hours, media_errors = excluded.media_errors,
			reallocated = excluded.reallocated, temp_c = excluded.temp_c,
			reason = excluded.reason`,
		w.Disk, w.TS, w.Model, w.Serial, w.PercentageUsed, w.TBWBytes,
		w.PowerOnHours, w.MediaErrors, w.Reallocated, w.TempC,
		domain.FormatReasons(w.Reasons))
	return err
}

// WearLatest returns the newest wear row per device.
func (d *DB) WearLatest() ([]domain.WearObservation, error) {
	rows, err := d.sql.Query(wearSelect + `
		JOIN (SELECT device, MAX(ts) AS ts FROM wear_log GROUP BY device) latest
		  ON latest.device = w.device AND latest.ts = w.ts
		ORDER BY w.device`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WearObservation
	for rows.Next() {
		w, err := scanWear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// WearLog returns the newest limit wear rows across all devices.
func (d *DB) WearLog(limit int) ([]domain.WearObservation, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.sql.Query(wearSelect+` ORDER BY w.ts DESC, w.device LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.WearObservation
	for rows.Next() {
		w, err := scanWear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

const wearSelect = `
	SELECT w.device, w.ts, w.model, w.serial, w.percentage_used, w.tbw_bytes,
	       w.power_on_hours, w.media_errors, w.reallocated, w.temp_c, w.reason
	FROM wear_log w`

type scanner interface {
	Scan(dest ...any) error
}

func scanWear(s scanner) (*domain.WearObservation, error) {
	var w domain.WearObservation
	var pct, tbw, hours, media, realloc, temp sql.NullInt64
	var reason string
	if err := s.Scan(&w.Disk, &w.TS, &w.Model, &w.Serial, &pct, &tbw,
		&hours, &media, &realloc, &temp, &reason); err != nil {
		return nil, err
	}
	w.PercentageUsed = nullInt(pct)
	w.TBWBytes = nullInt(tbw)
	w.PowerOnHours = nullInt(hours)
	w.MediaErrors = nullInt(media)
	w.Reallocated = nullInt(realloc)
	w.TempC = nullInt(temp)
	w.Reasons = domain.ParseReasons(reason)
	return &w, nil
}

func nullInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

// seriesTx resolves the series id inside the write transaction. No in-memory
// cache is kept: a rolled back transaction must not leave ids that do not exist.
func seriesTx(tx *sql.Tx, kind domain.Kind, name string) (int64, error) {
	if _, err := tx.Exec(`INSERT INTO series (kind, name) VALUES (?, ?)
		ON CONFLICT (kind, name) DO NOTHING`, string(kind), name); err != nil {
		return 0, err
	}
	var id int64
	if err := tx.QueryRow(`SELECT id FROM series WHERE kind = ? AND name = ?`,
		string(kind), name).Scan(&id); err != nil {
		return 0, err
	}
	return id, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, 0, n*2-1)
	for i := 0; i < n; i++ {
		if i > 0 {
			b = append(b, ',')
		}
		b = append(b, '?')
	}
	return string(b)
}
