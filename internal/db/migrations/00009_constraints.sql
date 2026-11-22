-- +goose Up
-- Cross-entity integrity that could not be expressed inline: optimistic
-- concurrency on mutable aggregates, value checks, and the indexes the
-- hot query paths rely on.

-- Optimistic concurrency: workflows and runs carry a row_version that
-- increments on every update; writers must match the version they read.
ALTER TABLE workflows ADD COLUMN row_version INT NOT NULL DEFAULT 1;
ALTER TABLE workflow_runs ADD COLUMN row_version INT NOT NULL DEFAULT 1;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION bump_row_version() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    NEW.row_version := OLD.row_version + 1;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workflows_row_version
    BEFORE UPDATE ON workflows
    FOR EACH ROW EXECUTE FUNCTION bump_row_version();

CREATE TRIGGER workflow_runs_row_version
    BEFORE UPDATE ON workflow_runs
    FOR EACH ROW EXECUTE FUNCTION bump_row_version();

-- Value checks.
ALTER TABLE workflow_versions
    ADD CONSTRAINT workflow_versions_number_positive CHECK (version_number >= 1);
ALTER TABLE workflow_versions
    ADD CONSTRAINT workflow_versions_hash_len CHECK (octet_length(content_hash) = 32);
ALTER TABLE run_events
    ADD CONSTRAINT run_events_sequence_positive CHECK (sequence >= 1);
ALTER TABLE scheduled_tasks
    ADD CONSTRAINT scheduled_tasks_max_attempts_positive CHECK (max_attempts >= 1);
ALTER TABLE sessions
    ADD CONSTRAINT sessions_token_hash_len CHECK (octet_length(token_hash) = 32);

-- Runs must reference a version of the same workflow.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION check_run_version_match() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM workflow_versions
        WHERE id = NEW.workflow_version_id AND workflow_id = NEW.workflow_id
    ) THEN
        RAISE EXCEPTION 'version % does not belong to workflow %',
            NEW.workflow_version_id, NEW.workflow_id;
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER workflow_runs_version_match
    BEFORE INSERT OR UPDATE ON workflow_runs
    FOR EACH ROW EXECUTE FUNCTION check_run_version_match();

-- Hot paths: event tail reads and run lookups by status.
CREATE INDEX run_events_run_sequence_idx ON run_events (run_id, sequence DESC);
CREATE INDEX workflow_runs_created_idx ON workflow_runs (created_at DESC);

-- +goose Down
DROP TRIGGER workflow_runs_version_match ON workflow_runs;
DROP FUNCTION check_run_version_match;
DROP TRIGGER workflow_runs_row_version ON workflow_runs;
DROP TRIGGER workflows_row_version ON workflows;
DROP FUNCTION bump_row_version;
ALTER TABLE workflow_runs DROP COLUMN row_version;
ALTER TABLE workflows DROP COLUMN row_version;
