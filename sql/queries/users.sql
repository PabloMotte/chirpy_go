-- name: CreateUser :one
INSERT INTO users (id, created_at, updated_at, email)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1
)
RETURNING *;

-- name: GetAllUsers :many
SELECT id, created_at, updated_at, email
FROM users
;

-- name: GetUser :one
SELECT id, created_at, updated_at, email
FROM users
WHERE email IN ($1)
;

-- name: GetUserByID :one
SELECT id, created_at, updated_at, email
FROM users
WHERE id IN ($1)
;

-- name: DeleteAllUsers :exec
DELETE
FROM users
;

-- name: DeleteUserByID :exec
DELETE
FROM users
WHERE id IN ($1)
;

-- name: DeleteUser :exec
DELETE
FROM users
WHERE email IN ($1)
;
