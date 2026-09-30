# Passion design principles

What any Passion screen must achieve, on a phone, a watch, a tablet or a desktop. The chosen look
sets the style: colours, type, shapes, lines and the character of motion. These thirteen principles
say what every look must do, and one of them is to have a character of its own (13). Each one gives
a reason with its source, a check you can run on any screen, and a Passion example. Quotes are word
for word from the named source.

## Who uses Passion, and where

- A climber in a gym, between attempts. Power work comes right after the warm-up, while they are
  fresh. The owner's rule: the app must never slow them down between attempts.
- During a hangboard or rest timer the phone lies on the floor or a shelf, about one to three
  metres away (the owner). It is touched with chalky hands, usually one hand.
- They want to train, not log. The owner does not want to be "constantly checking my phone to log
  everything". Cues are one to four lines.
- In a shared gym the phone is often muted. Some gyms are basements with no signal.
- At home, the same person plans a cycle or reads history, close up, sometimes on a larger screen.
- A new person starts with nothing: no profile, no plan, no history.
- The log is kept for years. It is only as useful as people's trust in it.

So Passion has two postures. In the gym: glance, one tap, hands free. At home: read, plan and
compare. Both use the same parts. On a screen used in the gym, the gym posture wins every conflict.

## Principles

### 1. The app never slows the person down

