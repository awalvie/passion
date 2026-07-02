---
name: In-session notes UX
description: Per-exercise run notes — ungate, auto-grow, plain text, drop live markdown preview
type: project
---

Complaint driving this: "notes on phone is quite poor." Current run.html had per-exercise notes behind a `<details id="run-notes-details">` gate (summary "Add notes") revealing a fixed 2-row textarea + live markdown preview (fetch to /preview/markdown on every keystroke). Two friction sources stacked: hidden behind a toggle AND cramped box.

Closest analogues: Strong / Hevy per-exercise notes = the model. Always-visible single row, field IS the affordance (note text shown when present, faint "Add notes" placeholder when empty — no separate has-notes badge needed). Tap expands inline to auto-growing textarea. Plain text, no markdown. Note persists + pre-fills next session as a rolling coaching cue. Crimpd = session-level notes on save screen, timer-first, notes an afterthought. Lattice = session-level + numeric RPE/quality (structured signal first-class). Mountain Project/8a tick comment = plain text, single always-visible field.

Recommendation given (ranked):
1. HIGHEST IMPACT, LOW COST (template-only + delete ~15 lines JS): remove the `<details>` gate so the note is always visible; auto-grow the textarea (oninput height=scrollHeight, or CSS field-sizing:content); DELETE runNotesPreview + preview div + /preview/markdown fetch. Markdown still renders on READ side (run.html lines 106/176/466 already do markdownHTML), storage stays text, nothing lost. Directly fixes the stated complaint.
2. MEDIUM COST decision, user must choose: ephemeral per-run note vs persistent-per-exercise note that pre-fills next session (Strong/Hevy rolling-cue model). Persisting = field + pre-fill query + behavioral change. Worth it only if notes should act as cross-session coaching reminders.
3. MEDIUM COST companion: add an always-visible RPE stepper (0-10, tap targets, no typing) alongside the note. Aligns with project_climbing_metrics goal (free text doesn't feed fatigue/volume metrics; a numeric field does). Lattice pattern. Skip if the only goal is findability.

Anti-patterns flagged: notes behind toggle / below fold (the bug); live markdown preview on input (friction, ~zero mobile payoff, noisy); fixed 2-row textarea that scrolls internally (hides own text); full-screen note mode (modal interrupts inline run flow — contradicts route-logging memory); LOSING INPUT ON NAVIGATION — skip path copies run-notes→skip-run-notes (line ~1326) but Done/auto-advance must also capture value before HTMX swap; autosave on blur/debounce so a mis-tap never eats the note.

Philosophy check: ungating + dropping markdown-input REDUCES chrome/options = on-philosophy (data over decoration, opinionated defaults). RPE stepper is the only additive control, earns its place via the committed metrics goal.
