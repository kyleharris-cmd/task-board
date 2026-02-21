# task-board

Local-first task board for human + agent collaboration.

## Phase 4 Scope

This repository now includes:

- Go CLI scaffold (`taskboard`)
- SQLite storage with embedded migrations
- Policy loader and validator for per-board governance
- State transition validator with readiness and rubric gates
- Audit/event writers for transitions, artifacts, and rubric results
- CLI commands for task creation, listing, claim/renew/release, transitions, artifact writes, rubric evaluations, and ready checks
- Interactive TUI (`taskboard tui`) with task list/detail panes and workflow key actions
- Local HTTP API server (`taskboard serve`) for agent integrations
- Hardening updates: stronger task ID generation and lease/release/readiness edge-case tests
- Unit/integration tests for policy, workflow, storage, app service, and HTTP API

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
- `taskboard serve --addr 127.0.0.1:7327`
- `taskboard start <task-id>`
- `taskboard design <task-id>`
- `taskboard review <task-id> --pass|--fail`
- `taskboard implement <task-id>`
- `taskboard finish <task-id>`
- `taskboard parent create --title ...`
- `taskboard parent design-edit <parent-id>`
- `taskboard child create --parent-id ... --title ... --files ...`
- `taskboard pickup <child-id>`

## HTTP API (v1)

- `GET /health`
- `GET /tasks?state=<state>`
- `POST /tasks`
- `POST /tasks/{id}/claim`
- `POST /tasks/{id}/renew`
- `POST /tasks/{id}/release`
- `POST /tasks/{id}/transition`
- `POST /tasks/{id}/artifacts`
- `POST /tasks/{id}/rubric`
- `POST /tasks/{id}/ready-check`

All mutating endpoints take a structured actor payload (`type`, `id`, `display_name`).

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
- `.taskboard/WORKFLOW.md`
- `.taskboard/PROMPTS/idea-to-design.txt`
- `.taskboard/PROMPTS/design-to-parent-and-children.txt`
- `.taskboard/PROMPTS/implement-child-task.txt`

## Workflow-First Usage

1. In target repo, run `tb init`.
2. Create parent design task: `tb parent create --title \"...\"`.
3. Create child tasks from parent design: `tb child create --parent-id ... --title ... --files ...`.
4. Pick up child task: `tb pickup <child-id>`.
5. Execute lifecycle: `tb start`, `tb design`, `tb review`, `tb implement`, `tb finish`.

## Actor Identity Rules

- Human workflow commands default to git identity (`user.name`, `user.email`).
- If missing, `tb` prompts to set git identity and asks repo-local vs global scope.
- Agent calls must explicitly declare actor identity (`--actor-type agent --actor-id ... --actor-name ...`).

## Development

```bash
go test ./...
```

## Binary Alias (`tb`)

Build the binary and create a short alias symlink:

```bash
mkdir -p bin
go build -o bin/taskboard ./cmd/taskboard
ln -sf taskboard bin/tb
```

You can then use either:

- `./bin/taskboard ...`
- `./bin/tb ...`
- `./tb ...` (repo-root wrapper script)
