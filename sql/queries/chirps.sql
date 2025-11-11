-- name: CreateChirp :one
INSERT INTO chirps (body, user_id)
VALUES (@body, @user_id)
RETURNING *;

-- name: GetChirps :many
SELECT * FROM chirps
ORDER BY created_at;

-- name: GetChirpByID :one
SELECT * FROM chirps
WHERE id = @id;