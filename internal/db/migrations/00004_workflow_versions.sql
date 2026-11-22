-- +goose Up
-- Workflow versions are immutable: once inserted, graph_json, definition
-- hash and metadata never change. New edits produce a new version row.

CREATE TABLE workflow_versions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workflow_id     UUID NOT NULL REFERENCES workflows (id) ON DELETE CASCADE,
    version_number  INT NOT NULL,
    -- draft: editable snapshot; published: frozen and runnable;
    -- superseded: frozen, replaced by a newer published version
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'published', 'superseded')),
    graph_json      JSONB NOT NULL,
    -- SHA-256 of the canonical graph JSON; detects drift and dedupes
    -- identical publishes
    content_hash    BYTEA NOT NULL,
    created_by      UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at    TIMESTAMPTZ,
    UNIQUE (workflow_id, version_number)
);

-- At most one draft per workflow.
CREATE UNIQUE INDEX workflow_versions_one_draft
    ON workflow_versions (workflow_id) WHERE status = 'draft';

CREATE INDEX workflow_versions_workflow_idx
    ON workflow_versions (workflow_id, version_number DESC);

-- Complete the workflows.current_version_id cycle.
ALTER TABLE workflows
    ADD CONSTRAINT workflows_current_version_fk
    FOREIGN KEY (current_version_id) REFERENCES workflow_versions (id)
    ON DELETE SET NULL;

-- Immutability: published/superseded versions reject updates and deletes.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_version_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'workflow_versions rows with status % are immutable', OLD.status;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workflow_versions_immutable
    BEFORE UPDATE OR DELETE ON workflow_versions
    FOR EACH ROW
    WHEN (OLD.status IN ('published', 'superseded'))
    EXECUTE FUNCTION reject_version_mutation();

-- +goose Down
ALTER TABLE workflows DROP CONSTRAINT workflows_current_version_fk;
DROP TABLE workflow_versions;
DROP FUNCTION reject_version_mutation;
