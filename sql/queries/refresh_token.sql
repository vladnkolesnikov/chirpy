-- name: CreateToken :exec
INSERT INTO refresh_tokens (token, user_id, expires_at, revoked_at)
VALUES (@token, @user_id, @expires_at, null);

-- name: GetToken :one
SELECT token, expires_at, revoked_at, user_id
FROM refresh_tokens
WHERE token = @token;

-- name: RevokeToken :exec
UPDATE refresh_tokens
SET revoked_at = $2,
    updated_at = $3
WHERE token = $1;