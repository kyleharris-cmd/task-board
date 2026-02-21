# task-board

Local-first task board for human + agent collaboration.

## Install (Recommended)

From this repository:

```bash
./scripts/install.sh
```

This will:

- Build `taskboard`
- Install `tb` into `~/.local/bin/tb`
- Add `~/.local/bin` to `PATH` in `~/.zshrc` if missing

Alternative:

```bash
make install
```

## Getting Started

From the `task-board` repo:

```bash
make install
source ~/.zshrc
tb --help
```

Then in any target project repo:

```bash
cd /path/to/your/repo
tb init
tb stat
```

## Quick Start (In Any Target Repo)

```bash
cd /path/to/your/repo
tb init
tb status
```

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
- `taskboard status` (alias: `taskboard stat`)

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
2. Create parent design task: `tb parent create --title "..."`.
3. Create child tasks from parent design: `tb child create --parent-id ... --title ... --files ...`.
4. Pick up child task: `tb pickup <child-id>`.
5. Execute lifecycle: `tb start`, `tb design`, `tb review`, `tb implement`, `tb finish`.

## Status Board (Read-First)

- Run `tb status` (or `tb stat`) to open a git-log style modal board.
- Parent/child tasks render as a tree with status icon + checkbox + owner + lease + state.
- Auto-refreshes every 5 seconds for async monitoring.
- Shows agent-active work via `active[AGENT]` lease marker.
- Keys: `j/k` move, `tab` toggle filter (`all`/`agent-active`), `space` collapse parent, `r` refresh, `q` quit.

## Actor Identity Rules

- Human workflow commands default to git identity (`user.name`, `user.email`).
- If missing, `tb` prompts to set git identity and asks repo-local vs global scope.
- Agent calls must explicitly declare actor identity (`--actor-type agent --actor-id ... --actor-name ...`).

## Development

```bash
make test
```
