CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE TABLE IF NOT EXISTS workflow_definitions (
  id BIGSERIAL PRIMARY KEY,
  workflow_id UUID NOT NULL,
  name TEXT NOT NULL,
  version INT NOT NULL,
  description TEXT,
  status TEXT NOT NULL,
  definition JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  created_by TEXT,
  updated_at TIMESTAMPTZ NOT NULL,
  updated_by TEXT,
  UNIQUE (workflow_id, version)
);

CREATE TABLE IF NOT EXISTS tasks (
  id BIGSERIAL PRIMARY KEY,
  task_id UUID NOT NULL UNIQUE,
  workflow_id UUID NOT NULL,
  workflow_version INT NOT NULL,
  title TEXT NOT NULL,
  status TEXT NOT NULL,
  context JSONB,
  trace_id TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  created_by TEXT,
  updated_at TIMESTAMPTZ NOT NULL,
  updated_by TEXT
);

CREATE TABLE IF NOT EXISTS task_steps (
  id BIGSERIAL PRIMARY KEY,
  step_id UUID NOT NULL UNIQUE,
  task_id UUID NOT NULL REFERENCES tasks(task_id) ON DELETE CASCADE,
  step_key TEXT NOT NULL,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  executor_type TEXT NOT NULL,
  executor_ref TEXT NOT NULL,
  action_type TEXT NOT NULL,
  action_config JSONB NOT NULL,
  timeout_seconds INT NOT NULL,
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 0,
  depends_on UUID[] DEFAULT '{}',
  action_id UUID NOT NULL,
  on_fail JSONB,
  evidence JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  dispatched_at TIMESTAMPTZ,
  acked_at TIMESTAMPTZ,
  resolved_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS executors (
  executor_id TEXT PRIMARY KEY,
  executor_type TEXT NOT NULL,
  display_name TEXT NOT NULL,
  capability_tags TEXT[] DEFAULT '{}',
  status TEXT NOT NULL,
  metadata JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS actions (
  id BIGSERIAL PRIMARY KEY,
  action_id UUID NOT NULL UNIQUE,
  rule_id UUID,
  rule_version INT,
  evaluation_id UUID,
  action_type TEXT NOT NULL,
  action_config JSONB NOT NULL,
  status TEXT NOT NULL,
  dedupe_key TEXT,
  cooldown_until TIMESTAMPTZ,
  priority TEXT NOT NULL,
  ttl_seconds INT,
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 0,
  last_error TEXT,
  created_at TIMESTAMPTZ NOT NULL,
  dispatched_at TIMESTAMPTZ,
  acked_at TIMESTAMPTZ,
  acked_by TEXT,
  resolved_at TIMESTAMPTZ,
  resolved_by TEXT,
  failed_at TIMESTAMPTZ,
  trace_id TEXT
);

CREATE TABLE IF NOT EXISTS action_state_transitions (
  id BIGSERIAL PRIMARY KEY,
  action_id UUID NOT NULL,
  from_status TEXT,
  to_status TEXT NOT NULL,
  transitioned_at TIMESTAMPTZ NOT NULL,
  reason TEXT,
  metadata JSONB
);

CREATE TABLE IF NOT EXISTS notifications (
  id BIGSERIAL PRIMARY KEY,
  notification_id UUID NOT NULL UNIQUE,
  action_id UUID NOT NULL,
  channel TEXT NOT NULL,
  priority TEXT NOT NULL,
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  payload JSONB,
  status TEXT NOT NULL,
  target_user_id TEXT,
  target_group TEXT,
  retry_count INT NOT NULL DEFAULT 0,
  max_retries INT NOT NULL DEFAULT 0,
  last_error TEXT,
  expires_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL,
  sent_at TIMESTAMPTZ,
  delivered_at TIMESTAMPTZ,
  failed_at TIMESTAMPTZ,
  trace_id TEXT
);

CREATE TABLE IF NOT EXISTS notification_attempts (
  id BIGSERIAL PRIMARY KEY,
  notification_id UUID NOT NULL,
  attempt_number INT NOT NULL,
  status TEXT NOT NULL,
  attempted_at TIMESTAMPTZ NOT NULL,
  response_code INT,
  response_body TEXT,
  error_message TEXT,
  duration_ms INT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id BIGSERIAL PRIMARY KEY,
  audit_id UUID NOT NULL UNIQUE,
  entity_type TEXT NOT NULL,
  entity_id TEXT NOT NULL,
  action TEXT NOT NULL,
  actor TEXT NOT NULL,
  actor_roles TEXT[] DEFAULT '{}',
  actor_ip INET,
  user_agent TEXT,
  old_values JSONB,
  new_values JSONB,
  diff JSONB,
  reason TEXT,
  risk_level TEXT NOT NULL,
  tags TEXT[] DEFAULT '{}',
  signature BYTEA,
  trace_id TEXT,
  session_id TEXT,
  request_method TEXT,
  request_path TEXT,
  response_status INT,
  duration_ms INT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS rules (
  id BIGSERIAL PRIMARY KEY,
  rule_id UUID NOT NULL,
  version INT NOT NULL,
  name TEXT NOT NULL,
  description TEXT,
  rule_type TEXT NOT NULL,
  config JSONB NOT NULL,
  action_type TEXT NOT NULL,
  action_config JSONB NOT NULL,
  scope_factory_id TEXT,
  scope_line_id TEXT,
  effective_from TIMESTAMPTZ NOT NULL,
  effective_until TIMESTAMPTZ,
  status TEXT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  created_by TEXT,
  updated_at TIMESTAMPTZ NOT NULL,
  updated_by TEXT,
  UNIQUE (rule_id, version)
);

CREATE TABLE IF NOT EXISTS rule_evaluations (
  id BIGSERIAL PRIMARY KEY,
  evaluation_id UUID NOT NULL UNIQUE,
  rule_id UUID NOT NULL,
  rule_version INT NOT NULL,
  rule_type TEXT NOT NULL,
  matched BOOLEAN NOT NULL,
  evaluated_at TIMESTAMPTZ NOT NULL,
  evidence JSONB,
  event_ids UUID[] DEFAULT '{}',
  trace_id TEXT
);

CREATE TABLE IF NOT EXISTS trust_hash_chain_entries (
  id BIGSERIAL PRIMARY KEY,
  event_id UUID NOT NULL UNIQUE,
  source_id TEXT NOT NULL,
  sequence_num BIGINT NOT NULL,
  event_hash TEXT NOT NULL,
  prev_hash TEXT NOT NULL,
  chain_hash TEXT NOT NULL,
  trust_level INT NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE (source_id, sequence_num)
);

CREATE TABLE IF NOT EXISTS trust_batch_signatures (
  id BIGSERIAL PRIMARY KEY,
  batch_id UUID NOT NULL UNIQUE,
  source_id TEXT NOT NULL,
  event_ids UUID[] DEFAULT '{}',
  batch_hash TEXT NOT NULL,
  signature TEXT NOT NULL,
  signature_alg TEXT NOT NULL,
  key_id TEXT NOT NULL,
  signed_at TIMESTAMPTZ NOT NULL,
  verified_at TIMESTAMPTZ,
  verification_status TEXT NOT NULL,
  verification_error TEXT,
  created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS trust_metadata (
  event_id UUID PRIMARY KEY,
  trust_level INT NOT NULL,
  chain_entry_id BIGINT,
  batch_id UUID,
  verified_at TIMESTAMPTZ,
  flags TEXT[] DEFAULT '{}',
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status);
CREATE INDEX IF NOT EXISTS idx_steps_task ON task_steps(task_id);
CREATE INDEX IF NOT EXISTS idx_actions_status ON actions(status);
CREATE INDEX IF NOT EXISTS idx_notifications_status ON notifications(status);
CREATE INDEX IF NOT EXISTS idx_audit_entity ON audit_logs(entity_type, entity_id);
CREATE INDEX IF NOT EXISTS idx_audit_trace ON audit_logs(trace_id);
CREATE INDEX IF NOT EXISTS idx_rule_eval_rule ON rule_evaluations(rule_id);
CREATE INDEX IF NOT EXISTS idx_hash_chain_source ON trust_hash_chain_entries(source_id);
CREATE INDEX IF NOT EXISTS idx_batch_sig_source ON trust_batch_signatures(source_id);
