# Passion — requirements

A self-hosted web app for climbing training. A person plans their training, follows a session
in the gym, and records what they did.

This file describes what the app must do and what must stay true. It does not describe how.
It names no tables and no columns on purpose.

**This file is not settled, and it is not a specification to build from.** It was written
before the rewrite and it has not been reviewed against the current design. Where it and
[V2_DESIGN.md](V2_DESIGN.md) disagree, the design wins. Treat anything here as a proposal to
question, not a requirement to satisfy, and raise the disagreement rather than resolve it
alone. Parts 1 and 2 in particular are one person's habits written as though they were rules.

Part 1a follows one person through six weeks, which is the fastest way to see what the app is
for. Part 2 describes what people do with it. Part 3 is the set of rules a design must keep,
all of them at once, and is the hard part. Part 4 says what the app does not do today. Part 5
says where it is going, so that nothing is designed in a way that blocks it.

---

## 1. Who uses it, and how much data there is

Anyone who self-hosts it, for as many accounts as they care to create. The number of accounts
is not capped and the design must not assume a small one. Accounts share nothing of their own
with each other.

Counted on the real installation:

- The shared catalog holds 105 entries: 89 exercises and 4 sessions, three of which offer a
  choice.
- One account that also loads a private catalog of its own sees about 272 entries in total.
- A person accumulates a few hundred recorded sessions a year, with about ten recorded
  exercises inside each.

Nothing about performance has been measured yet. The target is that the main pages answer in
under 200 ms against 500 recorded sessions and 5,000 recorded exercises.

A person chooses the units they read and type in: kilograms or pounds, centimetres or inches.
Changing that choice must never change a number already written down, and the number must read
the same to anyone else who sees it.

It runs as one binary against a PostgreSQL database. There are no other services: no queue, no
cache, no search engine. A person backs it up with `pg_dump`.

---

## 1a. What using it actually looks like

One person, from an empty account to a year of training, in order. Every session and exercise
named here is real material the app ships.

### She signs up

She has the app running on a machine at home. She opens it, gives an email address and a
password, and she is in. Nothing is waiting for her: no catalog of her own, no cycles, no
history. What she can see is the catalog the app ships.

### She fills in her profile

Her height and her ape index. Which grade systems she wants to read: Font for boulders, French
for routes, because those are the ones her gym uses. Her current bests, which she will come
back and raise later — most pull-ups in one go, and the most weight she has hung with.

She also puts in her weight. She will add it again every few weeks, and she wants to see the
line, not just today's number.

### She adds the places she trains

Two gyms — The Castle, which is indoors, and a crag she goes to in summer. Then, separately,
the boards she uses: a Kilter at The Castle and a Tension board at a friend's place. A board is
not a gym and she wants them listed apart, because "where did I climb" and "what was I climbing
on" are different questions.

### She looks around the catalog

She browses the exercises, filters them by tag to see everything tagged `fingers`, and reads
the notes on a few. She opens *Isometric Hang: Front 2 Half Crimp* and sees what it suggests:
one set, two reps, ten seconds hanging, twenty seconds rest, six seconds to get ready. The
notes tell her it is sub-maximal, feet on the floor, ease off at any joint discomfort.

She looks at the four sessions. She opens *Fingers & Strength* and sees it is six sections long:
a warm-up, a finger warm-up, wall crawls, antagonist and prehab work, mobility, and a journal.
It says it needs a hangboard and a bouldering wall.

None of this is hers. She can read all of it, and she can change any of it for herself whenever
she wants to, which she does later.

### She builds a cycle

She makes a training cycle and calls it "Spring fingers". She says how long it runs — six
weeks, or she could have named the date she is training towards instead.

She also says how long its repeating block is. Hers is seven days, so her training falls on the
same weekdays every time. Her partner runs a ten-day block, so his sessions walk through the
week, and the app follows the block rather than the calendar.

She gives it a purpose — strength — and tags it. Then she writes two goals, each in three
parts: where she is now, where she wants to be, and how she plans to get there. The first is
"front-two half crimp, bodyweight, ten seconds" to "plus five kilograms" by "twice a week,
sub-maximal, never to failure". The second is about a boulder she has been failing on.

### The trip gets in the way

She has a week away in the middle of the cycle, so she adds it to her calendar as a trip
covering those dates, and marks it as blocking training.

When she finishes setting the cycle up, the app stops before creating anything and tells her
that four of the days she just planned fall inside that trip. It lists the four dates. It
offers to leave them out. She takes the offer, and everything else she typed is still there
when she comes back from the warning — she does not have to fill the form in again.

