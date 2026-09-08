# Schema v2 — adversarial design review

> **Snapshot, not the plan.** This records a review as it stood on its own date. Decisions
> have moved since — see `V2_PLAN.md` §1.10 (private trees as owned content, fork-on-edit,
> `source_tree`) and §1.11 (`content_key` replacing the log's text link). **Where this file
> disagrees with `V2_PLAN.md` or `SCHEMA_V2.sql`, those two win.** Nothing here is edited to
> keep up. That is what makes it a snapshot.

Reviewing `docs/schema-v2-journey.html` against the new constraint: one schema definition, in
Go and GORM, that runs on PostgreSQL, MySQL/MariaDB and SQLite.

Every engine-behaviour claim below has a documentation URL in section 5. Two GORM claims were
verified by reading the source in this machine's module cache, with file and line given. No
number or quotation in this document was produced from memory.

---

## 1. Verdict

The spine is right: one wall between content and history, prescriptions on the use of a
movement rather than on the movement, and a log that renders with every content table dropped.
It is not fundamentally unsound and it does not need to be rethought from scratch. But the
single mechanism the whole ownership model hangs on — partial unique indexes on a nullable
author — does not exist on MySQL or MariaDB, and GORM drops the predicate silently rather than
failing, so on two of the three target engines this design produces exactly the two bugs it was
written to prevent; that plus the `content` supertype and `account_metric` are three real
rewrites, not tweaks.

---

## 2. Defects, ranked by severity

### D1 — Partial unique indexes do not port, and GORM fails silently. `PORTABILITY` `CORRECTNESS`

**What is wrong.** The design's two load-bearing indexes are

```sql
CREATE UNIQUE INDEX ux_shipped ON content(slug)           WHERE author_id IS NULL;
CREATE UNIQUE INDEX ux_mine    ON content(author_id,slug) WHERE author_id IS NOT NULL;
```

PostgreSQL supports partial unique indexes. SQLite has supported them since 3.8.0. MySQL's
`CREATE INDEX` grammar has no `WHERE` clause. MariaDB's has none either.

**Why it is worse than a missing feature.** GORM does not tell you. I read the installed source:

- `gorm@v1.31.1/migrator/migrator.go:830-862` — the generic `CreateIndex` builds
  `CREATE [class] INDEX ? ON ??` plus `USING`, `COMMENT` and `Option`. It never reads
  `idx.Where`.
- `gorm.io/driver/sqlite@v1.6.0/migrator.go:270-271` and
  `gorm.io/driver/postgres@v1.6.0/migrator.go:156-157` — only these two drivers append
  `" WHERE " + idx.Where`.
- `go-gorm/mysql/migrator.go` (fetched from master) contains no `idx.Where` and no
  `CreateIndex` override, so MySQL falls through to the generic migrator.

**Concrete consequence.** On MySQL and MariaDB the two tags become:

```sql
CREATE UNIQUE INDEX ux_shipped ON content(slug);            -- predicate gone
CREATE UNIQUE INDEX ux_mine    ON content(author_id, slug); -- predicate gone
```

Two things then break at once, with no migration error:

1. `UNIQUE(slug)` across the whole table. The second user to create anything called
   `my-warmup` gets a constraint violation on someone else's row. Every user shares one slug
   namespace with the catalog and with each other.
2. `UNIQUE(author_id, slug)` does not constrain shipped rows, because all three engines treat
   NULLs in a unique key as distinct. So duplicate shipped slugs are allowed, and the YAML
   importer — which matches on slug — stops being idempotent. That is the same class of bug as
   the "every restart leaks a generation of rows" defect already recorded in the handover
   brief.

**Fix.** Section 3. A system account row, `owner_id NOT NULL`, and a plain
`UNIQUE (owner_id, kind, slug)`.

---

### D2 — `author_id IS NULL` makes absence of data carry meaning. `CORRECTNESS` `INTEGRITY`

**What is wrong.** "Shipped" is encoded as the absence of an author. Rule 1 says "There is no
second flag that can disagree with it." There is: `visibility`. Assumption 02 says shipped
content is public by definition, so a row with `author_id IS NULL` and
`visibility = 'private'` is a state the schema permits and the design forbids. Two columns can
state the same fact differently. This is the failure Date's *principle of orthogonal design*
names, and the design applies that principle correctly everywhere else.

**Concrete consequence.** Two, both bad:

1. Every content read carries three-valued logic: `author_id IS NULL OR author_id = :me`. An
   `OR` over a nullable column with two different predicates indexes poorly on all three
   engines, and it is easy to write as `author_id = :me` by accident, which silently returns an
   empty library. The brief records that exact failure mode in the current app, at about 20
   sites.
2. One wrong referential action promotes private content to catalog content. If
   `content.author_id` is ever declared `ON DELETE SET NULL` — a one-word mistake, and the
   design already uses `SET NULL` on `forked_from` and on the log's back-links — then deleting
   an account converts every one of that person's private sessions into shipped, ownerless,
   publicly readable catalog content. There is no constraint that could catch it.

**Fix.** `owner_id BIGINT NOT NULL`. Shipped means `owner_id = <system account>`, and
`visibility` becomes the only column that says who may read. See section 3.

---

### D3 — The `content` supertype is Class Table Inheritance, and it is the wrong one here. `PERFORMANCE`

**Name the pattern.** Fowler, *Patterns of Enterprise Application Architecture*, gives three
options for mapping a type hierarchy to tables: **Single Table Inheritance** (one table, all
subtype columns present and nullable, a discriminator column), **Class Table Inheritance** (one
table per class, joined 1:1 on a shared key), and **Concrete Table Inheritance** (one table per
concrete class, no shared table). The design is Class Table Inheritance. The rejected "separate
tables for shipped and user content" idea is Concrete Table Inheritance.

**Why CTI is wrong here.** Fowler's own trade-off for CTI is that loading one object needs a
join per level, and its payoff grows with how much the subtypes differ. Here the shared part is
fat and the subtypes are thin: `content` holds id, kind, slug, name, author, visibility and
forked_from, while `movement` adds a kind, notes, media and defaults, `block` adds
`block_kind` and `pick_count`, and `session` adds `color` and `needs`. That is the shape STI is
for.

Worse, the app's most common read is *across* the hierarchy. Step 02 of the journey says
"Sessions, blocks and exercises, all in one list." Under CTI that one list is a three-way
`LEFT JOIN` or three queries plus a merge in Go. Under STI it is one indexed scan.

**Concrete consequence.** Rendering one session under the design as drawn:

```
content c_s -> session s -> session_block sb -> block b -> content c_b
            -> block_item bi -> movement m -> content c_m -> block_item_set bis
```

Eight joins, three of them self-joins back onto `content` for the same table in three different
roles. You cannot get a movement's *name* without a join, because the name lives in `content`
and the FK points at `movement`. Under STI the same page is four joins and the names come free.

There is also a duplicated fact. `content.kind` says 'movement', and the existence of a
`movement` row says 'movement'. Nothing keeps them in step: a `content` row with
`kind='session'` and a `movement` child is legal. The design's Rule 1 argument against a second
disagreeing flag applies to its own supertype.

**What CTI buys, and how to buy it cheaper.** It buys one real thing: `block_item.movement_id`
can only point at a movement, because it references `movement(content_id)`. That is worth
keeping. Buy it with a composite foreign key into the supertype instead:

```sql
ALTER TABLE content ADD CONSTRAINT ux_content_id_kind UNIQUE (id, kind);
-- then
FOREIGN KEY (target_id, target_kind) REFERENCES content (id, kind)
```

`target_kind` is a `NOT NULL` column with a default. If it says 'movement' and `target_id`
points at a session row, the pair `(id,'movement')` is not in `content` and the insert fails.
The guarantee is carried by the FK, which is dependable on every engine, not by a `CHECK`,
which is not (D7).

**Fix.** One `content` table. `kind NOT NULL`. Subtype columns nullable on it. `movement`,
`block` and `session` deleted — three tables and three joins gone. Typed references done with
`(id, kind)` composite FKs.

**What it costs.** About ten columns that are only meaningful for one kind, and the schema no
longer forbids a session carrying `pick_count`. That is the honest price. Back it with a
`CHECK` per kind where `CHECK` is enforced, and with the one Go package that owns content
writes.

---

### D4 — The hot-path index cannot serve the hot-path query. `PERFORMANCE`

**What is wrong.** The design names its own hot query and says both its indexes are present:

```sql
SELECT ... FROM log_entry e JOIN log l ON l.id = e.log_id
WHERE l.account_id = :me AND e.movement_slug = :slug
ORDER BY l.on_date DESC LIMIT 5;
```

with `log_entry(movement_slug)` and `log(account_id, on_date)`. The filter spans two tables and
the sort key is on the *other* table from the selective predicate. No planner can use one index
for both.

**Concrete consequence.** On a self-hosted single-user database this is invisible. On the
hosted version it is not: `log_entry(movement_slug)` matches every user's rows for that slug,
across the whole table. The engine reads all of them, joins to `log` to discard other accounts,
then sorts. "Weighted pull-up" is a popular exercise. This runs once per movement per run
start, which the design calls the hot path.

**Fix.** Freeze `account_id` and `on_date` onto `log_entry` as well. The frozen-log principle
already justifies it — those are exactly as immutable as the movement name. Then

```sql
CREATE INDEX ix_le_hot ON log_entry (account_id, movement_slug, on_date DESC);
```

answers the query with no join and no sort. This is the largest single performance win in the
design and it follows the design's own rule rather than bending it.

---

### D5 — `account_metric` is EAV, and it is the wrong shape for three kinds. `NORMALIZATION` `CORRECTNESS`

**Confirmed. Your instinct is right.** `kind` / `value_num` / `value_text` with a `CHECK` that
exactly one value column is set is the antipattern Karwin names **Entity-Attribute-Value** in
*SQL Antipatterns*. EAV earns its keep when the attribute set is open and unknown at design
time. Yours is closed and known: body weight, and a boulder grade. Assumption 12 already moved
benchmarks like a max hang out to the log, so nothing else is coming.

**Four concrete costs.**

1. **No ordering on grades.** `value_text` holds 'V4', 'V9', 'V10'. Text-ordered, `'V10' <
   'V9'`. "Show my grade progression" is not expressible in SQL. This is the whole point of the
   table and it does not work.
2. **No per-kind domain.** Nothing stops `kind='body_weight'` with `value_text='V4'` and
   `value_num` null, or a negative weight. The exclusive-or `CHECK` says one column is
   populated; it cannot say which one belongs to which kind. Encoding that needs `kind` inside
   the `CHECK`, which grows one branch per kind — and it is unenforced on MySQL before 8.0.16
   and MariaDB before 10.2.1 anyway.
3. **No unit.** `value_num` is a bare number. kg or lb, cm or in. The account carries "which
   grade system you read in", so grades have a system; weights have nothing.
4. **No uniqueness.** "One measurement on one day" is stated in prose and enforced by nothing.
   Two body weights on one date, and every chart is wrong.

**The correct alternative — two real tables with real columns.**

```sql
CREATE TABLE body_measurement (
  account_id BIGINT       NOT NULL,
  on_date    DATE         NOT NULL,     -- TEXT on SQLite, see D6
  weight_kg  NUMERIC(6,2) NULL,
  height_cm  NUMERIC(5,1) NULL,
  span_cm    NUMERIC(5,1) NULL,
  CONSTRAINT pk_body_measurement PRIMARY KEY (account_id, on_date),
  CONSTRAINT fk_bm_account FOREIGN KEY (account_id)
    REFERENCES account(id) ON DELETE CASCADE,
  CONSTRAINT ck_bm_weight CHECK (weight_kg IS NULL OR weight_kg > 0)
);

CREATE TABLE grade_milestone (
  account_id    BIGINT      NOT NULL,
  on_date       DATE        NOT NULL,
  discipline    VARCHAR(16) NOT NULL,   -- boulder | sport | board
  grade_system  VARCHAR(16) NOT NULL,   -- v | font | french
  grade_label   VARCHAR(16) NOT NULL,   -- 'V7', '7A'
  grade_ordinal INTEGER     NOT NULL,   -- the sortable, averageable value
  CONSTRAINT pk_grade_milestone PRIMARY KEY (account_id, discipline, on_date),
  CONSTRAINT fk_gm_account FOREIGN KEY (account_id)
    REFERENCES account(id) ON DELETE CASCADE
);
```

`grade_ordinal` is the part that matters. Store the integer you can sort, average and plot, and
keep the display label beside it. That is the standard treatment of an ordered categorical in
dimensional modelling — Kimball's advice to carry a sortable attribute rather than deriving
order from a label at query time. `height_cm` moves off `account` here too; assumption 01 calls
it unchanging and it is not (see D14).

**Cost:** one extra table. **Gain:** every chart becomes a plain `ORDER BY`, and every invariant
becomes a column type or a primary key rather than a `CHECK` you cannot rely on.

---

### D6 — `TEXT` dates: the decision is right, the type is wrong on two engines. `PORTABILITY`

**The decision is right.** A calendar date with no time and no offset is exactly correct for a
training log, and the brief already records the production bug caused by doing otherwise
(`scheduled_date` storing local midnight with a `+05:30` offset in a column SQLite compares as
text).

**The type is wrong.** PostgreSQL has `date`: 4 bytes, one-day resolution, no time zone. MySQL
has `DATE`: `'YYYY-MM-DD'`, date part with no time part, no time zone. Both give you range
queries, index use, and format validation for free. SQLite has no date storage class at all —
its five storage classes are NULL, INTEGER, REAL, TEXT and BLOB — so ISO-8601 TEXT is the
documented and correct choice there, and only there.

**What GORM does, and why it is a trap.** `time.Time` on the MySQL driver becomes `datetime`,
not `date` (verified in `go-gorm/mysql` `getSchemaTimeType`, which returns
`"datetime" + precision`). So the obvious Go type reintroduces a time component and invites the
exact bug you just fixed.

**Fix.** Do not use `time.Time` for a calendar date. Define one date-only type:

```go
type Date struct{ Y, M, D int }   // or wrap civil.Date

func (Date) GormDataType() string { return "date" }
func (d Date) GormDBDataType(db *gorm.DB, f *schema.Field) string {
    if db.Dialector.Name() == "sqlite" { return "text" }
    return "date"
}
func (d Date) Value() (driver.Value, error) { /* 'YYYY-MM-DD' */ }
func (d *Date) Scan(v any) error            { /* parse both text and time.Time */ }
```

One Go type, three correct column types, one wire format. If you instead keep TEXT everywhere:
ISO-8601 does sort lexicographically, so comparisons still work — but only while every writer
zero-pads, which is a discipline invariant, and assumption 15 says this design does not accept
those. On MySQL a TEXT date also collates under `utf8mb4_0900_ai_ci` and needs a prefix length
before it can be indexed.

---

### D7 — `CHECK` is not a dependable primary mechanism. `INTEGRITY`

**The real support matrix.**

| Engine | `CHECK` | Note |
|---|---|---|
| PostgreSQL | enforced | may not reference other rows or tables; assumed immutable |
| SQLite | enforced | the expression may not contain a subquery |
| MySQL | enforced **from 8.0.16** | before 8.0.16 `CHECK (expr)` was **parsed and ignored** |
| MariaDB | supported **from 10.2.1** (MDEV-7563) | deterministic functions only; no `AUTO_INCREMENT` columns |

So every row the design marks "hard — CHECK" is soft on MySQL 5.7 and 8.0.0–8.0.15, and soft on
MariaDB 10.1 and earlier. For an app you intend strangers to self-host, that is not a corner
case.

**The problem the design does not mention, and it is the bigger one.** SQLite's `ALTER TABLE`
can rename a table, rename a column, add a column and drop a column. It cannot add or drop a
`CHECK` constraint, cannot add a `UNIQUE` constraint, and cannot change a column's type. The
documented workaround is a twelve-step table rebuild. GORM's `AutoMigrate` creates constraints
when it creates the table; it will not rebuild an existing one. So a self-hoster who upgrades
gets the new column and silently does **not** get the new constraint, with no error. Every
`CHECK` you add after v1 exists only on fresh databases.

**What to do where `CHECK` is not dependable.**

1. **Assert the engine at startup.** Read the server version and refuse to boot below MySQL
   8.0.16 / MariaDB 10.2.1. One query, one clear error message, and the whole matrix row
   disappears.
2. **Prefer structure over `CHECK` for anything load-bearing.** `NOT NULL`, `PRIMARY KEY`,
   `UNIQUE` and `FOREIGN KEY` are dependable on all four engines. The composite-FK typing in D3
   converts the design's most important `CHECK` (exactly one of two nullable FKs) into an FK.
   Do that everywhere you can.
3. **Where `CHECK` is the only option**, treat it as defence in depth: validate in the one Go
   package that owns writes, and add a test that reads `sqlite_master` /
   `information_schema.check_constraints` and asserts the constraint is actually present on the
   live database. That test is what turns "we declared it" into "it is there".
4. **Stop using `AutoMigrate` for constraint changes.** Use it for tables, columns and indexes.
   Put constraint DDL in one explicit, idempotent, per-engine step, and accept that SQLite means
   a table rebuild.

---

### D8 — `ON DELETE CASCADE` is not sound enough to be the account-deletion mechanism. `INTEGRITY`

**Portability, per engine.**

- **PostgreSQL** — dependable.
- **MySQL/MariaDB** — InnoDB supports `ON DELETE CASCADE`. Two caveats: only some storage
  engines check foreign keys at all (InnoDB and NDB), and **cascaded foreign key actions do not
  activate triggers**. If you later add an immutability or audit trigger on the log — and the
  brief says triggers already sit beside `AutoMigrate` — it will not fire on a cascade.
- **SQLite** — foreign key enforcement is **off by default** and must be enabled separately for
  **each database connection**. The brief confirms this repo does it with `_foreign_keys=on` in
  the DSN. That covers the app's own pool. It does not cover a second DSN, a backup or repair
  tool, a `sqlite3` shell, or a migration path that opens its own handle. Delete an account over
  such a connection and every child row is orphaned, invisible and unreachable.
- **GORM** — `AutoMigrate` creates the FK constraints, and
  `DisableForeignKeyConstraintWhenMigrating: true` removes them. Whether your cascades exist at
  all is a config flag.

**Why this matters more than a normal integrity bug.** "Delete my account" is an erasure
promise. Under the design as written, its correctness is the conjunction of: every FK declared,
on every table, with the right action, created by `AutoMigrate`, on an engine that enforces FKs,
over a connection with enforcement switched on. Nothing verifies that conjunction, and the
failure is silent.

**Fix.** Keep the cascades as a backstop and make the mechanism explicit:

1. One `DeleteAccount(tx, id)` function that deletes in dependency order, in one transaction.
2. It refuses the system account id (see section 3).
3. A test that inserts a full account fixture, deletes, and asserts zero rows for that
   `account_id` in every table that has one — enumerated from the model list, so a new table
   fails the test until it is handled.
4. A startup assertion that `PRAGMA foreign_keys` returns 1 on SQLite, and that the MySQL
   default storage engine is InnoDB.

Keep the design's rejection of soft delete. It is correct here, and it is what makes real
erasure possible. The brief's note that soft deletes are `UPDATE`s that fire no cascade is
exactly why.

---

### D9 — Comma-separated `labels`, `needs`, `source`. `NORMALIZATION`

**Confirmed 1NF violation.** A repeating group inside one column. Codd's first normal form
excludes repeating groups; Date states it as every attribute holding exactly one value from its
domain. Karwin names this one too: **Jaywalking**.

**The concrete cost, beyond what the design already admits.**

1. **False positives.** `labels LIKE '%fingers%'` matches `fingerboard`. Silently wrong
   results, not an error.
2. **No index is usable, on any engine.** A leading-wildcard `LIKE` cannot use a B-tree index.
   Every tag filter is a full table scan of `content`.
3. **On MySQL you cannot even add the useless index.** GORM's MySQL driver emits `longtext` for
   a `string` field with no size and no index (verified in `getSchemaStringType`: no size and no
   index/PK/default → `longtext`). InnoDB's index key prefix limit is 3072 bytes on `DYNAMIC` or
   `COMPRESSED` rows and 767 on `REDUNDANT` or `COMPACT`, about 191 utf8mb4 characters, so a
   `longtext` needs an explicit prefix length before it can be indexed at all.
4. **Renaming a tag is string surgery.** Rename `fingers` and you corrupt `fingerstrength`.
   Non-atomic, and unreviewable.
5. **The workaround is not portable.** MySQL and MariaDB have `FIND_IN_SET`. PostgreSQL and
   SQLite do not. So the one query that is merely slow on MySQL does not run at all elsewhere.
6. **No vocabulary.** You cannot list the distinct tags, count usages, or stop a typo becoming a
   new tag.

**The replacement.**

```sql
CREATE TABLE tag (
  id       BIGINT      NOT NULL,
  kind     VARCHAR(16) NOT NULL,   -- 'label' | 'gear'
  slug     VARCHAR(64) NOT NULL,
  name     VARCHAR(64) NOT NULL,
  CONSTRAINT pk_tag PRIMARY KEY (id),
  CONSTRAINT ux_tag_kind_slug UNIQUE (kind, slug)
);

CREATE TABLE content_tag (
  content_id BIGINT NOT NULL,
  tag_id     BIGINT NOT NULL,
  CONSTRAINT pk_content_tag PRIMARY KEY (content_id, tag_id),
  CONSTRAINT fk_ct_content FOREIGN KEY (content_id)
    REFERENCES content(id) ON DELETE CASCADE,
  CONSTRAINT fk_ct_tag FOREIGN KEY (tag_id)
    REFERENCES tag(id) ON DELETE CASCADE
);
CREATE INDEX ix_content_tag_tag ON content_tag (tag_id, content_id);
```

`needs` (gear) is a second vocabulary, not a second table — one `tag.kind` column carries both.
`source` is **not** a tag: it is provenance prose, and `CLAUDE.md` requires it to be able to
name a coach or programme. Keep it as one text column. Only split it out if one item can cite
two sources.

**Do this now, not at "20 tables later".** The design prices it as "two tables added" and then
leaves it out of the recommended 18. Retrofitting after the catalog ships means a data migration
by string splitting, on live rows, which is precisely the operation you cannot review.

---

### D10 — Foreign keys are unindexed, and only one engine covers for you. `PERFORMANCE`

**What is wrong.** The design names two indexes. It names no index on any foreign key column.

- **MySQL/InnoDB** — "Such an index is created on the referencing table automatically if it does
  not exist." You are covered by accident.
- **PostgreSQL** — "the declaration of a foreign key constraint does not automatically create an
  index on the referencing columns", and the docs say it is often a good idea to add one because
  a delete on the parent requires a scan of the referencing table.
- **SQLite** — no automatic index either.

**Concrete consequence.** The same schema is fast on MySQL and slow on PostgreSQL and SQLite —
the two engines a self-hoster is most likely to run. Account deletion is the worst case: every
`ON DELETE CASCADE` on the parent triggers one full child-table scan per child table. A user
with a year of logs makes their own deletion an O(n) sweep of `log_entry`, `log_set` and
`log_climb`.

**Fix.** Declare an index on every FK column in the model and stop relying on the engine. At
minimum: `content.owner_id` (covered by the composite unique), `content.forked_from`,
`content_share.content_id`, `content_share.account_id`, `block_item.block_id`,
`block_item.target_id`, `session_block.session_id`, `session_block.block_id`, `log.account_id`
(covered by `(account_id,on_date)`), `log.session_id`, `log_entry.log_id`,
`log_entry.movement_id`, `log_set.log_entry_id`, `log_climb.log_entry_id`, `plan.account_id`,
`plan_slot.plan_id`, `scheduled.plan_id`, `scheduled.session_id`, `calendar_event.account_id`,
`place.account_id`, and the two new metric tables' `account_id` (covered by their PKs).

---

### D11 — The frozen log: right in intent, three real gaps. `CORRECTNESS`

**Is it correct practice? Yes, and the reasoning holds.**

**Which patterns it resembles.** In Kimball's dimensional vocabulary this is a fact table that
carries its dimension attributes *as values*, rather than as foreign keys into a dimension that
can change under it. Kimball's usual answer to "what did it look like that day" is a **Type 2
slowly changing dimension**: a new dimension row per change, with effective dates, and the fact
pointing at the version that was current. This design instead snapshots into the fact. It is not
event sourcing — there is no event stream that state is derived from, and no replay. It is not
an append-only ledger either, for the reason in gap 1 below. It is a snapshot/audit table.

**Type 2 would be wrong here, and the design is right to reject it.** Type 2 requires the
dimension to survive. Rule 3 requires history to render with every content table dropped, and
user content is hard-deleted with the account (D8). A Type 2 dimension cannot satisfy that.
Snapshot copies can. So the denormalization follows from a stated requirement, not from
laziness. Defend it.

**Gap 1 — it is not actually frozen.** Nothing in the design makes a finished `log`, `log_entry`
or `log_set` row immutable. A frozen record you can `UPDATE` is not frozen; it is a record
nobody has changed yet. Add `finalized_at` and a rule that no finalized row may be updated. The
brief already establishes the mechanics: triggers work beside `AutoMigrate`, and a naive
`BEFORE UPDATE OF col` trigger aborts ordinary saves because GORM's `Save()` writes every
column, so it needs a `WHEN OLD.x IS NOT NEW.x` guard. Trigger DDL is not portable, so be
honest: one per-engine trigger where you can, plus the one Go package, plus the test.

**Gap 2 — it does not freeze enough.** It freezes the session name and colour, and the movement
name, slug and targets. It does not freeze the *units of measurement*. `log_climb.grade` is
text and the account carries "which grade system you read in"; change that setting and every
past climb renders in the wrong system. Same for weight if you ever offer lb. Freeze
`grade_system` on `log_climb` and `weight_unit` on `log_set`. This is the design's own argument,
applied consistently.

**Gap 3 — the frozen slug is not a stable key.** Now, the attack and the defence.

*Defence:* the slug survives deletion, which an id does not, and that is exactly the
requirement. Assumption 10 is sound.

*Attack:* a slug has one meaning only inside one owner's namespace. Two users can each own a
movement with slug `deadhang`, and they are different exercises. Any analytics that groups by
`movement_slug` alone merges them. That is harmless while every query is account-scoped, and
wrong the first time you build a leaderboard, a "compare with other climbers" page, or an
aggregate over the whole instance — which a hosted version will want.

*Fix:* freeze both keys. `movement_slug` for the owner's own history, plus
`catalog_key VARCHAR(160) NULL` holding the shipped YAML slug when the movement came from the
catalog, directly or through a fork. `catalog_key` has one meaning across every account, because
you control the YAML. Then write down the invariant that makes it true: **a shipped slug is
immutable.** The importer matches on slug, so renaming a slug in YAML creates a new row rather
than updating one — that is already the behaviour, it just needs saying, and a test.

---

### D12 — `movement.d_reps` vs `block_item.reps`: not a normalization violation, but the read-time fallback is a mistake. `NORMALIZATION` `CORRECTNESS`

**Is it a normalization violation? No.** Normal forms are about functional dependencies within a
relation. `d_reps` depends on the movement; `reps` depends on the use of a movement in a block.
Different attributes of different relations that share a domain. 3NF is intact. Date's principle
of orthogonal design is the one that would apply, and it does not fire here either, because the
two columns record different facts: a default and a prescription. **It is an acceptable
defaulting pattern.**

**Three real problems with the implementation.**

1. **The resolution rule lives in application code, at every read site.** `COALESCE(bi.reps,
   m.d_reps)` written by hand in the run path, the session preview, the export and the analytics
   query will drift. The brief documents twenty read sites in the current app that each had to
   remember a rule and did not. Fix: one definition. A `VIEW` with the `COALESCE` is portable
   across all three engines, or one Go function that every caller uses. The design already gets
   this right on the write side — `log_entry.t_*` stores the resolved value, so history is
   immune.

2. **NULL is carrying meaning again.** `block_item.reps IS NULL` means "use the movement's
   default", so you cannot express "this block deliberately prescribes no rep count" — a timed
   hang, which is a large part of a climbing catalog. Make the timed case its own column, or add
   an explicit prescription kind, so absence never has to be interpreted.

3. **A default change silently rewrites future prescriptions.** The importer "matches on slug
   and updates the row in place." So editing `d_reps` in YAML changes the prescription of every
   `block_item` that left `reps` blank — including in a user's fork, because the design does not
   copy movements (assumption 05). History is safe. Next Tuesday's session is not. That
   contradicts the spirit of assumption 03. **Best fix, and it removes problem 1 as well:**
   resolve the default at *seed and author time*, so a `block_item` always carries its own
   numbers, and `movement.d_*` becomes an authoring convenience that is never read at run time.
   The fallback disappears from the read path entirely.

---

### D13 — The self-referencing tree: right structure, wrong model, and no cycle guard. `CORRECTNESS`

**Against the standard options.** Celko, *Trees and Hierarchies in SQL for Smarties*, lays out
adjacency list, path enumeration (materialized path) and nested sets; the closure table is the
fourth, popularised by Tropashko and by Karwin.

Your tree is: session → block → (movement | nested block). Depth 3, maybe 4. Order matters.
Reads are frequent, writes are rare, and the whole subtree is always read at once, never
partially.

- **Nested sets — wrong.** Every insert renumbers a large fraction of the table, and ordered
  sibling insertion is its worst case. You have ordered siblings and you insert often while
  authoring.
- **Materialized path — wrong.** You would be back to string surgery in a column, and on MySQL
  a path column has the same prefix-index problem as `labels` (D9).
- **Closure table — wrong here.** It is for deep trees with ancestor/descendant queries. You
  have no ancestor queries, and it costs O(depth) rows per node plus maintenance.
- **Adjacency list — correct.** Shallow, ordered, read whole, written rarely. **Keep it.**

**But a tree is not the right model, and that is the more useful answer.** The design already
says unbounded nesting is a *cost*, not a feature — it lists "nothing stops a session containing
a session" as the reason to reject the 16-table version. If nesting is bounded at one level, say
so structurally: a `block_item` targets a movement or a menu, and a **menu cannot contain
another menu**. Then there is no recursion, no cycle risk, and no need for a recursive CTE —
which matters, because recursive CTEs need MySQL 8.0 and MariaDB 10.2.2, so a recursive read
path would raise your minimum engine versions for no gain.

**The gap that is live right now: there is no cycle protection at all.** A self-referencing
adjacency list with no depth bound lets `block_item` point at an ancestor block. `CHECK` cannot
detect a cycle — SQLite forbids subqueries in `CHECK`, and PostgreSQL forbids referencing any
row but the one being checked. So the schema cannot prevent it, and a render loops forever. That
is a denial of service on your own page, reachable by one authoring mistake or one malformed
YAML file.

**Fix.** Bound the nesting structurally (recommended), and belt-and-braces it with a `depth`
column, `CHECK (depth BETWEEN 0 AND 1)`, maintained by the one write function, plus a hard depth
cap in the renderer.

**And replace the exclusive-or `CHECK` with an FK.** The design's "a movement or a menu, never
both and never neither" is the right invariant, and it is the same shape as the production defect
the brief found in `exercise_media` (two nullable FKs, no `CHECK`, 373,212 rows). But as a
`CHECK` it is unenforced on MySQL before 8.0.16, and on SQLite it can never be added to an
existing table (D7). Use one `target_id` plus `target_kind` with a composite FK into
`content(id, kind)` (D3, section 3). One column instead of two, no exclusive-or to enforce, and
the FK carries the guarantee on every engine.

---

### D14 — Everything else. `mixed`

**`position` with no uniqueness, and the swap problem.** `CORRECTNESS`
`session_block`, `block_item` and `log_entry` all order by `position` with no unique constraint.
Duplicate positions render in an order that depends on the plan, and differs between engines.
But `UNIQUE(parent_id, position)` is a portable trap: SQLite has no deferrable unique
constraints (verified in this repo per the brief), and MySQL's `CREATE TABLE` grammar has no
`DEFERRABLE` clause at all, so only PostgreSQL could defer the check. A two-row swap therefore
fails on two of three engines.
*Fix:* keep `UNIQUE(parent_id, position)` and make reordering **rewrite the whole sibling list**
— one `DELETE` plus one `INSERT` inside a transaction, which never transiently duplicates a
position. The lists are short. Also always write `ORDER BY position, id` so ties are
deterministic everywhere.

**Idempotence has no unique key to stand on.** `INTEGRITY`
`scheduled` is written for all twelve weeks at plan creation, and the YAML importer writes the
catalog at every boot. Neither has a stated natural unique key. That is the same shape as the
brief's worst current bug — two identical boots doubling three tables, 9,039 dead rows against
1,119 live. *Fix:* a natural unique key on every table a generator writes into.
`UNIQUE(owner_id, kind, slug)` for content, `UNIQUE(plan_id, on_date, session_id)` for
`scheduled`, and upsert against it. GORM's `clause.OnConflict` is portable: the MySQL driver
rewrites `ON CONFLICT` to `ON DUPLICATE KEY UPDATE` (verified in `go-gorm/mysql/mysql.go`,
`ClauseBuilders`). One caveat — `ON DUPLICATE KEY UPDATE` fires on *any* unique key on the
table, not just the columns you named, so keep one unique key per generated table or the wrong
row gets updated.

**No optimistic locking anywhere.** `CORRECTNESS`
Two tabs editing one session, or two devices logging one run, and the last write wins silently.
Worse, Rule 2's fork-on-edit is a read-modify-write across four tables; two concurrent first
edits produce two forks. *Fix:* `version INTEGER NOT NULL DEFAULT 0` on `content` and `log`,
updated with `WHERE id = ? AND version = ?` and a zero-rows-affected check — Fowler's
*Optimistic Offline Lock*. Make fork-on-edit one transaction and let `UNIQUE(owner_id, kind,
slug)` make the second fork fail rather than duplicate.

**Collation makes the same schema behave differently per engine.** `PORTABILITY`
MySQL's default collation for `utf8mb4` is `utf8mb4_0900_ai_ci` — case-insensitive and
accent-insensitive. SQLite's default collating sequence is `BINARY`, which compares with
`memcmp()`. PostgreSQL takes the column's collation from the database locale and is
case-sensitive at ICU level 3. So `Deadhang` and `deadhang` collide on MySQL and coexist on the
other two, and the same signup email succeeds on one engine and fails on another. *Fix:* never
depend on the engine for case folding. Normalize slugs and emails to lowercase in Go before
writing, and unique-index the normalized value. Note that a PostgreSQL nondeterministic
collation is not the answer — it disables B-tree deduplication and is Postgres-only.

**No timestamps on content or log.** `PERFORMANCE`
You cannot sort a library by newest, answer "what changed since the last import", or debug an
import. GORM gives you `CreatedAt` and `UpdatedAt` for free. Add them.

**Missing keys and constraints.** `INTEGRITY`
`content_share` is drawn with two FKs and no primary key — add `PRIMARY KEY (content_id,
account_id)` or the same person can be given the same thing twice. `account.email` has no
`UNIQUE` shown; it needs one. `block_item_set` and `log_set` are drawn with `PK set_index` plus
a parent FK, which is a correct composite key — but the "free merges" section then proposes
turning `block_item_set` into a JSON array. Two sections of the same document disagree. Pick
one.

**JSON columns as "free merges".** `PORTABILITY`
The design's own test — never queried across rows — is the right test. Media and planned per-set
targets pass it; keep them as JSON. `plan_goal` does not: "one before-and-after goal for the
cycle" is something you will want to query ("show goals I hit"), so keep it as a table.
Portability note: PostgreSQL has `jsonb`, MySQL has a native `json` type, MariaDB's `JSON` is an
alias for `LONGTEXT` with a check, and SQLite has JSON functions over TEXT. Storing JSON is
portable; *querying inside it* is not. Never put a `WHERE` clause inside a JSON column.

**An abandoned run is indistinguishable from a journal entry.** `CORRECTNESS`
Assumption 08 writes the log before you lift anything, and assumption 11 says a log with no
entries is a journal entry. So a run you opened and walked away from *is* a journal entry, and
your completion rate is wrong. The brief already flags "run state sprawl" — five booleans, six
live combinations — in the current app. The fix is one enumerated `status` column plus
`started_at` and `completed_at`, not zero columns.

**Retired catalog items have nowhere to go.** `CORRECTNESS`
The importer "never deletes and rebuilds", so an item removed from YAML lingers in the library
forever with no way to retire it. The brief records Strong's pattern: hide built-ins rather than
delete them, and restore one through the history of a workout that used it. Add `retired_at` to
`content` — not a soft delete of user data, a retirement marker for catalog rows, so the library
can hide it while an existing plan that references it still resolves. This is the one place the
"no soft delete" rule needs an exception, and it is a different thing.

**The visibility rule is worse than "soft".** `PERFORMANCE`
The full read predicate is four OR'd branches on three columns plus an `EXISTS` into
`content_share`, on every content read on every page. Four OR'd branches on different columns is
what makes a planner give up and scan, on all three engines. *Direction, to be measured rather
than assumed:* give shipped rows an explicit `visibility = 'shipped'` so the predicate collapses
to `visibility IN ('shipped','public') OR owner_id = :me OR id IN (SELECT ...)`, index
`(visibility, kind, name)` and `(owner_id, kind, name)`, and split the query by case in the one
content package rather than OR-ing it. Measure before choosing. Also worth knowing what
engine-agnostic costs you here: PostgreSQL row-level security could make this structural on one
engine, and going portable gives that up. That is a real price and the owner should know he is
paying it.

**`account.height_cm` is not immutable.** `CORRECTNESS`
Assumption 01 puts on the account "only what does not change: email, password, height, ape
index". Height changes, and ape index is measured, not given. Both belong in
`body_measurement` (D5). Email changes too, so it needs the same care as any mutable unique key.

**Table count is not a design metric.** The 24 → 18 → 16 framing is the wrong axis, and the
16-table option is rejected for a slightly wrong reason. The cost of one node table and one edge
table is not the recursive query; it is losing the type distinction and gaining an unbounded
tree with no cycle guard (D13). Say that instead.

**Reserved words.** `PORTABILITY`
`position` is a non-reserved key word in PostgreSQL that cannot be a function or type name, but
it is fine as a column name. GORM quotes identifiers, so this is not a live problem — it becomes
one the moment you hand-write raw SQL. Just know it.

---

## 3. The portable replacement for the shipped-content mechanism

This is the important answer. Four options, then the pick.

### Option A — a system account row *(chosen)*

`account` holds one reserved row for the app. `content.owner_id BIGINT NOT NULL REFERENCES
account(id)`. Shipped content is owned by the system account. Uniqueness is a plain
`UNIQUE (owner_id, kind, slug)`.

**For:** a plain unique constraint, supported identically on PostgreSQL, MySQL, MariaDB and
SQLite, with no partial index, no generated column, no discriminator and no extra table. No
three-valued logic: `owner_id` is never NULL, so the visibility predicate is
`owner_id IN (:me, :system)` — a single index range on one column instead of an `OR` across two
predicates. Every FK is always valid. It matches the decision already recorded in this repo: a
real `owner_id`, with `0` and `NULL` both rejected.

**Against, stated honestly:** Rule "deleting an account cannot reach shipped content" stops being
structural. The system account *is* an account, and deleting it would cascade the catalog away.
That is a guard, not a wall. Mitigate it three ways: `DeleteAccount` refuses the system id;
`account.is_system` is checked at every deletion entry point; and a per-engine `BEFORE DELETE`
trigger where you can write one, since the brief confirms triggers already sit beside
`AutoMigrate`. Also: the system row needs an email and password hash that cannot log in — use a
reserved address and an impossible hash, and exclude `is_system` rows from the login query.

**Do not hardcode `id = 1`.** Identify the system account by a reserved email under the plain
`UNIQUE (email)` you already need, resolve its id once at boot, and cache it.

### Option B — a NOT NULL discriminator column in the unique key

Keep `author_id NULL`, add `owner_key NOT NULL` = `author_id` or `0`, and
`UNIQUE (owner_key, slug)`.

This is option A without the account row. It keeps the "no account can reach the catalog" wall
and needs no fake login. **Reject it** because two columns then encode one fact and can disagree
— exactly what Rule 1 forbids, and D2 all over again. Keeping them in step needs a trigger or a
generated column, which is option C.

### Option C — a generated/computed column in the unique key

`owner_key GENERATED ALWAYS AS (COALESCE(author_id, 0)) STORED`, then
`UNIQUE (owner_key, slug)`.

**Support is actually there:** SQLite since 3.31.0, indexable, with `UNIQUE` allowed. MySQL
`VIRTUAL` and `STORED`, both indexable, `UNIQUE` allowed. MariaDB `PERSISTENT` (with `STORED` as
an alias), and indexes on both `VIRTUAL` and `PERSISTENT` are supported. PostgreSQL `STORED`.

**Reject it anyway, for three reasons:**

1. **It is four dialects of the same clause.** MariaDB's keyword is `PERSISTENT`, and the
   default kind differs by PostgreSQL version — PG 17's docs say "PostgreSQL currently
   implements only stored generated columns", while current PG docs say "A generated column is
   by default of the virtual kind." The same DDL means different things on two PostgreSQL
   releases. GORM has no portable generated-column tag, so you would write a per-driver `type:`
   string and your model would stop describing one schema.
2. **SQLite can never add it later.** "It is not possible to ALTER TABLE ADD COLUMN a STORED
   column." Any existing SQLite database needs a full table rebuild, which `AutoMigrate` will
   not do.
3. **It keeps NULL meaning "shipped"**, so D2 stays.

### Option D — separate tables for shipped and user content

`catalog_movement` / `user_movement`, and so on. Concrete Table Inheritance.

**For:** uniqueness is trivial per table, and shipped content is structurally unreachable from an
account — the strongest version of Rule 1 available.

**Against:** every library read becomes a `UNION`; every polymorphic FK has to be solved twice;
forking crosses tables; and every content table doubles. The owner has already rejected this
explicitly — "just add a column to show the source" — and he was right that the source marker
was never what broke. **Reject.**

### Also considered and rejected: `UNIQUE NULLS NOT DISTINCT`

PostgreSQL's `NULLS NOT DISTINCT` would make `UNIQUE (author_id, slug)` constrain shipped rows
directly. It exists in PostgreSQL only; MySQL, MariaDB and SQLite have no equivalent in their
grammars. Naming it so nobody proposes it later.

---

### The DDL

ANSI SQL, valid on PostgreSQL, MySQL 8.0.16+/MariaDB 10.2.1+ and SQLite. Only the primary-key
auto-increment spelling differs per engine, and GORM writes that for you.

```sql
CREATE TABLE account (
  id            BIGINT       NOT NULL,   -- auto-increment; engine-specific spelling
  email         VARCHAR(320) NOT NULL,
  password_hash VARCHAR(255) NOT NULL,
  is_system     SMALLINT     NOT NULL DEFAULT 0,
  grade_system  VARCHAR(16)  NOT NULL DEFAULT 'v',
  created_at    TIMESTAMP    NOT NULL,
  updated_at    TIMESTAMP    NOT NULL,
  CONSTRAINT pk_account            PRIMARY KEY (id),
  CONSTRAINT ux_account_email      UNIQUE (email),
  CONSTRAINT ck_account_is_system  CHECK (is_system IN (0, 1))
);

CREATE TABLE content (
  id          BIGINT       NOT NULL,
  kind        VARCHAR(16)  NOT NULL,          -- movement | block | session
  owner_id    BIGINT       NOT NULL,          -- system account = shipped
  slug        VARCHAR(160) NOT NULL,
  name        VARCHAR(200) NOT NULL,
  visibility  VARCHAR(16)  NOT NULL DEFAULT 'private',
  forked_from BIGINT       NULL,
  retired_at  TIMESTAMP    NULL,
  version     INTEGER      NOT NULL DEFAULT 0,
  created_at  TIMESTAMP    NOT NULL,
  updated_at  TIMESTAMP    NOT NULL,

  -- movement-only, block-only and session-only columns live here too (D3)

  CONSTRAINT pk_content            PRIMARY KEY (id),
  CONSTRAINT ux_content_owner_slug UNIQUE (owner_id, kind, slug),
  CONSTRAINT ux_content_id_kind    UNIQUE (id, kind),
  CONSTRAINT fk_content_owner      FOREIGN KEY (owner_id)
    REFERENCES account (id) ON DELETE CASCADE,
  CONSTRAINT fk_content_forked     FOREIGN KEY (forked_from)
    REFERENCES content (id) ON DELETE SET NULL,
  CONSTRAINT ck_content_kind       CHECK (kind IN ('movement','block','session')),
  CONSTRAINT ck_content_visibility CHECK (visibility IN ('shipped','private','shared','public'))
);

CREATE INDEX ix_content_owner  ON content (owner_id, kind, name);
CREATE INDEX ix_content_vis    ON content (visibility, kind, name);
CREATE INDEX ix_content_forked ON content (forked_from);
```

`ux_content_id_kind` is what makes typed references possible without a subtype table:

```sql
CREATE TABLE block_item (
  id          BIGINT       NOT NULL,
  block_id    BIGINT       NOT NULL,
  block_kind  VARCHAR(16)  NOT NULL DEFAULT 'block',
  target_id   BIGINT       NOT NULL,
  target_kind VARCHAR(16)  NOT NULL,          -- movement | block
  position    INTEGER      NOT NULL,
  sets        INTEGER      NULL,
  reps        INTEGER      NULL,
  weight_kg   NUMERIC(6,2) NULL,
  CONSTRAINT pk_block_item      PRIMARY KEY (id),
  CONSTRAINT ux_block_item_pos  UNIQUE (block_id, position),
  CONSTRAINT fk_bi_block        FOREIGN KEY (block_id, block_kind)
    REFERENCES content (id, kind) ON DELETE CASCADE,
  CONSTRAINT fk_bi_target       FOREIGN KEY (target_id, target_kind)
    REFERENCES content (id, kind) ON DELETE RESTRICT,
  CONSTRAINT ck_bi_block_kind   CHECK (block_kind = 'block'),
  CONSTRAINT ck_bi_target_kind  CHECK (target_kind IN ('movement','block'))
);

CREATE INDEX ix_bi_target ON block_item (target_id);
```

The important property: if `target_kind = 'movement'` and `target_id` points at a session row,
the pair `(id,'movement')` is not in `content` and the insert fails. The guarantee is carried by
the foreign key, which is dependable on every engine — not by the `CHECK`, which is not (D7).
The two `CHECK`s are a second line only.

**The one place this costs you.** GORM's association tags cannot express a composite FK into a
non-primary unique key. So `fk_bi_block` and `fk_bi_target` must be added by explicit DDL beside
`AutoMigrate` — and on SQLite they must be present in `CREATE TABLE`, because SQLite's
`ALTER TABLE` cannot add a constraint. Concretely: keep `AutoMigrate` for tables, columns and
indexes, add one small per-engine map of `ALTER TABLE ... ADD CONSTRAINT` statements applied once
and idempotently, and for SQLite create these two tables from raw DDL. If "exactly one schema
definition, no per-engine SQL" is a hard rule, then the composite FK is out and you fall back to
the `CHECK`-based exclusive-or with its known holes. Pick with that trade-off in front of you.

### Seeding, with this shape

The importer writes `WHERE owner_id = :system` and upserts on `(owner_id, kind, slug)`. Booting
twice changes nothing, and it is enforced by a constraint rather than by a filter someone has to
remember. GORM's `clause.OnConflict` is portable across the three drivers (the MySQL driver
rewrites `ON CONFLICT` to `ON DUPLICATE KEY UPDATE`). Keep exactly one unique key on `content`
that the importer can collide with, or `ON DUPLICATE KEY UPDATE` will update the wrong row.

