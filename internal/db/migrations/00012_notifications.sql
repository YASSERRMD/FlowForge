-- +goose Up
-- In-app notification records emitted by mock notification actions. These are
-- auditable, owner-scoped records; no real email or SMS is ever sent.

CREATE TABLE notifications (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    run_id     UUID NOT NULL REFERENCES workflow_runs (id) ON DELETE CASCADE,
    node_id    TEXT NOT NULL,
    channel    TEXT NOT NULL CHECK (channel IN ('email', 'statuspage', 'inapp')),
    recipient  TEXT,
    subject    TEXT,
    payload    JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notifications_run_idx ON notifications (run_id, created_at);

-- +goose Down
DROP TABLE notifications;
