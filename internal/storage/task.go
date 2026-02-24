package storage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kyleharris/task-board/internal/domain"
)

type CreateTaskInput struct {
	ID                string
	ShortRef          string
	BoardID           string
	Title             string
	Description       string
	ParentID          *string
	RequiredForParent bool
	Priority          int
	TaskType          string
	State             domain.State
	Now               time.Time
}

func (db *DB) CreateTask(ctx context.Context, in CreateTaskInput) error {
	_, err := db.SQL.ExecContext(
		ctx,
		`INSERT INTO tasks (id, short_ref, board_id, title, description, state, parent_id, required_for_parent, priority, task_type, rubric_passed, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 0, ?, ?)`,
		in.ID,
		in.ShortRef,
		in.BoardID,
		in.Title,
		strings.TrimSpace(in.Description),
		string(in.State),
		in.ParentID,
		boolToInt(in.RequiredForParent),
		in.Priority,
		in.TaskType,
		in.Now.UTC().Format(time.RFC3339),
		in.Now.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create task: %w", err)
	}
	return nil
}

func (db *DB) GetTask(ctx context.Context, taskID string) (domain.Task, error) {
	return db.getTaskBy(ctx, "id", taskID)
}

func (db *DB) GetTaskByShortRef(ctx context.Context, boardID, shortRef string) (domain.Task, error) {
	var (
		task              domain.Task
		parentID          sql.NullString
		archivedAtRaw     sql.NullString
		requiredForParent int
		rubricPassed      int
		updatedAtRaw      string
	)
	row := db.SQL.QueryRowContext(
		ctx,
		`SELECT id, short_ref, title, COALESCE(description, ''), state, archived_at, parent_id, required_for_parent, priority, task_type, rubric_passed, updated_at
		 FROM tasks WHERE board_id = ? AND short_ref = ?`,
		boardID,
		shortRef,
	)
	if err := row.Scan(&task.ID, &task.ShortRef, &task.Title, &task.Description, &task.State, &archivedAtRaw, &parentID, &requiredForParent, &task.Priority, &task.TaskType, &rubricPassed, &updatedAtRaw); err != nil {
		if err == sql.ErrNoRows {
			return domain.Task{}, fmt.Errorf("task %s not found", shortRef)
		}
		return domain.Task{}, fmt.Errorf("get task by short ref: %w", err)
	}
	if parentID.Valid {
		p := parentID.String
		task.ParentID = &p
	}
	if archivedAtRaw.Valid {
		if ts, err := time.Parse(time.RFC3339, archivedAtRaw.String); err == nil {
			task.ArchivedAt = &ts
		}
	}
	task.RequiredForParent = requiredForParent == 1
	task.RubricPassed = rubricPassed == 1
	if ts, err := time.Parse(time.RFC3339, updatedAtRaw); err == nil {
		task.UpdatedAt = ts
	}

	var children int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND archived_at IS NULL`, task.ID).Scan(&children); err != nil {
		return domain.Task{}, fmt.Errorf("count child tasks: %w", err)
	}
	task.IsParent = children > 0
	ready, err := db.AreRequiredChildrenRubricReady(ctx, task.ID)
	if err != nil {
		return domain.Task{}, err
	}
	task.ChildrenReady = ready
	return task, nil
}

func (db *DB) getTaskBy(ctx context.Context, column, value string) (domain.Task, error) {
	var (
		task              domain.Task
		parentID          sql.NullString
		archivedAtRaw     sql.NullString
		requiredForParent int
		rubricPassed      int
		updatedAtRaw      string
	)
	query := fmt.Sprintf(
		`SELECT id, short_ref, title, COALESCE(description, ''), state, archived_at, parent_id, required_for_parent, priority, task_type, rubric_passed, updated_at
		 FROM tasks WHERE %s = ?`,
		column,
	)
	row := db.SQL.QueryRowContext(
		ctx,
		query,
		value,
	)
	if err := row.Scan(&task.ID, &task.ShortRef, &task.Title, &task.Description, &task.State, &archivedAtRaw, &parentID, &requiredForParent, &task.Priority, &task.TaskType, &rubricPassed, &updatedAtRaw); err != nil {
		if err == sql.ErrNoRows {
			return domain.Task{}, fmt.Errorf("task %s not found", value)
		}
		return domain.Task{}, fmt.Errorf("get task: %w", err)
	}
	if parentID.Valid {
		p := parentID.String
		task.ParentID = &p
	}
	if archivedAtRaw.Valid {
		if ts, err := time.Parse(time.RFC3339, archivedAtRaw.String); err == nil {
			task.ArchivedAt = &ts
		}
	}
	task.RequiredForParent = requiredForParent == 1
	task.RubricPassed = rubricPassed == 1
	if ts, err := time.Parse(time.RFC3339, updatedAtRaw); err == nil {
		task.UpdatedAt = ts
	}

	var children int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND archived_at IS NULL`, task.ID).Scan(&children); err != nil {
		return domain.Task{}, fmt.Errorf("count child tasks: %w", err)
	}
	task.IsParent = children > 0

	ready, err := db.AreRequiredChildrenRubricReady(ctx, task.ID)
	if err != nil {
		return domain.Task{}, err
	}
	task.ChildrenReady = ready

	return task, nil
}

