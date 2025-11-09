-- +goose Up
CREATE TABLE chirps
(
    id         UUID PRIMARY KEY      DEFAULT gen_random_uuid(),
    created_at timestamptz  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamptz  NOT NULL DEFAULT CURRENT_TIMESTAMP,
    body       VARCHAR(140) NOT NULL,
    user_id    UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE
);

-- +goose Down
DROP TABLE chirps;