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
	"sort"
	"sync"
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
	sql *sql.DB
	ret Retention
	now func() time.Time

	// Prepared once: rebuilding the text and re-parsing it per row showed up as
	// a third of the SQLite profile (_exprDup, _sqlite3ExprListFlags).
	stmtMinute *sql.Stmt
	stmtHour   *sql.Stmt
	stmtDay    *sql.Stmt

	// mu guards the series id cache and lastPrune. The aggregate loop is the
	// only writer today; the lock keeps the cache correct if that changes.
	mu        sync.Mutex
	seriesIDs map[seriesKey]int64
	lastPrune time.Time
}

// Retention bounds how long each level of history is kept. A zero duration
// keeps that level forever.
type Retention struct {
	Minute time.Duration
	Hour   time.Duration
	Day    time.Duration
	Wear   time.Duration
}

// pruneEvery bounds how often retention runs. Pruning on every flush scans the
// day table and the wear log for rows that cannot exist until hours later.
const pruneEvery = time.Hour

// seriesKey identifies a series the way the series table does.
type seriesKey struct {
	kind domain.Kind
	name string
}

// Wear is one SSD wear snapshot.

// Open opens the database and applies the embedded migrations. A zero
// retention keeps that resolution forever.
func Open(path string, ret Retention) (*DB, error) {
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
	d := &DB{
		sql:       sqlDB,
		ret:       ret,
		now:       time.Now,
		seriesIDs: make(map[seriesKey]int64),
	}
	if err := d.prepare(); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("prepare sqlite %s: %w", path, err)
	}
	return d, nil
}

// prepare builds the statements the flush path runs most often. They are bound
// to the single connection, so preparing them once removes the parse cost from
// every flush.
func (d *DB) prepare() error {
	minute := fmt.Sprintf(`
		INSERT INTO %s (series_id, ts, avg, min, max, n)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT (series_id, ts) DO UPDATE SET
			avg = (%[1]s.avg * %[1]s.n + excluded.avg * excluded.n) /
			      (%[1]s.n + excluded.n),
			min = MIN(%[1]s.min, excluded.min),
			max = MAX(%[1]s.max, excluded.max),
			n   = %[1]s.n + excluded.n`, domain.Minute.Table())

	var err error
	if d.stmtMinute, err = d.sql.Prepare(minute); err != nil {
		return fmt.Errorf("prepare minute insert: %w", err)
	}
	if d.stmtHour, err = d.sql.Prepare(rollupQuery(domain.Hour.Table(), domain.Minute.Table())); err != nil {
		return fmt.Errorf("prepare hour rollup: %w", err)
	}
	if d.stmtDay, err = d.sql.Prepare(rollupQuery(domain.Day.Table(), domain.Hour.Table())); err != nil {
		return fmt.Errorf("prepare day rollup: %w", err)
	}
	return nil
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

	keys := make([]seriesKey, 0, len(clean))
	for _, p := range clean {
		keys = append(keys, seriesKey{kind: p.Kind, name: p.Name})
	}
	ids, fresh, err := d.resolveSeries(tx, keys)
	if err != nil {
		return err
	}

	insert := tx.Stmt(d.stmtMinute)
	defer insert.Close()
	for _, p := range clean {
		_, err = insert.Exec(ids[seriesKey{kind: p.Kind, name: p.Name}],
			p.TS, p.Avg, p.Min, p.Max, p.N)
		if err != nil {
			return fmt.Errorf("insert minute %s:%s: %w", p.Kind, p.Name, err)
		}
	}

	// Only the (bucket, series) pairs that received a point changed, so only
	// they are recomputed -- and per series, so SQLite walks the primary key
	// range instead of sorting every series through a temp B-tree.
	hours := map[int64]map[int64]struct{}{}
	days := map[int64]map[int64]struct{}{}
	for _, p := range clean {
		id := ids[seriesKey{kind: p.Kind, name: p.Name}]
		add(hours, domain.Hour.Bucket(p.TS), id)
		add(days, domain.Day.Bucket(p.TS), id)
	}
	if err := rollupBuckets(tx, d.stmtHour, hours, domain.Hour.Seconds()); err != nil {
		return err
	}
	if err := rollupBuckets(tx, d.stmtDay, days, domain.Day.Seconds()); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Cached only after commit: a rolled back transaction must not leave ids
	// that do not exist.
	d.remember(fresh)
	return d.pruneIfDue(d.now())
}