She also marks the end of the cycle as a deload. It shows on her calendar, and it does not stop
her training.

### She sets the repeating shape

She places her sessions within the block: one hangboard day, then two bouldering days spread
after it. Because her block is seven days long, that comes out as Tuesday, Thursday and
Saturday every time, and her partner's ten-day block walks through the week instead. The shape
repeats until the cycle ends, and the four days inside her trip are left out.

She can see it laid out as it will actually fall. Later she will drag one session to a different day
without touching the rest.

### She makes the sessions hers

*Fingers & Strength* is close to what she wants but not exactly it. She changes it for herself.
From that point what she runs is hers, and what every other account sees is unchanged.

She also writes a session from scratch — a short one for the mornings she has twenty minutes
and a doorway — and builds it out of exercises already in the catalog rather than typing them
again.

### She changes what is inside them

Inside her *Fingers & Strength* she drags *Wall Crawls* above *Antagonist & Prehab*, because
she wants it while her fingers are fresh. She drops the mobility section entirely.

She opens *Antagonist & Prehab*, which is four exercises: external rotations, lateral raises,
tricep extensions and overhead press. The app suggests three sets of twelve on the external
rotations with sixty seconds between sets. She has used 7.5 kg on those for a year, so that is
what she wants the app to ask of her. When a later release rewrites that exercise's notes with
better cues, she gets them.

She adds a choice to her session: five hangs, of which she must pick two on the day, so she is
not locked into one grip six weeks out. She reads each option's notes and clip while setting it
up.

One exercise she wants does not exist, so she writes it: a name, notes, how it is done — ten
seconds on, twenty off — and its numbers.

### She sets targets, including a ramp

The cycle asks her what she wants from each exercise.

For *Isometric Hang: Front 2 Half Crimp* she sets three sets for the whole cycle. Then she says
it should ease off for her deload, and sets that stretch to two sets. Saying so does not blank
what she already typed.

For her hangs she plans the set in rungs rather than one flat number: seven seconds, then ten,
then twelve, inside a single set, repeated four times. That ramp is the point of the exercise,
and nothing the cycle asks may flatten it into one number.

She clears a target she set by mistake, and everything she set under it goes too.

### Tuesday: she runs the session

She opens the app in the gym. Today's session is there. She presses start.

What the session asks of her today is what she will see in this record forever. If she edits
the exercise tomorrow, or deletes the cycle next month, today's record does not move.

She works through the sections. She could take them in any order, and today she does not.

The warm-up first. Then the choice: five hangs, pick two. She reads the notes, picks front-two
half crimp and middle-two half crimp, and the session carries on with those two standing where
the choice stood. If she changes her mind, the new pick replaces the old one rather than adding
to it.

The hang is timed. Six seconds to get ready, ten hanging, twenty resting, twice, and the timer
counts her through with a sound when the rest ends. She mutes it after the first set because
the gym is quiet. She pauses between sets to talk to someone and picks it up where she left
off. Her phone locks and the clock is still right when she opens it again.

On the second set she manages only eight seconds on the second rep. She types 8 where the app
asked for 10, and can see the gap while she is still standing at the board. Underneath, the app
shows what she did the last few times she did this hang.

On the external rotations she records three sets of twelve at 7.5 kg. She writes a note against
that one exercise — "right shoulder tight" — while still doing it.

She skips the mobility work because she is out of time, and it is recorded as skipped rather
than left hanging.

At the end she writes the session up: six hours of sleep, energy three out of five, felt like a
seven out of ten, was working on grip position. What went well: "left hand felt solid". What to
work on next: "start the second set less tired."

Then she sees a summary — what she did, what she skipped, how long it took, and how this
compares with the last several times she did this same session.

### Thursday: bouldering

*Boulder Session* at The Castle. The bouldering part is not something you count in sets, so
instead she logs climbs one at a time as she does them.

A 6C flashed. A 7A on the ninth try. A 7A+ she never got. Two traverses with no grade at all.
She rates one of them three stars, notes that she was working on heel hooks, and writes a line
about what she thought of it.

She never tells the app whether something counted as a send. That follows from how she says she
climbed it. The traverses have no grade, so they are not sends and never appear in her pyramid.
She logs the same problem again with one press, and everything constant about it carries over.

As she goes she can see a running count: how many climbs, how many sends, and the hardest grade
so far today.

### The one she missed