func (db *DB) ListTasks(ctx context.Context, state *domain.State, includeArchived bool) ([]domain.Task, error) {
	query := `SELECT id, short_ref, title, COALESCE(description, ''), state, archived_at, parent_id, required_for_parent, priority, task_type, rubric_passed, updated_at FROM tasks`
	args := []any{}
	clauses := make([]string, 0, 2)
	if !includeArchived {
		clauses = append(clauses, "archived_at IS NULL")
	}
	if state != nil {
		clauses = append(clauses, "state = ?")
		args = append(args, string(*state))
	}
	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += ` ORDER BY updated_at DESC, id ASC`

	rows, err := db.SQL.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}
	defer rows.Close()

	out := []domain.Task{}
	for rows.Next() {
		var (
			task              domain.Task
			parentID          sql.NullString
			archivedAtRaw     sql.NullString
			requiredForParent int
			rubricPassed      int
			updatedAtRaw      string
		)
		if err := rows.Scan(&task.ID, &task.ShortRef, &task.Title, &task.Description, &task.State, &archivedAtRaw, &parentID, &requiredForParent, &task.Priority, &task.TaskType, &rubricPassed, &updatedAtRaw); err != nil {
			return nil, fmt.Errorf("scan task row: %w", err)
		}
		if parentID.Valid {
			p := parentID.String
			task.ParentID = &p
		}
		if archivedAtRaw.Valid {
			if ts, err := time.Parse(time.RFC3339, archivedAtRaw.String); err == nil {
				task.ArchivedAt = &ts
			}
		}
		task.RequiredForParent = requiredForParent == 1
		task.RubricPassed = rubricPassed == 1
		if ts, err := time.Parse(time.RFC3339, updatedAtRaw); err == nil {
			task.UpdatedAt = ts
		}
		out = append(out, task)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate task rows: %w", err)
	}

	return out, nil
}

