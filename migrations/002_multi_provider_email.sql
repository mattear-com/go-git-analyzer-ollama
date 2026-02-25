-- CodeLens AI: Remove UNIQUE constraint on email to support multi-provider login
-- Same email can exist with different providers (google, github, etc.)

ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;

-- The unique constraint is now only on (provider, provider_id)
-- which was already created in 001_initial.sql as idx_users_provider