Saturday she trains and forgets to open the app. Sunday morning she writes it up from memory.

She picks the session, backdates it to Saturday, adds the exercises she actually did, drags them
into the order she really did them, and types the numbers — some as one summary, some set by
set. She names a gym she has not saved before and it is remembered for next time.

She gets distracted and leaves it half finished. It waits for her, and until she finishes it, it
counts towards nothing.

### She looks at her week

Everything she did, grouped by week: the two planned sessions, the one she wrote up afterwards,
and a bare note she left on Wednesday about her shoulder. The climbing session is summed up in a
line — so many boulders, so many routes, so many sends.

Her cycle tells her, in a sentence, that she is in week two, has done five of the six sessions
it asked for so far, and has one left this week. A Tuesday already gone by with nothing recorded
counts as missed, not as still to come. It is shown dimmed rather than in red, because the plan
is a suggestion and the record is what happened.

### Things change mid-cycle

She drags Thursday's session to Friday one week because of work.

The Castle renames itself to The Castle Climbing Centre, so she updates the place. Every session
she recorded there before today still reads The Castle, because that is what it was called on
the day.

A later release of the app improves one of the shipped exercises. She gets the improvement,
because she never rewrote that one. Another shipped exercise is dropped from the catalog
entirely. It disappears from her library, and it keeps working perfectly in every session and
every record that already uses it.

She deletes a cycle she set up and abandoned. Every session she actually did under it stays; only
the plan goes, with its targets, its goals and the rest days it created. The days she added by
hand stay too.

### The deload

Her hang target drops to two sets for that stretch, because that is what she said when she set
the cycle up. The rest of the cycle is still three. The session itself never changed.

### The end of the cycle

She looks back over six months. A heat map of the days she trained. Her current streak and her
longest. How long she has trained in total and what an average session takes. Which sessions she
does most. Her last twelve weeks as a bar a week. How much of her climbing was indoors, and how
much on a board.

A pyramid of the grades she has sent, and a send rate. The traverses are in none of it.

She opens *Isometric Hang: Front 2 Half Crimp* and sees every hang she has ever done on it, in
one unbroken line — including the ones from before she made that exercise her own, and before
the app rewrote it. Ten seconds at bodyweight in February, ten seconds at five kilograms in
April, which is exactly what she wrote in her goal.

### What broke in the old version, and must not break again

She once renamed an exercise and watched a year of records change with it, so a session she did
in January claimed to contain something that did not exist until June.

She once deleted a training cycle and lost the numbers it had been asking of her, so old
sessions no longer showed what she had been aiming at.

She once made a shared exercise her own and her progress chart split in two, as though she had
started from nothing on the day she changed a word in the notes.

All three are why part 3 exists.

---

## 2. What a person does with it

### 2.1 Getting an account

A person signs up with an email address and a password, and stays signed in between visits.
Signing up is open to anyone who can reach the installation, and the operator can close it
with one setting.

A person may change their password by proving they know the current one, and may change their
email address the same way. Recovering a forgotten password needs email, which the app cannot
send yet. It is coming; see part 5.

### 2.2 The shared catalog

The app ships a catalog that every account can read. It has three levels:

- **Exercises.** One thing you do. The only thing the catalog holds on its own.
- **Sessions.** An ordered list of exercises, divided into named sections.

A section is part of the session that holds it, not a separate item in the catalog. A session
shows which of its sections is a warm-up and which is a cool-down.

An exercise carries a name, notes, where it came from, clips, tags, and whatever numbers
describe how it is normally done — how many, how heavy, how long, how long to rest — and
whether those count per side. Most are optional and most exercises leave most of them unset.

What the app does with an exercise follows from how it is performed. Some are counted in sets
and reps — four sets of eight. Some are driven by a timer, with fixed work and rest — hang for
seven seconds, rest for three, repeat. Climbing is recorded as attempts on single problems, not
as sets. Some are simply done, with nothing counted. Those are the ways that exist today.

A session may offer a choice rather than a fixed exercise: pick two of these five. A choice
says how many of its options must be picked, and some choices may be passed over entirely.

A session also carries a colour and a note about what equipment it assumes, both shown before
the person leaves home.

### 2.3 A person's own material

A person may write their own exercises and sessions.

They may also change anything in the shared catalog for themselves — the name, the notes, the
numbers, which exercises it contains, the order of them, all of it. The change applies to that
person and to nobody else.

The app improves its own catalog over time. Those improvements have to be able to reach a
person who has changed something for themselves, and they must never quietly undo what that
person changed.

