// Package store persists aggregated metric buckets and the SSD wear log in
// SQLite. Minute buckets are rolled up into hours and days; old buckets are
// pruned. Every write is an idempotent upsert so a crash or retry cannot create
// duplicate history.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"mon/internal/model"
)

const (
	minute = int64(60)
	hour   = int64(3600)
	day    = int64(86400)
)

// Step selects the aggregation table used for a history query.
type Step string

const (
	StepMinute Step = "minute"
	StepHour   Step = "hour"
	StepDay    Step = "day"
)

// DB is the SQLite-backed history store.
type DB struct {
	sql             *sql.DB
	retentionMinute time.Duration
	retentionHour   time.Duration

	mu       sync.Mutex
	seriesID map[string]int64
}

// Wear is one SSD wear snapshot.
type Wear struct {
	Disk           string `json:"disk"`
	TS             int64  `json:"ts"`
	Model          string `json:"model,omitempty"`
	Serial         string `json:"serial,omitempty"`
	PercentageUsed *int64 `json:"percentage_used,omitempty"`
	TBWBytes       *int64 `json:"tbw_bytes,omitempty"`
	PowerOnHours   *int64 `json:"power_on_hours,omitempty"`
	MediaErrors    *int64 `json:"media_errors,omitempty"`
	Reallocated    *int64 `json:"reallocated,omitempty"`
	TempC          *int64 `json:"temp_c,omitempty"`
	Reason         string `json:"reason,omitempty"`
}

const schema = `
CREATE TABLE IF NOT EXISTS series (
	id   INTEGER PRIMARY KEY,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	UNIQUE (kind, name)
);

CREATE TABLE IF NOT EXISTS points_1m (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS points_1m_ts ON points_1m (ts);

CREATE TABLE IF NOT EXISTS points_1h (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS points_1h_ts ON points_1h (ts);

CREATE TABLE IF NOT EXISTS points_1d (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;

CREATE TABLE IF NOT EXISTS wear_log (
	id              INTEGER PRIMARY KEY,
	disk            TEXT NOT NULL,
	ts              INTEGER NOT NULL,
	model           TEXT NOT NULL DEFAULT '',
	serial          TEXT NOT NULL DEFAULT '',
	percentage_used INTEGER,
	tbw_bytes       INTEGER,
	power_on_hours  INTEGER,
	media_errors    INTEGER,
	reallocated     INTEGER,
	temp_c          INTEGER,
	reason          TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS wear_log_disk_ts ON wear_log (disk, ts);
`

// Open opens or creates the database. retentionMinute/retentionHour of zero
// keep that resolution forever.
func Open(path string, retentionMinute, retentionHour time.Duration) (*DB, error) {
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
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("migrate sqlite %s: %w", path, err)
	}
	return &DB{
		sql:             sqlDB,
		retentionMinute: retentionMinute,
		retentionHour:   retentionHour,
		seriesID:        make(map[string]int64),
	}, nil
}

// Close closes the underlying database.
func (d *DB) Close() error { return d.sql.Close() }

