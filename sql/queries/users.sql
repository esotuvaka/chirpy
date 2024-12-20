-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email, hashed_password)
VALUES (
    gen_random_uuid(),
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    $1,
    $2
)
RETURNING id, created_at, updated_at, email;


-- name: DeleteAllUsers :exec
DELETE FROM users;


-- name: FindUserByEmail :one
SELECT id, created_at, updated_at, email, hashed_password
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
RETURNING id, created_at, updated_at, email;