A person can see which shared items they have changed, and may put one back the way it
shipped.

### 2.4 Catalogs loaded from files

The shared catalog is loaded from a set of files that ship with the app. A person may point
the app at a second set of files of their own. What those files describe is visible only to
them, and they may edit it afterwards inside the app.

A person may also download their own exercises and sessions as files, singly or
several at once, to keep or to hand to someone else.

### 2.5 Planning

Set up a training cycle. Say how long it runs, either as a length of time or by naming the date
being trained towards. Give it a name, notes, a purpose and tags. Write goals, each as where the
person is now, where they want to be, and how they intend to get there.

Say how long its repeating block is. Seven days is the common case and makes training land on
the same weekdays every time, but a block of any length must work: a person running ten days
will have their sessions walk through the week instead.

Place each session within that block. Move one to another day, add one, or take one out.

Set what the cycle asks of an exercise, and say where in the cycle that changes — easing off
for a deload, for instance.

Schedule a one-off session by hand, with no cycle behind it.

A cycle that would run across a trip or an injury warns the person first, shows exactly which
days collide, and offers to leave those days out.

### 2.6 Running a session

Open a session on the day and work through it. A timer counts preparation, work, rest between
reps and rest between sets, sounds when rest ends, and can be paused, stepped through, and
muted.

A person may do the exercises in any order rather than only the next one, skip one, add one,
remove one, change one, or pull in a whole section part-way through. They may leave the
session part-done and pick it up later. They may finish early, in which case everything not
reached is recorded as skipped rather than left hanging.

They may also start a session with nothing in it, name it, and build it as they go.

While doing an exercise a person sees what they last did on it, writes notes against it, and
records what they actually did, set by set or as one summary.

When the session offers a choice, the person picks from it and may look at each option's
detail and clip first.

After finishing, the person sees a summary: what was done, what was skipped, how long it
took, the climbs logged, and how this compares with the last several times they did the same
session.

### 2.7 Climbing

Climbing is recorded one attempt at a time: the grade, how it was climbed, how many tries,
indoors or outdoors, on a board or not, how long it took, a rating out of three stars, what
the person was working on, and what they thought of it.

A person keeps a list of the places they climb — gyms and crags, each with a name and a
location — and a separate list of the boards they train on. A place may be renamed or
removed.

Logging the same climb again, or logging the next one in a circuit, starts from the last one
so nothing is retyped. While logging, the person sees a running count of climbs, sends and
the hardest grade so far.

### 2.8 Recording and editing afterwards

Enter a session that already happened, backdated, including one that was never planned. A
half-finished entry can be left and returned to.

Go back to a session already recorded and change any part of it, however long afterwards:
correct the numbers, add an exercise that was forgotten, add a climb that was forgotten, remove
one, reorder them into the order they were really done, fix the date, fix where it happened,
add notes that were not written at the time.

This is normal, not an exception. A common way to use the app is to record almost nothing in
the gym — the session happened, on this day, at this place — and fill the rest in that evening
or that weekend. A record is never finished.

None of that weakens the rules above. A person may keep editing for as long as they like, and in
the same period the session they ran may be renamed, changed or deleted, with none of it
appearing in what they recorded. See rule 6.

A person may also keep a journal, with no training behind it at all. A date, a title, and
whatever they want to say: a thought about a boulder, a note that a finger is sore, a plan for
the weekend. A person may write one on a day they did not train. It counts as a journal entry and never as a
session, so it inflates no total and breaks no streak.

Naming a place while writing up a session creates it if it does not exist yet.

### 2.9 Looking back

Everything done, grouped by week. A calendar. A heat map of activity by day. Current and
longest streak. Progress on one exercise over time. A pyramid of climbing grades. A send
rate. How many times a given session has been completed. How many of the sessions a cycle
planned were actually done, as the cycle went along. Average sleep, energy and effort. Indoor against
outdoor. Which sessions are done most. A trend over the last several weeks.

A person may narrow all of this to a span of time.

### 2.10 The body and the calendar

Weight over time, height, ape index, best pull-up count, best hang weight, and the date a
grade was first climbed.

Trips, injuries, rest periods, deloads and competitions, each over a range of days, each with
a kind that gives it a colour, and some of which block training.

---

## 3. The rules that must hold

Every one of these was learned from a real failure.

### 3.1 A record of the past never changes by itself

