---
name: disabled-button-convention
description: The app's established CSS pattern for disabled interactive controls
type: feedback
---

Passion's convention for a disabled button/control is a CSS `:disabled` rule, not inline JS style writes:

```css
.foo:disabled { opacity: 0.3–0.4; cursor: not-allowed; }
```

Seen in `.run-nav-arrow:disabled`, `.run-exercise-nav-btn:disabled`, `.run-transport-btn:disabled` (all in static/passion.css). A non-run-page alternative also in use: `classList.toggle("opacity-40", !ok)` (+ `pointer-events-none` when the element isn't a real `<button>`) — see exercise_library.html's `export-selected-btn`.

When reviewing any new form/control that toggles a disabled state via JS, check it follows one of these two idioms rather than ad hoc `el.style.opacity = ...`. Missing `cursor: not-allowed` or using an off-scale opacity value (e.g. 0.45 vs the established 0.3–0.4 range) is a recurring drift to flag.