func add(buckets map[int64]map[int64]struct{}, ts, id int64) {
	set := buckets[ts]
	if set == nil {
		set = make(map[int64]struct{}, 8)
		buckets[ts] = set
	}
	set[id] = struct{}{}
}

// rollupBuckets refreshes each bucket for the series that changed in it.
func rollupBuckets(tx *sql.Tx, stmt *sql.Stmt, buckets map[int64]map[int64]struct{}, width int64) error {
	if len(buckets) == 0 {
		return nil
	}
	b := tx.Stmt(stmt)
	defer b.Close()

	keys := make([]int64, 0, len(buckets))
	for ts := range buckets {
		keys = append(keys, ts)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })

	ids := make([]int64, 0, 64)
	for _, ts := range keys {
		ids = ids[:0]
		for id := range buckets[ts] {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			if _, err := b.Exec(ts, id, ts, ts+width); err != nil {
				return fmt.Errorf("rollup [%d,%d) series %d: %w", ts, ts+width, id, err)
			}
		}
	}
	return nil
}

// resolveSeries maps each key to its series id, inserting series it has not
// seen before. Newly created ids are returned separately so the caller can
// remember them only once the enclosing transaction commits.
func (d *DB) resolveSeries(tx *sql.Tx, keys []seriesKey) (map[seriesKey]int64, map[seriesKey]int64, error) {
	ids := make(map[seriesKey]int64, len(keys))
	fresh := make(map[seriesKey]int64)
	for _, k := range keys {
		if _, ok := ids[k]; ok {
			continue
		}
		if id, ok := d.cachedID(k); ok {
			ids[k] = id
			continue
		}
		if _, err := tx.Exec(`INSERT INTO series (kind, name) VALUES (?, ?)
			ON CONFLICT (kind, name) DO NOTHING`, string(k.kind), k.name); err != nil {
			return nil, nil, fmt.Errorf("insert series %s:%s: %w", k.kind, k.name, err)
		}
		var id int64
		if err := tx.QueryRow(`SELECT id FROM series WHERE kind = ? AND name = ?`,
			string(k.kind), k.name).Scan(&id); err != nil {
			return nil, nil, fmt.Errorf("select series %s:%s: %w", k.kind, k.name, err)
		}
		ids[k] = id
		fresh[k] = id
	}
	return ids, fresh, nil
}

func (d *DB) cachedID(k seriesKey) (int64, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	id, ok := d.seriesIDs[k]
	return id, ok
}

func (d *DB) remember(ids map[seriesKey]int64) {
	if len(ids) == 0 {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for k, id := range ids {
		d.seriesIDs[k] = id
	}
}

// rollup recomputes one parent bucket [from, to) from its child table.
// rollupQuery aggregates one parent bucket for a single series out of its
// children. GROUP BY keeps the statement from inserting a NULL row when the
// series has no children in the window.
func rollupQuery(parent, child string) string {
	return fmt.Sprintf(`
		INSERT INTO %s (series_id, ts, avg, min, max, n)
		SELECT series_id, ?, SUM(avg * n) / SUM(n), MIN(min), MAX(max), SUM(n)
		FROM %s
		WHERE series_id = ? AND ts >= ? AND ts < ?
		GROUP BY series_id
		ON CONFLICT (series_id, ts) DO UPDATE SET
			avg = excluded.avg, min = excluded.min,
			max = excluded.max, n = excluded.n`, parent, child)
}

// Prune deletes buckets older than their retention. It is safe to call at any
// time; InsertMinute gates the automatic call so it runs at most hourly.
func (d *DB) Prune(now time.Time) error {
	if d.ret.Minute > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Minute.Table()+` WHERE ts < ?`,
			now.Add(-d.ret.Minute).Unix()); err != nil {
			return fmt.Errorf("prune minutes: %w", err)
		}
	}
	if d.ret.Hour > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Hour.Table()+` WHERE ts < ?`,
			now.Add(-d.ret.Hour).Unix()); err != nil {
			return fmt.Errorf("prune hours: %w", err)
		}
	}
	if d.ret.Day > 0 {
		if _, err := d.sql.Exec(`DELETE FROM `+domain.Day.Table()+` WHERE ts < ?`,
			now.Add(-d.ret.Day).Unix()); err != nil {
			return fmt.Errorf("prune days: %w", err)
		}
	}
	if d.ret.Wear > 0 {
		if _, err := d.sql.Exec(`DELETE FROM wear_log WHERE ts < ?`,
			now.Add(-d.ret.Wear).Unix()); err != nil {
			return fmt.Errorf("prune wear log: %w", err)
		}
	}
	return nil
}

