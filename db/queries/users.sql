-- name: CreateUser :one
INSERT INTO users (email, display_name, password_hash, role)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: DisableUser :one
UPDATE users
SET disabled_at = now(), updated_at = now()
WHERE id = $1 AND disabled_at IS NULL
RETURNING *;

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, user_agent, ip, expires_at)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetActiveSessionByTokenHash :one
SELECT s.* FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now()
  AND u.disabled_at IS NULL;

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now() WHERE id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < now() - interval '7 days';
