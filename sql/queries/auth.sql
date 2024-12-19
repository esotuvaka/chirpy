-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    $2,
    $3,
    NULL
);

-- name: FindRefreshToken :one
SELECT *
FROM refresh_tokens
WHERE token = $1;