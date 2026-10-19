# Passion — Design reference

How the client in `client/src` is drawn. The tokens and the type scale are in "Style a
screen" in [DEVELOPMENT.md](DEVELOPMENT.md).

## Page titles

Every page says where you are once, in one fixed place for its type.

- Tab pages (Today, Plan, Library, History): a large title at the top left, with the date or a
  short line under it.
- Pages about one thing (a cycle, a session, an exercise, a past run): a back pill that names
  the parent, then the thing's name as the large heading. The top bar has no title.
- Task pages (New cycle): a small title centered in the top bar, a Cancel pill at the left in
  place of the back pill (Back on later steps), and the main action as a button at the bottom.
- A centered title stays centered on the screen and is cut off with "…" when it does not
  fit. It never moves toward the pills.

## Touch targets

Every control is at least 44 px (`size-11`, `h-11`). A control that must look smaller keeps
its size and grows its target with `relative before:absolute before:-inset-1 before:content-['']`,
as the set number circle in SetsPlayer does.

## Shared parts

These are the one way each part is drawn in `client/src`. Use them; do not redraw them.

- Page title: `page-title`.
- Section heading above a card: `text-xl font-bold` on a tab page (Today, Plan). Library's
  lists have none: the switch above them already names them.
  On a page about one thing or a task page, wrap the section in `lib/Section.svelte`, which
  draws a `text-[15px] font-bold` heading and puts its `aside`, such as a count, on the same
  baseline. A heading inside a card, such as History's months, is `text-[15px] font-bold`
  too. A count is `text-xs font-semibold text-ink-2`.
- List divider: an inset hairline on every row after the first, `list-row`. No `border-t`
  or `divide-y`.
- Icons: `lib/Icon.svelte`, never an inline `<svg>` for a shape it has. `stroke` sets the
  line weight.
- Check box: `lib/Checkbox.svelte`, inside a label that gives the 44 px target.
- Switch between views: `lib/Segmented.svelte` — a white pill on a `bg-well` track, 44 px.
- Error line: `lib/FormError.svelte`.
- Library row: a 32 px tile, the name in up to two lines, then `lib/OriginMark.svelte` — a
  lock for your private catalog, a person for one you made, nothing for built-in. An
  exercise's tile is its kind icon on `bg-well`; a session's is its icon or first letter on
  its colour. No chevron.
- Step page: `lib/RunPage.svelte` for every step of a running session. It is one dark look in both themes: the session green (`bg-run`), or the timer's phase
  colour (`prep`, `hang`, `rest-field`). From the top: the bar (back, the session clock,
  •••), the name centred, How-to and Note as a centred pair of `run-tag` chips, the
  kind's middle, and one bottom panel. Everything above the bottom panel sits on the
  centre line. The field, the contours and the bar are `lib/RunField.svelte`.
- Session screen: the running session sits on the same `RunField` as its steps. Cards are
  glass, the current section and set are lime (`tint`), done marks are the green check of
  a done set (`rest-on-hero`), and the bottom panel shows `NOW` with the exercise or pick
  you are on, then Continue and a round Finish. The list is also where the session
  changes: an exercise not yet started has a ••• to move or remove it, each open section
  ends with an Add exercise row, and Add section sits under the list. A hold on such a
  row lifts it as a card with a lime ring; a lime line marks where it lands, and a folded
  section it would land in takes a lime wash.
  - A timer's state (`READY · PREP 10 S`, `WORK · REP 1 OF 2`) is a small-caps label
    centred above its digits.
  - The bottom panel starts with a `NEXT` label and a row for the next exercise: its kind
    icon in a glass tile, the name, the dose under it, and a chevron that opens its how-to.
    `NEXT` never names a timer phase.
  - The buttons are `run-btn` (a main action, `bg-tint`, or `bg-live` to go on),
    `run-chip` (a small one such as +30 s), `run-round` (Skip, pause) and `run-icon` (the
    circle around an icon on either).
- Set card: `lib/SetsPlayer.svelte`. One glass card: a "Last time · date" row that opens
  the History sheet, the REPS and WEIGHT headings, one row per set, then "+ Add set". A set
  row is a number circle (tap marks the set done), a reps field, a weight field with a
  `kg` placeholder, and a small × that removes the set. Done is a green row with a check
  (`rest-on-hero`), the current set has a lime ring, number and left bar, and later sets
  are dimmed. Each field shows last time's value under it as `last 20`.
- Climb card: `lib/ClimbsPlayer.svelte`. The page reads as a log, top to bottom: the setup
  row (`Boulder · Indoor`, Change) that opens a sheet, the climbs logged oldest first, then
  one glass card for the climb on the go. The card is `CLIMB 3 · FONT` with the sends so
  far, a sideways grade strip with the picked grade large, the goes as circles (grey
  numbered attempts, green checked sends) that open the edit sheet, then Attempt (glass,
  never red: an attempt is not a failure) and Send (`rest-on-hero`), Focus and Note rows,
  and "+ Add climb". A tap on a logged climb opens the same sheet with the grade, the
  counts, and Delete climb. The card is taller than the view, so on open of a step that
  has climbs and after Add climb the list scrolls to put Send near the bottom, above the undo note, with the newest
  climb just above the card. Until the session has a climb, the setup row has a lime inset
  ring and reads "Check first" until it is opened or a climb is logged.
- Past session: `routes/(app)/(tabs)/history/[id]`. The date line carries the start, the
  end and the length. Journal and each section are `Section` headings with cards under
  them.
  - An exercise card's head is the name, a grey line under it, and its time on the name's
    baseline at the right. A value is its number in `digits text-xl` with the unit small
    and grey (`7m 25s`, `15 kg`).
  - Sets are numbered `list-row` rows, work on the left and weight on the right. Climbs
    are rows of grade, result and time; a send has the green check disc (`rest-on-hero`).
  - Notes are `reading`, split into paragraphs at blank lines. A climb's focus has the
    `target` icon in a `bg-well` circle.
  - Steps skipped or never done read as one grey line under the cards, `Skipped · A, B`
    or `Not done · C`, with a `minus` in a `bg-well` circle.
