CREATE TABLE IF NOT EXISTS collab_sessions (
  id BIGSERIAL PRIMARY KEY,
  session_id UUID NOT NULL UNIQUE,
  task_id UUID NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
  workflow_id UUID NOT NULL,
  workflow_version INT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  context JSONB,
  trace_id TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  UNIQUE (task_id)
);

CREATE TABLE IF NOT EXISTS collab_participants (
  id BIGSERIAL PRIMARY KEY,
  participant_id UUID NOT NULL UNIQUE,
  session_id UUID NOT NULL REFERENCES collab_sessions(session_id) ON DELETE CASCADE,
  type TEXT NOT NULL,
  ref TEXT NOT NULL,
  capabilities TEXT[] DEFAULT '{}',
  trust_score INT NOT NULL DEFAULT 0,
  joined_at TIMESTAMPTZ NOT NULL,
  last_seen_at TIMESTAMPTZ,
  UNIQUE (session_id, ref)
);

CREATE TABLE IF NOT EXISTS collab_steps (
  id BIGSERIAL PRIMARY KEY,
  step_id UUID NOT NULL UNIQUE,
  session_id UUID NOT NULL REFERENCES collab_sessions(session_id) ON DELETE CASCADE,
  step_key TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  required_capabilities TEXT[] DEFAULT '{}',
  depends_on TEXT[] DEFAULT '{}',
  lease_ttl_seconds INT NOT NULL DEFAULT 900,
  consensus_policy JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  resolved_at TIMESTAMPTZ,
  UNIQUE (session_id, step_key)
);

CREATE TABLE IF NOT EXISTS collab_claims (
  id BIGSERIAL PRIMARY KEY,
  claim_id UUID NOT NULL UNIQUE,
  step_id UUID NOT NULL REFERENCES collab_steps(step_id) ON DELETE CASCADE,
  participant_id UUID NOT NULL REFERENCES collab_participants(participant_id) ON DELETE CASCADE,
  status TEXT NOT NULL,
  lease_until TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS collab_artifacts (
  id BIGSERIAL PRIMARY KEY,
  artifact_id UUID NOT NULL UNIQUE,
  step_id UUID NOT NULL REFERENCES collab_steps(step_id) ON DELETE CASCADE,
  producer_id UUID NOT NULL REFERENCES collab_participants(participant_id) ON DELETE CASCADE,
  kind TEXT NOT NULL,
  content JSONB NOT NULL,
  version INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (step_id, version)
);

CREATE TABLE IF NOT EXISTS collab_decisions (
  id BIGSERIAL PRIMARY KEY,
  decision_id UUID NOT NULL UNIQUE,
  step_id UUID NOT NULL REFERENCES collab_steps(step_id) ON DELETE CASCADE,
  policy JSONB NOT NULL,
  deadline TIMESTAMPTZ,
  status TEXT NOT NULL,
  result TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL,
  decided_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS collab_votes (
  id BIGSERIAL PRIMARY KEY,
  vote_id UUID NOT NULL UNIQUE,
  decision_id UUID NOT NULL REFERENCES collab_decisions(decision_id) ON DELETE CASCADE,
  participant_id UUID NOT NULL REFERENCES collab_participants(participant_id) ON DELETE CASCADE,
  choice TEXT NOT NULL,
  comment TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (decision_id, participant_id)
);

CREATE TABLE IF NOT EXISTS collab_events (
  id BIGSERIAL PRIMARY KEY,
  event_id UUID NOT NULL UNIQUE,
  session_id UUID NOT NULL REFERENCES collab_sessions(session_id) ON DELETE CASCADE,
  step_id UUID REFERENCES collab_steps(step_id) ON DELETE SET NULL,
  type TEXT NOT NULL,
  actor TEXT NOT NULL,
  payload JSONB,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_collab_sessions_status ON collab_sessions(status);
CREATE INDEX IF NOT EXISTS idx_collab_participants_session ON collab_participants(session_id);
CREATE INDEX IF NOT EXISTS idx_collab_steps_session_status ON collab_steps(session_id, status);
CREATE INDEX IF NOT EXISTS idx_collab_steps_depends_on ON collab_steps USING GIN(depends_on);
CREATE INDEX IF NOT EXISTS idx_collab_claims_step_status ON collab_claims(step_id, status);
CREATE UNIQUE INDEX IF NOT EXISTS idx_collab_claims_step_active ON collab_claims(step_id) WHERE status='ACTIVE';
CREATE INDEX IF NOT EXISTS idx_collab_claims_lease_until ON collab_claims(lease_until);
CREATE INDEX IF NOT EXISTS idx_collab_artifacts_step ON collab_artifacts(step_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_collab_decisions_step ON collab_decisions(step_id, created_at DESC);
CREATE UNIQUE INDEX IF NOT EXISTS idx_collab_decisions_pending_per_step ON collab_decisions(step_id) WHERE status='PENDING';
CREATE INDEX IF NOT EXISTS idx_collab_votes_decision ON collab_votes(decision_id);
CREATE INDEX IF NOT EXISTS idx_collab_events_session ON collab_events(session_id, created_at DESC);
