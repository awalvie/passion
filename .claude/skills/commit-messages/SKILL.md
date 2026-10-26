---
name: commit-messages
description: Write commit messages for this project. Use when asked to commit, write a commit message, or check commit message style.
---

Commit messages follow Conventional Commits (https://www.conventionalcommits.org/en/v1.0.0/):

- `type(scope): subject`, with the feature area as the scope
- Lowercase, except names such as `RunField` or `NEXT`
- Imperative mood, present tense ("add", "fix", "remove", "allow", not "added", "fixes")
- No trailing period
- Short — one line, under ~60 characters
- No body unless the why is genuinely non-obvious
- A change that breaks existing data or the API gets `!` before the colon: `feat(api)!: …`

## Types

| Type | Use for |
|---|---|
| `feat` | Something a person can now do or see |
| `fix` | Something that behaved wrong and now behaves right |
| `refactor` | Code changes that change no behaviour |
| `perf` | Faster or lighter, same behaviour |
| `test` | Tests only |
| `docs` | Docs and readme only |
| `ci` | CI and deploy workflows |
| `chore` | Agents, CLAUDE.md, tooling, dependencies |

## Examples from this project

```
feat(run): add 30 s to a rest
feat(sets): remove a set with its ×
fix(run): count only steps done, not skipped
feat(cycle): drag a session to another day
fix(catalog): stop timing warm-ups and tempo reps
```

## Scope naming

Use the feature area, not the file name:

| Changed area | Scope |
|---|---|
| A running session and its step pages | `run` |
| The session screen while it runs | `session` |
| The set card | `sets` |
| The climb card | `climbs` |
| Today tab | `today` |
| Plan tab | `plan` |
| One cycle | `cycle` |
| Session templates | `templates` |
| Exercise library | `library` |
| History tab | `history` |
| The Progress page | `progress` |
| Profile and settings | `profile` |
| Shared client parts | `client` |
| Handlers | `api` |
| Queries and migrations | `db` |
| Catalog files | `catalog` |
| Readme | `readme` |
| Agents, CLAUDE.md | `agents`, `claude` |
| Deploy | `deploy` |
