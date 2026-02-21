# task-board

Local-first task board for human + agent collaboration.

## Phase 2 Scope

This repository now includes:

- Go CLI scaffold (`taskboard`)
- SQLite storage with embedded migrations
- Policy loader and validator for per-board governance
- State transition validator with readiness and rubric gates
- Audit/event writers for transitions, artifacts, and rubric results
- CLI commands for task creation, listing, claim/renew/release, transitions, artifact writes, rubric evaluations, and ready checks
- Unit tests for policy, workflow, storage, and service-level task lifecycle

## Current Commands

- `taskboard init --repo-root <path>`
- `taskboard policy validate --file <path>`
- `taskboard task create --title ... [--parent-id ...]`
- `taskboard task list [--state "Backlog"]`
- `taskboard task claim --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task renew --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task release --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task transition --id ... --to ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task ready-check --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard artifact add --id ... --type ... --content ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard rubric evaluate --id ... --pass --required-fields-complete --actor-type ... --actor-id ... --actor-name ...`

## Default Repo Layout (target repos)

- `.taskboard/board.db`
- `.taskboard/policy.yaml`
- `.taskboard/tasks/<task-id>/...`

## Development

```bash
go test ./...
```
