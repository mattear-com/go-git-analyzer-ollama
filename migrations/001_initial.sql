-- CodeLens AI: Initial PostgreSQL Schema
-- Requires: pgvector extension

CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "vector";

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email       VARCHAR(255) NOT NULL UNIQUE,
    name        VARCHAR(255) NOT NULL,
    avatar_url  TEXT DEFAULT '',
    provider    VARCHAR(50) NOT NULL,          -- google, github, etc.
    provider_id VARCHAR(255) NOT NULL,
    role        VARCHAR(50) NOT NULL DEFAULT 'user', -- user, admin
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_users_provider ON users(provider, provider_id);

-- ============================================================
-- REPOSITORIES
-- ============================================================
CREATE TABLE repos (
    id             UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name           VARCHAR(255) NOT NULL,
    url            TEXT NOT NULL,
    default_branch VARCHAR(100) DEFAULT 'main',
    local_path     TEXT DEFAULT '',
    status         VARCHAR(50) NOT NULL DEFAULT 'cloning', -- cloning, ready, error
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_repos_user ON repos(user_id);
CREATE INDEX idx_repos_status ON repos(status);

-- ============================================================
-- SNAPSHOTS (immutable, one per commit hash)
-- ============================================================
CREATE TABLE snapshots (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    repo_id     UUID NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    commit_hash VARCHAR(64) NOT NULL,
    branch      VARCHAR(255) DEFAULT '',
    message     TEXT DEFAULT '',
    author      VARCHAR(255) DEFAULT '',
    file_count  INTEGER DEFAULT 0,
    status      VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, vectorized, analyzed
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_snapshots_repo_commit ON snapshots(repo_id, commit_hash);
CREATE INDEX idx_snapshots_status ON snapshots(status);

-- ============================================================
-- EMBEDDINGS (pgvector, versioned by snapshot/commit)
-- ============================================================
CREATE TABLE embeddings (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    repo_id     UUID NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    file_path   TEXT NOT NULL,
    chunk_index INTEGER NOT NULL DEFAULT 0,
    content     TEXT NOT NULL,
    language    VARCHAR(50) DEFAULT '',
    vector      vector(1024),  -- BGE-M3 / Qwen3-embedding dimension
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_embeddings_snapshot ON embeddings(snapshot_id);
CREATE INDEX idx_embeddings_repo ON embeddings(repo_id);
CREATE INDEX idx_embeddings_vector ON embeddings USING ivfflat (vector vector_cosine_ops) WITH (lists = 100);

-- ============================================================
-- ANALYSIS RESULTS
-- ============================================================
CREATE TABLE analysis_results (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    snapshot_id UUID NOT NULL REFERENCES snapshots(id) ON DELETE CASCADE,
    repo_id     UUID NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    strategy    VARCHAR(100) NOT NULL,  -- architecture, code_quality, functionality, devops
    summary     TEXT DEFAULT '',
    details     JSONB DEFAULT '{}',
    score       DOUBLE PRECISION DEFAULT 0.0,
    suggestions TEXT[] DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_analysis_snapshot ON analysis_results(snapshot_id);
CREATE INDEX idx_analysis_strategy ON analysis_results(strategy);

-- ============================================================
-- AUDIT LOGS (compliance)
-- ============================================================
CREATE TABLE audit_logs (
    id          UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     VARCHAR(255) NOT NULL,  -- UUID or 'anonymous'
    action      VARCHAR(100) NOT NULL,  -- login, repo_access, analysis_run, etc.
    resource    VARCHAR(100) NOT NULL,  -- api, repo, analysis, etc.
    resource_id VARCHAR(255) DEFAULT '',
    details     JSONB DEFAULT '{}',
    ip          VARCHAR(45) DEFAULT '',
    user_agent  TEXT DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_user ON audit_logs(user_id);
CREATE INDEX idx_audit_action ON audit_logs(action);
CREATE INDEX idx_audit_created ON audit_logs(created_at DESC);

-- ============================================================
-- Trigger: auto-update updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION update_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();

CREATE TRIGGER trg_repos_updated_at
    BEFORE UPDATE ON repos
    FOR EACH ROW EXECUTE FUNCTION update_updated_at();
