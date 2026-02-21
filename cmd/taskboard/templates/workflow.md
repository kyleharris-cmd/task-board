# Taskboard Workflow

This repository uses taskboard for human + agent delivery workflow.

## Core Loop

1. `tb start <task-id>`: claim task + add context + move to Context Added.
2. `tb design <task-id>`: add design details + move to Design Drafted.
3. `tb review <task-id>`: add rubric notes + evaluate + ready-check.
4. `tb implement <task-id>`: claim/renew + move to In Progress.
5. `tb finish <task-id>`: add implementation/test/docs artifacts + move to Done.

## Parent / Child

- Parent task stores canonical `parent_design` artifact.
- Child tasks reference parent and include `child_design` + context/files.
- Use `tb pickup <child-id>` to generate a complete implementation brief.