### The GORM struct tags that produce it

```go
type Account struct {
    ID           uint64    `gorm:"primaryKey"`
    Email        string    `gorm:"size:320;not null;uniqueIndex:ux_account_email"`
    PasswordHash string    `gorm:"size:255;not null"`
    IsSystem     bool      `gorm:"not null;default:false"`
    GradeSystem  string    `gorm:"size:16;not null;default:v"`
    CreatedAt    time.Time
    UpdatedAt    time.Time
}

type Content struct {
    ID         uint64  `gorm:"primaryKey;uniqueIndex:ux_content_id_kind,priority:1"`
    Kind       string  `gorm:"size:16;not null;uniqueIndex:ux_content_owner_slug,priority:2;uniqueIndex:ux_content_id_kind,priority:2;index:ix_content_owner,priority:2"`
    OwnerID    uint64  `gorm:"not null;uniqueIndex:ux_content_owner_slug,priority:1;index:ix_content_owner,priority:1"`
    Slug       string  `gorm:"size:160;not null;uniqueIndex:ux_content_owner_slug,priority:3"`
    Name       string  `gorm:"size:200;not null;index:ix_content_owner,priority:3"`
    Visibility string  `gorm:"size:16;not null;default:private;index:ix_content_vis,priority:1;check:ck_content_visibility,visibility IN ('shipped','private','shared','public')"`
    ForkedFrom *uint64 `gorm:"index:ix_content_forked"`
    RetiredAt  *time.Time
    Version    int     `gorm:"not null;default:0"`
    CreatedAt  time.Time
    UpdatedAt  time.Time

    Owner Account  `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE"`
    Fork  *Content `gorm:"foreignKey:ForkedFrom;constraint:OnDelete:SET NULL"`
}
```

Four tag notes that matter:

1. **`size:` on every string is not optional.** GORM's MySQL driver emits `longtext` for a
   `string` field with no size and no index or default, and `varchar(191)` when the field is
   indexed, has a default, or is a primary key. Setting `size:` explicitly makes the three
   engines produce the same column and keeps you clear of InnoDB's 3072-byte index key limit.
2. **`uniqueIndex:<name>,priority:N`** is how composite unique keys are built — same index name
   across the fields, `priority` fixing the column order.
3. **Never use `where:` on an index.** It is silently dropped on MySQL (D1). If it appears
   anywhere in the models, the schema is not portable.
4. **`check:<name>,<expr>`** produces `CONSTRAINT <name> CHECK (<expr>)` at table creation only.
   It is not enforced on MySQL before 8.0.16 or MariaDB before 10.2.1, and `AutoMigrate` cannot
   add it to an existing SQLite table. Treat every `check:` tag as documentation plus defence in
   depth, never as the only enforcement (D7).

---

## 4. What is right and should be kept

- **The wall.** Content flows one way into the log, and nothing reads back across it. This is
  the design's best idea and every defect above leaves it intact.
- **Rule 3, the frozen log.** Deliberate, documented denormalization with a stated requirement
  behind it. The reasoning for rejecting a Type 2 dimension is correct: Type 2 needs the
  dimension to survive, and hard deletes mean it does not.
- **Prescriptions live on the use of a movement, not on the movement.** `block_item` as a
  junction carrying its own attributes is textbook and right.
- **No per-user override table, and no stored progression.** Assumption 07. Progression is read
  from history rather than guessed and stored. It matches what Hevy, Strong, TrainingPeaks and
  Crimpd converged on, per the brief's verified survey.
- **Real deletes, no soft delete of user data.** Makes erasure honest. Keep it, with the one
  `retired_at` exception for catalog rows (D14).
- **Fork on edit rather than overlay.** One rule, one code path, no reconciliation. Right call.
- **The honesty of the enforcement table.** Naming three invariants as unenforced, and naming the
  Go package boundary as a boundary rather than a constraint, is better engineering than most
  designs manage. Keep the habit. The corrections above are that the table's "hard" column is
  optimistic on MySQL and MariaDB, not that the column should not exist.
- **YAML as the source of the catalog, imported in place, never delete-and-rebuild.** Correct,
  and the double-boot row-count test is the right test.
- **A journal entry is a log with no entries, not a second table.** Right, once an abandoned run
  is a distinct state (D14).
- **Adjacency list for the composition tree.** The right one of Celko's four for a shallow,
  ordered, read-whole tree (D13).

---

## 5. Engine support matrix

Every row was read from the vendor's own documentation at the URL given. GORM rows marked
*(source)* were verified by reading the code in this machine's Go module cache or the project's
GitHub master.

| Construct | PostgreSQL | MySQL | MariaDB | SQLite | Doc |
|---|---|---|---|---|---|
| Partial / filtered index (`WHERE`) | Yes | **No** — no `WHERE` in the `CREATE INDEX` grammar | **No** — no `WHERE` in the grammar | Yes, since 3.8.0 | [PG](https://www.postgresql.org/docs/current/indexes-partial.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-index.html) · [MariaDB](https://mariadb.com/docs/server/reference/sql-statements/data-definition/create/create-index) · [SQLite](https://sqlite.org/partialindex.html) |
| Partial **unique** index | Yes — "enforces uniqueness among the rows that satisfy the index predicate" | No | No | Yes — "A partial index definition may include the UNIQUE keyword" | [PG](https://www.postgresql.org/docs/current/indexes-partial.html) · [SQLite](https://sqlite.org/partialindex.html) |
| NULLs distinct in a unique key | Yes, by default | Yes — "permits multiple NULL values" | Yes | Yes — "NULL values are considered distinct from all other values, including other NULLs" | [PG](https://www.postgresql.org/docs/current/ddl-constraints.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-index.html) · [SQLite](https://sqlite.org/lang_createtable.html) |
| `UNIQUE NULLS NOT DISTINCT` | Yes | No | No | No | [PG](https://www.postgresql.org/docs/current/ddl-constraints.html) |
| `CHECK` enforced | Yes | **From 8.0.16.** Before that `CHECK (expr)` was "parsed and ignored" | **From 10.2.1** (MDEV-7563) | Yes | [MySQL 8.0](https://dev.mysql.com/doc/refman/8.0/en/create-table-check-constraints.html) · [MariaDB 10.2.1 release notes](https://mariadb.com/docs/release-notes/community-server/old-releases/10.2/10.2.1) · [MariaDB CONSTRAINT](https://mariadb.com/docs/server/reference/sql-statements/data-definition/constraint) · [SQLite](https://sqlite.org/lang_createtable.html) |
| `CHECK` may contain a subquery | No | No — "Subqueries are not permitted" | No | No — "may not contain a subquery" | [PG](https://www.postgresql.org/docs/current/ddl-constraints.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table-check-constraints.html) · [SQLite](https://sqlite.org/lang_createtable.html) |
| `CHECK` may reference other rows/tables | No — "does not support CHECK constraints that reference table data other than the new or updated row" | No | No | No | [PG](https://www.postgresql.org/docs/current/ddl-constraints.html) |
| `ALTER TABLE` add/drop `CHECK` or `UNIQUE` | Yes | Yes | Yes | **No** — rename table, rename column, add column, drop column only | [SQLite](https://sqlite.org/lang_altertable.html) |
| Generated column, indexable | `STORED` yes | `VIRTUAL` and `STORED`, both indexable | `VIRTUAL` and `PERSISTENT`, both indexable | Yes, since 3.31.0 — "may have NOT NULL, CHECK, and UNIQUE constraints" | [PG](https://www.postgresql.org/docs/current/ddl-generated-columns.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table-generated-columns.html) · [MariaDB](https://mariadb.com/docs/server/reference/sql-statements/data-definition/create/generated-columns) · [SQLite](https://sqlite.org/gencol.html) |
| Default generated-column kind | **Changed:** PG 17 "implements only stored"; current docs "by default of the virtual kind" | `VIRTUAL` | `VIRTUAL` | `VIRTUAL` | [PG 17](https://www.postgresql.org/docs/17/ddl-generated-columns.html) · [PG current](https://www.postgresql.org/docs/current/ddl-generated-columns.html) |
| `ALTER TABLE ADD` a `STORED` generated column | Yes | Yes | Yes | **No** — "not possible to ALTER TABLE ADD COLUMN a STORED column" | [SQLite](https://sqlite.org/gencol.html) |
| Index on an expression | Yes | Yes — functional key parts | **No** — `index_col_name` is `col_name [(length)]` only | Yes, since 3.9.0, but "only in CREATE INDEX statements, not within UNIQUE or PRIMARY KEY constraints" | [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-index.html) · [MariaDB](https://mariadb.com/docs/server/reference/sql-statements/data-definition/create/create-index) · [SQLite](https://sqlite.org/expridx.html) |
| Native `DATE` type, no time zone | Yes — 4 bytes, 1-day resolution, "cannot have an associated time zone" | Yes — `'YYYY-MM-DD'`, "a date part but no time part", range 1000-01-01 to 9999-12-31 | Yes | **No** — "SQLite does not have a storage class set aside for storing dates and/or times" | [PG](https://www.postgresql.org/docs/current/datatype-datetime.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/datetime.html) · [SQLite](https://sqlite.org/datatype3.html) |
| Foreign keys enforced by default | Yes | InnoDB and NDB only — "InnoDB and NDB tables support checking of foreign key constraints" | InnoDB | **No** — "disabled by default (for backwards compatibility), so must be enabled separately for each database connection" | [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html) · [SQLite](https://sqlite.org/foreignkeys.html) |
| `ON DELETE CASCADE` | Yes | Yes — but "Cascaded foreign key actions do not activate triggers" | Yes | Yes | [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html) · [SQLite](https://sqlite.org/foreignkeys.html) |
| Auto index on the referencing FK columns | **No** — "does not automatically create an index on the referencing columns"; docs advise adding one | Yes — "created on the referencing table automatically if it does not exist" | Yes (InnoDB) | No | [PG](https://www.postgresql.org/docs/current/ddl-constraints.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table-foreign-keys.html) |
| `DEFERRABLE` constraints | Yes | **No** — absent from the `CREATE TABLE` grammar | No | No (verified in this repo, per the handover brief) | [MySQL](https://dev.mysql.com/doc/refman/8.4/en/create-table.html) |
| Index key length limit | — | InnoDB: 3072 bytes (`DYNAMIC`/`COMPRESSED`), 767 (`REDUNDANT`/`COMPACT`), ~191 utf8mb4 chars; max 16 columns per index | as MySQL | — | [MySQL](https://dev.mysql.com/doc/refman/8.4/en/innodb-limits.html) |
| Default collation, case sensitivity | From the database locale; case-**sensitive** at ICU level 3 | `utf8mb4_0900_ai_ci` — case- and accent-**insensitive** | as MySQL family | `BINARY` — "compares string data using memcmp()" | [PG](https://www.postgresql.org/docs/current/collation.html) · [MySQL](https://dev.mysql.com/doc/refman/8.4/en/charset-mysql.html) · [SQLite](https://sqlite.org/datatype3.html) |
| Recursive CTE | Yes | Documented in the 8.0 manual; that page records restrictions lifted in 8.0.14 and 8.0.19 | **Not verified** — check before relying on it | Yes | [MySQL](https://dev.mysql.com/doc/refman/8.0/en/with.html) · [SQLite](https://sqlite.org/lang_with.html) |
| GORM `where:` on an index → `WHERE` clause | Yes *(source: `driver/postgres@v1.6.0/migrator.go:156-157`)* | **No — silently dropped.** `gorm@v1.31.1/migrator/migrator.go:830-862` never reads `idx.Where`, and `go-gorm/mysql/migrator.go` has no override | **No**, same path | Yes *(source: `driver/sqlite@v1.6.0/migrator.go:270-271`)* | [GORM indexes](https://gorm.io/docs/indexes.html) |
| GORM `string` with no `size` and no index | `text` | `longtext` *(source: `go-gorm/mysql` `getSchemaStringType`)*; `varchar(191)` when indexed, PK, or with a default | as MySQL | `text` | [GORM models](https://gorm.io/docs/models.html) |
| GORM `time.Time` | `timestamptz` | `datetime` *(source: `getSchemaTimeType`)* — **not** `date` | `datetime` | `datetime` | [GORM models](https://gorm.io/docs/models.html) |
| GORM `clause.OnConflict` | `ON CONFLICT` | rewritten to `ON DUPLICATE KEY UPDATE` *(source: `go-gorm/mysql/mysql.go` `ClauseBuilders`)* | as MySQL | `ON CONFLICT` | [GORM create](https://gorm.io/docs/create.html) |
| GORM `AutoMigrate` scope | "create tables, missing foreign keys, constraints, columns and indexes … change existing column's type if its size, precision changed, or if it's changing from non-nullable to nullable"; "It **WON'T** delete unused columns" — and it will not rebuild an existing table to add a constraint | | | | [GORM migration](https://gorm.io/docs/migration.html) |
| GORM `check:` tag | `CONSTRAINT <name> CHECK (<expr>)` at table creation | subject to the `CHECK` row above | subject to the `CHECK` row above | cannot be added to an existing table | [GORM constraints](https://gorm.io/docs/constraints.html) |

---

## 6. Principles this design should be following

| Principle | Source | Followed? |
|---|---|---|
| First normal form — no repeating groups in a column | Codd | **No.** `labels`, `needs`, `source` (D9) |
| Third normal form on the content tables | Codd | **Yes**, and the log's break from it is deliberate and justified |
| Principle of orthogonal design — no two relations or columns able to record the same fact | Date | **No.** `author_id IS NULL` and `visibility` can disagree (D2); `content.kind` and the subtype table's existence can disagree (D3). **Yes** for `d_reps` vs `reps`, which are different facts (D12) |
| Avoid giving NULL a meaning; NULL is absence, not a value | Date and Darwen on three-valued logic | **No.** NULL means "shipped" (D2) |
| Single Table Inheritance when subtypes are thin and cross-type reads are common; Class Table Inheritance when they are not | Fowler, *Patterns of Enterprise Application Architecture* | **No.** It is CTI with thin subtypes and a cross-type read on every page (D3) |
| Optimistic Offline Lock for concurrent edits to one record | Fowler, *PoEAA* | **No.** No `version` column anywhere (D14) |
| Preserve history by storing the attribute values as of the event, not a key into a mutable dimension | Kimball, dimensional modelling — Type 2 SCD and its snapshot alternative | **Yes** in intent. Incomplete: not immutable, units not frozen, slug ambiguous across owners (D11) |
| Carry a sortable ordinal beside a display label for ordered categoricals | Kimball | **No.** Grades are text in `value_text` and cannot be ordered (D5) |
| Adjacency list for shallow, ordered, read-whole hierarchies; nested sets and closure tables for other shapes | Celko, *Trees and Hierarchies in SQL for Smarties* | **Yes** on the structure. **No** on the bound: no depth limit, no cycle guard (D13) |
| Do not use Entity-Attribute-Value for a closed, known attribute set | Karwin, *SQL Antipatterns* | **No.** `account_metric` (D5) |
| Do not store a delimited list in a column ("Jaywalking") | Karwin, *SQL Antipatterns* | **No.** (D9) |
| Every invariant is a constraint or an index, or it is named as unenforced | the design's own assumption 15 | **Partly.** Honest about the three soft rules, but silently optimistic about `CHECK` on MySQL and MariaDB, and about partial indexes on both (D1, D7) |
| An importer that runs on every boot must be idempotent against a real unique key | — | **No.** Its unique key is the one that does not port (D1, D14) |
