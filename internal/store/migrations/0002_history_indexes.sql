-- +goose Up
-- Retention deletes by ts, and /api/wear orders and groups by ts. Without
-- these indexes each of those statements scans the whole table.
CREATE INDEX points_day_ts ON points_day (ts);
CREATE INDEX wear_log_ts ON wear_log (ts);

-- +goose Down
DROP INDEX wear_log_ts;
DROP INDEX points_day_ts;
