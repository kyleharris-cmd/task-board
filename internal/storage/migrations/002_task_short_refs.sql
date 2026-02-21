ALTER TABLE tasks ADD COLUMN short_ref TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_tasks_board_short_ref ON tasks(board_id, short_ref);

CREATE TABLE IF NOT EXISTS task_ref_counters (
  board_id TEXT PRIMARY KEY,
  next_value INTEGER NOT NULL,
  FOREIGN KEY (board_id) REFERENCES boards(id)
);