1. A record is copied when it is written, and nothing that happens afterwards reaches it.
   Renaming an exercise, editing its numbers, or deleting it outright leaves every earlier
   record exactly as it was. This covers every name a record shows: the exercise, the session,
   the section it sat in, and the gym or crag it happened at. Rename the gym and last year's
   sessions still name the old one. A past session must still read in full after everything it
   was built from has been deleted.

4. What was asked of the person is part of the record. A session done under a four-week cycle
   still shows that cycle's numbers after the cycle is deleted.

5. What a session asks of the person must not change once that session has begun. Before it
   begins there is no copy yet: a session scheduled for a future day follows the session it
   points at, so editing that session tonight changes what tomorrow asks. The copy is taken
   when the person presses start.

6. A record changes when its owner changes it, and at no other time. That is the whole of the
   distinction: the rules above forbid change arriving from anywhere else, and say nothing about
   the person editing their own history whenever they like.

   These two pull against each other and both must hold. A person may keep filling in a
   session for weeks after it happened. In the same weeks, the session they ran it from may be
   renamed, reordered, have exercises swapped out, or be deleted entirely, and none of that may
   reach the record.

### 3.2 Identity

8. A name is never an identity. Two unrelated exercises may share one: a "hang" on a
   fingerboard and a "hang" on the wall are not the same exercise.

9. When a person changes a shared exercise for themselves, their record of doing that
   exercise must stay whole: one history, covering before and after the change. This must
   still hold after a second such change, and a third.

10. An identifier, once used, is never handed out again. A person deletes a session, creates
    a new one the next day, and last year's records must not start pointing at the new one.

11. A target set against an exercise must keep applying after the person changes that
    exercise for themselves.

12. An exercise typed straight into a write-up, matching nothing the person has saved, charts
    on its own and never merges with other such entries.

### 3.3 Shared material and personal material

13. The app ships a catalog. Every account reads it. No account's changes may alter what
    another account reads.

14. A person may load a private catalog from files. Only that person sees it. They may edit
    what it loaded.

15. A person who has changed a shared exercise for themselves must still be able to receive
    a later improvement to it, and must never lose their change to one.

16. Loading a catalog from files again must update what changed in the files, and must not
    silently overwrite anything the person has since edited by hand. The person must be told
    what was skipped.

17. A person's change is kept whole or not at all, and it covers everything inside the thing
    they changed. They must never be left with half of their own edit.

18. An exercise that leaves the files must disappear from the library while continuing to work
    in every plan and record already using it.

19. Renaming a file, or renaming the exercise inside it, must not produce a second exercise
    alongside the first.

20. Two people may load the same set of files. Each gets their own, and neither sees the
    other's.

21. Deleting an account disables it at once and erases everything belonging to that person
    thirty days later. It never removes anything the app ships. The thirty days exist so that
    a mistaken delete does not destroy a year of training, and the email address stays taken
    until they pass.

### 3.4 Planning and numbers

22. Correcting an exercise must reach every session that uses it.

23. When a person adapts a shared session, rules 22 and 9 must still hold for every exercise
    inside it.

24. The app must never create material on a person's behalf. Anything that becomes theirs,
    they asked for.

25. Numbers may be planned for each set separately. A hangboard ladder of 7 seconds, then 10,
    then 12, inside one set, repeated four times, is a normal thing to plan and to record
    against.

26. The same number can be set in more than one place. What the person is asked for must be
    the same on every screen.

27. A planned ramp must never collapse. If a ladder plans 7, 10 and 12 seconds and something
    else asks for 10, the person gets 7, 10 and 12.

28. A weight a person has set for themselves must be honoured wherever that exercise appears,
    including inside a session the app ships.

29. Zero kilograms is a real answer and means bodyweight. The app must also be able to tell a
    number nobody gave from one somebody did.

30. Which units a person reads and types in is their preference. Changing it must never change
    a number already recorded, and two people looking at the same shared exercise may be
    reading it in different units.

31. The difference between what was asked and what was done must be visible while the person
    is still in the gym.

### 3.5 Structure must stay sound

32. A session must never be able to contain itself, however deep the nesting.

33. A set of options may not contain another set of options, and an option may never itself be
    a set of options.

34. A record shows the exercises the person actually did. A choice they were offered must not
    read as something they did.

35. Reordering must never lose or duplicate an item. Dragging the fourth of six exercises to
    the top must leave six exercises.

36. Recording the same exercise as done twice — a double tap, a retried request, two devices —
    must count once.

37. Scheduling the same session on the same day twice must not quietly produce two of them.
    Building the same cycle twice must not double everything it planned.

