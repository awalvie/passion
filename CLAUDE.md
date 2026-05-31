# Passion — Claude Code instructions

## Keep the README current

After implementing any feature, updating a handler, adding a config option, or changing the project structure, update [readme.md](readme.md) to reflect the change. Specifically:

- New config variables → add a row to the Configuration table
- New top-level directory or renamed one → update Project structure
- New developer workflow pattern → add or update [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
- Removed or renamed Make targets → update the Make targets table

Do not rewrite sections that aren't affected by the change.

## HTMX

Always set `hx-target="this"` on lazy-load divs inside forms. HTMX inherits `hx-target` from ancestor elements, which causes requests to swap into the wrong container.

## Git commits

Never commit unless the user explicitly asks. Implement changes, then stop — do not run `git add` or `git commit` on your own initiative.

## Code style

- No comments unless the WHY is non-obvious.
- No error handling for scenarios that can't happen — trust internal guarantees.
- Prefer editing existing files over creating new ones.

## Dropdowns / selects

Always use `class="input text-sm"` (or `class="input"`) on `<select>` elements. This inherits `--panel` background, `--border` border colour, and `--text` foreground from the site theme. Never leave a `<select>` with browser-default styling.

## UX / design decisions

When a UX issue or design question is ambiguous, ask before implementing. Don't pick a direction and build it.

For all design and UI work, refer to [docs/DESIGN.md](docs/DESIGN.md) first. It documents the site's design philosophy, colour tokens, typography scale, card/badge/button patterns, icon conventions, and layout rules. Follow those patterns rather than inventing new ones.

## When something is unclear, research first — then ask

Before making assumptions about what a user request means, read the relevant code. If the intent is still ambiguous after reading, ask a specific question. Do not guess and implement — a wrong assumption wastes the user's time and causes frustration. One targeted question is always better than a wrong implementation.

## Specialist agents

Agents live in `.claude/agents/` and each has persistent memory in `.claude/agent-memory/<name>/`. Use them proactively — don't wait to be asked.

### When designing (before implementation)

- **scout** — consult when designing a new feature or UX pattern. Ask "how do other training apps handle X?" to inform the approach before building.

### When implementing (during changes)

- **pixel** — consult when proposing template or CSS changes. Before finalizing a UI change, ask pixel whether it's consistent with the design system.
- **schema** — consult when proposing model or query changes. Before finalizing a DB change, ask schema whether it's safe and well-indexed.
- **copy** — consult when writing new user-facing text. Before finalizing labels, errors, or empty states, ask copy whether the tone is consistent.

### After implementation (post-change review)

Dispatch these on the changed files after completing a feature:

- **pixel** — on any template/CSS changes
- **qa** — on any handler or logic changes
- **simplify** — on any implementation (look for dead code, over-abstraction)
- **scribe** — on any structural change (sync docs with reality)
- **copy** — on any new user-facing text
- **schema** — on any model/query changes

Run the relevant subset in parallel based on what changed. Not every agent applies to every change.

### Self-improvement

Agents should actively look for ways to improve themselves. When an agent:

- Encounters a pattern it doesn't have a rule for → it should propose adding the rule to its own definition
- Gets corrected by the user → it should save feedback to its memory AND consider whether its checklist or workflow needs updating
- Notices its checklist is incomplete or outdated → it should propose an update to its `.claude/agents/<name>.md` file

Agents evolve with the project. Their definitions are living documents, not fixed specs.
