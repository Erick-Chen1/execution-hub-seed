CREATE TABLE IF NOT EXISTS events (
  event_id UUID PRIMARY KEY,
  client_record_id TEXT,
  source_type TEXT NOT NULL,
  source_id TEXT NOT NULL,
  ts_device TIMESTAMPTZ,
  ts_gateway TIMESTAMPTZ,
  ts_server TIMESTAMPTZ NOT NULL,
  key TEXT,
  event_type TEXT NOT NULL,
  payload JSONB NOT NULL,
  schema_version TEXT NOT NULL,
  trust_level INT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_source ON events(source_id);
