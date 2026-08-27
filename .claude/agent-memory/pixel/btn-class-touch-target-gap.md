---
name: btn class touch target gap
description: .btn-primary/.btn-accent/.btn-ghost are colour skins with no size; 44px is opt-in via .btn and is only required for mid-session primary actions
type: feedback
---

`static/passion.css` puts `min-height: 2.75rem` (44px) on `.btn` alone. `.btn-primary`,
`.btn-accent` and `.btn-ghost` set background, colour and border only — no size. This is
**by design**, not a gap.

**Settled 2026-08-27 — do not re-propose the "fix".** The 2026-08-27 audit reported this as
~70 violations and recommended moving `min-height` onto the tier classes. That was wrong on
both counts, and the recommendation was rejected after being tested:

- The house button pattern is `rounded-md px-4 py-2 text-sm font-medium` + a tier class, used
  ~240 times in `templates/`. Only ~13 sites pair it with `.btn`. So `.btn` is the rarely-used
  class, and a rule documented on it was never "violated" by the other 227.
- Roughly half of all tier-class call sites are **deliberately** small: 28px table row icons,
  26px inline pills, `flex-1 px-3 py-1.5 text-xs` segmented-control halves, `text-[10px]`
  micro chips.
- Injecting `min-height: 2.75rem` into `.btn-ghost` was measured on `/templates`: row icons
  jumped 28px → 44px and destroyed table density. It also **defeated explicit local
  overrides** — a button with `min-h-[1.75rem]` still computed to 44px, because the
  stylesheet class and the Tailwind utility have equal specificity and the stylesheet wins.
  Five call sites rely on that override, so there would be no opt-out short of `!important`.

The 44px floor is already enforced where it matters — `.run-session-actions` and the run
player's own controls set it directly in `passion.css`.

**How to apply**: `docs/DESIGN.md` now scopes the rule to "any button that is the primary
action in a mid-session flow" (run transport, run completion, Start, Continue) — audit that
list, not every button. Everywhere else, a 28-38px button is correct. Never add sizing to a
tier class. When a small button sits next to a `.btn` one and the mismatch looks wrong, fix it
locally — that was the one real finding here (`dashboard.html`'s `Continue →` at 44px beside
`Discard` at 26px, since corrected by dropping `.btn`).