// pruneIfDue runs retention when the last run is at least pruneEvery old. The
// timestamp is recorded before pruning so a failing prune cannot be retried on
// every flush.
func (d *DB) pruneIfDue(now time.Time) error {
	d.mu.Lock()
	if !d.lastPrune.IsZero() && now.Sub(d.lastPrune) < pruneEvery {
		d.mu.Unlock()
		return nil
	}
	d.lastPrune = now
	d.mu.Unlock()
	return d.Prune(now)
}

// ListSeries returns every recorded series.
func (d *DB) ListSeries() ([]domain.Series, error) {
	rows, err := d.sql.Query(`SELECT id, kind, name FROM series ORDER BY kind, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Series
	for rows.Next() {
		var s domain.Series
		var kind string
		if err := rows.Scan(&s.ID, &kind, &s.Name); err != nil {
			return nil, err
		}
		s.Kind = domain.Kind(kind)
		out = append(out, s)
	}
	return out, rows.Err()
}

// Series returns the rows for the given ids. Missing ids are simply absent from
// the result; the caller decides whether that is an error.
func (d *DB) Series(ids []int64) ([]domain.Series, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := d.sql.Query(`SELECT id, kind, name FROM series WHERE id IN (`+
		placeholders(len(ids))+`) ORDER BY id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Series
	for rows.Next() {
		var s domain.Series
		var kind string
		if err := rows.Scan(&s.ID, &kind, &s.Name); err != nil {
			return nil, err
		}
		s.Kind = domain.Kind(kind)
		out = append(out, s)
	}
	return out, rows.Err()
}

// History queries one aggregation table for the given series ids.
func (d *DB) History(ids []int64, from, to int64, res domain.Resolution) ([]api.SeriesHistory, error) {
	table := res.Table()
	if table == "" {
		return nil, fmt.Errorf("unknown history resolution %q", res)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	// Metadata once per series, not once per point: joining series into the
	// row query re-read and re-allocated kind and name for every bucket.
	meta, err := d.Series(ids)
	if err != nil {
		return nil, err
	}
	known := make(map[int64]domain.Series, len(meta))
	for _, s := range meta {
		known[s.ID] = s
	}

	args := []any{from, to}
	for _, id := range ids {
		args = append(args, id)
	}
	query := fmt.Sprintf(`
		SELECT series_id, ts, avg
		FROM %s
		WHERE ts >= ? AND ts < ? AND series_id IN (%s)
		ORDER BY series_id, ts`, table, placeholders(len(ids)))

	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]api.SeriesHistory, 0, len(known))
	byID := make(map[int64]int, len(known))
	for rows.Next() {
		var id int64
		var p api.SeriesPoint
		if err := rows.Scan(&id, &p.TS, &p.Avg); err != nil {
			return nil, err
		}
		i, ok := byID[id]
		if !ok {
			s, exists := known[id]
			if !exists {
				// The series row disappeared between the two queries.
				continue
			}
			i = len(out)
			byID[id] = i
			out = append(out, api.SeriesHistory{
				ID: id, Kind: string(s.Kind), Name: s.Name,
				Points: make([]api.SeriesPoint, 0, 64),
			})
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
func (d *DB) WearLog(limit int, since int64) ([]domain.WearObservation, error) {
	if limit <= 0 {
		limit = 200
	}
	query := wearSelect
	args := []any{limit}
	if since > 0 {
		query += ` WHERE w.ts >= ? ORDER BY w.ts DESC, w.device LIMIT ?`
		args = []any{since, limit}
	} else {
		query += ` ORDER BY w.ts DESC, w.device LIMIT ?`
	}
	rows, err := d.sql.Query(query, args...)
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