// InsertMinute stores completed minute buckets, then refreshes the hour and day
// aggregates for the hours and days those buckets touch.
func (d *DB) InsertMinute(points []model.Point) error {
	clean := points[:0]
	for _, p := range points {
		if p.N <= 0 || p.Kind == "" || p.Name == "" {
			continue
		}
		if math.IsNaN(p.Avg) || math.IsInf(p.Avg, 0) ||
			math.IsNaN(p.Min) || math.IsNaN(p.Max) {
			continue
		}
		p.TS = p.TS - mod(p.TS, minute)
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
		id, err := d.seriesTx(tx, p.Kind, p.Name)
		if err != nil {
			return err
		}
		_, err = tx.Exec(`
			INSERT INTO points_1m (series_id, ts, avg, min, max, n)
			VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT (series_id, ts) DO UPDATE SET
				avg = (points_1m.avg * points_1m.n + excluded.avg * excluded.n) /
				      (points_1m.n + excluded.n),
				min = MIN(points_1m.min, excluded.min),
				max = MAX(points_1m.max, excluded.max),
				n   = points_1m.n + excluded.n`,
			id, p.TS, p.Avg, p.Min, p.Max, p.N)
		if err != nil {
			return fmt.Errorf("insert minute %s:%s: %w", p.Kind, p.Name, err)
		}
	}

	minHour, maxHour := int64(math.MaxInt64), int64(math.MinInt64)
	for _, p := range clean {
		h := p.TS - mod(p.TS, hour)
		minHour = min(minHour, h)
		maxHour = max(maxHour, h)
	}
	for h := minHour; h <= maxHour; h += hour {
		if err := rollup(tx, "points_1h", "points_1m", h, h+hour); err != nil {
			return err
		}
	}
	for dd := minHour - mod(minHour, day); dd <= maxHour; dd += day {
		if err := rollup(tx, "points_1d", "points_1h", dd, dd+day); err != nil {
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
		if _, err := d.sql.Exec(`DELETE FROM points_1m WHERE ts < ?`,
			now.Add(-d.retentionMinute).Unix()); err != nil {
			return fmt.Errorf("prune minutes: %w", err)
		}
	}
	if d.retentionHour > 0 {
		if _, err := d.sql.Exec(`DELETE FROM points_1h WHERE ts < ?`,
			now.Add(-d.retentionHour).Unix()); err != nil {
			return fmt.Errorf("prune hours: %w", err)
		}
	}
	return nil
}

// History queries one aggregation table for the given series names. An empty
// names slice returns every series of that kind.
func (d *DB) History(kind string, names []string, from, to int64, step Step) ([]model.SeriesHistory, error) {
	table, err := tableFor(step)
	if err != nil {
		return nil, err
	}
	args := []any{kind}
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

	var out []model.SeriesHistory
	byName := map[string]int{}
	for rows.Next() {
		var kind, name string
		var p model.SeriesPoint
		if err := rows.Scan(&kind, &name, &p.TS, &p.Avg, &p.Min, &p.Max, &p.N); err != nil {
			return nil, err
		}
		i, ok := byName[name]
		if !ok {
			i = len(out)
			byName[name] = i
			out = append(out, model.SeriesHistory{Kind: kind, Name: name})
		}
		out[i].Points = append(out[i].Points, p)
	}
	return out, rows.Err()
}

func tableFor(step Step) (string, error) {
	switch step {
	case StepMinute:
		return "points_1m", nil
	case StepHour:
		return "points_1h", nil
	case StepDay:
		return "points_1d", nil
	default:
		return "", fmt.Errorf("unknown history step %q", step)
	}
}

// LastWear returns the newest wear row for a disk, or nil when there is none.
func (d *DB) LastWear(disk string) (*Wear, error) {
	row := d.sql.QueryRow(wearSelect+` WHERE disk = ? ORDER BY id DESC LIMIT 1`, disk)
	w, err := scanWear(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return w, err
}

// AppendWear stores a wear row and returns its id.
func (d *DB) AppendWear(w Wear) error {
	_, err := d.sql.Exec(`
		INSERT INTO wear_log
			(disk, ts, model, serial, percentage_used, tbw_bytes,
			 power_on_hours, media_errors, reallocated, temp_c, reason)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		w.Disk, w.TS, w.Model, w.Serial, w.PercentageUsed, w.TBWBytes,
		w.PowerOnHours, w.MediaErrors, w.Reallocated, w.TempC, w.Reason)
	return err
}

// WearLatest returns the newest wear row per disk.
func (d *DB) WearLatest() ([]Wear, error) {
	rows, err := d.sql.Query(wearSelect + `
		WHERE id IN (SELECT MAX(id) FROM wear_log GROUP BY disk)
		ORDER BY disk`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Wear
	for rows.Next() {
		w, err := scanWear(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// WearLog returns the newest limit wear rows across all disks.
func (d *DB) WearLog(limit int) ([]Wear, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := d.sql.Query(wearSelect+` ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Wear
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
	SELECT disk, ts, model, serial, percentage_used, tbw_bytes,
	       power_on_hours, media_errors, reallocated, temp_c, reason
	FROM wear_log`

type scanner interface {
	Scan(dest ...any) error
}

func scanWear(s scanner) (*Wear, error) {
	var w Wear
	var pct, tbw, hours, media, realloc, temp sql.NullInt64
	if err := s.Scan(&w.Disk, &w.TS, &w.Model, &w.Serial, &pct, &tbw,
		&hours, &media, &realloc, &temp, &w.Reason); err != nil {
		return nil, err
	}
	w.PercentageUsed = nullInt(pct)
	w.TBWBytes = nullInt(tbw)
	w.PowerOnHours = nullInt(hours)
	w.MediaErrors = nullInt(media)
	w.Reallocated = nullInt(realloc)
	w.TempC = nullInt(temp)
	return &w, nil
}

func nullInt(v sql.NullInt64) *int64 {
	if !v.Valid {
		return nil
	}
	n := v.Int64
	return &n
}

func (d *DB) seriesTx(tx *sql.Tx, kind, name string) (int64, error) {
	key := kind + "\x00" + name
	d.mu.Lock()
	id, ok := d.seriesID[key]
	d.mu.Unlock()
	if ok {
		return id, nil
	}
	if _, err := tx.Exec(`INSERT INTO series (kind, name) VALUES (?, ?)
		ON CONFLICT (kind, name) DO NOTHING`, kind, name); err != nil {
		return 0, err
	}
	if err := tx.QueryRow(`SELECT id FROM series WHERE kind = ? AND name = ?`,
		kind, name).Scan(&id); err != nil {
		return 0, err
	}
	d.mu.Lock()
	d.seriesID[key] = id
	d.mu.Unlock()
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

func mod(a, b int64) int64 { return ((a % b) + b) % b }
