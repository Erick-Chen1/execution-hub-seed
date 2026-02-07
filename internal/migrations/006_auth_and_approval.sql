CREATE TABLE IF NOT EXISTS users (
  id BIGSERIAL PRIMARY KEY,
  user_id UUID NOT NULL UNIQUE,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  role TEXT NOT NULL,
  user_type TEXT NOT NULL,
  owner_user_id UUID,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'users_owner_fk') THEN
    ALTER TABLE users
      ADD CONSTRAINT users_owner_fk FOREIGN KEY (owner_user_id) REFERENCES users(user_id) ON DELETE SET NULL;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_users_owner ON users(owner_user_id);
CREATE INDEX IF NOT EXISTS idx_users_role ON users(role);
CREATE INDEX IF NOT EXISTS idx_users_type ON users(user_type);

CREATE TABLE IF NOT EXISTS sessions (
  id BIGSERIAL PRIMARY KEY,
  session_id UUID NOT NULL UNIQUE,
  token_hash TEXT NOT NULL,
  user_id UUID NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ,
  user_agent TEXT,
  ip_address INET
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'sessions_user_fk') THEN
    ALTER TABLE sessions
      ADD CONSTRAINT sessions_user_fk FOREIGN KEY (user_id) REFERENCES users(user_id) ON DELETE CASCADE;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_token_hash ON sessions(token_hash);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE IF NOT EXISTS approvals (
  id BIGSERIAL PRIMARY KEY,
  approval_id UUID NOT NULL UNIQUE,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  operation TEXT NOT NULL,
  status TEXT NOT NULL,
  request_payload JSONB NOT NULL,
  requested_by TEXT NOT NULL,
  requested_by_type TEXT NOT NULL,
  required_roles TEXT[] NOT NULL,
  applies_to TEXT NOT NULL,
  min_approvals INT NOT NULL,
  approvals_count INT NOT NULL DEFAULT 0,
  rejections_count INT NOT NULL DEFAULT 0,
  requested_at TIMESTAMPTZ NOT NULL,
  decided_at TIMESTAMPTZ,
  executed_at TIMESTAMPTZ,
  execution_error TEXT
);

CREATE INDEX IF NOT EXISTS idx_approvals_status ON approvals(status);
CREATE INDEX IF NOT EXISTS idx_approvals_entity ON approvals(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_approvals_operation ON approvals(operation);

CREATE TABLE IF NOT EXISTS approval_decisions (
  id BIGSERIAL PRIMARY KEY,
  decision_id UUID NOT NULL UNIQUE,
  approval_id UUID NOT NULL,
  decision TEXT NOT NULL,
  decided_by TEXT NOT NULL,
  decided_by_type TEXT NOT NULL,
  decided_by_role TEXT NOT NULL,
  comment TEXT,
  decided_at TIMESTAMPTZ NOT NULL
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'approval_decisions_fk') THEN
    ALTER TABLE approval_decisions
      ADD CONSTRAINT approval_decisions_fk FOREIGN KEY (approval_id) REFERENCES approvals(approval_id) ON DELETE CASCADE;
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_approval_decisions_approval_id ON approval_decisions(approval_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_approval_decisions_actor ON approval_decisions(approval_id, decided_by);