- Why: [WWDC 2018, Designing Fluid Interfaces](https://developer.apple.com/videos/play/wwdc2018/803/):
  "Everything needs to respond instantly." Strong fills in last time's numbers, so a set is one
  tap, and starts rest by itself. Rauno Freiberg says an interface feels best when it works out
  what you want without asking.
- First run: a new person can run a session with nothing but an account (docs/V2_DESIGN.md).
  Fitbod asks only about training experience before it shows a first workout.
- When a save will almost surely work, the screen changes at once (Vercel). A real wait shows
  only after 150–300 ms and stays at least 300–500 ms, so it does not flicker (Vercel).
- Check: From opening the app to the end of a session, count every form, typed field, spinner
  and wait. Which of them could go?
- Do: tap Done and rest starts by itself. Don't: ask for a profile, or for reps, before the next step.

### 2. In the gym, the screen reads from where the person trains

- Why: Crouton won a [2024 Apple Design Award](https://www.apple.com/newsroom/2024/06/apple-announces-winners-of-the-2024-apple-design-awards/)
  because it "lets users keep their focus on the counter rather than the screen". Passion's
  counter is the wall, one to three metres from the phone. MacroFactor makes the day's one number
  its header.
- Size follows the reading distance, not the screen size, so the rule holds on a phone, a tablet
  and a watch. Only what must be read from there is large, not every word. Screens read close up,
  such as history, can hold more. Sizes step clearly apart
  (Material). Vercel gives compared numbers a fixed width. A running timer needs the same, so it
  does not jump.
- Check: Put the device where it sits during a hang, and step back to where you train. Can you
  read the phase, the time left and what comes next?
- Do: "Rest", the seconds left, then "Next: max hang, set 2 of 3". Don't: a small timer under a
  table of sets.

### 3. One thing leads each screen, and what belongs together sits together

- Why: [WWDC 2025, Meet Liquid Glass](https://developer.apple.com/videos/play/wwdc2025/219/): "When
  every element is tinted, nothing stands out". The HIG allows one or two prominent buttons per
  view. Passion takes one, because there is no time to choose between attempts. Linear dims its
  navigation once you have arrived. Everything else waits one tap away (HIG, progressive
  disclosure).
- Notion puts less space between neighbours of the same kind, so they read as one group. Groups
  must show without relying on lines: space does most of the work, and a soft surface can hold a
  group.
- Check: Squint. Does exactly one thing stand out, and is it what the person needs next? Cover
  the lines. Can you still see the groups?
- Do: during a hang, the navigation steps back and the timer leads. The sets of one exercise sit
  closer to each other than to the next exercise. Don't: give Start, Edit and Log the same weight.

### 4. Every target fits a chalky fingertip

- Why: [Material](https://m3.material.io/foundations/designing/structure) sizes a touch target to
  "a physical size of about 9mm, regardless of screen size." The HIG counts the space between
  controls as much as their size. Every swipe also has a tap (Vercel).
- Apple's minimum is smaller. Passion takes Material's, because chalk and a phone on the floor make
  every tap imprecise.
- The size is physical, so it holds on a watch, where the HIG allows at most three icon buttons
  or two text buttons in a row. With a mouse on a desktop, Material allows smaller targets.
- Check: Measure the hit area, not the icon. Is it about 9 mm, with clear space around it? Can one
  thumb do every gym action?
- Do: wide pause and next controls where the thumb rests. Don't: tiny plus and minus arrows for
  reps.

### 5. A timer change reaches the person without the screen

- Why: Crimpd's [App Store notes](https://apps.apple.com/us/app/crimpd/id1252333138) let the
  person "keep using the app while the timer and audio cues keep running." The V1 player plays a
  two-tone sound and a double buzz when rest ends, and still buzzes on a muted phone
  (docs/V2_DESIGN.md). The HIG says motion and colour should never be the only signal, and that
  haptics should be optional.
- A timer on the lock screen or a watch is a later goal, not a commitment (owner, 2026-09-24). The
  need it serves holds now: the person can follow the timer without looking at it.
- Check: Start a rest, lock the phone and look away. Does each change arrive on time, by sound and
  by buzz? When you look back, is the time right? Can the person turn each cue off?
- Do: when rest ends, play the sound, buzz twice and show the word "Hang". Don't: mark the change
  with colour alone.

### 6. Nothing is lost by accident

- Why: [Cultured Code](https://culturedcode.com/things/blog/) builds "software you can rely on
  every day, for years to come."
- Costly actions never get the primary style, even when they are the likely choice (HIG). They are
  slow to confirm and quick to release (Emil Kowalski), as with get-a-grip's "Hold to end".
- With no signal, a recorded session arrives intact once the device reconnects
  (docs/REQUIREMENTS.md). Until then, the person can see what has not synced yet.
- A record shows what it showed when it was written (docs/V2_DESIGN.md). If you rename a gym,
  last year's session keeps the old name.
- Check: Could a stray tap or a chalky palm destroy something? Log a set with no signal. Does the
  screen say it is waiting, and does the set arrive later? Edit a template. Does any finished
  session change?
- Do: end a session with a hold. Don't: silently drop a set that was logged offline.

### 7. Screens used every session get full craft, but no flourish and no delay

- Why: [Family](https://benji.org/family-values): "the potential for delight increases as the
  frequency of feature usage decreases." Yet Family's token send, which many people use daily, must
  still be "efficient and enjoyable, without being overbearing", and Family aims for "a consistent
  level of polish everywhere". How often a screen is used sets how loud it may be, not how well it
  is made.
- A flourish adds time or noise: a bounce, a burst, a sound that does more than confirm. Craft is
  how the screen is made: its surfaces, icons, accent and a press that answers at once. Everyday
  screens get all the craft and none of the flourish.
- The HIG avoids motion on frequent actions and lets people cancel it. Vercel animates only for
  cause and effect, or for deliberate delight. Rauno Freiberg notes that Mac context menus open
  with no motion, yet "the selected item briefly blinks the accent color (pink) to provide
  assurance that the element was successfully selected."
- A tap cuts any motion short, and motion stays under about 300 ms (Emil Kowalski). With reduced
  motion on, movement becomes a fade (HIG). Boardsesh keeps its liveliest spring for celebrations,
  such as the session summary.
- Family argues that careful motion can feel just as fast. On screens used every session, Passion
  follows the HIG instead.
- Check: On a screen seen every session, does any motion, sound or flourish do more than confirm?
  Does any animation make the person wait before their next tap counts? What happens to it with
  reduced motion on? Is the screen as well made as the rarest one, with the same surfaces, icons
  and accent?
- Do: a logged set answers the tap at once with a small tick, and the next set is ready. Keep the
  flourish for a new best, a first send at a grade or a finished cycle. Don't: celebrate every
  logged set, or strip a screen bare because it is used every session.

### 8. Colour carries meaning and passes contrast in both themes

- Why: [Stripe](https://stripe.com/blog/accessible-color-systems): "The web accessibility
  guidelines suggest a minimum contrast ratio of 4.5 for small text, and 3.0 for large text."
  Stripe also checks each colour on the lightest tint of every hue.
- These ratios are floors, not targets. Text and icons that carry meaning pass them. Surfaces and
  lines that only group can stay soft, because groups never rely on lines (principle 3). Hard
  contrast everywhere is for the system's higher-contrast setting, where "the color differences
  become far more apparent" (HIG).
- A colour that signals means one thing, and no colour carries meaning alone (HIG). Neutrals and
  surface tones set the look and signal nothing.
- Light and dark follow the system setting, and a setting can pin one (owner, 2026-09-24).
- Check: Name what each signal colour on the screen means. One with no meaning, or with two, is a
  fault. Measure the contrast of text and icons in light and in dark. Does the screen still work in
  greyscale?
- Do: dim a missed session instead of showing it as an error, because the plan was only a
  suggestion (docs/REQUIREMENTS.md). Don't: one accent for Start, rest and the current place.

### 9. Few words, in the climber's words, one word for each thing

- Why: [Revolut](https://www.revolut.com/blog/post/our-top-5-design-principles-at-revolut/): "We
  focus on the little things, like common terminology". Railbound won an Apple Design Award for an
  onboarding that leaves out words. Each score has a short description, so that a 6 always means
  the same thing (docs/V2_DESIGN.md). Every screen offers a next step (Vercel).
- Check: Is every cue four lines or fewer on a phone? Does each thing have one name everywhere? Does
  any word come from the code and not the gym? Does every empty state say what to do next?
- Do: "Half crimp, feet on the floor. Ease off at any joint pain." Don't: "rest" in the player and
  "recovery" in history.

### 10. With large text or a screen reader, a person can still run a whole session

- Why: [HIG, Accessibility](https://developer.apple.com/design/human-interface-guidelines/accessibility):
  "Ideally, give people the option to enlarge text by at least 200 percent". On a watch the HIG
  asks for 140 percent. get-a-grip caps the size only of the few large elements that would clip.
  Everything else keeps growing. Apple's award winners are praised for a named accommodation,
  such as stitch.'s options for colour blindness, low vision and motion sensitivity. Nothing
  important disappears on a timer (HIG).
- Check: At the largest text size, do the phase and the time still show without clipping? With a
  screen reader on, can you run a hangboard session and log a climb? Does the reader say "rest
  over" once, and not every second?
- Do: give every control a spoken label that says what it does. Don't: show a message that
  vanishes before the person can walk back to the phone.

### 11. One set of parts, in the gym and at home, on every platform

- Why: [Airbnb](https://medium.com/airbnb-design/building-a-visual-language-behind-the-scenes-of-our-airbnb-design-system-224748775e4e):
  "There should be no isolated features or outliers." Revolut reuses its existing patterns, so
  people do not relearn each new feature. The HIG ties each haptic to one meaning. A part that
  carries on into the next step stays the same (Family).
- Both postures share parts: a session, a logged set and a grade look the same in the player, in
  the plan and in history. One type scale, one spacing scale and one set of motion timings serve
  every screen. A new part or setting needs a real problem behind it (Revolut).
- There is one look on every platform, with each platform's own back, text size and gestures
  (owner, 2026-09-24). Boardsesh draws a different look for each platform, and Passion does not.
  Keep top-level places few. The HIG suggests five or fewer.
- Check: Does this screen use a part, colour, icon, sound, size, gap or duration that no other
  screen uses? Do back, text size and gestures work as they do in other apps on this device?
- Do: today's session on the plan looks like the session the player opens. Don't: a history card
  with a layout of its own.

### 12. At home, the whole plan comes first, and every change can be undone

- Why: [HIG, Charting data](https://developer.apple.com/design/human-interface-guidelines/charting-data):
  "Not every collection of data needs to be displayed in a chart." Passion's design keeps charts
  minimal, with no row of metric toggles, and shows climbing as one number (docs/V2_DESIGN.md).
  Whoop shows a score first, the trend on request and the detail after that.
- The planner shows the cycle laid out as it will actually fall (docs/REQUIREMENTS.md). Linear
  starts simple and adds power as it is needed. A larger screen shows more of the plan, not more
  controls.
- Every change can be undone: moving a session, rebuilding a cycle, deleting a run. Passion's
  requirements keep a deleted account for 30 days, so that a mistaken delete does not destroy a
  year of training (docs/REQUIREMENTS.md). Every edit gets the same care.
- Check: Can you see the shape of the whole cycle, with today marked? Could a number, a simple
  mark or a list do this chart's job? After any change, can you undo it in one step?
- Do: after Thursday's session moves to Friday, offer Undo. Don't: a history page that opens on
  four charts with toggles.

### 13. Every screen has a character, and its parts feel like one family

- Why: [Apple's earlier HIG](https://web.archive.org/web/20210101115254/https://developer.apple.com/design/human-interface-guidelines/ios/overview/themes/):
  "Aesthetic integrity represents how well an app's appearance and behavior integrate with its
  function." It asks that "icons are precise and lucid, adornments are subtle and appropriate".
  Apple's [Visuals and Graphics award](https://developer.apple.com/design/awards/) looks for "a
  distinctive and cohesive theme". Its 2026 app winner is a tide tracker full of charts.
- Character helps people use the app. In [Google's research](https://design.google/library/expressive-material-design-google-research),
  "Participants were able to spot key UI elements up to four times faster in the M3 Expressive
  designs". It also warns: "No amount of emotion can compensate for a lack of clarity."
- Soft surfaces and depth give structure. The HIG: "A material is a visual effect that creates a
  sense of depth, layering, and hierarchy between foreground and background elements."
  [Material 3 Expressive](https://m3.material.io/blog/building-with-m3-expressive): "Create visual
  hierarchy with surface tones." Linear rounded its borders and softened their contrast, because
  "Structure should be felt not seen".
- Icons carry meaning, so words stay few. The HIG: "An effective icon is a graphic asset that
  expresses a single concept in ways people instantly understand." Linear uses icons to make
  statuses "recognizable at a glance", and cut them where "their presence had grown excessive".
  Google found that "removing text labels from email actions resulted in decreased usability", so
  an icon stands alone only where its meaning is instant, and it keeps a spoken label (principle
  10). Things shows a project's progress as a small pie, not a sentence.
- One accent marks what matters most. [HIG, Branding](https://developer.apple.com/design/human-interface-guidelines/branding):
  "Apply your app's accent color judiciously. Using your brand color too broadly can overwhelm your
  interface and dilute its impact." Boardsesh keeps its one glow for the one main action on a page.
- The parts are one family, with a natural feel of use. Airbnb treats its components "as elements
  of a living organism. They have a function and personality". Icons share "a consistent size,
  level of detail, stroke thickness (or weight), and perspective" (HIG).
  [Cultured Code](https://culturedcode.com/things/features/) redesigned Things "Not just how it
  looks – but also how it works, and how it feels." Rauno Freiberg: "Executing well on details
  like these make products feel like a natural extension of ourselves."
- Character never overrides principles 1 to 12. Where a soft tone would fail contrast, or an icon
  would hide meaning, that principle wins. Where there is no conflict, a flat, bare screen is a
  fault, not a safe default.
- Check: Cover the words. Can you still tell what each part is and what matters most? Could an
  icon, a mark or a number replace any word without losing meaning? Is there one accent, on the
  thing that matters? Do soft surfaces, not hard lines, show what sits above what? Put the screen
  beside two others. Do they look like one family, and like Passion?
- Do: a session card shows its dose as an icon and a number each, for edge, sets and hang time, on
  a soft raised surface, with the accent only on Start. Don't: rows of black text on white split by
  hard lines, where every fact is a sentence and nothing stands out.
