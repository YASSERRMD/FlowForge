-- name: CreateWorkflow :one
INSERT INTO workflows (owner_id, name, description)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetWorkflow :one
SELECT * FROM workflows WHERE id = $1;

-- name: ListWorkflowsByOwner :many
SELECT * FROM workflows
WHERE owner_id = $1 AND archived_at IS NULL
ORDER BY updated_at DESC
LIMIT $2 OFFSET $3;

-- name: UpdateWorkflowMeta :one
UPDATE workflows
SET name = $2, description = $3, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: ArchiveWorkflow :one
UPDATE workflows
SET status = 'archived', archived_at = now(), updated_at = now()
WHERE id = $1 AND archived_at IS NULL
RETURNING *;

-- name: SetWorkflowCurrentVersion :exec
UPDATE workflows
SET current_version_id = $2, updated_at = now()
WHERE id = $1;

-- name: CreateWorkflowVersion :one
INSERT INTO workflow_versions
    (workflow_id, version_number, status, graph_json, content_hash, created_by)
VALUES ($1, $2, 'draft', $3, $4, $5)
RETURNING *;

-- name: GetWorkflowVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND version_number = $2;

-- name: GetDraftVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND status = 'draft';

-- name: NextVersionNumber :one
SELECT COALESCE(MAX(version_number), 0) + 1
FROM workflow_versions
WHERE workflow_id = $1;

-- name: PublishVersion :one
-- Marks the draft published; supersedes the previously published version in
-- the same transaction (see repository).
UPDATE workflow_versions
SET status = 'published', published_at = now()
WHERE id = $1 AND status = 'draft'
RETURNING *;

-- name: SupersedePublishedVersions :exec
UPDATE workflow_versions
SET status = 'superseded'
WHERE workflow_id = $1 AND status = 'published' AND id <> $2;

-- name: GetWorkflowForOwner :one
SELECT * FROM workflows
WHERE id = $1 AND owner_id = $2 AND archived_at IS NULL;

-- name: UpdateDraftGraph :one
-- Optimistic save: only applies when the caller's row_version matches.
UPDATE workflow_versions
SET graph_json = $3, content_hash = $4
WHERE id = $1 AND row_version = $2 AND status = 'draft'
RETURNING *;

-- name: ListWorkflowVersions :many
SELECT * FROM workflow_versions
WHERE workflow_id = $1
ORDER BY version_number DESC;

-- name: ActivateWorkflow :exec
UPDATE workflows SET status = 'active', updated_at = now() WHERE id = $1;

-- name: GetPublishedVersion :one
SELECT * FROM workflow_versions
WHERE workflow_id = $1 AND status = 'published';
