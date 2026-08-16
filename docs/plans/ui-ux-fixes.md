# UI/UX fixes — plan

## FINAL DECISIONS (post pixel + scout review) — this section supersedes conflicts below

- **1** Start btn → `min-height:2.75rem`, padding `0.5rem 0.85rem`, font `0.875rem`; verify vertical centering.
- **2** Try non-destructive fixes first (stacking context / explicit cursor). Dropping `backdrop-filter`
  is an aesthetic change — ask before doing that.
- **3** Single-row desktop nav; keep **Start Session as the distinct terminal CTA**; verify at 768–900px
  (fall back to stacked if it crowds). **Theme toggle → moved into the account (▾) dropdown** [decided].
- **4** Standalone notes: **muted card, neutral left border (`--border`), `notebook-pen` icon, a visible
  "Note" badge (focus-badge slot), full-legibility body text, no exercise/duration/location chips.**
  Stats: notes excluded from **Volume, this-week, this-month, streak, AND averages** — notes touch no
  stats [decided]. Handler partitions standalone journals out of every stat bucket.
- **5** Kebab (⋯) overflow menu in the detail header (reuse `site-nav-dropdown`), destructive inline color,
  `hx-confirm` **naming the cycle**. Also fix the sibling event-delete `onclick confirm` → `hx-confirm`.
- **6** Per-row delete on index (mobile card via stretched-link `after:absolute after:inset-0` + `relative z-10`
  button; desktop table Actions column). Match training-log delete convention
  (`hx-target="closest …" hx-swap="outerHTML"`). **Fix `.Weeks`→scheduled-count column bug (in-scope).**
- **7a** **Grouped, visible chips** (not a dropdown): group "Your sessions" vs "Starter templates",
  focus-relevant first, "Show all" expander if long. Live "Your week" preview stays source of truth.
- **7b** **Real tag input with climbing-equipment autocomplete** (free text allowed), this field only.
  Promote `.gc-chip` into `passion.css` while here.
- **7c** Remove `*`; grey helper line that **explains the choice** (not restates heading) — via `copy`.
  Disabled Generate button states *why*.
- **7d** Day chips fill their cell (even 7/4-across) + **stronger filled selected state**.
- **7e** Inline goal + equipment (drop disclosure); keep whole builder ~one viewport.
- **7 (extra)** **Focus cascades into session relevance** [decided] — reinforces 7a grouping.
- Post-change: pixel (templates/CSS incl. dark mode), qa (stat partitioning), copy (helper lines +
  confirm text), simplify, scribe.

---


Batch of bug/polish fixes reported by the user, grounded in screenshots of the running app
(seeded, :3001). Nothing implemented yet. Decisions already made with the user are marked
**[decided]**; open proposals are marked **[proposal]** for reviewer/user input.

---

## 1. Header — hamburger vs Start button size mismatch (mobile)

**Observed:** On mobile the `☰` menu button is a 44px box (`.site-header-menu-btn`
min-width/height `2.75rem`), but the `Start` button (`.site-header-start-btn`) has no
min-height — padding-only, ~28–30px tall. They sit side by side and visibly differ.

**Fix:** Give `.site-header-start-btn` `min-height: 2.75rem` (and align padding/font) so both
hit the 44px touch target and read as one control group. CSS-only, `static/passion.css`.

---

## 2. Header — delayed hover cursor on nav options

**Observed/reported:** hovering nav options, the cursor takes a beat to switch to the pointer.
**Primary hypothesis:** `.site-header` uses `backdrop-filter: blur(10px)` on a `position:sticky`,
`z-index:50` layer. Backdrop-filtered compositing layers are a known cause of laggy hit-testing
in Chromium — the pointer cursor updates late.

**Fix (to validate live):**
- Give the interactive nav layer its own stacking context above the blur
  (`.site-header-nav`/links `position: relative; z-index: 1`), and/or
- ensure links declare `cursor: pointer` explicitly (anchors do by default; buttons like the
  dropdown toggle already do).
- If the lag persists, reduce/soften the blur or drop `backdrop-filter` on the sticky header.

