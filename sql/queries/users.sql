-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    $1,
    $2
)
RETURNING id, created_at, updated_at, email, is_chirpy_red;


-- name: DeleteAllUsers :exec
DELETE FROM users;


-- name: FindUserByEmail :one
SELECT id, created_at, updated_at, email, hashed_password, is_chirpy_red
FROM users
WHERE email = $1;


-- name: FindUserById :one
SELECT *
FROM users
WHERE id = $1;


-- name: UpdateUserLogin :one
UPDATE users
SET
    updated_at = CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    email = $2,
    hashed_password = $3
WHERE id = $1
RETURNING id, created_at, updated_at, email, is_chirpy_red;


-- name: UpgradeUser :exec
UPDATE users
SET is_chirpy_red = TRUE
WHERE id = $1;