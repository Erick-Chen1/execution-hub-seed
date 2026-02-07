ALTER TABLE notifications
  ADD COLUMN IF NOT EXISTS dedupe_key TEXT;

CREATE INDEX IF NOT EXISTS idx_notifications_dedupe_key ON notifications(dedupe_key);
CREATE INDEX IF NOT EXISTS idx_notifications_dedupe_key_created_at ON notifications(dedupe_key, created_at DESC);
