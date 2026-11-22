-- +goose Up
-- +goose StatementBegin
ALTER TABLE functions ADD COLUMN is_ui BOOLEAN NOT NULL DEFAULT false;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE functions DROP COLUMN IF EXISTS is_ui;
-- +goose StatementEnd
