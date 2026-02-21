-- name: UpsertBoard :exec
INSERT INTO boards (id, repo_root, created_at, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET repo_root = excluded.repo_root, updated_at = excluded.updated_at;

-- name: GetBoardByRepoRoot :one
SELECT id, repo_root, created_at, updated_at
FROM boards
WHERE repo_root = ?;
