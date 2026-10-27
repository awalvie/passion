# Passion — Claude Code instructions

## Keep the docs current

After implementing a feature, changing a handler, adding a config option, or changing the project structure, update the doc it touches:

- A feature a person would notice → its section in [readme.md](readme.md)
- New config key or variable → the configuration table in [docs/DEPLOY.md](docs/DEPLOY.md)
- New or renamed top-level directory → "Where things go" in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
- New, removed or renamed Make target → the targets table in [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
- New developer workflow pattern → [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md)
- New token, shared component or layout rule → "Style a screen" below

Write the docs in plain, short English. Do not rewrite sections the change does not touch.

## Git commits

Never commit unless the user explicitly asks. Implement changes, then stop — do not run `git add` or `git commit` on your own initiative.

## Never put paid-programme content in the public catalog

`catalog/` is published under the repository's licence. Content from a paid programme is
not ours to redistribute, so it belongs in the private catalog repository only.

This applies to every file under `catalog/`:

- Do not copy a coach's prose, cues, or exercise notes. Write our own words.
- Do not reproduce a programme's session structure — its exercise selection, order, and
  weekly progression. Selection and arrangement are protected even when the individual
  numbers are not.
- Do not invent a name and attribute it to a real coach. That is worse than copying.
- Doses, sets, reps and rest times are facts and are fine to use.
- `source:` may name a coach or programme the method is publicly known by, as long as the
  words describing it are ours.

If a catalog entry cannot meet this, put it in `passion-private-catalog` instead.

## Code style

- No comments unless the WHY is non-obvious.
- No error handling for scenarios that can't happen — trust internal guarantees.
- Prefer editing existing files over creating new ones.

## Style a screen

Tokens are in `client/src/tokens.css`; each needs a light and a dark value.

- `bg-ground` page, `bg-surface` card, `bg-well` field. Text `text-ink`/`-2`/`-3`, lines `border-line`.
- Lime (`bg-tint`) marks Start, Log, a page's main button, today's date, the day a dragged session would land on and the current tab, never text. Apricot (`bg-live`) is a running session.
- Time off is hatched with `--hatch` stripes.
- Type: `text-xs` labels, `text-[15px]` body, `text-xl` card titles. `digits` for numbers, `reading` for the user's notes.
- The landing page (`/welcome`) has its own larger type for the headline and section titles.
- Fields and `<select>` use `.input`, except a lone free-text note: `bg-surface text-[15px] font-semibold placeholder:text-ink-3`.
- Reuse `client/src/lib/` parts (`Section`, `Segmented`, `Tabs`, `Sheet`, `Chart`…) before making new ones.
- A choice from a list is a `Select`: the phone's own picker, a themed list from `lg`. Days are picked on a `DayPicker`.
- Controls are at least 44 px. List rows use `list-row`. A row that opens a page ends with a chevron.
- Tabs sit at the bottom on a phone, in a left rail from `md`. Today, Plan, a cycle, Journal and Progress go wide from `lg`.
- From `lg` Plan's views and a cycle put their list or calendar beside a 330 px pane. A row or a day only picks; the pane acts on it (`DayPane`). New cycle has no rail, and shows its preview in the pane from step 3.
- A `Sheet` rises from the bottom on a phone and is a centred dialog from `lg`; `wide` gives it room for two columns.
- The session builder has no rail. From `lg` a pane beside the session shows what is picked; from `xl` the library is a third column on the left. Below `lg` both open as sheets.

## Rules the code does not show

- Change a running session through `openRun`, never `request`; it saves on the phone first.
- Exercises and sessions are retired, never deleted. Unfinished runs count in no chart.
- After changing a handler, run `make openapi`; CI fails on a stale `swagger.json`.
- Tables with `updated_at` need the `touch_updated_at()` trigger.
- The server has four dependencies. Do not add a fifth without a strong reason.

## UX / design decisions

When a UX issue or design question is ambiguous, ask before implementing. Don't pick a direction and build it.

For all design and UI work, follow "Style a screen" above rather than inventing new tokens or components.

## When something is unclear, research first — then ask

Before making assumptions about what a user request means, read the relevant code. If the intent is still ambiguous after reading, ask a specific question. Do not guess and implement — a wrong assumption wastes the user's time and causes frustration. One targeted question is always better than a wrong implementation.

## Specialist agents

Agents live in `.claude/agents/` and each has persistent memory in `.claude/agent-memory/<name>/`. Use them proactively — don't wait to be asked.

### When designing (before implementation)

- **scout** — consult when designing a new feature or UX pattern. Ask "how do other training apps handle X?" to inform the approach before building.
- **librarian** — consult when adding exercises, reviewing catalog gaps, or planning training content. Ask "what exercises should I add for X?" or "is the catalog missing anything for Y?"

### When implementing (during changes)

- **pixel** — consult when proposing component or CSS changes. Before finalizing a UI change, ask pixel whether it's consistent with the design system.
- **copy** — consult when writing new user-facing text. Before finalizing labels, errors, or empty states, ask copy whether the tone is consistent.

### After implementation (post-change review)

Dispatch these on the changed files after completing a feature:

- **pixel** — on any component/CSS changes
- **qa** — on any handler or logic changes
- **simplify** — on any implementation (look for dead code, over-abstraction)
- **scribe** — on any structural change (sync docs with reality)
- **copy** — on any new user-facing text
- **librarian** — on any exercise model or YAML import changes

Run the relevant subset in parallel based on what changed. Not every agent applies to every change.

### Self-improvement

Agents should actively look for ways to improve themselves. When an agent:

- Encounters a pattern it doesn't have a rule for → it should propose adding the rule to its own definition
- Gets corrected by the user → it should save feedback to its memory AND consider whether its checklist or workflow needs updating
- Notices its checklist is incomplete or outdated → it should propose an update to its `.claude/agents/<name>.md` file

Agents evolve with the project. Their definitions are living documents, not fixed specs.
