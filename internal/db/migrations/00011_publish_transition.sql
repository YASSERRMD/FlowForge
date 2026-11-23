-- +goose Up
-- The 00004 immutability trigger rejected every UPDATE on published rows,
-- including the published → superseded transition that publishing a newer
-- version requires. Replace it with a function that permits exactly that
-- one status-only transition and rejects everything else.

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_version_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'UPDATE'
       AND OLD.status = 'published'
       AND NEW.status = 'superseded'
       AND NEW.graph_json = OLD.graph_json
       AND NEW.content_hash = OLD.content_hash
       AND NEW.version_number = OLD.version_number THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'workflow_versions rows with status % are immutable', OLD.status;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION reject_version_mutation() RETURNS trigger
    LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'workflow_versions rows with status % are immutable', OLD.status;
END;
$$;
-- +goose StatementEnd
