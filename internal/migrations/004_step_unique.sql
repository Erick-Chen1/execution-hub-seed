DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'task_steps_task_stepkey_unique'
  ) THEN
    ALTER TABLE task_steps
      ADD CONSTRAINT task_steps_task_stepkey_unique UNIQUE (task_id, step_key);
  END IF;
END $$;
