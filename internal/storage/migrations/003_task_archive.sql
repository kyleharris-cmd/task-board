ALTER TABLE tasks ADD COLUMN archived_at TEXT;

CREATE INDEX IF NOT EXISTS idx_tasks_board_archived_at ON tasks(board_id, archived_at);
