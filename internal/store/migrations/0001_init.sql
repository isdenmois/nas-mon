-- +goose Up
CREATE TABLE series (
	id   INTEGER PRIMARY KEY,
	kind TEXT NOT NULL,
	name TEXT NOT NULL,
	UNIQUE (kind, name)
);

CREATE TABLE points_minute (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX points_minute_ts ON points_minute (ts);

CREATE TABLE points_hour (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;
CREATE INDEX points_hour_ts ON points_hour (ts);

CREATE TABLE points_day (
	series_id INTEGER NOT NULL,
	ts        INTEGER NOT NULL,
	avg       REAL NOT NULL,
	min       REAL NOT NULL,
	max       REAL NOT NULL,
	n         INTEGER NOT NULL,
	PRIMARY KEY (series_id, ts)
) WITHOUT ROWID;

-- One row per significant wear observation. The identity columns are stored
-- with the row because the wear policy compares each reading against the
-- identity recorded at the previous row, not against the current one.
CREATE TABLE wear_log (
	device          TEXT NOT NULL,
	ts              INTEGER NOT NULL,
	model           TEXT NOT NULL DEFAULT '',
	serial          TEXT NOT NULL DEFAULT '',
	percentage_used INTEGER,
	tbw_bytes       INTEGER,
	power_on_hours  INTEGER,
	media_errors    INTEGER,
	reallocated     INTEGER,
	temp_c          INTEGER,
	reason          TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (device, ts)
) WITHOUT ROWID;

-- +goose Down
DROP TABLE wear_log;
DROP TABLE points_day;
DROP TABLE points_hour;
DROP TABLE points_minute;
DROP TABLE series;
