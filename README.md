# task-board

Local-first task board for human + agent collaboration.

## Phase 1 Scope

This repository currently includes:

- Go CLI scaffold (`taskboard`)
- SQLite storage with embedded migrations
- Policy loader and validator for per-board governance
- State transition validator with readiness and rubric gates
- Audit/event writers for transitions, artifacts, and rubric results
- Unit tests for policy loading, workflow validation, and audit logging

## Current Commands

- `taskboard init --repo-root <path>`: create `.taskboard/board.db`, apply migrations, and write default policy.
- `taskboard policy validate --file <path>`: validate policy YAML.

## Default Repo Layout (target repos)

- `.taskboard/board.db`
- `.taskboard/policy.yaml`
- `.taskboard/tasks/<task-id>/...`

## Development

```bash
go test ./...
```
