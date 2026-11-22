-- +goose Up
-- Scheduled tasks: the durable queue workers claim from. A row is runnable
-- when run_at <= now() and no live lease holds it. Workers claim with
-- SELECT ... FOR UPDATE SKIP LOCKED and renew lease_expires_at while
-- executing; a crashed worker's lease simply expires.

CREATE TABLE scheduled_tasks (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_type        TEXT NOT NULL,
    -- dedupes identical enqueues (e.g. run step dispatch)
    dedupe_key       TEXT NOT NULL,
    payload          JSONB NOT NULL DEFAULT '{}',
    run_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    leased_by        TEXT,
    lease_expires_at TIMESTAMPTZ,
    attempts         INT NOT NULL DEFAULT 0,
    max_attempts     INT NOT NULL DEFAULT 8,
    last_error       TEXT,
    status           TEXT NOT NULL DEFAULT 'pending'
                     CHECK (status IN ('pending', 'leased', 'done', 'dead')),
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (task_type, dedupe_key)
);

-- Claim query: runnable rows in schedule order, skipping locked rows.
CREATE INDEX scheduled_tasks_runnable_idx
    ON scheduled_tasks (run_at)
    WHERE status IN ('pending', 'leased');

-- Reaping expired leases.
CREATE INDEX scheduled_tasks_lease_expiry_idx
    ON scheduled_tasks (lease_expires_at)
    WHERE status = 'leased';

-- +goose Down
DROP TABLE scheduled_tasks;
