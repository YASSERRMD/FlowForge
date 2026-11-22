-- +goose Up
-- Step attempts: every execution attempt of one graph node inside a run.
-- attempt_number > 1 means a retry; idempotency_token lets effectful steps
-- dedupe retried side effects.

CREATE TABLE step_attempts (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id            UUID NOT NULL REFERENCES workflow_runs (id) ON DELETE CASCADE,
    node_id           TEXT NOT NULL,
    node_type         TEXT NOT NULL,
    attempt_number    INT NOT NULL CHECK (attempt_number >= 1),
    status            TEXT NOT NULL DEFAULT 'pending'
                      CHECK (status IN
                          ('pending', 'running', 'succeeded', 'failed', 'skipped', 'canceled')),
    input_json        JSONB,
    output_json       JSONB,
    error_code        TEXT,
    error_message     TEXT,
    idempotency_token UUID NOT NULL DEFAULT gen_random_uuid(),
    started_at        TIMESTAMPTZ,
    finished_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (run_id, node_id, attempt_number)
);

-- Exactly one idempotency token per logical step execution: retries of the
-- same logical execution share the token.
CREATE UNIQUE INDEX step_attempts_token_key ON step_attempts (idempotency_token);

CREATE INDEX step_attempts_run_idx ON step_attempts (run_id, node_id);
CREATE INDEX step_attempts_active_idx
    ON step_attempts (run_id) WHERE status IN ('pending', 'running');

-- +goose Down
DROP TABLE step_attempts;
