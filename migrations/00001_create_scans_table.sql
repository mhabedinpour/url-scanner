-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS scans (
    id UUID PRIMARY KEY NOT NULL DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,

    url TEXT NOT NULL,
    status VARCHAR(50) NOT NULL,
    result JSONB NOT NULL DEFAULT '{}'::JSONB,

    attempts SMALLINT NOT NULL DEFAULT 0,
    last_error TEXT,

    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP,
    deleted_at TIMESTAMP
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS scans;
-- +goose StatementEnd
