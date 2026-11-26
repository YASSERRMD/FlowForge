-- name: CreateRun :one
INSERT INTO workflow_runs
    (workflow_id, workflow_version_id, variables, idempotency_key, triggered_by, trigger_type)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workflow_id, idempotency_key) WHERE idempotency_key IS NOT NULL
DO NOTHING
RETURNING *;

-- name: GetRun :one
SELECT * FROM workflow_runs WHERE id = $1;

-- name: ListRunsByWorkflow :many
SELECT * FROM workflow_runs
WHERE workflow_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: StartRun :one
UPDATE workflow_runs
SET status = 'running', started_at = now(), updated_at = now()
WHERE id = $1 AND status = 'queued'
RETURNING *;

-- name: FinishRun :one
UPDATE workflow_runs
SET status = $2, finished_at = now(), updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running')
  AND $2 IN ('succeeded', 'failed', 'canceled')
RETURNING *;

-- name: RequestRunCancellation :one
UPDATE workflow_runs
SET cancel_requested_at = now(), updated_at = now()
WHERE id = $1 AND status IN ('queued', 'running')
RETURNING *;

-- name: CreateStepAttempt :one
INSERT INTO step_attempts (run_id, node_id, node_type, attempt_number)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: StartStepAttempt :one
UPDATE step_attempts
SET status = 'running', started_at = now(), input_json = $3
WHERE id = $1 AND attempt_number = $2 AND status = 'pending'
RETURNING *;

-- name: FinishStepAttempt :one
UPDATE step_attempts
SET status = $2, output_json = $3, error_code = $4, error_message = $5,
    finished_at = now()
WHERE id = $1 AND status = 'running'
RETURNING *;

-- name: ListStepAttemptsByRun :many
SELECT * FROM step_attempts
WHERE run_id = $1
ORDER BY node_id, attempt_number;

-- name: AppendRunEvent :one
INSERT INTO run_events (run_id, sequence, event_type, payload)
VALUES (
    $1,
    (SELECT COALESCE(MAX(sequence), 0) + 1 FROM run_events WHERE run_id = $1),
    $2,
    $3
)
RETURNING *;

-- name: ListRunEvents :many
SELECT * FROM run_events
WHERE run_id = $1 AND sequence > $2
ORDER BY sequence
LIMIT $3;

-- name: EnqueueOutbox :one
INSERT INTO outbox_messages (topic, payload)
VALUES ($1, $2)
RETURNING *;

-- name: ClaimOutboxBatch :many
UPDATE outbox_messages
SET publish_attempts = publish_attempts + 1
WHERE id IN (
    SELECT id FROM outbox_messages
    WHERE published_at IS NULL
    ORDER BY id
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: MarkOutboxPublished :exec
UPDATE outbox_messages SET published_at = now() WHERE id = $1;

-- name: GetRunByIdempotencyKey :one
SELECT * FROM workflow_runs
WHERE workflow_id = $1 AND idempotency_key = $2;

-- name: GetRunForUpdate :one
SELECT * FROM workflow_runs WHERE id = $1 FOR UPDATE;

-- name: NextAttemptNumber :one
SELECT COALESCE(MAX(attempt_number), 0) + 1 FROM step_attempts
WHERE run_id = $1 AND node_id = $2;

-- name: CountActiveStepAttempts :one
SELECT count(*) FROM step_attempts
WHERE run_id = $1 AND status IN ('pending', 'running');

-- name: GetWorkflowVersionByID :one
SELECT * FROM workflow_versions WHERE id = $1;

-- name: ListSucceededStepOutputsByRun :many
-- Latest succeeded output per node, used to build the expression context.
SELECT DISTINCT ON (node_id) node_id, output_json
FROM step_attempts
WHERE run_id = $1 AND status = 'succeeded'
ORDER BY node_id, attempt_number DESC;

-- name: ListLatestNodeStatusesByRun :many
-- Latest attempt status per node, used by skip propagation.
SELECT DISTINCT ON (node_id) node_id, status
FROM step_attempts
WHERE run_id = $1
ORDER BY node_id, attempt_number DESC;

-- name: CreateSkippedStepAttempt :one
INSERT INTO step_attempts
    (run_id, node_id, node_type, attempt_number, status, output_json, finished_at)
VALUES ($1, $2, $3, $4, 'skipped', $5, now())
RETURNING *;

-- name: CountPendingNodeTasks :one
SELECT count(*) FROM scheduled_tasks
WHERE task_type = 'execute_node' AND dedupe_key = $1 AND status = 'pending';
