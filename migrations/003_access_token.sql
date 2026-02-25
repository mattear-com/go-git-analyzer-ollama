-- CodeLens AI: Add access_token column to store OAuth provider tokens
-- Needed to call GitHub API for listing repos, etc.

ALTER TABLE users ADD COLUMN IF NOT EXISTS access_token TEXT DEFAULT '';
