---
name: guided-builder-density
description: De-clutter direction for the guided cycle builder — sectioning + color restraint over disclosure
type: project
---

The guided cycle builder (templates/new_cycle_guided.html) got a density review 2026-08.
User feedback: content/UX is great but it "looks cluttered" — too much text, "a lot of
buttons with lots of colors." Wanted a MINIMAL de-clutter ("a tiny bit"), not a redesign.

Direction recommended and (pending) chosen — value-for-effort ranked:

1. Selected chips must NOT use solid accent fill. `.gc-chip input:checked + span` was
   `background: var(--accent)` → ~7 saturated blue blocks competing with the primary
   Generate button. Switch to the subtle tint the preview `.gc-tag` already uses:
   `accent-bg` bg + `accent` text + `accent-border`, keep font-weight 600. Selected =
   quiet highlight, never a billboard. (Juggernaut/Strong convention.)

2. Split the one tall card into 3 titled cards following what→when→what-each-day
   (RP / TrainingPeaks arc): "The cycle" (name, goals, focus) / "Schedule" (weeks, days) /
   "Sessions" (per-day rows, equipment). Focus belongs WITH identity, not stranded.
   Structure comes from grouping + whitespace, not color.

3. Cut helper (.gc-help) lines that merely restate the label. KEEP only those that teach
   non-obvious behavior: goals before→after, and "defaults rotate through your sessions."
   Rule: a helper survives only if it explains behavior the control can't show itself.

4. Energy chips (Easy/Mod/Hard per day) are the noisiest region — advisory only. Either
   let them inherit the subtle fill, or (better if touching them) collapse each row's 3
   chips into ONE segmented control. Judgment call, not mandatory.

Disclosure decision: HOLD the earlier "inline optional detail" call. Do NOT hide focus/
days/sessions. Only mandatory disclosure fix is the [hidden] CSS bug leaving date-picker
and rest-range inputs always visible. Equipment-behind-a-toggle is optional polish.

Why it matters for future consults: don't re-recommend wizards/steppers for this builder,
don't recommend solid-fill selected states anywhere, and don't pile on helper text.
