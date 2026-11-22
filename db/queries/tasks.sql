-- name: EnqueueTask :one
INSERT INTO scheduled_tasks (task_type, dedupe_key, payload, run_at, max_attempts)
VALUES ($1, $2, $3, COALESCE($4::timestamptz, now()), $5)
ON CONFLICT (task_type, dedupe_key) DO NOTHING
RETURNING *;

-- name: ClaimTasks :many
-- Claims up to $2 runnable tasks for worker $1, skipping rows locked by
-- other workers. Expired leases are treated as runnable again.
UPDATE scheduled_tasks
SET status = 'leased',
    leased_by = $1,
    lease_expires_at = now() + make_interval(secs => $3),
    attempts = attempts + 1,
    updated_at = now()
WHERE id IN (
    SELECT id FROM scheduled_tasks
    WHERE run_at <= now()
      AND (
        status = 'pending'
        OR (status = 'leased' AND lease_expires_at < now())
      )
    ORDER BY run_at
    LIMIT $2
    FOR UPDATE SKIP LOCKED
)
RETURNING *;

-- name: RenewTaskLease :one
UPDATE scheduled_tasks
SET lease_expires_at = now() + make_interval(secs => $3), updated_at = now()
WHERE id = $1 AND leased_by = $2 AND status = 'leased'
RETURNING *;

-- name: CompleteTask :exec
UPDATE scheduled_tasks
SET status = 'done', updated_at = now()
WHERE id = $1 AND leased_by = $2;

-- name: FailTask :one
-- Returns the task to pending for a retry, or marks it dead once attempts
-- are exhausted.
UPDATE scheduled_tasks
SET status = CASE WHEN attempts >= max_attempts THEN 'dead' ELSE 'pending' END,
    last_error = $3,
    run_at = CASE WHEN attempts >= max_attempts THEN run_at
                  ELSE now() + make_interval(secs => $4) END,
    leased_by = NULL,
    lease_expires_at = NULL,
    updated_at = now()
WHERE id = $1 AND leased_by = $2
RETURNING *;
