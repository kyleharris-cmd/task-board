# task-board

Local-first task board for human + agent collaboration.

## Phase 3 Scope

This repository now includes:

- Go CLI scaffold (`taskboard`)
- SQLite storage with embedded migrations
- Policy loader and validator for per-board governance
- State transition validator with readiness and rubric gates
- Audit/event writers for transitions, artifacts, and rubric results
- CLI commands for task creation, listing, claim/renew/release, transitions, artifact writes, rubric evaluations, and ready checks
- Interactive TUI (`taskboard tui`) with task list/detail panes and workflow key actions
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
- `taskboard tui --actor-type ... --actor-id ... --actor-name ...`

## TUI Keys

- `j/k` or arrows: move selection
- `r`: refresh tasks
- `c`: claim selected task
- `n`: renew selected lease
- `u`: release selected lease
- `>`/`l`: transition to next lifecycle state
- `<`/`h`: transition to previous lifecycle state
- `x`: run ready-check on selected task
- `a`: add `context` artifact (inline input)
- `d`: add `design` artifact (inline input)
- `b`: add `rubric_review` artifact (inline input)
- `q`: quit

## Default Repo Layout (target repos)

- `.taskboard/board.db`
- `.taskboard/policy.yaml`
- `.taskboard/tasks/<task-id>/...`

## Development

```bash
go test ./...
```
