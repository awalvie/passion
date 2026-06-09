# Passion — Design reference

## Philosophy

Clean, information-dense, low-chrome. The UI should feel like a well-designed notebook — structured, readable, and out of the way of the data. No decorative elements. Density is preferred over whitespace for repeat-use screens (logs, calendars). Every piece of UI should earn its space.

Key principles:
- **Data over decoration**: numbers and labels, not illustrations or gradients.
- **Progressive disclosure**: show the summary; let the user drill in. Don't surface everything at once.
- **Consistent visual language**: use the same card, badge, and icon patterns everywhere so the app feels unified.
- **Icon as label**: in compact contexts, a lucide icon (moon, zap, flame) can replace a text label entirely. Bold the number beside it.

---

## Color tokens

All colors come from CSS variables. Never hardcode hex values. Use semantic tokens:

| Token | Use |
|---|---|
| `var(--text)` | Primary body text |
| `var(--muted)` | Secondary text, hints, placeholders, labels |
| `var(--bg)` | Page background |
| `var(--panel)` | Card / component background |
| `var(--border)` | Borders and hairline dividers |
| `var(--card-muted)` | Muted background inside cards (stat boxes, chips) |
| `var(--accent)` | Highlights, active states, selected, progress fill |
| `var(--accent-bg)` | Subtle accent tint (badge backgrounds, focus chips) |
| `var(--accent-border)` | Focus outlines, accent borders |
| `var(--destructive)` | Delete, error, danger |

**Dynamic template colours** (e.g. `{{ .Color }}`): use for left-border accents on list cards. For subtle background tints use `color-mix(in srgb, {{ .Color }} 4%, var(--panel))`.

**Climbing tick palette** (`--tick-*`): a family of hue-specific bg/fg pairs for grade and outcome badges. Never hardcode the hex values — always use the token pair so dark mode inverts correctly.

| Token pair | Use |
|---|---|
| `--tick-green-bg` / `--tick-green-fg` | Onsight, sent, outdoor venue |
| `--tick-amber-bg` / `--tick-amber-fg` | Flash, board venue |
| `--tick-red-bg` / `--tick-red-fg` | Redpoint, free solo rope style |
| `--tick-sky-bg` / `--tick-sky-fg` | Hangdog |
| `--tick-violet-bg` / `--tick-violet-fg` | Lead rope style |
| `--tick-blue-bg` / `--tick-blue-fg` | Top-rope / follow rope style |
| `--tick-cyan-bg` / `--tick-cyan-fg` | Auto-belay rope style |
| `--tick-star` | Star rating active state |
| `--tick-sent-accent` | Sent tick left-border accent |
| `--tick-working-accent` | Working/attempt tick left-border accent |

---

## Typography

Font: Inter (weights 400, 500, 600, 700).

| Size | Class | Use |
|---|---|---|
| 1.25rem | `text-xl font-bold` | Page titles (h1) |
| 0.875rem | `text-sm font-semibold` | Card titles, section headings |
| 0.875rem | `text-sm` | Body text, form labels |
| 0.75rem | `text-xs` | Secondary metadata, hints |
| 0.6875rem | `text-[11px]` | Inline badges, group dividers, metric labels |
| 0.625rem | `text-[10px]` | Stat box labels, table headers |

**Hierarchy pattern**: one bold title line, then one or two secondary lines in `text-[11px]` muted. Avoid going deeper than three visual tiers per card.

**Numbers in stats**: `text-2xl font-bold tabular-nums` for hero numbers; `font-medium tabular-nums` for inline metric values. Always pair with a muted unit or label.

---

## Cards

```html
<!-- Standard card -->
<div class="card card-pad"> … </div>

<!-- List item with colour accent -->
<div class="card" style="border-left: 4px solid {{ .Color }}; padding: 0.625rem 0.875rem;">
```

- `.card`: border + rounded corners.
- `.card-pad`: adds `padding: 1rem`. Use for standalone panels.
- List entry cards: use `padding: 0.625rem 0.875rem` (not card-pad) with a 4px left border.
- Stat boxes inside panels: `rounded-lg px-3 py-2.5` with `background: var(--card-muted)`.

**Action buttons on cards**: use `group` + `opacity-0 group-hover:opacity-100 transition-opacity` to reveal edit/delete icons on hover. Keep them at `p-1.5` with `color: var(--muted)`.

