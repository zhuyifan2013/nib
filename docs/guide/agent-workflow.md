---
id: guide-agent-workflow
title: Agent workflow
type: doc
status: active
owner: markdash
tags: [guide, workflow]
created_at: 2026-09-15
updated_at: 2026-09-16
---

# Agent workflow

## Before starting

1. Read `docs/index.md` to understand project status, current focus, and the document map.
2. Based on the task type, read only the documents the index points to:
   - Building a feature / fixing a defect -> relevant `tasks/`, `decisions/`, `tests/`
   - Choosing an approach -> relevant `decisions/`, `risks/`
   - Releasing / troubleshooting -> `operations/`
3. Use `git log` / `git status` to understand recent changes; do not assume state from memory.
4. When starting a tracked task, set its `status` to `in_progress`.

## During work

- An approach is chosen -> create a `decisions/` document (context, alternatives, decision, impact).
- A hazard or external dependency is discovered -> create / update a `risks/` document.
- Blocked -> set task `status: blocked` and fill `blocked_by`.
- Tentative conclusions, meeting notes, unconfirmed items -> write to `notes/` first, not straight into formal documents.

## Before finishing (required)

1. Read `documentation-policy.md` and classify the work as:
   `no-knowledge-change` / `progress-change` / `decision-change` /
   `risk-change` / `test-change` / `operations-change` / `stale-doc-found`.
2. For every class except `no-knowledge-change`, update the corresponding Markdown file.
3. Update `updated_at`, progress, and verification details on affected documents.
4. Run `markdash validate`; fix reported errors before delivering.
5. If `markdash serve` is running, the dashboard refreshes automatically; otherwise no manual sync is needed (validate refreshes the cache).
