ALTER TABLE task_steps ADD COLUMN IF NOT EXISTS trace_id TEXT;

UPDATE task_steps ts
SET trace_id = t.trace_id
FROM tasks t
WHERE ts.task_id = t.task_id AND (ts.trace_id IS NULL OR ts.trace_id = '');

CREATE INDEX IF NOT EXISTS idx_steps_trace ON task_steps(trace_id);
