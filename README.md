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

## Interface Model

Task Board supports three interaction paths. Use the one that matches your actor and intent:

- Human primary interface: `tb stat` (interactive TUI, first-class path for day-to-day task work).
- Agent/automation CLI: `tb ...` subcommands (`task`, `parent`, `child`, lifecycle commands, `tb codex`).
- HTTP API integrations: `taskboard serve` + `/tasks` endpoints.

Rule of thumb:

- Humans should primarily work in `tb stat`.
- Agents and scripts should primarily use non-interactive CLI or API calls.

Canonical workflow states:

1. `Scoping`
2. `Design`
3. `In Progress`
4. `PR`
5. `Complete`

## Quick Start (In Any Target Repo)

```bash
cd /path/to/your/repo
tb init
tb status
```

For human usage, prefer `tb stat`/`tb status` over individual subcommands.

## Human Workflow (Primary)

Use `tb stat` for your normal async coordination loop:

1. Open board: `tb stat`.
2. Scan status/owner/lease at a glance.
3. Open a task with `Enter`, edit, create parent/child tasks, and move state.
4. Quit and re-open anytime for async status checks.

`tb status` and `tb stat` are aliases.

### `tb stat` View and Navigation

- Parent/child tasks render as a tree with status icon + checkbox + owner + lease + state.
- Auto-refreshes every 5 seconds.
- Shows agent-active work via `active[AGENT]` lease marker.
- `tb stat` is editable by default for human workflow.
- Use `tb stat --read-only` when you only want monitoring with no in-panel writes.
- Keys:
- `j/k`: move selection.
- `Enter`: open highlighted task editor.
- `Tab`: toggle filter (`all`/`agent-active`).
- `Space`: collapse/expand parent.
- `r`: manual refresh.
- `?`: open command palette/help.
- `q`: quit.

### `tb stat` Command Mode (Task Actions)

- `:(e)dit <row-number>`: edit selected row design artifact (`:e1` and `:edit1` supported).
- `:cp [optional title]`: create parent task from editor template.
- `:cc [optional title]`: create child task from current row context.
- `:s|:state|:to <state>`: transition highlighted task state.
- `Tab` after `:s`/`:state`/`:to`: cycle policy-allowed next states, `Enter` to apply.
- `:a|:archive`: archive highlighted task.
- `:d !|:delete !`: permanently delete highlighted task (explicit confirmation required).

### Inline Editor Behavior

- First line template for `:cp`/`:cc`: `Title: ...` (prefix is stripped on save).
- `tab`: open/cycle directory-level path suggestions.
- `j/k`: move suggestion selection.
- `enter`: insert selected suggestion (directories drill down).
- `esc` or `ctrl+s`: save and close.
- `ctrl+q`: cancel without saving.
- Optional vim mode: `TB_KEYMAP=vim tb stat`.
  - Status view: `gg`/`G` jump, `h`/`l` collapse/expand parent.
  - Editor view: `NORMAL`/`INSERT` flow (`i/a/o` insert, `Esc` normal, `Enter` save).

## Agent Workflow (CLI/Automation)

For Codex or other agents, prefer explicit CLI flow and machine-readable output:

1. Ensure board exists: `tb init`.
2. Ask for the next actionable task (`tb next --json`).
3. Read board snapshot (`tb codex` for human-readable, `tb codex --json` for machine-readable).
4. Pick up and move work with lifecycle commands (`tb pickup`, `tb start`, `tb design`, `tb review`, `tb implement`, `tb done`) or lower-level `tb task ...` commands.
4. Persist context/design artifacts via `tb artifact add` or TUI edit flows if intentionally interactive.

Agent identity must be explicit for mutating CLI/API calls:

- `--actor-type agent --actor-id ... --actor-name ...`

## CLI Reference (Non-Interactive)

Most commands below are useful for scripting, automation, or agent integrations. Human users should primarily work from `tb stat`.

- `taskboard init --repo-root <path>`
- `taskboard policy validate --file <path>`
- `taskboard policy migrate --file <path> [--dry-run]`
- `taskboard task create --title ... [--parent-id ...]`
- `taskboard task list [--state "Scoping"]`
- `taskboard task claim --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task renew --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task release --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task transition --id ... --to ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard task archive --id ...`
- `taskboard task delete --id ... --force`
- `taskboard task ready-check --id ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard artifact add --id ... --type ... --content ... --actor-type ... --actor-id ... --actor-name ...`
- `taskboard rubric evaluate --id ... --pass --required-fields-complete --actor-type ... --actor-id ... --actor-name ...`
- `taskboard tui --actor-type ... --actor-id ... --actor-name ...`
- `taskboard serve --addr 127.0.0.1:7327`
- `taskboard start <task-id>`
- `taskboard design <task-id>`
- `taskboard review <task-id> --pass|--fail`
- `taskboard implement <task-id>`
- `taskboard done <task-id>`
- `taskboard finish <task-id>`
- `taskboard parent create --title ...`
- `taskboard parent design-edit <parent-id>`
- `taskboard child create --parent-id ... --title ... --files ...`
- `taskboard pickup <child-id>`
- `taskboard status` (alias: `taskboard stat`)
- `taskboard codex [--json]`
- `taskboard next [--json]`

## HTTP API (v1)

- `GET /health`
- `GET /tasks?state=<state>&include_archived=1`
- `POST /tasks`
- `POST /tasks/{id}/claim`
- `POST /tasks/{id}/renew`
- `POST /tasks/{id}/release`
- `POST /tasks/{id}/transition`
- `POST /tasks/{id}/archive`
- `POST /tasks/{id}/artifacts`
- `POST /tasks/{id}/rubric`
- `POST /tasks/{id}/ready-check`
- `DELETE /tasks/{id}?force=1`

All mutating endpoints take a structured actor payload (`type`, `id`, `display_name`).

## Codex Session Access

- `tb init` now ensures `AGENTS.md` exists and includes the taskboard protocol snippet.
- Use `tb codex` for a quick non-interactive snapshot (active checkouts + design-queue tasks).
- Use `tb codex --json` for machine-readable output in agent sessions.

## Task References

- Every task gets a short human-friendly reference (`T-1`, `T-2`, ...).
- Commands accept either short refs or full internal IDs.
- `tb task list` shows both short ref and internal ID.

## Actor Identity Rules

- Human workflow commands default to git identity (`user.name`, `user.email`).
- If missing, `tb` prompts to set git identity and asks repo-local vs global scope.
- Agent calls must explicitly declare actor identity (`--actor-type agent --actor-id ... --actor-name ...`).
- Lease gating is policy-driven and can be actor-specific via `lease_required_by_actor` in `.taskboard/policy.yaml`.

## Default Repo Layout (target repos)

- `.taskboard/board.db`
- `.taskboard/policy.yaml`
- `.taskboard/tasks/<task-id>/...`
- `.taskboard/WORKFLOW.md`
- `.taskboard/PROMPTS/idea-to-design.txt`
- `.taskboard/PROMPTS/design-to-parent-and-children.txt`
- `.taskboard/PROMPTS/implement-child-task.txt`

## Development

```bash
make test
```
