# Restyle

The client has the look of `passion-design/final/designs/direction` (`today-light.png`,
`today-dark.png`, `style.css`, `design.html`). It started with Today and now covers every
V2 screen: the player, Plan, History, Settings, sign-in, and the Library.

## Tokens

- `tokens.css` holds the direction palette for light and dark, under the `data-theme`
  attribute that `app.html` sets.
- `app.css` maps the tokens into Tailwind colours and shadows, adds a `dark:` variant tied
  to `data-theme`, paints `body` in `--ground` and `--ink`, and gives `.input` the field
  look.
- `app.html` sets `theme-color` to the two ground colours.
- How to use them is in [DEVELOPMENT.md](../../DEVELOPMENT.md#style-a-screen).

## Type

The scale is 12, 15, 20 and 32 px, at weights 600 to 800. `font-sans` asks for Figtree.
Figtree is not self-hosted yet, so the system font shows. Self-hosting it is a new asset and
waits for the owner's yes.

## Shared parts

`Button` (lime, apricot and surface pills), `NavBar`, `Menu`, `TabBar` and `FormError` use the
tokens. The tab bar has a rounded top, a lime pill behind the active icon, and the design's
icons: a peak for Today, a calendar for Plan, a stack for Library, a clock for History.

## Today

From the top:
- The topo lines and the soft glow behind the header (`Topo.svelte`, `topo.ts`).
- "Today", the date, the cycle pill ("Week 3 of 6", one bar per week, only when a cycle covers
  today), and Settings.
- The week strip (`WeekStrip.svelte`), Monday to Sunday. A done day shows a check, today is
  lime, a planned day is a raised circle. It does not take taps.
- One forest card per session today (`SessionCard.svelte`): the cycle and week or "One-off",
  the name, the number of sections, one dot per section at the design's heights, the
  equipment line, Preview and Start. A started session shows "Back to session" in apricot,
  a done one shows "Done".
- With no session, a card with "Plan a session".
- Other session and Open session, as two pills.

`+page.ts` loads the whole week and the cycles. `dates.ts` gains `mondayOf` and `cycleWeek`,
with tests.

Left out, because the data does not exist yet: the duration, the stats under "This cycle",
and the live trail with the "now" dot.
