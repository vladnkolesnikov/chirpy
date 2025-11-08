-- name: CreateUser :one
INSERT INTO users (email)
VALUES (@email)
RETURNING *;

-- name: DeleteAllUsers :exec
DELETE FROM users;