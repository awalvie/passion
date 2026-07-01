---
name: UX audit findings (guided-run gaps)
description: Confirmed friction in the guided run player — journal dead-end, Working badge bug, duplicate-run Start
type: project
---

Full journey audit (2026-06) confirmed several structural gaps. Recorded so I don't re-derive them:

**Journal asymmetry (the big one).** The journal/reflection infrastructure is good and already wired — `run_summary.html` lazy-loads `/runs/{id}/journal` (RPE/sleep/energy/focus/went-well/next-focus) plus a "Previous runs" progress comparison. BUT only OPEN sessions and MANUAL log entries reach it. Guided template runs dead-end: completing the last exercise renders the "Workout complete" card (run.html ~L21-28) with only "Return to dashboard"; `handleRunStop` (run_actions.go ~L53) redirects guided runs to `/dashboard` while open sessions go to `/summary`. Fix is cheap: route guided-run finish/complete to `/runs/{id}/summary` (which already shows the journal). The empty journal is even created on stop already (run_actions.go ~L45) — it's just never shown.

**"Working" ascent style has no display mapping.** `tickStyleDisplay` (climbing_ticks.go ~L15-33) has cases for onsight/flash/redpoint/hangdog/repeat/attempt but NOT "working" — the form (run_ticks.html ~L469) offers a Working radio. Result: a Working tick stores Style="working", Sent=false, but renders with no style chip at all (only the card's --working tint). Either add a "working" case or collapse Working→Hangdog/Attempt. Note: my earlier route-logging spec recommended an outcome quick-action row (Flash/Send/+Attempt/Working); the build instead used a full 5-way radio, which reintroduced the friction the spec aimed to cut.

**Start = duplicate runs.** `handleScheduledSessionsByID` case "start" (scheduled_sessions.go ~L40) creates a SessionRun unconditionally — no check for an existing running/completed run on that scheduled session. Dashboard (dashboard.html ~L149) shows Start with no `{{ if not .Done }}` guard, so it appears on Done sessions too. Pattern other apps use: Start becomes "Resume" when a running run exists, "View" when completed (Strong/Crimpd never let you double-start).

**History has zero climbing analytics.** history.go/history.html compute streaks, weekly bars, heatmap, template breakdown — all from SessionRun counts. No grade pyramid, send-rate, or hardest-grade trend from ClimbingTick data, despite ticks being the signature feature. The climber who logs 10 boulders sees "1 session" + a heatmap dot. Biggest review/learning gap; medium cost (aggregate query over ticks + one new section).
