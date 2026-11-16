-- +goose Up
-- How many apps a Refresh plans to fetch store details for, so the UI can show progress.
ALTER TABLE sync_runs ADD COLUMN details_total INT NOT NULL DEFAULT 0;

-- At most one running Refresh per user.
CREATE UNIQUE INDEX sync_runs_one_running ON sync_runs (steam_id) WHERE status = 'running';

-- +goose Down
DROP INDEX sync_runs_one_running;
ALTER TABLE sync_runs DROP COLUMN details_total;
