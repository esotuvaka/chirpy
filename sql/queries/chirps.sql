-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    $1,
    $2
)
RETURNING *;

-- name: ListChirps :many
SELECT * 
FROM chirps
ORDER BY created_at DESC;

-- name: GetChirp :one
SELECT id, created_at, updated_at, body, user_id
FROM chirps 
WHERE id = $1;