---

## Badges and chips

**Tag/focus chip** (accent colour):
```html
<span class="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium"
      style="background:var(--accent-bg);color:var(--accent)">Label</span>
```

**Muted chip** (location, duration, count):
```html
<span class="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px]"
      style="background:var(--card-muted);color:var(--muted)">
  <i data-lucide="map-pin" style="width:0.6rem;height:0.6rem" aria-hidden="true"></i>
  Value
</span>
```

**Inline metric** (icon-as-label pattern for compact contexts):
```html
<span class="inline-flex items-center gap-1 text-[11px]" style="color:var(--muted)">
  <i data-lucide="moon" style="width:0.6rem;height:0.6rem" aria-hidden="true"></i>
  <span class="font-medium tabular-nums" style="color:var(--text)">4</span>/5
</span>
```

Don't stack more than ~4 chips in one row. If metadata overflows, demote the least important field.

---

## Icons (Lucide)

| Size | Style attribute | Use |
|---|---|---|
| 0.6rem × 0.6rem | Badge icons, inline metric labels |
| 0.75rem × 0.75rem | Small action icons, status dots |
| 0.875rem × 0.875rem | Button icons |
| 1rem × 1rem | Header / nav icons |
| 1.25rem × 1.25rem | Hero / empty-state icons |

Always `aria-hidden="true"` on decorative icons. Add `aria-label` on icon-only buttons.

**Semantic icon conventions**:
- `moon` → sleep score
- `zap` → energy / Flash outcome
- `flame` → RPE / effort / Attempt outcome
- `check-circle` → sent / confirmed completion
- `dumbbell` → exercise count
- `mountain` → climbing ticks
- `map-pin` → location
- `clock` → duration
- `pencil` → edit
- `trash-2` → delete
- `plus` → add action
- `copy-plus` → log again / duplicate a prior entry
- `sliders-horizontal` → refine / advanced options

---

## Layout

**Page container**: `.passion-container` (max-width 72rem, padded). Use `.passion-container-wide` (max-width 84rem) for calendar/data-heavy pages.

**Two-column content layout** (most detail pages):
```html
<div class="md:grid md:grid-cols-[3fr_2fr] md:gap-6 md:items-start space-y-5 md:space-y-0">
  <div><!-- main content --></div>
  <div class="md:sticky md:top-6 space-y-4"><!-- sidebar panels --></div>
</div>
```

**Week group dividers** (training log, history):
```html
<div class="pt-4 pb-1 flex items-center gap-3">
  <div class="text-[11px] font-bold muted uppercase tracking-widest shrink-0">Week label</div>
  <div class="flex-1 h-px" style="background:var(--border)"></div>
</div>
```

**Empty states**: centered card, icon at 30% opacity, heading + subtext.
```html
<div class="card card-pad text-center py-12">
  <i data-lucide="notebook-pen" style="width:2rem;height:2rem;opacity:0.3;margin:0 auto 0.75rem"></i>
  <p class="text-sm font-semibold">Nothing here yet</p>
  <p class="mt-1 text-xs muted">Helper copy.</p>
</div>
```

---

## Buttons

- `.btn-primary`: dark background, white text — primary CTA per page.
- `.btn-accent`: accent-coloured — secondary emphasis.
- `.btn-ghost`: transparent with border — tertiary/cancel.
- Small icon buttons: `rounded p-1.5 hover:opacity-70` with `color: var(--muted)`.
- Destructive actions: inline `style="color:var(--destructive)"`, not a full red button unless it's a standalone destructive page.

---

## Forms

- All inputs and selects: `class="input text-sm"`. Never leave browser-default styling.
- Labels: `text-xs font-semibold muted mb-1` (uppercase, tracked).
- Textareas: `rows="3" class="w-full input text-sm"`.
- Inline steppers (cycle targets): `.stepper-btn` / `.stepper-input`, highlight modified state with `.stepper-input--modified`.

---

## Responsive

Mobile-first. `md:` (768px) is the primary breakpoint.

- Sidebars: full-width overlay on mobile (`position: fixed; inset: 0`), sticky aside on desktop.
- Navigation: hidden checkbox toggle on mobile, full nav on desktop.
- Grids collapse to a single column below `md:`.
- Touch targets: minimum 44px height (`.btn` enforces this).
