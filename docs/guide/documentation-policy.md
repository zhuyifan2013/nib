---
id: guide-documentation-policy
title: Documentation update policy
type: doc
status: active
owner: markdash
tags: [guide, policy]
created_at: 2026-09-15
updated_at: 2026-09-16
---

# Documentation update policy

Single decision rule:

> Without reading this chat, could the next agent continue the work, verify results, or troubleshoot using only code and docs?
> If not -> update. If yes -> do not create documentation noise.

## Must update

- Requirements, scope, or acceptance criteria change
- Task status changes: created / started / blocked / resumed / done / cancelled
- A technical or product decision is made; an old decision is overturned or replaced
- A risk, external dependency, constraint, or compatibility issue is discovered
- A test plan, result, defect, or verification conclusion is added
- Environment, deployment, configuration, build, or run instructions change
- An existing document disagrees with code / reality (update it or mark `status: stale`)

## Do not update

- Merely reading code, searching files, or understanding current state
- Temporary debugging with no reusable conclusion
- Abandoned experiments with no future value
- Pure formatting, comments, or no-impact renames
- Information already accurately documented with nothing new

## Event -> document

| Event | Action |
|---|---|
| New task arrives | Create / update `tasks/*.md` |
| Work starts | `status: in_progress` |
| Blocked | `status: blocked` + `blocked_by` |
| Task finished | `status: done` + result and verification |
| Approach chosen | Create `decisions/*.md` |
| Decision replaced | Old -> `status: superseded`, link new via `supersedes` |
| Risk discovered | Create / update `risks/*.md` |
| Test conclusion | Update `tests/*.md` |
| Run/deploy change | Update `operations/*.md` |
| Research / meeting notes | Write `notes/*.md` |
| Tentative note adopted | Promote from notes into the proper folder |

## Threshold

Only record information that affects later decisions, execution, testing, handoff, or troubleshooting.
Prefer fewer documents, but state fields (status, blocked_by, result, verification) must not be missing.
