---
name: commit-messages
description: Write commit messages for this project. Use when asked to commit, write a commit message, or check commit message style.
---

Commit messages follow the style of https://tangled.org/tangled.org/core/commits/master:

- All lowercase
- `component:` prefix, using the top-level directory or feature area as the component
- Imperative mood, present tense ("add", "fix", "remove", "allow", not "added", "fixes")
- No trailing period
- Short — one line, under ~60 characters
- No body unless the why is genuinely non-obvious

## Examples from this project

```
open-session: redirect to first exercise on start
calendar: fix move dialog header layout
training-cycles: default start date to today
templates: add kind selector to new exercise form
start-session-picker: fix select theming in dark mode
```

## Component naming

Use the feature area, not the file name:

| Changed area | Prefix |
|---|---|
| Open session page | `open-session:` |
| Session templates | `templates:` |
| Training cycles | `training-cycles:` |
| Exercise library | `library:` |
| Training log | `training-log:` |
| Dashboard | `dashboard:` |
| Calendar | `calendar:` |
| Auth / login | `auth:` |
| DB models / migrations | `db:` |
| CSS / static assets | `static:` |
| Multiple unrelated areas | `*:` |