Verify by hovering in a real browser (can't be measured from a screenshot). Lowest-confidence
item — will iterate.

---

## 3. Nav bar appearance (desktop)

**Observed:** desktop header renders as **two rows** — row 1 is the logo (far left) + theme
toggle (far right) with a large empty gap; row 2 is History/Log/Cycles/Calendar/Library/account/
Start Session, right-aligned with an empty left. Feels unbalanced and stranded.

**[proposal]** Collapse to a **single row** on desktop (≥768px): logo left; nav links + theme
toggle + Start Session grouped on the right. Keep the existing two-row/stacked layout on mobile
(it reads fine — see screenshot). Tidy spacing/alignment and pull the orphaned theme toggle into
the right-hand group. `topbar.html` layout classes + `static/passion.css`.

---

## 4. Quick notes distinct from sessions (training log)

**Observed:** standalone quick notes (`SessionJournal.RunID == nil`) render in the exact same
card as logged sessions and are counted in the Volume/streak stats.

**[decided]** Two changes:
- **Visual:** give standalone notes a lighter "note" treatment — a note icon (`notebook-pen`),
  drop the exercise/duration/tick chips that imply a workout, muted styling; keep it clearly a
  jotting, not a session. `templates/training_log.html` (+ a small CSS class).
- **Stats:** stop counting standalone notes in the session Volume/streak/this-week/this-month
  numbers. Requires the training-log handler to separate standalone journals from session
  journals when computing stats. `http/server/training_log*.go`.
- Update the empty-state copy if needed ("No sessions logged yet" already reads correctly).

---

## 5. Delete-cycle button placement (`/training-cycles/{id}`)

**Observed:** delete lives at the **bottom of the collapsed "Cycle details" disclosure** — an
unrelated metadata form — so it's both buried and semantically misplaced.

**[proposal]** Move it out of the details form into an **overflow (kebab `⋯`) menu in the page
header**, next to "Add event", reusing the app's existing `site-nav-dropdown` menu pattern
(menu item: "Delete cycle", destructive color, same `hx-confirm`). Alternative if reviewers
prefer: a dedicated subtle "Danger zone" strip at the very bottom of the page. Rename stays in
the details form (autosave).

---

## 6. Delete-cycle from the index (`/training-cycles`)

**Observed:** no delete on the index; the desktop table also has a latent bug — the "Scheduled
sessions" column renders `.Weeks`, not a session count (`training_cycles.html:50`).

**Fix:** add a delete control per row, reusing the existing `POST /training-cycles/{id}/delete`
with `hx-confirm`:
- Mobile card list: a trash icon-button on each card (outside the row's link).
- Desktop table: an "Actions" column with a trash icon-button.
- On success remove the row (verify the delete handler's HTMX response supports row removal vs.
  its current redirect; adjust handler if needed).
- **Bonus (flag, not bundled):** fix the `.Weeks`→scheduled-count column bug while here.

---

## 7. Guided cycle builder (`/training-cycles/new/guided`)

### 7a. "What sessions will you run?" — too many chips
**Observed:** 13 session chips in a big grid.
**[decided]** Replace with a **multi-select dropdown**: open to tick multiple sessions; selected
ones shown as removable chips below. Preserves round-robin. Native `<details>`/checkbox-driven so
it still submits without JS; JS keeps the live "your week" preview + selected-chip rendering.

### 7b. Equipment — make it multi-option
**Observed:** single free-text "Where / equipment" field saved into cycle notes.
**[decided]** Replace with a **tag input** (type + enter to add multiple chips, same pattern as
the Label tags used elsewhere). Store as before (comma-joined into cycle notes/venue) — no schema
change.

### 7c. Blue `*` markers
**Observed:** a blue `*` after each section heading reads as a meaningless/interactive marker.
**[decided]** Remove the `*`; add a **light-grey helper line** under each heading phrased as a
self-question (e.g. "Which movement quality is this block built around?"). Drop the asterisk
convention entirely on this form.

### 7d. Day chips oddly placed
**Observed:** `WHICH DAYS CAN YOU TRAIN?` uses a 7-col grid but the small chips float left inside
wide cells, so Mon…Sun are scattered with uneven gaps.
**[decided]** Make each day chip **fill its grid cell** (equal width, centered) so the row reads
as an even 7-across (4-across on narrow mobile, already handled).

### 7e. "Add detail" disclosure
**Observed:** goal/venue are hidden behind an "Add detail" `<details>` disclosure.
**[decided]** **Inline** those fields (goal + equipment tags) directly in the form; remove the
disclosure.

---

## Sequencing & review
1. **Review this plan** — pixel (design-system) + scout (UX best-practice) — before coding.
2. Implement grouped: (a) header 1–3, (b) training-log 4, (c) cycles 5–6, (d) guided builder 7.
3. Post-change: pixel on templates/CSS, qa on the handler/stats logic, simplify, scribe (README).
4. Verify in the running app (screenshots) at desktop + mobile.

**No schema changes.** Stats change (item 4) is the only handler-logic change; everything else is
templates/CSS + one new tag-input/dropdown interaction.