func (db *DB) UpdateTaskState(ctx context.Context, taskID string, to domain.State, now time.Time) error {
	res, err := db.SQL.ExecContext(ctx, `UPDATE tasks SET state = ?, updated_at = ? WHERE id = ?`, string(to), now.UTC().Format(time.RFC3339), taskID)
	if err != nil {
		return fmt.Errorf("update task state: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}

	return nil
}

func (db *DB) ArchiveTask(ctx context.Context, taskID string, now time.Time) error {
	res, err := db.SQL.ExecContext(ctx, `UPDATE tasks SET archived_at = ?, updated_at = ? WHERE id = ?`, now.UTC().Format(time.RFC3339), now.UTC().Format(time.RFC3339), taskID)
	if err != nil {
		return fmt.Errorf("archive task: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("task %s not found", taskID)
	}
	return nil
}

func (db *DB) HasActiveChildren(ctx context.Context, taskID string) (bool, error) {
	var children int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND archived_at IS NULL`, taskID).Scan(&children); err != nil {
		return false, fmt.Errorf("count child tasks: %w", err)
	}
	return children > 0, nil
}

func (db *DB) DeleteTaskCascadeRecords(ctx context.Context, taskID string) error {
	tx, err := db.SQL.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete task transaction: %w", err)
	}
	stmts := []string{
		`DELETE FROM task_leases WHERE task_id = ?`,
		`DELETE FROM task_transitions WHERE task_id = ?`,
		`DELETE FROM task_artifacts WHERE task_id = ?`,
		`DELETE FROM rubric_results WHERE task_id = ?`,
		`DELETE FROM tasks WHERE id = ?`,
	}
	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt, taskID); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("delete task cascade records: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete task transaction: %w", err)
	}
	return nil
}

func (db *DB) AreRequiredChildrenRubricReady(ctx context.Context, parentID string) (bool, error) {
	var totalRequired int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND required_for_parent = 1 AND archived_at IS NULL`, parentID).Scan(&totalRequired); err != nil {
		return false, fmt.Errorf("count required child tasks: %w", err)
	}
	if totalRequired == 0 {
		return true, nil
	}

	var ready int
	if err := db.SQL.QueryRowContext(ctx, `SELECT COUNT(*) FROM tasks WHERE parent_id = ? AND required_for_parent = 1 AND rubric_passed = 1 AND archived_at IS NULL`, parentID).Scan(&ready); err != nil {
		return false, fmt.Errorf("count ready child tasks: %w", err)
	}

	return ready == totalRequired, nil
}

func (db *DB) LatestArtifactVersion(ctx context.Context, taskID string, artifactType domain.ArtifactType) (int, error) {
	var version sql.NullInt64
	if err := db.SQL.QueryRowContext(
		ctx,
		`SELECT MAX(version) FROM task_artifacts WHERE task_id = ? AND artifact_type = ?`,
		taskID,
		string(artifactType),
	).Scan(&version); err != nil {
		return 0, fmt.Errorf("query latest artifact version: %w", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

func (db *DB) PresentArtifactTypes(ctx context.Context, taskID string) ([]domain.ArtifactType, error) {
	rows, err := db.SQL.QueryContext(ctx, `SELECT DISTINCT artifact_type FROM task_artifacts WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query present artifact types: %w", err)
	}
	defer rows.Close()

	out := []domain.ArtifactType{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan artifact type: %w", err)
		}
		out = append(out, domain.ArtifactType(raw))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate artifact types: %w", err)
	}

	return out, nil
}

type ArtifactSnapshot struct {
	MarkdownPath    string
	ContentSnapshot string
	Version         int
	CreatedAt       time.Time
}

func (db *DB) LatestArtifactSnapshot(ctx context.Context, taskID string, artifactType domain.ArtifactType) (ArtifactSnapshot, bool, error) {
	var (
		snapshot   ArtifactSnapshot
		createdRaw string
	)
	row := db.SQL.QueryRowContext(
		ctx,
		`SELECT markdown_path, content_snapshot, version, created_at
		 FROM task_artifacts
		 WHERE task_id = ? AND artifact_type = ?
		 ORDER BY version DESC
		 LIMIT 1`,
		taskID,
		string(artifactType),
	)
	if err := row.Scan(&snapshot.MarkdownPath, &snapshot.ContentSnapshot, &snapshot.Version, &createdRaw); err != nil {
		if err == sql.ErrNoRows {
			return ArtifactSnapshot{}, false, nil
		}
		return ArtifactSnapshot{}, false, fmt.Errorf("query latest artifact snapshot: %w", err)
	}
	if ts, err := time.Parse(time.RFC3339, createdRaw); err == nil {
		snapshot.CreatedAt = ts
	}
	return snapshot, true, nil
}
