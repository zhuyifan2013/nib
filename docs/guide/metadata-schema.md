---
id: guide-metadata-schema
title: Frontmatter metadata schema
type: doc
status: active
owner: markdash
tags: [guide, schema]
created_at: 2026-09-15
updated_at: 2026-09-16
---

# Frontmatter metadata schema

Every Markdown document tracked by the dashboard must start with YAML frontmatter.

## Common fields

| Field | Required | Description |
|---|---|---|
| `id` | yes | Unique repo-wide, kebab-case |
| `title` | yes | Display title |
| `type` | yes | `project` / `task` / `decision` / `risk` / `test` / `note` / `doc` |
| `status` | yes | See per-type enums |
| `owner` | no | Person or agent |
| `tags` | no | Array of strings |
| `created_at` | yes | `YYYY-MM-DD` |
| `updated_at` | yes | Update on every substantive change |
| `related` | no | Array of related document ids, used for backlinks |

## Type-specific fields

### task
- `status`: `backlog | todo | in_progress | blocked | review | done | cancelled`
- `priority`: `low | medium | high | urgent`
- `due`: `YYYY-MM-DD`
- `blocked_by`: array of strings (reasons or dependency ids)
- `project`: related project id

### decision
- `status`: `proposed | accepted | deprecated | superseded`
- `supersedes`: id of the decision being replaced (optional)

### risk
- `status`: `open | mitigated | closed`
- `probability`: `low | medium | high`
- `impact`: `low | medium | high`
- `mitigation`: short description (optional)

### test
- `status`: `planned | passing | failing | skipped`
- `kind`: `unit | integration | e2e | manual`

### project
- `status`: `active | on_hold | done | archived`
- `health`: `green | yellow | red`

## Example

```yaml
---
id: login-flow
title: Login flow
type: task
status: in_progress
priority: high
owner: ai
due: 2026-09-20
tags: [auth, frontend]
related: [decision-session-strategy]
blocked_by: []
updated_at: 2026-09-16
---
```

## Do not mix the four dimensions

- type (what it is, mutually exclusive, decides the view): project / task / decision / risk / test / note / doc
- status (its stage, each type has an enum): e.g. task todo / in_progress / blocked / done
- priority (how important, task/risk only): urgent / high / medium / low, shown as P0-P3 on the dashboard
- tags (free-form topic classification, multiple, filtering only): e.g. cli / web / ci; never use a tag for status or priority

Rule: never create tags named like `todo` or `in_progress`; state lives only in `status`.

## Dashboard display conventions

- backlog = unscheduled idea; todo = committed this round and ready to pick up; both count as "to do" on the overview.
- Status shows as a human-readable label plus a colored dot; priority shows as P0 Urgent / P1 High / P2 Medium / P3 Low.
- Types are distinguished by a left color bar (task blue, decision purple, risk red, test green).

## Rules

- Never hand-write the generated block in `docs/index.md` or anything under `.markdash/cache/`.
- File name should match the id: `docs/tasks/login-flow.md`.
- Links always use ids, not relative paths, so files can be moved.
