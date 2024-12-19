// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.27.0
// source: auth.sql

package database

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createRefreshToken = `-- name: CreateRefreshToken :exec
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    CURRENT_TIMESTAMP AT TIME ZONE 'UTC',
    $2,
    $3,
    NULL
)
`

type CreateRefreshTokenParams struct {
	Token     string
	UserID    uuid.NullUUID
	ExpiresAt time.Time
}

func (q *Queries) CreateRefreshToken(ctx context.Context, arg CreateRefreshTokenParams) error {
	_, err := q.db.ExecContext(ctx, createRefreshToken, arg.Token, arg.UserID, arg.ExpiresAt)
	return err
}

const findRefreshToken = `-- name: FindRefreshToken :one
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token = $1
`

func (q *Queries) FindRefreshToken(ctx context.Context, token string) (RefreshToken, error) {
	row := q.db.QueryRowContext(ctx, findRefreshToken, token)
	var i RefreshToken
	err := row.Scan(
		&i.Token,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.UserID,
		&i.ExpiresAt,
		&i.RevokedAt,
	)
	return i, err
}