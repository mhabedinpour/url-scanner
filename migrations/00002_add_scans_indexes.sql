-- +goose Up
-- +goose StatementBegin
CREATE INDEX IF NOT EXISTS scans_user_id_status_deleted_at_idx ON scans (user_id, status, deleted_at);
CREATE INDEX IF NOT EXISTS scans_url_status_deleted_at_idx ON scans (url, status, deleted_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS scans_user_id_status_deleted_at_idx;
DROP INDEX IF EXISTS scans_url_status_deleted_at_idx;
-- +goose StatementEnd
