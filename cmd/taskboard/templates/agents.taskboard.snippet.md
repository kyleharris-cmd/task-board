## Taskboard Protocol

This repository uses `tb` for planning and execution tracking.

- Initialize if needed: `tb init`
- Store canonical design on parent tasks (`parent_design` artifact)
- Create child tasks for implementable units with clear file context
- Use workflow commands: `tb start`, `tb design`, `tb review`, `tb implement`, `tb finish`
- Use `tb pickup <child-id>` to generate implementation brief for fresh agents