### 3.6 Climbing

38. Whether a climb counts as a send follows from how it was climbed and is never asked as a
    separate question.

39. A grade is optional. Some climbing has no grade, such as a traverse or a continuous lap,
    and such a climb is never a send.

40. Grades come in several systems, the person chooses which they read, and grades must sort
    correctly within their own system regardless. A grade dropped from a system must still
    sort, so that old climbs stay in the pyramid.

41. Places are named and may be renamed or deleted. Past records keep the name the place had
    on the day.

42. Climbs recorded years ago, under older words for the same things, must still read
    correctly today.

### 3.7 Time and the day

43. "Today" is the person's day, in their own time zone, not the server's. Every streak, heat
    map and calendar edge depends on this. Each record stores the local date it counted as,
    worked out from the person's time zone when the record is written. Nothing recomputes that
    date afterwards, so training abroad and flying home never moves a past day.

44. A streak must not break because today's session has not happened yet.

45. A session that was started and never finished must not appear in any statistic. Neither
    must a write-up still being typed.

46. A journal entry is not training. It counts towards no total, no streak and no average, and
    a person writing one on a rest day has still rested.

47. "What you last did" must ignore an exercise that was skipped.

48. The person's session called Power and the app's session called Power are two different
    workouts, and "times completed" must never add them together.

### 3.8 Recording without a network

**Nothing here is built yet. It is stated because it constrains the design more than any other
rule, and because adding it afterwards must not mean starting over. Part 4 lists the other
things that do not exist today. See also part 5.**

The need is narrow and worth stating precisely. A person trains in a basement gym with no
signal, on one device, and wants the session to arrive when they come back up the stairs. They
are not editing the same session from two places at once.

49. A person must be able to record a whole session on a device that cannot reach the server,
    and have it arrive intact when that device reconnects.

50. Where a person has used more than one device, the app must be able to reconcile the work
    recorded on each.

### 3.9 Operating it

51. Upgrading must never require a second command.

52. PostgreSQL is the only supported database. The design may use what it offers.

53. The app must refuse to open a database written by a newer version of itself.

54. The app must refuse to start if the database cannot enforce the rules the design depends
    on.

55. A file the app cannot read must stop it loudly and name the file. It must never load half
    of itself.

56. Rolling back is restoring a backup. There is no reverse migration.

57. Nothing has shipped yet, so the first migration is still free to change. That stops being
    true the first time real training data exists.

---

## 4. What the app does not do today

Stated so that nothing is built for it now. Some of it is coming; see part 5.

- It never sends mail, so there are no notifications, no reminders and no password recovery.
- It stores no uploaded files. Clips are links to somewhere else.
- There is no search across the app, only a search by name within the lists of material.
- There is no administrator inside the app. Operator tasks are command-line commands.
- There is no sharing between accounts, no following, no comments, no teams and no coaching
  relationship. Accounts cannot see each other at all. See part 5.
- A person cannot yet choose their units; the app shows and accepts one fixed set throughout.
  See rule 30.
- It assumes a browser with a connection. It is not installable, it does nothing useful when
  the signal drops, and it is not built for a phone screen first.

---

## 5. Where it is going

Far off, and deliberately not being built now. It is stated because a design that makes any of
it impossible is the wrong design.

- **Being usable from a phone, not only from a laptop browser.** BoardSesh, the closest thing
  to this app in its own corner, is built as one repository holding a web client and a phone
  client, both of which are ordinary customers of a single server that exposes its data
  directly rather than rendering pages. Whatever form this takes here, the consequence for the
  design is the same: a person's training has to be reachable by something that is not a
  rendered web page, and usable from more than one device.

- **Recording while offline**, which is rules 49 and 50 and the reason they are stated at all.
  A phone in a basement gym is the whole point of putting this on a phone. Worth knowing:
  BoardSesh does not appear to do this. It is real-time over a live connection. This is a place
  where we want more than the nearest comparable app, so it will not be solved by copying one.

- **Email**, once there is something to send it with: confirming a new account, recovering a
  forgotten password, reminding a person of a session they planned, and telling them things
  worth knowing. That means an email address has to be something the app can prove belongs to
  the person, which today it never checks.

- **People handing material to each other.** Some form of friends, or at least a way to give
  someone an exercise or a session you wrote. Three things have to survive it. The person
  receiving it must be able to change it without that reaching the person who wrote it. The
  person who wrote it must be able to keep improving their own without rewriting what the other
  now has. And neither person's record of training must ever merge with the other's.
