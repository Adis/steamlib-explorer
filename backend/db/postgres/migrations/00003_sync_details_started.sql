-- +goose Up
-- When a Refresh started fetching store details, so the UI can estimate the time left
-- from the observed pace. Existing runs keep NULL.
ALTER TABLE sync_runs ADD COLUMN details_started_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE sync_runs DROP COLUMN details_started_at;
