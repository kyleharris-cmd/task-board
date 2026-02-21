CREATE TABLE IF NOT EXISTS boards (
  id TEXT PRIMARY KEY,
  repo_root TEXT NOT NULL UNIQUE,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS tasks (
  id TEXT PRIMARY KEY,
  board_id TEXT NOT NULL,
  title TEXT NOT NULL,
  description TEXT,
  state TEXT NOT NULL,
  parent_id TEXT,
  required_for_parent INTEGER NOT NULL DEFAULT 1,
  priority INTEGER NOT NULL DEFAULT 3,
  task_type TEXT NOT NULL DEFAULT 'default',
  rubric_passed INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (board_id) REFERENCES boards(id),
  FOREIGN KEY (parent_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_tasks_board_state ON tasks(board_id, state);
CREATE INDEX IF NOT EXISTS idx_tasks_parent ON tasks(parent_id);

CREATE TABLE IF NOT EXISTS task_leases (
  task_id TEXT PRIMARY KEY,
  actor_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_display_name TEXT NOT NULL,
  lease_expires_at TEXT NOT NULL,
  auto_renew INTEGER NOT NULL DEFAULT 0,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_task_leases_expires_at ON task_leases(lease_expires_at);

CREATE TABLE IF NOT EXISTS task_transitions (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  from_state TEXT NOT NULL,
  to_state TEXT NOT NULL,
  actor_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_display_name TEXT NOT NULL,
  reason TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_task_transitions_task_id_created_at ON task_transitions(task_id, created_at);

CREATE TABLE IF NOT EXISTS task_artifacts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  artifact_type TEXT NOT NULL,
  markdown_path TEXT NOT NULL,
  content_snapshot TEXT NOT NULL,
  version INTEGER NOT NULL,
  actor_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_display_name TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_task_artifacts_task_type_version ON task_artifacts(task_id, artifact_type, version);

CREATE TABLE IF NOT EXISTS rubric_results (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  task_id TEXT NOT NULL,
  rubric_version TEXT NOT NULL,
  required_fields_complete INTEGER NOT NULL,
  pass_fail INTEGER NOT NULL,
  actor_type TEXT NOT NULL,
  actor_id TEXT NOT NULL,
  actor_display_name TEXT NOT NULL,
  notes TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY (task_id) REFERENCES tasks(id)
);

CREATE INDEX IF NOT EXISTS idx_rubric_results_task_id_created_at ON rubric_results(task_id, created_at);
