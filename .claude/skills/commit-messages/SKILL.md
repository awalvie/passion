---
name: commit-messages
description: Write commit messages for this project. Use when asked to commit, write a commit message, or check commit message style.
---

Commit messages follow the style of https://tangled.org/tangled.org/core/commits/master:

- Lowercase, except names such as `RunField` or `NEXT`
- `component:` prefix, using the top-level directory or feature area as the component
- Imperative mood, present tense ("add", "fix", "remove", "allow", not "added", "fixes")
- No trailing period
- Short — one line, under ~60 characters
- No body unless the why is genuinely non-obvious

## Examples from this project

```
run: add 30 s to a rest
sets: remove a set with its ×
climbs: log each go instead of an outcome
cycle: drag a session to another day
catalog: stop timing warm-ups and tempo reps
```

## Component naming

Use the feature area, not the file name:

| Changed area | Prefix |
|---|---|
| A running session and its step pages | `run:` |
| The session screen while it runs | `session:` |
| The set card | `sets:` |
| The climb card | `climbs:` |
| Today tab | `today:` |
| Plan tab | `plan:` |
| One cycle | `cycle:` |
| Session templates | `templates:` |
| Exercise library | `library:` |
| History tab | `history:` |
| Profile and settings | `profile:` |
| Shared client parts | `client:` |
| Handlers | `api:` |
| Queries and migrations | `db:` |
| Catalog files | `catalog:` |
| Docs, readme | `docs:`, `readme:` |
| Agents, CLAUDE.md | `agents:`, `claude:` |
| CI, deploy | `ci:`, `deploy:` |
