-- +goose Up
-- Run events: the ordered, append-only history of a run. Consumers rebuild
-- any view of a run from this stream; (run_id, sequence) is the total order.

CREATE TABLE run_events (
    id              BIGINT GENERATED ALWAYS AS IDENTITY,
    run_id          UUID NOT NULL REFERENCES workflow_runs (id) ON DELETE CASCADE,
    sequence        BIGINT NOT NULL,
    event_type      TEXT NOT NULL,
    payload         JSONB NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (run_id, sequence)
);

CREATE INDEX run_events_id_idx ON run_events (id);

-- Transactional outbox: notifications are written in the same transaction
-- as the state change they describe, then relayed to Redis/WebSockets.
-- Nothing is ever published without being persisted first.
CREATE TABLE outbox_messages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    topic           TEXT NOT NULL,
    payload         JSONB NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    publish_attempts INT NOT NULL DEFAULT 0
);

CREATE INDEX outbox_unpublished_idx
    ON outbox_messages (id) WHERE published_at IS NULL;

-- +goose Down
DROP TABLE outbox_messages;
DROP TABLE run_events;
