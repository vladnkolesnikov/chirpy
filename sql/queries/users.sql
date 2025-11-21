-- name: CreateUser :one
INSERT INTO users (email, hashed_password)
VALUES (@email, @password)
RETURNING id, created_at, updated_at, email;

-- name: DeleteAllUsers :exec
DELETE FROM users;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = @email;

-- name: UpdateUserInfo :one
UPDATE users
SET email           = $2,
    hashed_password = $3
WHERE id = $1
RETURNING *;
