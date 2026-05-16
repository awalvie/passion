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

## Code style

- No comments unless the WHY is non-obvious.
- No error handling for scenarios that can't happen — trust internal guarantees.
- Prefer editing existing files over creating new ones.

## Dropdowns / selects

Always use `class="input text-sm"` (or `class="input"`) on `<select>` elements. This inherits `--panel` background, `--border` border colour, and `--text` foreground from the site theme. Never leave a `<select>` with browser-default styling.

## UX / design decisions

When a UX issue or design question is ambiguous, ask before implementing. Don't pick a direction and build it.

For all design and UI work, refer to [docs/DESIGN.md](docs/DESIGN.md) first. It documents the site's design philosophy, colour tokens, typography scale, card/badge/button patterns, icon conventions, and layout rules. Follow those patterns rather than inventing new ones.
