-- +goose Up
-- Baseline: shared extensions used across the FlowForge schema.
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS citext;   -- case-insensitive email matching

-- +goose Down
DROP EXTENSION IF EXISTS citext;
DROP EXTENSION IF EXISTS pgcrypto;
