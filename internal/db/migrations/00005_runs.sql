-- +goose Up
-- Workflow runs: one durable execution of a published version.

CREATE TABLE workflow_runs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id         UUID NOT NULL REFERENCES workflows (id) ON DELETE RESTRICT,
    workflow_version_id UUID NOT NULL REFERENCES workflow_versions (id) ON DELETE RESTRICT,
    -- queued -> running -> succeeded | failed | canceled
    status              TEXT NOT NULL DEFAULT 'queued'
                        CHECK (status IN
                            ('queued', 'running', 'succeeded', 'failed', 'canceled')),
    variables           JSONB NOT NULL DEFAULT '{}',
    -- caller-supplied key making run creation idempotent
    idempotency_key     TEXT,
    triggered_by        UUID REFERENCES users (id) ON DELETE SET NULL,
    trigger_type        TEXT NOT NULL DEFAULT 'manual'
                        CHECK (trigger_type IN ('manual', 'schedule', 'api', 'retry')),
    cancel_requested_at TIMESTAMPTZ,
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Idempotent run creation per workflow.
CREATE UNIQUE INDEX workflow_runs_idempotency_key
    ON workflow_runs (workflow_id, idempotency_key)
    WHERE idempotency_key IS NOT NULL;

CREATE INDEX workflow_runs_workflow_idx
    ON workflow_runs (workflow_id, created_at DESC);
CREATE INDEX workflow_runs_status_idx
    ON workflow_runs (status) WHERE finished_at IS NULL;

-- +goose Down
DROP TABLE workflow_runs;
