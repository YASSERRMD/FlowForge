-- +goose Up
-- Workflows: the user-facing design document. A workflow owns an ordered
-- list of immutable versions (00004); the row itself tracks identity,
-- ownership and lifecycle only.

CREATE TABLE workflows (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_id        UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    -- lifecycle: draft (never published), active, archived
    status          TEXT NOT NULL DEFAULT 'draft'
                    CHECK (status IN ('draft', 'active', 'archived')),
    -- current published version; FK added in 00004 once
    -- workflow_versions exists
    current_version_id UUID,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    archived_at     TIMESTAMPTZ,
    UNIQUE (owner_id, name)
);

CREATE INDEX workflows_owner_idx ON workflows (owner_id) WHERE archived_at IS NULL;
CREATE INDEX workflows_status_idx ON workflows (status);

-- +goose Down
DROP TABLE workflows;
