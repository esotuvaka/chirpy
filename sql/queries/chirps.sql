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


-- name: ListChirpsByAuthor :many
SELECT *
FROM chirps
WHERE user_id = $1
ORDER BY created_at ASC;


-- name: GetChirp :one
SELECT id, created_at, updated_at, body, user_id
FROM chirps 
WHERE id = $1;


-- name: DeleteChirp :exec
DELETE FROM chirps
WHERE id = $1;