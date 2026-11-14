-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, user_id, expires_at, revoked_at)
VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    $3,
    $4
)
RETURNING *;

-- name: GetAllRefreshTokens :many
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
ORDER BY created_at
;

-- name: GetRefreshToken :one
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE token IN ($1) and revoked_at is NULL
;

-- name: GetRefreshTokenByUserID :one
SELECT token, created_at, updated_at, user_id, expires_at, revoked_at
FROM refresh_tokens
WHERE user_id IN ($1) and revoked_at is NULL
ORDER BY created_at
;

-- name: DeleteAllRefreshTokens :exec
DELETE
FROM refresh_tokens
;

-- name: DeleteRefreshToken :exec
DELETE
FROM refresh_tokens
WHERE token IN ($1)
;

-- name: DeleteAllRefreshTokensByUserID :exec
DELETE
FROM refresh_tokens
WHERE user_id IN ($1)
;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at=NOW(), updated_at=NOW()
WHERE token IN ($1)
;

-- name: RevokeAllRefreshTokensByUserID :exec
UPDATE refresh_tokens
SET revoked_at=NOW(), updated_at=NOW()
WHERE user_id IN ($1)
;
