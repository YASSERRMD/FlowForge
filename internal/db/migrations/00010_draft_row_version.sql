-- +goose Up
-- Draft versions are the only mutable workflow_versions rows, so they carry
-- the optimistic-concurrency token used by the draft save API: every update
-- must name the row_version it read, and the bump trigger advances it.

ALTER TABLE workflow_versions ADD COLUMN row_version INT NOT NULL DEFAULT 1;

CREATE TRIGGER workflow_versions_row_version
    BEFORE UPDATE ON workflow_versions
    FOR EACH ROW EXECUTE FUNCTION bump_row_version();

-- +goose Down
DROP TRIGGER workflow_versions_row_version ON workflow_versions;
ALTER TABLE workflow_versions DROP COLUMN row_version;
