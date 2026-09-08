-- +goose Up
ALTER TABLE user_sessions
ADD COLUMN user_email TEXT NOT NULL DEFAULT '';

CREATE INDEX idx_user_sessions_org_email_active
ON user_sessions(org_id, user_email, revoked_at, expires_at);