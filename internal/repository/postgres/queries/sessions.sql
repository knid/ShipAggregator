-- Sessions CRUD queries
-- Table: sessions

-- name: GetSessionByID :one
SELECT s.id, s.member_id, s.token_hash, s.ip_address, s.user_agent, 
       s.expires_at, s.created_at, s.last_activity_at
FROM sessions s
WHERE s.id = $1 AND s.expires_at > NOW();

-- name: GetSessionByTokenHash :one
SELECT s.id, s.member_id, s.token_hash, s.ip_address, s.user_agent, 
       s.expires_at, s.created_at, s.last_activity_at
FROM sessions s
WHERE s.token_hash = $1 AND s.expires_at > NOW();

-- name: GetSessionWithMember :one
SELECT s.id, s.member_id, s.token_hash, s.ip_address, s.user_agent, 
       s.expires_at, s.created_at, s.last_activity_at,
       m.organization_id, m.email, m.name, m.role_id, m.status as member_status
FROM sessions s
JOIN members m ON s.member_id = m.id
WHERE s.token_hash = $1 AND s.expires_at > NOW() AND m.status = 'active';

-- name: ListSessionsByMember :many
SELECT s.id, s.member_id, s.token_hash, s.ip_address, s.user_agent, 
       s.expires_at, s.created_at, s.last_activity_at
FROM sessions s
WHERE s.member_id = $1 AND s.expires_at > NOW()
ORDER BY s.last_activity_at DESC;

-- name: CreateSession :one
INSERT INTO sessions (
    member_id,
    token_hash,
    ip_address,
    user_agent,
    expires_at
) VALUES (
    $1, $2, $3, $4, $5
)
RETURNING id, member_id, token_hash, ip_address, user_agent, expires_at, created_at, last_activity_at;

-- name: UpdateSessionActivity :exec
UPDATE sessions
SET last_activity_at = NOW()
WHERE id = $1;

-- name: ExtendSession :one
UPDATE sessions
SET 
    expires_at = $2,
    last_activity_at = NOW()
WHERE id = $1 AND expires_at > NOW()
RETURNING id, member_id, token_hash, ip_address, user_agent, expires_at, created_at, last_activity_at;

-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions
WHERE token_hash = $1;

-- name: DeleteSessionsByMember :exec
DELETE FROM sessions
WHERE member_id = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions
WHERE expires_at <= NOW();

-- name: CountActiveSessionsByMember :one
SELECT COUNT(*) as total
FROM sessions
WHERE member_id = $1 AND expires_at > NOW();

-- name: DeleteOldestSessionsByMember :exec
DELETE FROM sessions
WHERE id IN (
    SELECT s2.id FROM sessions s2
    WHERE s2.member_id = $1 AND s2.expires_at > NOW()
    ORDER BY s2.last_activity_at ASC
    LIMIT $2
);
