# Go structure and database layer — a researched reference

Written for the Passion rewrite: one developer, ~12k lines of handlers, HTMX and
`html/template`, GORM, PostgreSQL and SQLite now, MySQL/MariaDB possible later,
a JSON API later.

**Rules used to write this.** Every claim below has a link. Quotations are
verbatim from a page or file that was fetched. Where the community genuinely
disagrees, both sides are given. Anything unverified is marked
`COULD NOT VERIFY`.

One correction worth recording: during this research a web-search summariser
invented a Hacker News quotation about the project-layout thread. The Algolia
API shows those submissions scored 26, 3 and 2 points. The quote does not
exist and is not used here.

---

## The answer, in one page

Read this page. The rest is evidence for it.

**Layout.** `main.go` in the root, five packages under `internal/`: `config`,
`model`, `store`, `app`, `web`. No `pkg/`. Split by *file* freely, by *package*
reluctantly. Dave Cheney's guidance is literally titled "Consider fewer, larger
packages", and Russ Cox's minimal standard is a LICENSE, a go.mod, and "Go code
in your repo... as you see fit". Comparable apps run 11-30 packages; six is
deliberately at the small end.

**Do not follow `golang-standards/project-layout`.** Russ Cox filed an issue
against it titled "this is not a standard Go project layout", and its own README
says it is "an overkill" for a project like this.

**The store gets no interface.** GORM already owns the engine seam, so there is
exactly one implementation — and both Go's Code Review Comments and Google's
style guide say an interface needs two. Export a concrete `*store.Store`. If a
handler wants a fake in tests, the *handler's* package declares a three-method
interface. Gotify does exactly this and supports all three engines.

**The per-engine seam is three places, total.** GORM's `Dialector` interface is
eight methods and already covers identity columns, `DATE` vs text, adding a
CHECK to an existing SQLite table, placeholders, quoting, and error codes. What
is left for you:

1. connection setup — one `switch` in `store.Open()`, about 15 lines
2. the rare migration statement that must differ
3. raw SQL — the one real leak, so ban it outside `store`

Adding MySQL later is a new `case`, a driver import, and a CI run.

**Transactions — the one non-obvious correctness item.** Exported store methods
open the transaction; unexported helpers take `tx *gorm.DB` and never touch
`s.db`. This matters because GORM detects transaction nesting on *the handle*: a
method using `s.db` inside an outer `Transaction` silently opens a second,
independent transaction, and since `tx` and `s.db` are the same type, nothing
warns you.

**Context** goes first on every store method, and no struct holds one.

**Migrations:** goose, embedded, one numbered sequence, Go migrations as the
escape hatch. Never `AutoMigrate` in production — it never drops a column,
index, or constraint.

**SQLite, in the DSN so every pooled connection gets it:**
`_busy_timeout=5000` (first, order matters), `_journal_mode=WAL`,
`_foreign_keys=on`, **`_txlock=immediate`**, plus `SetMaxOpenConns(1)`.

`_txlock=immediate` is the highest-value item here and the easiest to miss.
`busy_timeout` **provably cannot** retry a deferred transaction's read-to-write
upgrade — SQLite's own docs say the busy handler is skipped when retrying could
deadlock. Django shipped the same fix. Also note the cgo driver already defaults
`busy_timeout` to 5000, so a missing timeout is likely not the cause of the
current 500s; a missing WAL and a deferred `BEGIN` are.

**Tests:** real databases, no store mocks, one suite run against both engines. A
**file** in `t.TempDir()` — not `file::memory:?cache=shared`, which breaks
`busy_timeout`.

**The five things I would push back on** — full versions in §5.3: SQLite and
multi-user genuinely conflict, so document the limit instead of engineering
around it; WAL + one connection is knowingly conservative; MySQL is not fully
additive because it has no partial indexes; the cgo-vs-pure-Go driver choice
gets more expensive the longer it waits; and `app/` is the package most likely
to earn deletion.

---

## 1. Go project layout

### 1.1 What the Go team says

`https://go.dev/doc/modules/layout` is the official page. For a server it says,
verbatim:

> Server projects typically won't have packages for export, since a server is
> usually a self-contained binary (or a group of binaries). Therefore, it's
> recommended to keep the Go packages implementing the server's logic in the
> `internal` directory. Moreover, since the project is likely to have many other
> directories with non-Go files, it's a good idea to keep all Go commands
> together in a `cmd` directory

and:

> It's recommended to keep packages in `internal` as much as possible.

The tree it shows for a server is `go.mod`, `internal/{auth,metrics,model}`,
`cmd/api-server/main.go`. The word `pkg` does not appear on the page at all.

Note its layout for a *single command* is flat: `go.mod`, `main.go`, `auth.go`,
`client.go` in the root, with no `cmd/`.

The other official source is the Go blog, `https://go.dev/blog/package-names`
(Sameer Ajmani, 4 February 2015):

> Avoid meaningless package names. Packages named util, common, or misc provide
> clients with no sense of what the package contains.

> Don't use a single package for all your APIs. Many well-intentioned
> programmers put all the interfaces exposed by their program into a single
> package named api, types, or interfaces, thinking it makes it easier to find
> the entry points to their code base. This is a mistake.

> If you cannot come up with a package name that's a meaningful prefix for the
> package's contents, the package abstraction boundary may be wrong.

The Go standard library is itself the best worked example: `net/http` is **one
package of ~30,000 non-test lines across 71 files**, split as `client.go`,
`server.go`, `transport.go` — separate files, not separate packages.

### 1.2 `internal/` — what it actually enforces

From the Go toolchain's own documentation (`go help gopath`, and
`src/cmd/go/alldocs.go` in the Go repo), verbatim:

> Code in or below a directory named "internal" is importable only
> by code that shares the same import path above the internal directory.

The example given: for module `example.com/m`, code in
`example.com/m/foo/internal/baz` can be imported by `example.com/m/foo`,
`example.com/m/foo/bar` and `example.com/m/foo/quux`, but **not** by
`example.com/m/crash/bang`.

- Source: <https://github.com/golang/go/blob/master/src/cmd/go/alldocs.go>
- Introduced in Go 1.4. From <https://go.dev/doc/go1.4>, verbatim: "The Go
  language does not have the power to enforce this distinction, but as of Go 1.4
  the go command introduces a mechanism to define "internal" packages that may
  not be imported by packages outside the source subtree in which they reside."

**Plain answer:** it is enforced by the `go` command, so a bad import fails the
build. It is not a language rule. `cmd/` and `pkg/` have no enforcement of any
kind — they are pure convention.

For a self-hosted application that nobody imports as a library, `internal/`
buys one real thing: it makes every rename and move a private matter. It costs
one directory level.

### 1.3 `golang-standards/project-layout` — do not follow it

The repository has ~56,500 stars and is not archived. Its README carries a
disclaimer, verbatim:

> This is **`NOT an official standard defined by the core Go dev team`**. This
> is a set of common historical and emerging project layout patterns in the Go
> ecosystem.

> **`If you are trying to learn Go or if you are building a PoC or a simple
> project for yourself this project layout is an overkill. Start with something
> really simple instead (a single main.go file and go.mod is more than
> enough).`**

**Russ Cox — the Go tech lead — opened an issue against it.**
<https://github.com/golang-standards/project-layout/issues/117>, titled "this is
not a standard Go project layout". Verbatim:

> The README makes clear that this is not official, but even the claim "it is a
> set of common historical and emerging project layout patterns in the Go
> ecosystem" is not accurate.
>
> For example, the _vast_ majority of packages in the Go ecosystem do _not_ put
> the importable packages in a `pkg` subdirectory. More generally what is
> described here is just very complex, and Go repos tend to be much simpler.
>
> It is unfortunate that this is being put forth as "golang-standards" when it
> really is not.

And in a [follow-up comment](https://github.com/golang-standards/project-layout/issues/117#issuecomment-828503689),
verbatim — the most useful passage in the whole thread:

> But for the record, the minimal standard layout for an importable Go repo is
> really:
>
> - Put a LICENSE file in your root
> - Put a go.mod file in your root
> - Put Go code in your repo, in the root or organized into a directory tree as
>   you see fit
>
> That's it. That's the "standard".

> It is not required to put commands in cmd/.
> It is not required to put packages in pkg/. […]
> The importable golang.org/x repos break every one of these "rules".

Others in the same thread:

| Who | Position | Link |
|---|---|---|
| rakyll (JBD, ex-Go team) | "I saw many projects trying to "fix" their layout based on what's provided as a reference here. It creates confusion and conflict among contributors." | [comment](https://github.com/golang-standards/project-layout/issues/117#issuecomment-821885559) |
| theckman (Gophers Slack admin) | Filed [#10](https://github.com/golang-standards/project-layout/issues/10) — "pkg directory should not be recommended for use"; "the Go community has been converging on the idea that the `pkg/` directory is a useless abstraction / indirection." | [comment](https://github.com/golang-standards/project-layout/issues/117#issuecomment-825908097) |
| Eli Bendersky | On `pkg/`: "in the majority of cases it's an antipattern. It's much better to start your project without it". Also: "90% of Go projects don't need a separate directory for their packages at all". | [post](https://eli.thegreenplace.net/2019/simple-go-project-layout-with-modules/) |

**The other side, stated fairly.** The demand is real, and Peter Bourgon (author
of go-kit) argued directly against Russ Cox's minimal answer, verbatim:

> I think what we're witnessing here, in the broad strokes, is that _this isn't
> enough_. Saying that we don't have to follow this or that guide, and that more
> or less everything is permissible, isn't helpful to the Gophers who are asking
> these questions.

— [comment](https://github.com/golang-standards/project-layout/issues/117#issuecomment-828784351)

**Verdict for this project: do not follow it.** It is not a standard, its own
README says it is overkill for a project like this, and the Go tech lead says it
is "far too complex". Take `cmd/` and `internal/` from the official page instead,
and skip `pkg/`, `api/`, `configs/`, `web/`, `build/`, `test/`.

### 1.4 Ben Johnson's position

His article is "Standard Package Layout"
(<https://www.gobeyond.dev/standard-package-layout/>, 10 August 2016). Four
tenets, verbatim:

> 1. Root package is for domain types
> 2. Group subpackages by dependency
> 3. Use a shared mock subpackage
> 4. Main package ties together dependencies

His follow-up, "Packages as layers, not groups"
(<https://www.gobeyond.dev/packages-as-layers/>, 12 January 2021), verbatim:

> Why are Go packages so different from other languages? It's because they're
> not groups—they're layers.

His reference application is WTF Dial, <https://github.com/benbjohnson/wtf>. Its
README states the approach verbatim:

> 1. Application domain types go in the root—`User`, `UserService`, `Dial`, etc.
> 2. Implementations of the application domain go in subpackages—`sqlite`, `http`, etc.
> 3. Everything is tied together in the `cmd` subpackages—`cmd/wtf` & `cmd/wtfd`.

> We also include interfaces for managing our application domain data types
> which are used as contracts for the underlying implementations. For example,
> we define a `wtf.DialService` interface for CRUD (Create/Read/Update/Delete)
> actions and SQLite does the actual implementation.

Measured: 6,080 non-test lines, 11 hand-written packages, no `internal/`, no
`pkg/`. Interfaces in the root; implementations in `sqlite/`, `http/`, `inmem/`,
`csv/`, `mock/`.

**Two honest caveats.**

First, **it puts interfaces on the producer side**, which is the opposite of
what Google's Go style guide recommends (see §3.4). He was asked this directly
in his own repo's discussions and answered, verbatim:

> I add the interfaces to the root so they can be shared by the subpackages. If
> the interface lived in the subpackage then you can get in situations where you
> have circular dependencies.

> Another option is to not use interfaces at all. […] You can define smaller
> interfaces but I just don't personally like doing it. I find it's a lot of
> extra types without a lot of value.

— <https://github.com/benbjohnson/wtf/discussions/73>

Second, **his root-package-holds-all-domain-types rule is in direct tension with
the official Go blog**, which says putting all your interfaces in one package is
"a mistake" (§1.1). Reasonable people read this both ways. His rule works
because the root package is named after the application, not after a layer.

He is also explicit that his own repo is over-documented on purpose: "Honestly,
too documented for most projects but the goal here is to be as clear as possible
for anyone reading the code."

His most directly useful line for a 12k-line app, from Standard Package Layout,
verbatim:

> Throwing all your code in a single package can actually work very well for
> small applications. […] I've seen this work for applications up to 10K SLOC.
> Beyond that size, it gets extremely difficult to navigate the code and isolate
> your code.

`COULD NOT VERIFY`: any substantial named blog post or forum thread attacking
his layout. The real, citable pushback is in his own repo's discussions (#7,
#73, #79).

### 1.5 What real codebases of this size actually do

Measured by clone and count; non-test, hand-written lines, generated files
excluded.

| Repo | Non-test LOC | Packages | `cmd/` | `internal/` | `pkg/` |
|---|---|---|---|---|---|
| [benbjohnson/wtf](https://github.com/benbjohnson/wtf) | 6,080 | 11 | yes | no | no |
| [dstotijn/hetty](https://github.com/dstotijn/hetty) | 6,304 | 13 | yes | no | **yes** |
| [gotify/server](https://github.com/gotify/server) | 9,595 | 30 | no | no | no |
| [miniflux/v2](https://github.com/miniflux/v2) | 44,243 | 87 | no | yes | no |
| [usememos/memos](https://github.com/usememos/memos) | 46,087 | 48 | yes | yes | no |

For contrast, larger ones by top-level directory count:
[hugo](https://github.com/gohugoio/hugo) 41 dirs, flat domain packages, no
`cmd/`, no `pkg/`; [prometheus](https://github.com/prometheus/prometheus) 23
dirs, `cmd/` and `internal/`, no `pkg/`;
[gitea](https://github.com/go-gitea/gitea) 21 dirs — `cmd models modules routers
services templates` — no `internal/`, no `pkg/`;
[grafana](https://github.com/grafana/grafana) uses `pkg/`.

**Nobody at this scale splits handlers per feature.** Every small app measured
keeps handlers in one package with many files:

| Package | Shape |
|---|---|
| [miniflux `internal/ui`](https://github.com/miniflux/v2/tree/main/internal/ui) | one package, 93 files, 6,220 lines, one file per route |
| [gotify `api`](https://github.com/gotify/server/tree/master/api) | one package, 10 files, 3,120 lines |
| wtf `http` | one package, 6 files, 1,664 lines |
| stdlib `net/http` | one package, 71 files, 30,114 lines |

### 1.6 Evidence on package count

This matters because the requirement is "fewer, larger, well-named packages".
There is authoritative guidance for exactly that, and it is Dave Cheney's.
<https://dave.cheney.net/practical-go/presentations/qcon-china.html>, §5.1, whose
title is literally **"Consider fewer, larger packages"**. Verbatim:

> One of the things I tend to pick up in code review for programmers who are
> transitioning from other languages to Go is they tend to overuse packages.

> The advice I find myself repeating is to prefer fewer, larger packages.
> Your default position should be to not create a new package.

> Coming from Java? If you're coming from a Java or C# background, consider this
> rule of thumb.
> - A Java package is equivalent to a single .go source file.
> - A Go package is equivalent to a whole Maven module or .NET assembly.

> Avoid elaborate package hierarchies, resist the desire to apply taxonomy

And on splitting a package into `client`/`server`/`common`, verbatim:

> I believe the solution to this is to reduce the number of packages, to combine
> the client, server, and common code into a single package named after the
> function of the package. For example, the net/http package does not have
> client and server sub packages, instead it has a client.go and server.go file,
> each holding their respective types, and a transport.go file for the common
> message transport code.

Google's Go style guide, <https://google.github.io/styleguide/go/best-practices#package-size>,
cuts both ways and is worth reading in full. Verbatim:

> if the user must import both packages in order to use either in any meaningful
> way, combining them together is usually the right thing to do.

> All of that being said, putting your entire project in a single package would
> likely make that package too large. When something is conceptually distinct,
> giving it its own small package can make it easier to use.

> Go style is flexible about file size, because maintainers can move code within
> a package from one file to another without affecting callers. But as a general
> guideline: it is usually not a good idea to have a single file with many
> thousands of lines in it, or having many tiny files. There is no "one type, one
> file" convention as in some other languages.

**Takeaway.** Split by file freely; split by package reluctantly. A file move
inside a package is invisible to callers, so files are the cheap unit and
packages are the expensive one.

---

## 2. The database layer with more than one engine

The brief changed while this was being researched. Separate per-engine
implementations are off; GORM stays. So the question is no longer "one
implementation or two" — it is **where the narrow per-engine seam should sit,
given GORM handles the bulk**.

The evidence below points the same way as the relaxed requirement, by roughly
eight to one. That is worth saying plainly — the decision to keep GORM is also
the decision the research supports — and so is the one dissenting project, which
is covered honestly in §2.1 rather than left out.

### 2.1 What real multi-engine Go projects actually do

Nine genuinely multi-engine Go projects had their store layer read for this
document. The census:

| Architecture | Count | Projects |
|---|---|---|
| One implementation + inline dialect switches, ORM/builder does the bulk | 6 | Gitea, GoToSocial, Woodpecker, Vikunja, **Gotify**, ory/kratos |
| Shared base struct + per-engine embed overriding only DSN and a few SQL strings | 2 | Authelia, Grafana |
| **Separate full per-engine implementations** | **1** | **Memos** |
| Code generation | 0 | none found |

Single-engine, read as counter-examples: **Miniflux** (PostgreSQL only),
**Mattermost** (PostgreSQL only since v11), **Navidrome** and **PocketBase**
(SQLite only), **Tailscale** (no SQL store at all).

**The honest answer to "does anyone maintain two near-identical
implementations?" is yes — one project, and it is worth looking at before
dismissing the idea.**
[Memos](https://github.com/usememos/memos/blob/main/store/driver.go) defines a
**65-method `Driver` interface** and implements it three times, in
`store/db/sqlite/`, `store/db/postgres/` and `store/db/mysql/`. The
directories have matching filenames and near-identical sizes (roughly 133 KB /
139 KB / 133 KB across ~28-30 files each). A matched pair was diffed to confirm
this is real duplication rather than a shared core: `sqlite/memo_comment.go` and
`postgres/memo_comment.go` differ only in the package name, the function names
(`authorizeSQLiteMemoComment` vs `authorizePostgresMemoComment`), the
placeholders (`?` vs `$1`), and one query fusion. Same logic, written twice.

So the pattern exists. It is also **one project in nine**, and the eight others
— including every one closest to this project in size and shape — chose a single
implementation. That is the evidence for the relaxed requirement, stated without
exaggeration.

Authelia and Grafana show the middle shape, and both deliberately stop short of
duplication. Authelia's per-engine files are **58, 414 and 64 lines** against a
**1,924-line shared `sql_provider.go`**, and each simply embeds the shared
struct:

```go
type SQLiteProvider struct {
	SQLProvider
}
```

Its PostgreSQL provider overrides just 7 SQL strings (the upserts) and calls
`db.Rebind` on ~109 others to turn `?` into `$N` —
[sql_provider_backend_sqlite.go](https://github.com/authelia/authelia/blob/master/internal/storage/sql_provider_backend_sqlite.go).

**The closest practical comparable is Gotify** — a small self-hosted Go app,
GORM, all three engines. **Its entire per-engine seam is about 15 lines in one
file, and there are zero dialect switches anywhere in its query code**
([database/database.go](https://github.com/gotify/server/blob/master/database/database.go)):

```go
switch dialect {
case "mysql":
    db, err = gorm.Open(mysql.Open(connection), gormConfig)
case "postgres":
    db, err = gorm.Open(postgres.Open(connection), gormConfig)
case "sqlite3":
    db, err = gorm.Open(sqlite.Open(connection), gormConfig)
}
```

plus a SQLite pool cap, a MySQL connection lifetime, and an `os.MkdirAll` for
the SQLite file's directory.

**The closest architectural comparable is GoToSocial** — PostgreSQL and SQLite
only, the same pair as this project. Its per-engine code is **about 3.7% of its
database layer** (~911 of ~24,343 lines), and the per-engine packages contain
only driver wrapping and error translation — no query logic. Note it has moved to
Codeberg: <https://codeberg.org/superseriousbusiness/gotosocial>.

Also useful as a counter-example: **Miniflux** supports **only** PostgreSQL, uses
plain `database/sql`, and has **no store interface at all** — just
`type Storage struct { db *sql.DB }` with 123 methods across 17 files.
Supporting one engine is a legitimate choice a respected project made. It is not
on the table here, but it is the baseline against which the cost of a second
engine should be judged.

#### What actually differs between PostgreSQL and SQLite, from the code

Collected from the projects above, roughly by how often it appeared:

| # | Difference | SQLite | PostgreSQL |
|---|---|---|---|
| 1 | Placeholders | `?` | `$N` |
| 2 | Case-insensitive match | `LIKE`; `LOWER()` is **ASCII-only** | `ILIKE` |
| 3 | String concatenation | `\|\|` | `CONCAT()` |
| 4 | DDL types | `INTEGER PRIMARY KEY AUTOINCREMENT`, `BLOB` | `SERIAL`, `BYTEA`, `TIMESTAMP WITH TIME ZONE` |
| 5 | `ALTER TABLE` | cannot alter default, nullability or type; dropping a column means a table rebuild | native |
| 6 | Introspection | `pragma_table_info()`, `sqlite_master` | `information_schema` |
| 7 | NULL sort order | — | sorts NULLs first ascending; needs explicit `NULLS LAST` |
| 8 | Row locking | `FOR UPDATE` unsupported / no-op | `FOR UPDATE`, advisory locks |
| 9 | Date functions | `DATE('now')` | `NOW()` |
| 10 | Scalar functions | `MAX(a,b)`; no `REGEXP` unless registered | `GREATEST(a,b)` |
| 11 | Error codes | driver-specific | driver-specific |
| 12 | Temp tables | no `ON COMMIT DROP` | `ON COMMIT DROP` |
| 13 | Full-text search | FTS5 or `LIKE` fallback | `to_tsvector` |

Row 2 is a divergence the audit did not list and is worth knowing: SQLite's
`LOWER()` folds ASCII only, so case-insensitive matching on non-ASCII text
differs. Gitea has a dedicated `BuildCaseInsensitiveLike` for exactly this
([models/db/common.go](https://github.com/go-gitea/gitea/blob/main/models/db/common.go)).

**Two things that do NOT differ, and are the usual reasons people
over-engineer this:**

- **Upsert.** Both use `INSERT ... ON CONFLICT ... DO UPDATE`. Only MySQL
  differs. GoToSocial's upsert wrapper treats PostgreSQL and SQLite as a single
  case and flags only MySQL.
- **`RETURNING`.** Supported by SQLite since 3.35.0. GORM's SQLite dialector
  detects the version and enables `RETURNING` automatically
  ([sqlite.go](https://github.com/go-gorm/sqlite/blob/master/sqlite.go)).

### 2.2 GORM already defines the seam — and it is eight methods

This is the single most important finding in this document. GORM's per-engine
boundary is a real interface in its source, and it is small.

From <https://github.com/go-gorm/gorm/blob/master/interfaces.go>, verbatim:

```go
type Dialector interface {
	Name() string
	Initialize(*DB) error
	Migrator(db *DB) Migrator
	DataTypeOf(*schema.Field) string
	DefaultValueOf(*schema.Field) clause.Expression
	BindVarTo(writer clause.Writer, stmt *Statement, v interface{})
	QuoteTo(clause.Writer, string)
	Explain(sql string, vars ...interface{}) string
}
```

Adding MySQL later means GORM already has a `Dialector` for it. Nothing in the
application implements this interface.

**Every divergence measured in the audit is already inside one of those
methods.** Checked against the drivers' actual source:

| Divergence | Where GORM handles it | Evidence |
|---|---|---|
| Identity columns | `DataTypeOf` | SQLite driver returns `"integer PRIMARY KEY AUTOINCREMENT"` for an auto-increment int. Postgres driver emits `<int> GENERATED BY DEFAULT AS IDENTITY` for `gorm:"generated:identity"`. |
| `DATE` real type vs text | `DataTypeOf` | SQLite driver maps `schema.Time` → `"datetime"`, `schema.Bool` → `"numeric"`, `schema.String` → `"text"`. |
| CHECK on an existing table | `Migrator` | SQLite driver's `CreateConstraint` / `DropConstraint` / `AlterColumn` / `DropColumn` all call `recreateTable`, which performs SQLite's documented table-rebuild procedure. |
| Placeholder syntax | `BindVarTo` | Postgres writes `$N`; SQLite writes `?`. |
| Identifier quoting | `QuoteTo` | Per driver. |
| Engine error codes | `ErrorTranslator` | See below. |

Sources: [sqlite driver `dataTypeOf`](https://github.com/go-gorm/sqlite/blob/master/sqlite.go),
[sqlite driver `migrator.go`](https://github.com/go-gorm/sqlite/blob/master/migrator.go),
[postgres driver `DataTypeOf`](https://github.com/go-gorm/postgres/blob/master/postgres.go).

The SQLite `recreateTable` finding is worth dwelling on, because "a CHECK cannot
be added to an existing table" is a real SQLite limitation. SQLite's own docs,
<https://www.sqlite.org/lang_altertable.html>, verbatim:

> SQLite supports a limited subset of ALTER TABLE. The ALTER TABLE command in
> SQLite allows these alterations of an existing table: it can be renamed; a
> column can be renamed; a column can be added to it; or a column can be dropped
> from it.

GORM's SQLite driver works around this in 489 lines of per-engine DDL code that
reads the existing `CREATE TABLE` from `sqlite_master`, rewrites it, copies the
rows into a temp table, drops the original, renames, and recreates the saved
indexes and triggers. That is code the project does not have to write or
maintain.

**Error translation is a seam worth switching on explicitly.** Set
`gorm.Config{TranslateError: true}` and each driver maps its native codes to
shared sentinel errors, so handlers compare against `gorm.ErrDuplicatedKey`
rather than an engine-specific code:

- Postgres: `23505 → ErrDuplicatedKey`, `23503 → ErrForeignKeyViolated`,
  `23514 → ErrCheckConstraintViolated`
  ([error_translator.go](https://github.com/go-gorm/postgres/blob/master/error_translator.go))
- SQLite: `1555, 2067 → ErrDuplicatedKey`, `787 → ErrForeignKeyViolated`
  ([error_translator.go](https://github.com/go-gorm/sqlite/blob/master/error_translator.go))

Gotify sets `TranslateError: true`. One asymmetry to know about, checked
across all three drivers: **only PostgreSQL maps the check-constraint case.**
MySQL's map is `1062 -> ErrDuplicatedKey`, `1451`/`1452 -> ErrForeignKeyViolated`;
SQLite's is as above. Neither includes a check violation. So
`gorm.ErrCheckConstraintViolated` is PostgreSQL-only, and will stay that way when
MySQL is added.

### 2.3 What GORM does *not* cover — this is your seam

Three things, and only three.

**1. Connection construction.** DSN, pool limits, SQLite pragmas. GORM cannot
guess these. One `switch` in one function. Covered in §2.6.

**2. Migration statements that must differ.** GORM's `Migrator` handles DDL when
you call `AutoMigrate`, but versioned migrations are your SQL. Covered in §2.7.

**3. Raw SQL.** Verified from the `Dialector` interface above: placeholder
binding and quoting happen in `BindVarTo` and `QuoteTo`, which are only reached
through GORM's builder. Anything passed to `db.Raw` or `db.Exec` is sent as
written. **Raw SQL is the one place a per-engine difference leaks silently into
application code.** The rule that follows is simple: no raw SQL outside the
store package, and prefer GORM's builder plus `clause.OnConflict` so the seam
stays GORM's problem.

That is the entire answer to "where should the seam sit". Adding MySQL later
means: add a case to the connection switch, add a migration set, and audit any
raw SQL. Additive, not invasive.

### 2.4 Interface granularity — and why you probably want no interface at all

**The origin of "accept interfaces, return structs".** It is usually cited
without a source. It traces to Jack Lindamood (`cep21`), "Preemptive Interface
Anti-Pattern in Go", 23 June 2016,
<https://medium.com/@cep21/preemptive-interface-anti-pattern-in-go-54c18ac0668a>,
verbatim:

> A great rule of thumb for Go is **accept interfaces, return structs**.
> Accepting interfaces gives your API the greatest flexibility and returning
> structs allows the people reading your code to quickly navigate to the correct
> function.

> Interfaces are an abstraction and abstraction is sometimes useful. However,
> **unnecessary abstraction creates unnecessary complication**. Don't over
> complicate code until it's needed.

His [follow-up post](https://medium.com/@cep21/what-accept-interfaces-return-structs-means-in-go-2fe879e25ee8)
is careful to say it is not absolute, verbatim: "this isn't a hard rule". His
reasoning is worth keeping because it explains the asymmetry: you control your
return type, so you know when to abstract it; you cannot anticipate your
caller's input, so the bias for abstraction sits on the input side. He also names
an exception directly relevant here — for a *private* function "there is no
ambiguity on function input since *you control* that, so bias towards not
preemptive abstraction."

Google's style guide calls it "an adage" and phrases it as "Functions should take
interfaces as arguments but return concrete types"
(<https://google.github.io/styleguide/go/decisions#interfaces>).

The authoritative statement of the underlying rule is in Go's own Code
Review Comments, <https://go.dev/wiki/CodeReviewComments>, verbatim:

> Go interfaces generally belong in the package that uses values of the
> interface type, not the package that implements those values.

> Do not define interfaces on the implementor side of an API "for mocking";
> instead, design the API so that it can be tested using the public API of the
> real implementation.

> The implementing package should return concrete (usually pointer or struct)
> types: that way, new methods can be added to implementations without requiring
> extensive refactoring.

> Do not define interfaces before they are used: without a realistic example of
> usage, it is too difficult to see whether an interface is even necessary, let
> alone what methods it ought to contain.

That page's own example labels the producer-defines-the-interface version
`// DO NOT DO IT!!!` and closes: "Instead return a concrete type and let the
consumer mock the producer implementation."

Google's Go style guide is more direct still.
<https://google.github.io/styleguide/go/best-practices#avoid-unnecessary-interfaces>,
verbatim:

> designing a "service" or a "repository" or similar pattern doesn't mean you
> need a named interface type (e.g., `type Service interface`). Focus on the
> behavior and its concrete implementation first.

> Every exported type increases the cognitive load for the reader. When you
> export a test double alongside the real implementation, you force the reader
> to understand three entities (the interface, the real implementation, and the
> test double) instead of one.

And on when an interface *is* justified, verbatim:

> Multiple implementations: When there are two or more concrete types that must
> be handled by the same logic […] the API consumer could define an interface.

From the same guide's
[interface ownership section](https://google.github.io/styleguide/go/best-practices#interface-ownership-and-visibility),
verbatim:

> The consumer defines the interface: In Go, interfaces generally belong in the
> package that uses them, not the package that implements them. The consumer
> should define only the methods they actually use

with a named exception that is exactly the case this project no longer has:

> The interface is the product: When a package's primary purpose is to provide a
> common protocol that many different implementations must follow, the producer
> defines the interface. For example, `io.Writer`, `hash.Hash`.

Backing this, a Go proverb from Rob Pike (Gopherfest SV 2015,
<https://go-proverbs.github.io/>): **"The bigger the interface, the weaker the
abstraction."**

**This has a direct consequence.** Once GORM owns the engine seam, the store has
**one** concrete implementation, not two. By the rule above, that removes the
reason to define a `Store` interface at all. A giant `Store` interface would be
a producer-side interface with a single implementor, which is the case both
sources tell you to avoid.

**Gotify is a live example of the recommended shape.** Its `database` package
exports a concrete type and no interface:

```go
type GormDatabase struct {
	DB *gorm.DB
}
```

Its `api` package — the *consumer* — declares a small interface with only the
methods that one handler group uses
([api/message.go](https://github.com/gotify/server/blob/master/api/message.go)):

```go
// The MessageDatabase interface for encapsulating database access.
type MessageDatabase interface {
	GetMessagesByApplicationSince(appID uint, limit int, since uint) ([]*model.Message, error)
	GetApplicationByID(id uint) (*model.Application, error)
	// … 6 more
}

type MessageAPI struct {
	DB       MessageDatabase
	Notifier Notifier
}
```

That is per-aggregate, consumer-side, and minimal — Code Review Comments,
Google's guide and the Go proverb all at once, in a real app of comparable size.

**One empirical datapoint on the giant-interface end.** A published refactor
report describes reaching "130+ methods in one single interface! That's a lot of
methods and that is not what SOLID interface should look like", and splitting it
per aggregate so that `api.Storage.GetUserByID(ctx, id)` became
`api.Storage.User().Get(ctx, id)` —
<https://maddevs.io/blog/effective-refactoring-of-heavy-database-interface/>.
That is the failure mode a single `Store` interface grows into, and the fix was
per-aggregate splitting. Starting with no interface avoids the whole arc.

**The disagreement, stated plainly.** Ben Johnson does the opposite: producer-side
interfaces in the root domain package (§1.4). His stated reason is avoiding
circular imports between subpackages. That reason is real, but it applies when
you have several implementation subpackages that must share contracts. With one
GORM-backed store, there is no cycle to break.

### 2.5 Transactions

The audit found 4 `Begin()` calls against ~41 `Create` calls, tied to real
inconsistent-state bugs. So the test for any pattern here is not elegance. It is:
**does the atomic version get written by default, without discipline?**

#### GORM's closure, and the trap in it

From <https://github.com/go-gorm/gorm/blob/master/finisher_api.go>:

```go
func (db *DB) Transaction(fc func(tx *DB) error, opts ...*sql.TxOptions) (err error)
```

Reading the body verifies three good properties:

1. **It rolls back on a returned error *and* on a panic, and commits only on
   `nil`.** The source sets `panicked := true` and the deferred func rolls back
   `if panicked || err != nil`. It does not `recover()`, so the panic keeps
   propagating. The docs state the error half verbatim: "return any error will
   rollback" / "return nil will commit the whole transaction"
   (<https://gorm.io/docs/transactions.html>).
2. **`tx` is a `*gorm.DB` — the same type as the root handle.** So no `Querier`
   interface is needed; any function taking `*gorm.DB` works with either.
3. **It nests via `SAVEPOINT`.** The body checks
   `db.Statement.ConnPool.(TxCommitter)` and, if the handle is already a
   transaction, issues a savepoint rather than opening a second transaction.

**And here is the trap, which matters more than any of the three.** Property 3
is detected on **the handle it is called on**, not on any ambient state. A
function handed the **root** `*gorm.DB` that calls `db.Transaction(...)` opens a
**brand-new, independent transaction**. It does not nest, and it does not roll
back with the outer one.

Because property 2 makes `tx` and the root handle the *same type*, **the compiler
cannot tell you when this happens.** It is a silent correctness bug, not a build
error.

So this does **not** work:

```go
// WRONG: each store method uses s.db, so this is three separate transactions.
err := s.db.Transaction(func(tx *gorm.DB) error {
	if err := s.CreateContent(ctx, c); err != nil { return err }   // uses s.db
	return s.CreateItems(ctx, items)                                // uses s.db
})
```

Note also that GORM's own nested-transaction documentation example calls the
inner `Transaction` on **`tx`**, not on `db` — which is the whole point, easy to
miss when skimming. And `gorm.Config{DisableNestedTransaction: true}` turns the
savepoint behaviour off entirely.

#### How real projects solve it

Four distinct answers, all verified:

| Pattern | Project | Shape |
|---|---|---|
| Transaction in the `context` | [Gitea](https://github.com/go-gitea/gitea/blob/main/models/db/context.go), Authelia | `db.WithTx(ctx, func(ctx context.Context) error)`; stores read the engine out of `ctx` |
| Unexported funcs take `(ctx, tx)` inside one package | [WTF Dial](https://github.com/benbjohnson/wtf/blob/main/sqlite/dial.go) | exported method opens the tx, unexported package-level funcs do the work |
| Transaction provider builds tx-bound stores | [Three Dots Labs](https://github.com/ThreeDotsLabs/go-web-app-antipatterns/blob/master/04-transactions/05-tx-provider/tx.go) | `Transact(func(adapters Adapters) error)` |
| Push the whole multi-table write into one method | [Memos](https://github.com/usememos/memos/blob/main/store/driver.go) | `ApplyMemoMutation(ctx, mutation)` — but written three times, once per engine |
| Don't cross stores at all | Mattermost | `SqlReactionStore.Save` writes the Posts table with raw SQL rather than calling `PostStore` |

**Ben Johnson's is the one to copy, and the reason is structural.** From
`sqlite/dial.go`:

```go
func (s *DialService) CreateDial(ctx context.Context, dial *wtf.Dial) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil { return err }
	defer tx.Rollback()
	if err := createDial(ctx, tx, dial); err != nil { return err }
	return tx.Commit()
}
```

Every exported service method opens its own transaction. The multi-table work is
done by **unexported package-level functions taking `(ctx, tx, ...)`** —
`createDial` calls `insertDialValue(ctx, tx, ...)` and
`createDialMembership(ctx, tx, ...)`, the latter living in a different *file* of
the same package. Cross-entity atomicity works **because the store is one Go
package**, so those helpers can call each other without any interface, context
smuggling, or provider type.

That is a direct argument for the one-`store`-package recommendation in §5.1:
the package boundary is what makes the transaction problem easy or hard.

**Against the context approach**, which is otherwise tempting because Gitea uses
it: it hides the transaction from every signature, so a function taking `ctx` may
or may not be transactional and you cannot tell by reading it. The sharpest
statement of the objection, from <https://rednafi.com/go/repo-txn-uow/>,
verbatim:

> Context values are untyped and invisible. If someone forgets to set the
> transaction in context, or sets it on the wrong context, the store silently
> falls back to the connection pool and the operations aren't atomic.

That failure mode is the same class as GORM's root-handle trap, just moved.

#### Recommendation

**Exported store methods open their own transaction. Multi-table work happens in
unexported helpers that take `tx *gorm.DB` explicitly.**

```go
func (s *Store) CreateContentWithItems(ctx context.Context, c *model.Content, items []model.ContentItem) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createContent(ctx, tx, c); err != nil {
			return err
		}
		return createItems(ctx, tx, c.ID, items)
	})
}

func createContent(ctx context.Context, tx *gorm.DB, c *model.Content) error { ... }
func createItems(ctx context.Context, tx *gorm.DB, contentID uint, items []model.ContentItem) error { ... }
```

The rule that makes this hold, and it is one line: **an unexported store helper
takes `tx *gorm.DB` as a parameter; it never touches `s.db`.** Then a helper can
be reused from any transaction, composition is explicit in the signature, and
the root-handle trap cannot occur — because a helper has no access to the root
handle.

**On the aggregate boundary**, Three Dots Labs states the relevant rule
verbatim, and it answers the `content` / `content_item` question directly
(<https://threedots.tech/post/database-transactions-in-go/>):

> Anti-pattern: One repository per database table — Don't create a repository
> for each database table. Instead, think of the data that needs to be
> transactionally stored together.

So `content` and `content_item` are one aggregate with one entry point, not two
stores to be coordinated. That removes most of the need for cross-store
transactions rather than solving it.

One aside worth knowing: GORM wraps every single write in a transaction by
default. `gorm.Config{SkipDefaultTransaction: true}` disables it, and the docs
claim "about 30%+ performance improvement" for doing so
(<https://gorm.io/docs/transactions.html>). Leave it on; the note is here so the
default is not mistaken for no transaction at all.

### 2.6 Context

The audit found no query receives the request context. Fix this by convention
from day one, and the convention is not a matter of taste — it is stated in the
`context` package's own documentation, verbatim (`src/context/context.go`):

> Do not store Contexts inside a struct type; instead, pass a Context explicitly
> to each function that needs it. This is discussed further in
> https://go.dev/blog/context-and-structs. The Context should be the first
> parameter, typically named ctx

Go Code Review Comments repeats it (<https://go.dev/wiki/CodeReviewComments>),
verbatim:

> Most functions that use a Context should accept it as their first parameter:
> `func F(ctx context.Context, /* other arguments */) {}`

> Don't add a Context member to a struct type; instead add a ctx parameter to
> each method on that type that needs to pass it along.

Google's style guide adds the one exception that matters for a web app
(<https://google.github.io/styleguide/go/decisions#contexts>), verbatim: "When
passed to a function or method, `context.Context` is always the first parameter",
except "In an HTTP handler, where the context comes from `req.Context()`". And
"Do not add a context member to a struct type."

And the Go blog, <https://go.dev/blog/context-and-structs> (Jean Barkhuysen and
Matt T. Proud, 24 February 2021), concludes verbatim:

> When designing an API with context, remember the advice: pass
> `context.Context` in as an argument; don't store it in structs.

**So: `ctx context.Context` is the first parameter of every store method, the
`Store` struct does not hold one, and handlers get theirs from
`r.Context()`.** Attach it with `db.WithContext(ctx)` — a one-line session
wrapper, `func (db *DB) WithContext(ctx context.Context) *DB { return db.Session(&Session{Context: ctx}) }`
([gorm.go](https://github.com/go-gorm/gorm/blob/master/gorm.go)) — which
composes with `Transaction`, because `Begin` carries
`tx.Statement.Context` into the driver's `BeginTx`. GORM's docs also note
"Setting the `Context` with `WithContext` is goroutine-safe."

One honest note: this is a convention, not a universal practice. Gitea and Memos
put `ctx` first on every store method; **Mattermost's `sqlstore` does not take a
context at all** — `func (s *SqlReactionStore) Save(reaction *model.Reaction)`.
That is legacy, and it is the position this project is currently in. The
convention is right; "everyone does it" would be wrong.

### 2.7 SQLite: WAL, the pool, and per-connection pragmas

The audit says SQLite is opened with GORM defaults and a concurrent write
returns `SQLITE_BUSY`. The facts below are from SQLite's own documentation and
from the driver's source, because this is an area where folklore is common.

**First, a correction to the diagnosis.** `gorm.io/driver/sqlite` depends on
`github.com/mattn/go-sqlite3` — verified in its
[go.mod](https://github.com/go-gorm/sqlite/blob/master/go.mod) — and that driver
**already sets `PRAGMA busy_timeout = 5000` on every connection by default**.
In [sqlite3.go](https://github.com/mattn/go-sqlite3/blob/master/sqlite3.go) the
default is `busyTimeout := 5000` and it is applied unconditionally as
`PRAGMA busy_timeout = %d` when a connection opens. So a busy timeout is not
the missing piece. Two other things are.

**Fact 1 — journal mode.** The default journal mode is not WAL. From
<https://www.sqlite.org/wal.html>, verbatim:

> Writers merely append new content to the end of the WAL file. Because writers
> do nothing that would interfere with the actions of readers, writers and
> readers can run at the same time.

> However, since there is only one WAL file, there can only be one writer at a
> time.

So WAL buys **concurrent readers during a write**. It does not buy concurrent
writers. Nothing does.

**Fact 2 — WAL is persistent, and this is the key to the pragma question.**
Verbatim from the same page:

> Unlike the other journaling modes, PRAGMA journal_mode=WAL is persistent. If a
> process sets WAL mode, then closes and reopens the database, the database will
> come back in WAL mode.

The stated downside, verbatim:

> All processes using a database must be on the same host computer; WAL does not
> work over a network filesystem.

For a self-hosted single-binary app that is not a constraint.

**Fact 3 — foreign keys are per-connection and off by default.** From
<https://www.sqlite.org/pragma.html#pragma_foreign_keys>, verbatim:

> This pragma is a no-op within a transaction; foreign key constraint
> enforcement may only be enabled or disabled when there is no pending BEGIN or
> SAVEPOINT.

> As of SQLite version 3.6.19, **the default setting for foreign key enforcement
> is OFF.** However, that might change in a future release of SQLite. […] To
> minimize future problems, applications should set the foreign key enforcement
> flag as required by the application and not depend on the default setting.

**Fact 4 — busy_timeout is per-connection.** From the same page, verbatim:

> Each database connection can only have a single busy handler. This PRAGMA sets
> the busy handler for the process, possibly overwriting any previously set busy
> handler.

#### How the structure guarantees pragmas on every pooled connection

This is constraint (d), and the facts above answer it exactly.

| Setting | Scope | Therefore |
|---|---|---|
| `journal_mode=WAL` | **Per database file, persistent** | Needs setting once, ever. Setting it in the DSN is harmless and self-healing. |
| `foreign_keys` | **Per connection** | Must be in the DSN. |
| `busy_timeout` | **Per connection** | Already 5000 on the cgo driver; set it explicitly to be intentional. |

**The mechanism: put them in the DSN.** Both GORM SQLite drivers pass the DSN
straight to `sql.Open` and set no pragmas of their own, and the *driver* applies
DSN pragmas inside its connection-open path. So every connection
`database/sql` opens — now or after the pool closes and reopens one — gets them.
Executing `PRAGMA foreign_keys = ON` once after `gorm.Open` is the bug: it
configures whichever single connection happened to serve that statement.

GoToSocial documents exactly this reasoning in its own code, verbatim:

> to ensure that all of configuration options are set across _all_ of our SQLite
> connections, given that … each connection is a separate SQLite instance
> opening the same database. And the `database/sql` package provides transparent
> connection pooling.

Litestream's own documentation states the consequence in one line
(<https://litestream.io/tips/>), verbatim:

> This pragma must be set on each database connection, as it does not persist
> across connections.

**There is a real bug report for exactly this mistake**, which is the best
citation for constraint (d).
<https://github.com/mattn/go-sqlite3/issues/255>, titled "howto make pragma
statements persistent, e.g. for foreign_keys checks", verbatim from the
reporter:

> Writing a small RESTful webserver application, we rely on constrains in the
> sqlite db. In our app the foreign_key checks were working **once every second
> time**. We figured that go-sqlite3 will open several connections, probably
> recycling some, and some of the pool did not have the "PRAGMA foreign_keys =
> ON" set.

This was also measured directly during this research, using five simultaneous
`*sql.Conn` from one pool and reading `PRAGMA foreign_keys` on each. **This is
our own measurement, not a published source:**

| Approach | Result across 5 connections |
|---|---|
| DSN `?_foreign_keys=on` | `[1 1 1 1 1]` |
| `db.Exec("PRAGMA foreign_keys = ON")` once after Open | **`[1 0 0 0 0]`** |
| `sql.Register` + `ConnectHook` | `[1 1 1 1 1]` |

Two other real mechanisms exist if the DSN is not enough — a `ConnectHook` on a
registered driver name (Authelia registers `sqlite3e` this way; Navidrome does it
for a custom collation), or wrapping `driver.Driver` (Grafana, GoToSocial). Both
are used for registering custom SQL functions rather than pragmas. **Use the
DSN.**

Worth knowing that even Ben Johnson's own reference app uses the unsafe pattern:
WTF Dial sets `journal_mode` and `foreign_keys` with `db.Exec` after `sql.Open`
and sets no pool limits. Being a teaching repo with one user, it never hits the
failure.

#### cgo, and the pure-Go alternatives

The official GORM SQLite driver requires cgo. Its README says so verbatim: "The
official SQLite driver for GORM, based on go-sqlite3 (requires CGO)." That
affects cross-compilation and static binaries, and two of the Go proverbs are
"Cgo is not Go" and "Cgo must always be guarded with build tags"
(<https://go-proverbs.github.io/>).

Drop-in pure-Go alternatives exist, and GORM's own README lists them —
`glebarez/sqlite`, `libtnb/sqlite`, `ncruces/go-sqlite3/gormlite` — with the note
"Usage is identical — simply swap the import path". `glebarez/sqlite` pulls in
`modernc.org/sqlite`, so it is genuinely pure Go. **Gitea's own default is the
pure-Go modernc driver**, with the cgo one behind a build tag.

The catch: "simply swap the import path" is true of the *import* and not of the
*DSN*. Every parameter name changes, and a wrong name fails silently. Also note
that `mattn` defaults `busy_timeout` to 5000 while `modernc` does not — the
pragma runs only if the DSN asks for it. **So decide the driver before writing
the DSN, not after.**

#### The DSN parameter names differ per driver, and getting it wrong is silent

Gitea maintains both drivers side by side, which is the cleanest available proof
that these are not interchangeable:

```go
// mattn (cgo) — models/db/driver_sqlite_mattn.go
params = append(params, "_busy_timeout="+strconv.Itoa(opts.BusyTimeout))
params = append(params, "_txlock=immediate")
params = append(params, "_journal_mode="+opts.JournalMode)

// modernc (pure Go, Gitea's DEFAULT) — models/db/driver_sqlite_modernc.go
params = append(params, fmt.Sprintf("_pragma=busy_timeout(%d)", opts.BusyTimeout))
params = append(params, "_txlock=immediate")
params = append(params, fmt.Sprintf("_pragma=journal_mode(%s)", opts.JournalMode))
```

- [driver_sqlite_mattn.go](https://github.com/go-gitea/gitea/blob/main/models/db/driver_sqlite_mattn.go)
- [driver_sqlite_modernc.go](https://github.com/go-gitea/gitea/blob/main/models/db/driver_sqlite_modernc.go)

For `mattn/go-sqlite3`, verified against its DSN documentation and its parser:

| Parameter | Aliases | Notes |
|---|---|---|
| `_journal_mode` | `_journal` | |
| `_busy_timeout` | `_timeout` | Defaults to 5000 already |
| `_foreign_keys` | `_fk` | Boolean |
| `_synchronous` | `_sync` | |
| `_txlock` | | `immediate`, `deferred`, `exclusive` |

For `modernc.org/sqlite` the form is `_pragma=<name>(<value>)`, repeatable.
Newer versions have added mattn-style shorthand keys — `_busy_timeout` is
present at v1.56.0 and absent at v1.34.5 — so **`_pragma=` is the always-safe
form.**

**The silent-failure hazard is real and has a live example.** Neither driver
validates unknown *keys*, only the values of keys it recognises. Navidrome's
default DSN contains `synchronous=normal` **without the leading underscore**,
which `mattn` ignores entirely because it only reads `_synchronous` / `_sync`
([consts.go](https://github.com/navidrome/navidrome/blob/master/consts/consts.go)).
It happens to be harmless there only because mattn auto-sets
`synchronous=NORMAL` whenever WAL is requested. A *known* key with a bad value
does error — `_txlock=nonsense` returns `Invalid _txlock: nonsense`. So a typo'd
name fails silently and a typo'd value fails loudly.

Grafana hit this hard enough to write a **mattn-to-modernc DSN translator**
registered under the driver name `sqlite3`, which also injects
`_time_format=sqlite` because otherwise modernc serialises `time.Time` with
`String()` and values do not round-trip
([sqlite_nocgo.go](https://github.com/grafana/grafana/blob/main/pkg/util/sqlite/sqlite_nocgo.go)).
That is a concrete warning about switching drivers later.

**Syncthing draws the cleanest line on what goes where**, and it is a good rule
to copy: **connection-scoped** settings (`foreign_keys`, `recursive_triggers`,
`synchronous`, `_txlock`) go in the DSN; **database-scoped, persistent** ones
(`journal_mode`, `auto_vacuum`, `application_id`) are executed once after open.
It also carries both DSN dialects behind build tags — `_fk=true&_rt=true&_sync=1&_txlock=immediate`
for cgo, and `_pragma=foreign_keys(1)&_pragma=recursive_triggers(1)&_pragma=synchronous(1)&_txlock=immediate`
for pure Go — the same intent in two spellings. Note `_txlock` is the one
parameter spelled identically in both drivers.

Putting `journal_mode` in the DSN as well is still worth doing: it is harmless,
and it keeps fresh databases correct without a separate step.

#### Pragma order matters: busy_timeout before journal_mode

Three independent sources say so. GoToSocial, verbatim:

> We specifically set the `busy_timeout` PRAGMA before the `journal_mode`. … And
> for whatever reason (:shrug:) SQLite is very particular about setting this
> BEFORE the `journal_mode` is set, otherwise you can end up running into more of
> these `SQLITE_BUSY` return codes than you might expect.

PocketBase says the same in one line: "the busy_timeout pragma must be first
because the connection needs to be set to block on busy before WAL mode is set"
([db_connect.go](https://github.com/pocketbase/pocketbase/blob/master/core/db_connect.go)).
And modernc now enforces it in the driver, sorting `busy_timeout` to the front
with the comment "Busy timeout must be one of the first PRAGMAs set from query
params as some that make changes to the database might otherwise unexpectedly
fail with SQLITE_BUSY."

#### `_txlock=immediate` — the non-obvious one

SQLite's default transaction is `BEGIN DEFERRED`. From
<https://www.sqlite.org/lang_transaction.html>, verbatim:

> If the first statement after BEGIN DEFERRED is a SELECT, then a read
> transaction is started. Subsequent write statements will upgrade the
> transaction to a write transaction if possible, or return SQLITE_BUSY.

> IMMEDIATE causes the database connection to start a new write immediately,
> without waiting for a write statement.

That first case has its own error code, and **the busy timeout cannot save you
from it.** This is now verified from the primary source, not folklore. From
<https://www.sqlite.org/c3ref/busy_handler.html>, verbatim:

> The presence of a busy handler does not guarantee that it will be invoked when
> there is lock contention. If SQLite determines that invoking the busy handler
> could result in a deadlock, it will go ahead and return SQLITE_BUSY to the
> application instead of invoking the busy handler.

The example the page gives is precisely a read lock trying to promote to
reserved. And `busy_timeout` is implemented *via* that handler: "The
sqlite3_busy_handler() interface is used to implement sqlite3_busy_timeout()."

From <https://www.sqlite.org/rescode.html>, the error and **the documented
remedy**, verbatim:

> The SQLITE_BUSY_SNAPSHOT error code is an extended error code for SQLITE_BUSY
> that occurs on WAL mode databases when a database connection tries to promote
> a read transaction into a write transaction but finds that another database
> connection has already written to the database and thus invalidated prior
> reads.

> the application can use BEGIN IMMEDIATE instead of just BEGIN … if it
> succeeds, then SQLite guarantees that no subsequent operations on the same
> database through the next COMMIT will return SQLITE_BUSY.

A read-then-write transaction is what most handlers do, and GORM issues
`BEGIN` deferred. `_txlock=immediate` makes the driver issue `BEGIN IMMEDIATE`
for every transaction, which is the remedy SQLite itself names.
**Three projects do this: GoToSocial, Gitea and Authelia** — and Authelia sets
nothing else at all.

ory/kratos states the problem in its own code comment, verbatim:

> SQLite in WAL mode fails a deferred transaction's read-to-write upgrade with
> SQLITE_BUSY_SNAPSHOT immediately (busy_timeout cannot help a stale snapshot),
> so an immediate retry under write contention can livelock through the whole
> retry budget.

**Two outside authorities reach the same conclusion, which is worth citing
because this is the least intuitive item in this document.**

Django's documentation, on SQLite
(<https://docs.djangoproject.com/en/5.1/ref/databases/>), verbatim — a major
framework that shipped the same fix:

> To make sure your transactions wait until `timeout` before raising "Database
> is Locked", change the transaction mode to `IMMEDIATE`.

and on relying on the timeout alone:

> This will make SQLite wait a bit longer before throwing "database is locked"
> errors; it won't really do anything to solve them.

Bert Hubert, the PowerDNS author, in
<https://berthub.eu/articles/posts/a-brief-post-on-sqlite3-database-locked-despite-timeout/>,
verbatim:

> don't ever upgrade transactions to read-write. If you know you are going to
> write in a transaction, use 'BEGIN IMMEDIATE', or start off with the write.

One caution against treating WAL as a cure-all, from
<https://www.sqlite.org/wal.html>, verbatim:

> The second advantage of WAL-mode is that writers do not block readers and
> readers do not block writers. This is mostly true. But there are some obscure
> cases where a query against a WAL-mode database can return SQLITE_BUSY, so
> applications should be prepared for that happenstance.

Grafana and PocketBase both add an application-level retry on top of everything
above. That is the belt-and-braces option, not a substitute for
`_txlock=immediate`.

#### The pool — and a correction to the obvious answer

`SetMaxOpenConns(1)` has direct precedent, from
[gotify/server](https://github.com/gotify/server/blob/master/database/database.go),
verbatim:

```go
if dialect == "sqlite3" {
	// We use the database connection inside the handlers from the http
	// framework, therefore concurrent access occurs. Sqlite cannot handle
	// concurrent writes, so we limit sqlite to one connection.
	sqldb.SetMaxOpenConns(1)
}
```

**But the projects that thought hardest about this do not cap at 1
unconditionally.** What they actually do:

| Project | SQLite pool policy |
|---|---|
| **GoToSocial** | `1` **only when journal_mode is not WAL**; otherwise `8 × GOMAXPROCS`. Comment: "Specifically for SQLite databases with a journal mode of anything EXCEPT 'wal', only 1 concurrent connection is supported." |
| **PocketBase** | **Two pools on the same file** — a concurrent one for `SELECT`, and a non-concurrent one at `SetMaxOpenConns(1)` / `SetMaxIdleConns(1)` for everything else |
| **rqlite** | **Two pools** — `rwDB.SetMaxOpenConns(1)`, `roDB` unbounded |
| **Syncthing** | `max(16, 4*NumCPU)` normally; **`1` during migrations** |
| **Gotify** | `1` for SQLite unconditionally, 10 otherwise — and it sets no pragmas at all |
| **dex** | `SetMaxOpenConns(1)`. Comment: "always allow only one connection to sqlite3, any other thread/go-routine attempting concurrent access will have to wait" |
| Grafana, Gitea, Vikunja, Woodpecker, Memos, kratos | no SQLite-specific cap |

PocketBase's own interface docs explain the routing, verbatim:

> To minimize SQLITE_BUSY errors, it automatically routes the SELECT queries to
> the underlying concurrent db pool and everything else to the nonconcurrent
> one.

> The returned db instance is limited only to a single open connection, meaning
> that it can process only 1 db operation at a time (other queries queue up).

Note that PocketBase achieves this by routing at its own query-builder layer.
**That is hard to reproduce under GORM**, because a single `*gorm.DB` wraps one
`ConnPool` — you would need two `*gorm.DB` values and a discipline about which
to use. That is the main reason not to attempt it here.

**GoToSocial's rule is the important one: WAL is precisely what buys you more
than one connection.** Capping at 1 while also enabling WAL discards WAL's main
benefit — concurrent readers. Gotify caps at 1 because it has no WAL, which is
self-consistent.

So the honest menu is:

| Option | Concurrency | Complexity |
|---|---|---|
| WAL + busy_timeout + `_txlock=immediate`, `MaxOpenConns(1)` | writes serialise; reads serialise too | lowest; cannot produce `SQLITE_BUSY` in one process |
| WAL + busy_timeout + `_txlock=immediate`, several connections | concurrent reads, writes queue on the busy timeout | low; correct, but depends on the timeout being generous |
| Two pools, one writer + many readers (PocketBase) | best | highest; two `*sql.DB` on one file |

**Recommendation: option 1 to start, option 2 when read concurrency is wanted,
never option 3 for an app this size.** Option 1 removes
`SQLITE_BUSY_SNAPSHOT` as a single-process concern entirely, because two
connections are needed to produce it.

**The counter-argument, stated fairly.** Capping at 1 while enabling WAL does
throw away WAL's main benefit. The sharpest version of the objection is that
"WAL's central property is that readers do not block a writer and a writer does
not block readers, and pinning the pool to one connection gives all of that
back." That is mechanically correct. The reason to start at 1 anyway is that it
is the configuration that *cannot* fail, and the audit's problem is correctness,
not throughput. Option 2 is a one-line change when throughput becomes the
complaint.

The best-known write-up recommending the two-pool split is Sylvain Kerkour's
"Optimizing SQLite for servers", <https://kerkour.com/sqlite-for-servers>. It is
a well-informed blog post rather than an authority — the author is not a SQLite
maintainer — but its recommendations line up with the primary sources above:

```
writeDB.SetMaxOpenConns(1)
readDB.SetMaxOpenConns(max(4, runtime.NumCPU()))
```

plus WAL, `busy_timeout`, `synchronous = NORMAL`, `foreign_keys`, and `BEGIN
IMMEDIATE` transactions. Its self-reported throughput figures were not
reproduced for this document.

There is also community evidence that a timeout alone is the *worse* answer.
From <https://github.com/mattn/go-sqlite3/issues/274>, verbatim: "adding
`_busy_timeout` makes the program really, really slow. What helped was using
`db.SetMaxOpenConns(1)`. Which is the same as wrapping a mutex around every DB
access." The same commenter's caveat, also verbatim, is worth carrying: "using
one connection across multiple goroutines ... might cause problems, since
connections in `database/sql` are stateful, which means that a `db.Begin()` in
one goroutine effects other goroutines." These are issue-tracker comments, not
authority.

#### A pool hazard specific to migrations

This one is easy to miss and follows from two facts already established.
GORM's SQLite migrator wraps its table rebuilds in `RunWithoutForeignKey`, which
executes `PRAGMA foreign_keys = OFF` and defers `PRAGMA foreign_keys = ON`
([migrator.go](https://github.com/go-gorm/sqlite/blob/master/migrator.go)).
Pragmas are **per connection**. So on a pool with more than one connection,
those two statements and the rebuild in between can land on different
connections, and the `OFF` may not apply where it is needed.

**So pin the pool to one connection for the duration of migrations, whatever the
steady-state setting is.** Syncthing does exactly this — its normal pool is
`max(16, 4*NumCPU)` but it opens the migration path with `(1, 1)`.

Related: an in-memory SQLite database is **per connection**, so a pool of N
would give each goroutine its own empty database. If in-memory is ever used, it
must be pinned at 1 permanently — another reason §4.2 recommends a file in
`t.TempDir()` for tests.

**What `SetMaxOpenConns(1)` implies, stated honestly.** Mechanically, every
SQLite query in the process serialises through one connection — reads queue
behind writes, and `database/sql` blocks callers until the connection frees.
Whether that is *slow enough to matter* depends on query duration and
concurrency, and this document will not invent a number for it. The honest
framing: it converts a correctness bug into a queueing characteristic, and
option 2 is the documented upgrade path when the queue is the problem.

That is also the clearest argument for offering PostgreSQL: **SQLite for
single-user deployments, PostgreSQL when several people write at once.** Worth a
line in the deployment docs.

`COULD NOT VERIFY`: a measured benchmark of the throughput cost of
`MaxOpenConns(1)`. Do not claim one.

#### A trap that will bite in tests

Vikunja rejects shared-cache in-memory SQLite because "Shared cache
(`file::memory:?cache=shared`) uses table-level locking where `_busy_timeout` is
ineffective (returns SQLITE_LOCKED, not SQLITE_BUSY) and concurrent connections
deadlock." It uses a temp **file** with WAL for ephemeral runs instead.

This matters because `file::memory:?cache=shared` is the reflexive choice for Go
tests. **Use a real file in `t.TempDir()`.** It is as fast in practice and it
exercises the same code path as production.

#### Where this belongs in the structure

In the store's constructor, not a separate connection package. Gotify does all
of it — dialect switch, pool limits, per-engine tuning — in one function,
`database.New`. That keeps the answer to "how is the database opened?" in one
place, which the "prefer fewer files" rule favours.

One mechanism makes this airtight and is worth knowing: GORM's SQLite driver
accepts a connection you built yourself. From its
[source](https://github.com/go-gorm/sqlite/blob/master/sqlite.go):

```go
type Config struct {
	DriverName string
	DSN        string
	Conn       gorm.ConnPool
}
```

and `Initialize` uses `dialector.Conn` when it is non-nil instead of calling
`sql.Open` itself. `*sql.DB` satisfies `gorm.ConnPool`. So there are two
verified hand-offs:

- `sqlite.New(sqlite.Config{Conn: mySQLDB})` — build the `*sql.DB` yourself, set
  the pool limits, and hand it over. No path remains by which GORM opens a
  connection the store did not configure.
- `sqlite.New(sqlite.Config{DriverName: "sqlite3_hooked", DSN: dsn})` — make
  GORM open a driver name you registered, which is how you would add a
  `ConnectHook` or a custom SQL function later.

Neither is needed on day one — a DSN plus `db.DB()` then `SetMaxOpenConns` is
enough, and it is what Gotify does. They are worth knowing because they are the
escape hatches if the DSN ever stops being sufficient.

### 2.8 Migrations

This is where GORM stops helping, so it deserves the most care.

**`AutoMigrate` is a schema-diffing tool, not a history.** Its documented limits,
verbatim from <https://gorm.io/docs/migration.html>:

> **NOTE:** AutoMigrate will create tables, missing foreign keys, constraints,
> columns and indexes. It will change existing column's type if its size,
> precision changed, or if it's changing from non-nullable to nullable. It
> **WON'T** delete unused columns to protect your data.

Confirmed against the source: `AutoMigrate` never calls `DropColumn`, and it
also never drops a constraint or index that is no longer in the model. So the
schema drifts in one direction only, and the drift is invisible.

GORM's own documentation says to move on, verbatim:

> While GORM's `AutoMigrate` feature works in most cases, at some point you may
> need to switch to a versioned migrations strategy. Once this happens, the
> responsibility for planning migration scripts and making sure they are in line
> with what GORM expects at runtime is moved to developers.

**What the tools can and cannot do.** The important finding is negative, and it
changes the recommendation:

| Tool | Multi-engine story | Per-engine conditional in one migration? |
|---|---|---|
| [golang-migrate](https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md) | One source, one database per instance | **No.** Docs: "The migration files are generally processed directly by the drivers as raw operations." Multiple sources is an [open request](https://github.com/golang-migrate/migrate/issues/562). |
| [goose](https://github.com/pressly/goose/blob/main/dialect.go) | 13+ dialects incl. postgres, sqlite3, mysql | **Not built in.** `SetDialect` only selects the version-table store; it does not translate SQL. Go migrations are typed `func(ctx context.Context, tx *sql.Tx) error` — **no dialect argument**. |
| [Atlas](https://atlasgo.io/atlas-schema/hcl) | Declarative and versioned | **No.** Verbatim: "the Atlas DDL does not attempt to abstract away the differences between various databases. This means that the schema documents are tied to a specific database engine and version." Hence `.pg.hcl`, `.lt.hcl`. |

**So no off-the-shelf tool solves the per-engine statement.** That is why the
real projects hand-roll it.

#### What four real multi-engine projects chose

| Project | Choice | Engines | Mechanism |
|---|---|---|---|
| [Grafana](https://github.com/grafana/grafana/tree/main/pkg/services/sqlstore/migrator) | One set + escape hatch | sqlite, pg, mysql, mssql | Typed Go migrations over a ~55-method `Dialect`, plus `RawSQLMigration` |
| [Gitea](https://github.com/go-gitea/gitea/tree/main/modelmigration) | One set + escape hatch | sqlite, pg, mysql, mssql | Numbered Go funcs using xorm `SyncWithOptions`; dialect `switch` in `base` helpers |
| [PhotoPrism](https://github.com/photoprism/photoprism/tree/develop/internal/entity/migrate) | Separate sets per engine | sqlite, mysql/mariadb | `Dialects map[string]Migrations`, generated `dialect_sqlite3.go` / `dialect_mysql.go` |
| [Authelia](https://github.com/authelia/authelia/tree/master/internal/storage/migrations) | Separate sets per engine | sqlite, pg, mysql | `migrations/<engine>/V####.*.{up,down}.sql` over `embed.FS`, own runner |

**Two–two. There is no majority, and this document will not manufacture one.**
But the split is not random: **the two projects that share one migration set
already own a dialect abstraction; the two that split write raw SQL.**

Grafana's escape hatch is the most instructive artefact found in this whole
research, because it is exactly the shape of the problem. From
`pkg/services/sqlstore/migrations/dashboard_mig.go` — one migration ID, three
engines:

```go
mg.AddMigration("Update uid column values in dashboard", NewRawSQLMigration("").
	SQLite("UPDATE dashboard SET uid=printf('%09d',id) WHERE uid IS NULL;").
	Postgres("UPDATE dashboard SET uid=lpad('' || id::text,9,'0') WHERE uid IS NULL;").
	Mysql("UPDATE dashboard SET uid=lpad(id,9,'0') WHERE uid IS NULL;"))
```

and where only some engines need work, the others are simply absent
(`annotation_mig.go`):

```go
mg.AddMigration("Increase tags column to length 4096", NewRawSQLMigration("").
	Postgres("ALTER TABLE annotation ALTER COLUMN tags TYPE VARCHAR(4096);").
	Mysql("ALTER TABLE annotation MODIFY tags VARCHAR(4096);"))
```

**And here is the trap in that design, worth knowing before copying it:**
`RawSQLMigration.SQL()` returns `""` for an engine with no entry and no
`Default`, and an empty migration is a silent no-op, not an error. Omitting an
engine looks identical to deliberately skipping it.

Authelia's cost is visible in the other direction: 58 migration files × 3
engines = 174 files for ~58 logical changes, where the actual per-engine diffs
are a handful of tokens:

| | postgres | sqlite |
|---|---|---|
| identity | `id SERIAL CONSTRAINT ..._pkey PRIMARY KEY` | `id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT` |
| timestamps | `TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP` | `TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP` |
| fixed char | `jti CHAR(36)` | `jti VARCHAR(36)` |

**PhotoPrism is the closest analogue to this project** — a well-known Go app on
GORM, running SQLite and MySQL/MariaDB. It runs *both* mechanisms in stages:
`AutoMigrate` for additive schema sync, wrapped by its own versioned per-dialect
raw-SQL runner selected at runtime by `db.Dialect().GetName()`. Worth knowing
that this combination exists in production; also worth knowing it means two
sources of schema truth.

#### Recommendation

**`goose`, embedded, one numbered sequence, SQL files by default, and a Go
migration for the rare divergent step.**

- Dialect coverage is already there for a later MySQL. It declares 13+
  dialects — postgres, sqlite3, mysql (MariaDB via the MySQL driver), mssql,
  clickhouse, spanner, tidb, turso and more —
  [dialect.go](https://github.com/pressly/goose/blob/main/dialect.go).
- Embedding matters for a single-binary self-hosted app. README, verbatim:
  "Supports Go migrations written as plain functions." and "Supports embedded
  migrations."
- The Go migration types, from
  [register.go](https://github.com/pressly/goose/blob/main/register.go):

```go
type GoMigrationContext      func(ctx context.Context, tx *sql.Tx) error
type GoMigrationNoTxContext  func(ctx context.Context, db *sql.DB) error
```

  The second is for statements that cannot run inside a transaction.

**One honest correction about the escape hatch.** Goose hands you a Go function;
it does **not** hand you the dialect. The engine has to come from your own
code — a package-level value set once in `store.Open()`. That is three lines,
but it is your seam, not the tool's:

```go
func up0007(ctx context.Context, tx *sql.Tx) error {
	stmt := `ALTER TABLE content ADD COLUMN kind TEXT NOT NULL DEFAULT ''`
	if store.Engine == "postgres" {
		stmt = `...postgres form...`
	}
	_, err := tx.ExecContext(ctx, stmt)
	return err
}
```

**And take one lesson from Grafana's trap: make the missing-engine case loud.**
If a divergent migration does not have a branch for the engine it is running on,
return an error rather than doing nothing. That is one `default:` clause, and it
is the difference between a failed deploy and a silently wrong schema.

**Keep one migration directory, not one per engine.** The audit's own findings
support this: upsert, partial unique indexes, CTEs and `RETURNING` all work on
both engines, so the shared case is the overwhelming majority. Authelia's
174-files-for-58-changes is the cost of the alternative, and its diffs show the
divergence is a handful of tokens per table.

Migration *ordering* — one of the audit's four divergences — is a property of the
tool, not the engine. One numbered sequence with a version table gives one order
for all engines by construction.

#### Two SQLite DDL limits to design around

Beyond the CHECK-on-existing-table case in §2.2, `ADD COLUMN` in SQLite cannot
add a column that is `PRIMARY KEY` or `UNIQUE`, has a `CURRENT_TIMESTAMP` or
expression default, is `NOT NULL` without a non-NULL default, or is
`GENERATED ALWAYS ... STORED`
(<https://www.sqlite.org/lang_altertable.html>). The `NOT NULL` one bites most
often: adding a required column needs a default in the same statement.

#### When MySQL arrives, the honest cost

MySQL has no partial indexes at all. That is not a syntax difference a dialect
seam can paper over; it is a missing feature. A partial unique index has to
become either a full unique index over a different column set, or an
application-level check. Plan for that as a schema decision, not a translation.
This is the strongest argument for keeping the number of partial indexes small
and documenting each one.

---

## 3. Structuring for a later JSON API

### 3.1 The pattern, with a verified example

**Gitea is the clearest real example of one core serving both.** Its `routers/`
directory contains `web/` and `api/` side by side
(<https://github.com/go-gitea/gitea/tree/main/routers>), both sitting over
`services/` and `models/`.

This is verifiable at the level of individual calls, not just directory names.
Both of these files call the *same* service functions:

| File | Calls |
|---|---|
| [routers/web/repo/issue_label.go](https://github.com/go-gitea/gitea/blob/main/routers/web/repo/issue_label.go) | `issue_service.ClearLabels`, `issue_service.AddLabel`, `issue_service.RemoveLabel` |
| [routers/api/v1/repo/issue_label.go](https://github.com/go-gitea/gitea/blob/main/routers/api/v1/repo/issue_label.go) | `issue_service.AddLabels`, `issue_service.RemoveLabel`, `issue_service.ReplaceLabels`, `issue_service.ClearLabels` |

And the shared functions in
[services/issue/label.go](https://github.com/go-gitea/gitea/blob/main/services/issue/label.go)
are transport-agnostic — no HTTP types in the signature, `ctx` first:

```go
func RemoveLabel(ctx context.Context, issue *issues_model.Issue, doer *user_model.User, label *issues_model.Label) error
func ClearLabels(ctx context.Context, issue *issues_model.Issue, doer *user_model.User) error
```

Mattermost shows the same layering, though only for JSON: its
`server/channels/api4/` handlers call into an `App` layer rather than a store —
[api4/channel.go](https://github.com/mattermost/mattermost/blob/master/server/channels/api4/channel.go)
contains 235 `c.App.<Method>` calls. Its web client is a separate SPA, so it is
evidence for the layer, not for dual HTML/JSON rendering.

### 3.2 What actually keeps handlers thin enough for this to stay cheap

One rule, and it is testable by reading a signature: **no function below the
handler takes or returns an HTTP type.** No `*http.Request`, no
`http.ResponseWriter`, no framework context, no form struct. If the shared
function's signature mentions HTTP, the second transport cannot reuse it.

That single rule is what the Gitea signatures above satisfy. It is cheaper to
hold than a layering doctrine, because it is checkable at a glance.

The handler then has exactly four jobs: parse input, authorise, call one
function, render. Anything else in a handler is the thing that will have to be
duplicated later.

**The best-regarded write-up on the surrounding mechanics** is Mat Ryer's "How I
write HTTP services in Go after 13 years"
(<https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/>, 10
February 2024). Note the older "after eight years" post still resolves but its
body has been replaced by a pointer to this one, so cite the 2024 URL. His named
patterns, verbatim headings:

| Pattern | What it is |
|---|---|
| "The `NewServer` constructor" | "a big constructor that takes in all dependencies as arguments", returns `http.Handler` |
| "Map the entire API surface in `routes.go`" | "This file is the one place in your service where all routes are listed." |
| "Maker funcs return the handler" | "My handler functions don't implement `http.Handler` or `http.HandlerFunc` directly, they return them." |
| "Handle decoding/encoding in one place" | generic `encode[T any]` / `decode[T any]` helpers |
| "`func main()` only calls `run()`" | `func run(ctx context.Context, w io.Writer, args []string) error` |

**One thing he explicitly retracted**, and worth knowing because the older advice
is still widely repeated, verbatim:

> My handlers used to be methods hanging off a server struct, but I no longer do
> this. If a handler function wants a dependency, it can bloody well ask for it
> as an argument. No more surprise dependencies when you're just trying to test a
> single handler.

The routes-in-one-file pattern is the one that most directly serves requirement
3: when `api/` arrives it gets its own `routes.go`, and the two route tables are
readable side by side.

### 3.3 View models — where they live and who maps

Gitea keeps its API response types in a separate package,
[`modules/structs/`](https://github.com/go-gitea/gitea/blob/main/modules/structs/issue.go),
and a dedicated [`services/convert/`](https://github.com/go-gitea/gitea/blob/main/services/convert/issue.go)
package that maps domain models to them — that file imports
`api "gitea.dev/modules/structs"` and exposes
`func ToIssue(ctx context.Context, doer *user_model.User, issue *issues_model.Issue) *api.Issue`.
The web side does not use those types; it passes domain models to templates.

That split is the honest tradeoff:

| Approach | For | Against |
|---|---|---|
| Templates render domain structs directly | No mapping layer, no duplication, fewest types | Template output is coupled to the schema; a column rename touches templates |
| Separate view/API types plus a mapper | The JSON contract is explicit and stable; renames do not leak | One more type and one more mapping per aggregate |

**Recommendation for now: render domain structs in templates, and do not build
a view-model layer yet.** HTML templates are not a public contract — they ship
with the binary and can be changed in the same commit as the schema. A JSON API
*is* a public contract, so when it arrives, give it its own response structs and
put the mapping next to the JSON handlers. That way the cost is paid once, by
the consumer that actually needs the stability, and the HTML side never pays it.

This is the one place where deferring is genuinely cheap, because adding
response structs later does not require changing the store or the shared
functions — only the new handlers.

---

## 4. Testing

### 4.1 Multi-engine store testing — what projects really do

The honest count is lower than expected. Of four multi-engine projects whose CI
was read, **only one runs the full suite against every engine.**

| Project | Same suite per engine? | Evidence |
|---|---|---|
| **Gitea** | **Yes — 4 engines** for integration tests; unit tests run once | [pull-db-tests.yml](https://github.com/go-gitea/gitea/blob/main/.github/workflows/pull-db-tests.yml) — jobs `test-sqlite`, `test-pgsql-shard-1/2`, `test-mysql`, `test-mssql` |
| **GoToSocial** | **No — SQLite only in CI**, though the engine is env-switchable | its Woodpecker config runs one `go test ./...` with no DB service; test default is `GTS_DB_TYPE=sqlite`, `:memory:` |
| **Authelia** | **No** for Go unit tests — those use `gomock`, not a database. Per-engine coverage is docker-compose e2e suites | `internal/storage/sql_provider_mock_test.go` uses `go.uber.org/mock/gomock`; `internal/suites/suite_postgres.go`, `suite_mysql.go`, `suite_mariadb.go` |
| **Mattermost** | **No — PostgreSQL only** | every `drivername:` in its CI is `postgres`; MySQL removed in v11 |

**Gitea is still the model, and here is the mechanism.** One integration suite,
engine chosen by an environment variable. Its `Makefile` documents the target as
"run integration test for GITEA_TEST_DATABASE (sqlite, mysql, pgsql, mssql)"
(<https://github.com/go-gitea/gitea/blob/main/Makefile>), and the CI jobs invoke
it as `GITEA_TEST_DATABASE=sqlite make test-integration` and so on, against real
database servers in CI service containers.

One precision: the literal `GITEA_TEST_DATABASE=<engine> make test-integration`
line is visible in the workflow for sqlite, mysql and mssql. The two PostgreSQL
jobs delegate to a composite action (`./.github/actions/pgsql-shard`), so the
`pgsql` value is cited from the Makefile help text rather than the workflow file.

**Why run it twice anyway.** The seam is the risky part, so a suite that only
runs against SQLite tests everything except the thing most likely to break. Since
the store has one implementation, the suite is written once — running it twice
costs CI time, not developer time. It is also the practical answer to "will
adding MySQL be additive?": you find out in one CI run.

That said, the count above means this is a *recommendation*, not an industry
default. Three of four comparable projects settled for less.

### 4.2 Mocking the database versus using a real one

Go's own Code Review Comments is unusually direct, verbatim
(<https://go.dev/wiki/CodeReviewComments>):

> Do not define interfaces on the implementor side of an API "for mocking";
> instead, design the API so that it can be tested using the public API of the
> real implementation.

Google's Go style guide reinforces it, verbatim
(<https://google.github.io/styleguide/go/best-practices>):

> Don't define back doors only for tests: Do not export a test double
> implementation of an interface from an API that consumes it. Instead, prefer
> to design the API so that it can be tested using the public API of the
> real implementation.

> When testing component integrations, especially where HTTP or RPC are used as
> the underlying transport between the components, prefer using the real
> underlying transport to connect to the test version of the backend.

It also refers to "our codebase's preference for real objects". Being precise
about scope: **it says nothing specifically about mocking SQL databases.** The
inference is reasonable; the citation is about transports and test doubles
generally.

Mat Ryer lands in the same place from the practitioner side, verbatim:

> I err on the side of end-to-end testing at this level, rather than unit
> testing all the pieces inside. I would rather call the `run` function to
> execute the whole program as close to how it will run in production as
> possible… Then when I hit the API from my test code, I am going through all
> the layers and even interacting with a real database.

**On `go-sqlmock`,** the usual tool for the other approach: its README describes
its purpose as simulating "any **sql** driver behavior in tests, without needing
a real database connection". Two things from that same README worth knowing
before adopting it — its own caveat, verbatim: "**sqlmock** will not provide a
standard sql parsing matchers, since various drivers may not follow the same SQL
standard", with the default matcher being a **regex against your SQL string**;
and its status: "## Looking for maintainers — I do not have much spare time for
this library and willing to transfer the repository ownership" alongside "this
library is now complete and stable." It points readers at `go-txdb` for
"functionally test with a real database".

A regex-against-SQL-strings mock is the worst possible fit for a project whose
whole risk is that the SQL differs per engine.

**The genuine disagreement.** Ben Johnson goes the other way and ships a `mock`
package as tenet 3 of his layout, described in WTF Dial's README, verbatim:

> There is also a `mock` package which implements simple mocks for each of the
> application domain interfaces. This allows each subpackage's unit tests to
> share a common set of mocks so layers can be tested in isolation.

Authelia does the same with `gomock` for its storage unit tests. Both positions
are coherent.

**The deciding factor for this project** is that a real SQLite database in
`t.TempDir()` is nearly free — no server, no container, no fixture teardown
beyond deleting a directory. When the real thing is that cheap, the mock's
failure mode is bad: it cannot catch a broken query, a missing index, a cascade
that does not fire, or a transaction that does not roll back — which are exactly
the bugs the audit found. And with no `Store` interface (§2.4) there is nothing
to mock anyway.

Use a **file** in `t.TempDir()`, not `file::memory:?cache=shared` — see the
shared-cache locking trap in §2.7.

### 4.3 The risk of testing on SQLite while running PostgreSQL

This is the failure mode a shared store implementation makes easy to fall into.
SQLite is permissive where PostgreSQL is strict. From
<https://www.sqlite.org/datatype3.html>, verbatim:

> The important idea here is that the type is recommended, not required. Any
> column can still store any type of data.

So a SQLite-only suite can pass on data PostgreSQL would reject — for instance
SQLite accepts `3.14` into an `INTEGER` column while PostgreSQL errors. Other
verified divergences that hide bugs: JSON operators (`->>`, `@>` versus
`json_extract`), no native arrays, window-function and materialised-view gaps,
and database-level locking versus MVCC row locks, so "code that works fine in
SQLite might deadlock in PostgreSQL". Source:
<https://neon.com/blog/testing-sqlite-postgres> — **note this is a vendor blog
from a PostgreSQL company, so read the framing with that in mind**; the specific
divergences are checkable against each engine's docs.

This is the concrete reason §4.1's "run the suite against both" is not
over-engineering: SQLite tests are fast enough for the inner loop, and the
PostgreSQL run is what makes them trustworthy.

---

## 5. Recommendation

### 5.1 Directory tree

```
passion/
├── go.mod
├── main.go                  package main: flags, config, open store, migrate, serve
├── internal/
│   ├── config/              Config struct and its loading from env and flags
│   ├── model/               domain types with GORM tags; no DB calls, no HTTP types
│   ├── store/               ALL database access, and the entire per-engine seam
│   │   ├── store.go           Open(), dialect switch, pool + pragmas, Migrate()
│   │   ├── content.go         queries for one aggregate, one file each
│   │   ├── log.go
│   │   ├── plan.go
│   │   ├── account.go
│   │   └── migrations/        embedded goose migrations, one numbered sequence
│   ├── app/                 operations spanning aggregates; one exported func = one transaction
│   └── web/                 HTML handlers, routes, template rendering
│       ├── web.go             Server struct, route table, render helper
│       ├── content.go         one file per screen area
│       ├── log.go
│       ├── plan.go
│       └── account.go
├── templates/               html/template files
├── static/                  css, js, images
└── catalog/                 YAML content
```

**Five packages under `internal/`, plus `main`.** For scale: wtf has 11 packages
at 6k lines, hetty 13 at 6k, gotify 30 at 9.6k (§1.5). Six is at the small end
of what comparable apps do, which is the intent.

| Choice | Why, with a source |
|---|---|
| `main.go` in the root, no `cmd/` | Fewest directories for one binary. Hugo and Miniflux both do this — root `main.go`, no `cmd/` (§1.5). Russ Cox: "It is not required to put commands in cmd/." Add `cmd/` only if a second binary appears; `go.dev/doc/modules/layout` prefers `cmd/` for servers, so this is a close call decided on directory count. |
| `internal/` | The one boundary the `go` command actually enforces (§1.2), and `go.dev/doc/modules/layout` recommends it explicitly for a server: "it's recommended to keep the Go packages implementing the server's logic in the `internal` directory". |
| No `pkg/` | Russ Cox: "the _vast_ majority of packages in the Go ecosystem do _not_ put the importable packages in a `pkg` subdirectory". Eli Bendersky: "in the majority of cases it's an antipattern" (§1.3). |
| One `web` package, many files | Every comparable app does this: miniflux `internal/ui` is one package of 93 files; gotify `api` is one package of 10 (§1.5). Google's guide: files are the cheap unit, packages are not (§1.6). |
| One `store` package, one file per aggregate | Dave Cheney: "prefer fewer, larger packages. Your default position should be to not create a new package." The `net/http` precedent — `client.go`, `server.go` in one package, not sub-packages (§1.6). |
| `model` separate from `store` | So `web` can name a domain type without importing the database layer. Google's guide's litmus test — must a user import both to use either? — says no here. |
| `app` separate from `store` | Requirement 3. Gitea's `services/` is what lets `routers/web` and `routers/api/v1` call the same function (§3.1). Without it, shared logic has nowhere to live above the store but below two transports. |

**The rule that keeps `app` from becoming dead weight: if an `app` function only
calls one store method, delete it and have the handler call the store
directly.** `app` earns its place only for operations that span aggregates, open
a transaction, or touch something other than the database. Simple reads go
`web → store` with no middleman. If after the rewrite `app` is all
pass-throughs, delete the package — `web` and `store` are both transport-free
already, so that is a rename, not a redesign.

When the JSON API arrives it becomes `internal/api/`, a sibling of `web/`, with
the same shape, calling the same `app` and `store` functions. That mirrors
`routers/web` and `routers/api/v1` in Gitea exactly.

### 5.2 The store layer, spelled out

**Interface granularity: no interface.** `store` exports a concrete
`*store.Store`. No `Store` interface, no `ContentStore`, none per aggregate.

The reason is that the original requirement is gone. Google's Go style guide says
define an interface when "there are two or more concrete types that must be
handled by the same logic". With GORM owning the engine seam there is exactly
one, so the interface would have one implementor — the shape Code Review
Comments calls out as "interfaces on the implementor side of an API 'for
mocking'", and which it also warns against with "Do not define interfaces before
they are used" (§2.4). And the Go proverb: "The bigger the interface, the weaker
the abstraction."

If `web` or `app` later wants to fake the store in a test, the **consumer**
declares a three-method interface next to the code that uses it, exactly as
gotify's `api.MessageDatabase` does. That interface is small because it lists
only what one handler needs.

**Where the per-engine seam sits: three places, and nowhere else.**

| Seam | Location | Size |
|---|---|---|
| Connection: DSN, pool limits, SQLite pragmas | `store.go`, one `switch` in `Open()` | ~15 lines, as in gotify |
| Divergent migration steps | a Go migration in `store/migrations/` | per-statement, rare |
| Raw SQL | banned outside `store`; prefer GORM's builder | zero, by rule |

Everything else the audit measured — identity columns, `DATE` vs text, CHECK on
an existing table, placeholders, quoting, error codes — is already inside GORM's
eight-method `Dialector` and its per-driver `Migrator` (§2.2). The application
implements none of it.

**Adding MySQL later:** add a case to the `switch`, add a driver import, run the
existing suite against it, and resolve the partial-index question. No new
implementation, no new interface, no touched call sites.

**Transactions: exported store methods open the transaction; unexported helpers
take `tx *gorm.DB`.**

```go
func (s *Store) CreateContentWithItems(ctx context.Context, c *model.Content, items []model.ContentItem) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := createContent(ctx, tx, c); err != nil {
			return err
		}
		return createItems(ctx, tx, c.ID, items)
	})
}

func createContent(ctx context.Context, tx *gorm.DB, c *model.Content) error { ... }
```

The one-line rule: **an unexported store helper takes `tx *gorm.DB` and never
touches `s.db`.** That is what makes atomicity the default rather than the
disciplined choice, and it exists because of the trap in §2.5 — GORM detects
nesting on *the handle*, so a method that uses `s.db` inside an outer
`Transaction` silently opens a second, independent transaction, and because
`tx` and `s.db` are the same type the compiler cannot warn you. A helper that
has no access to `s.db` cannot make that mistake.

This is Ben Johnson's WTF Dial pattern, and it works for the same structural
reason: **the store is one Go package**, so unexported helpers in different
files can compose freely without interfaces or context smuggling.

Related, and it removes most of the problem rather than solving it — Three Dots
Labs, verbatim: "Don't create a repository for each database table. Instead,
think of the data that needs to be transactionally stored together." So
`content` and `content_item` get **one entry point**, not two stores to
coordinate.

**Context:** `ctx context.Context` first on every `store` and `app` method; no
struct holds one; handlers get theirs from `r.Context()`. Sourced to the
`context` package's own docs ("The Context should be the first parameter,
typically named ctx"), Code Review Comments, Google's style guide, and
`go.dev/blog/context-and-structs` (§2.6). Attach with `db.WithContext(ctx)`.

**Migrations:** `goose`, SQL files by default, embedded, one numbered sequence
for all engines, and a Go migration as the escape hatch when a statement must
differ. Do not call `AutoMigrate` in production — it never drops a column,
index, or constraint, so the schema drifts one way and invisibly (§2.8).
Remember the escape hatch is one you build: goose passes the Go migration no
dialect argument. **Make the missing-engine case return an error, not a no-op** —
that is the trap in Grafana's otherwise excellent `RawSQLMigration`.

**SQLite setup**, all inside `Open()`:

| Setting | Value | Reason |
|---|---|---|
| `_busy_timeout=5000` | first in the DSN | Order matters — before `journal_mode` (§2.7). |
| `_journal_mode=WAL` | | Concurrent readers during a write. Persistent per file. |
| `_foreign_keys=on` | | Off by default in SQLite, and per connection. |
| **`_txlock=immediate`** | | The highest-value, least-obvious item. `busy_timeout` provably cannot retry a deferred read-to-write upgrade. |
| `SetMaxOpenConns(1)` | SQLite only | Simplest configuration that cannot produce `SQLITE_BUSY`. |
| `SetMaxOpenConns(1)` | **during migrations, always** | GORM's SQLite rebuilds toggle `foreign_keys` per connection (§2.7). |

Constraint (d) is answered by *putting them in the DSN*, because the driver
re-applies DSN pragmas on every connection the pool opens. A one-off `PRAGMA`
after `gorm.Open` configures one connection — measured as `[1 0 0 0 0]` across
five (§2.7), and reported as a real bug in mattn issue 255.

Also set `gorm.Config{TranslateError: true}` so handlers compare against
`gorm.ErrDuplicatedKey` instead of engine-specific codes (§2.2).

**And decide the driver before writing the DSN.** The cgo and pure-Go drivers
take different parameter names, a wrong *name* is silently ignored, and Gitea's
own default is the pure-Go one (§2.7).

**Testing:** one suite, run against both engines, real databases, no store
mocks. A **file** in `t.TempDir()`, not `file::memory:?cache=shared`. Precedent:
Gitea, via `GITEA_TEST_DATABASE=<engine> make test-integration`. Be aware this
is a recommendation rather than an industry default — only one of four
comparable projects actually does it (§4.1).

### 5.3 Where the requirements fight simplicity

The brief asked for the disagreement. Here it is, ordered by how much it matters.

**1. The original "separate implementations behind an interface" would have been
the largest single source of complexity — relaxing it was correct, but the
evidence is 8-to-1, not unanimous.** Of nine multi-engine Go projects examined,
**one — Memos — really does maintain three near-identical implementations of a
65-method interface**, and a diff of a matched file pair confirms it is genuine
duplication (§2.1). So the idea is not absurd; it is just the minority, and the
eight others include every project closest to this one in size and shape.
Recording why it loses here: the interface would have one implementor, double
the review surface for queries identical on both engines, and still not solve
the only difference that matters (MySQL's missing features). GORM's eight-method
`Dialector` is the seam, and it already exists.

**2. "SQLite support" and "multi-user deployment" are in genuine tension, and no
code structure resolves it.** SQLite permits one writer at a time by design
(§2.7). **What I would do: state the limit in the deployment docs rather than
engineer around it** — SQLite for single-user or household use, PostgreSQL when
several people write concurrently. Specifically, do **not** build PocketBase's
two-pool read/write split: it is genuinely effective, but it is hard to
reproduce under GORM (one `*gorm.DB` wraps one `ConnPool`) and it solves a case
PostgreSQL already solves by being selected.

**3. WAL plus `SetMaxOpenConns(1)` is a knowingly conservative combination.**
GoToSocial's rule is that WAL is precisely what earns you more than one
connection, so capping at 1 discards WAL's main benefit. That objection is
correct. **What I would do: still start at 1**, because it is the configuration
that cannot fail and the audit's problem is correctness; then raise the cap as a
one-line change if throughput becomes the complaint. The point of flagging it is
so that raising it later reads as the plan, not a panic.

**4. "MySQL later" is not fully additive, and honesty is cheaper than a seam.**
MySQL has no partial indexes at all — a missing feature, not a syntax
difference, so no dialect abstraction can hide it. **What I would do: keep the
number of partial unique indexes small and write a one-line comment at each
saying what it enforces**, so the port is a short list of known decisions. The
rejected interface design would not have helped here either.

**5. GORM's official SQLite driver requires cgo.** Drop-in pure-Go alternatives
exist and GORM's README lists them, and Gitea's own default is the pure-Go
modernc driver. **What I would do: decide now, before the DSN is written.**
"Simply swap the import path" is true of the import and false of the DSN — every
parameter name changes, and a wrong name fails silently. Grafana had to write a
translator between the two dialects; Navidrome ships a silently-ignored
parameter today (§2.7). This is the one decision that gets more expensive with
delay.

**6. Versioned migrations are more work than `AutoMigrate`, and worth it
anyway.** Using both is the worst option — the schema then has two sources of
truth. PhotoPrism does run both, staged, so it is survivable; it is still two
places to look. GORM's own docs point at versioned migrations (§2.8). **What I
would do: goose only, and accept writing the DDL by hand.**

**7. `app/` is the package in the tree I am least sure about.** It is justified
by requirement 3 and by the transaction findings, but a fifth package holding
only pass-throughs is exactly the complexity to avoid. The deletion rule in
§5.1 is the safeguard — apply it without sentiment after the first month.

**8. One thing that is cheap and should be deferred: view models.** Building a
DTO layer now for an API that does not exist adds a type and a mapper per
aggregate to serve no consumer. Templates can render domain structs because
templates are not a public contract (§3.3).

**9. Two asymmetries to remember rather than fix.** With `TranslateError: true`,
`gorm.ErrCheckConstraintViolated` is **PostgreSQL-only** — neither SQLite nor
MySQL maps it (§2.2). And SQLite's `LOWER()` is ASCII-only, so case-insensitive
matching on non-ASCII text differs between the engines (§2.1). Do not build
abstractions for either; just do not rely on them.

---

## Sources

Official Go:
- <https://go.dev/doc/modules/layout> — organising a Go module
- <https://go.dev/blog/package-names> — Sameer Ajmani, 2015
- <https://go.dev/blog/context-and-structs> — Barkhuysen and Proud, 2021
- <https://go.dev/wiki/CodeReviewComments> — interfaces, contexts
- <https://go.dev/doc/go1.4> — `internal/` introduced
- <https://github.com/golang/go/blob/master/src/cmd/go/alldocs.go> — the `internal` rule
- `src/context/context.go` — "The Context should be the first parameter"
- <https://go-proverbs.github.io/> — Rob Pike, Gopherfest SV 2015

Style and design guidance:
- <https://google.github.io/styleguide/go/best-practices> — package size, interfaces, test doubles, real transports
- <https://google.github.io/styleguide/go/decisions#interfaces> — consumer defines the interface
- <https://dave.cheney.net/practical-go/presentations/qcon-china.html> — "Consider fewer, larger packages"
- <https://eli.thegreenplace.net/2019/simple-go-project-layout-with-modules/>
- <https://www.gobeyond.dev/standard-package-layout/>, <https://www.gobeyond.dev/packages-as-layers/> — Ben Johnson
- <https://github.com/golang-standards/project-layout/issues/117> — Russ Cox's criticism
- <https://medium.com/@cep21/preemptive-interface-anti-pattern-in-go-54c18ac0668a> — Jack Lindamood, origin of "accept interfaces, return structs"
- <https://grafana.com/blog/how-i-write-http-services-in-go-after-13-years/> — Mat Ryer, 2024
- <https://threedots.tech/post/database-transactions-in-go/> — Three Dots Labs on transactions and aggregates
- <https://rednafi.com/go/repo-txn-uow/> — the case against transactions in context
- <https://maddevs.io/blog/effective-refactoring-of-heavy-database-interface/> — the 130-method interface refactor

Code read for this document:
- <https://github.com/gotify/server/blob/master/database/database.go>, `api/message.go`
- <https://github.com/go-gorm/gorm/blob/master/interfaces.go>, `migrator.go`, `finisher_api.go`, `gorm.go`, `errors.go`
- <https://github.com/go-gorm/sqlite/blob/master/sqlite.go>, `migrator.go`, `error_translator.go`, `go.mod`
- <https://github.com/go-gorm/postgres/blob/master/postgres.go>, `error_translator.go`
- <https://github.com/go-gitea/gitea/blob/main/models/db/context.go>, `services/issue/label.go`, `routers/`, `modelmigration/`, `models/db/driver_sqlite_mattn.go`, `driver_sqlite_modernc.go`, `Makefile`, `.github/workflows/pull-db-tests.yml`
- <https://github.com/usememos/memos/blob/main/store/driver.go> and `store/db/{sqlite,postgres}/`
- <https://github.com/authelia/authelia/tree/master/internal/storage> — provider, per-engine backends, migrations
- <https://github.com/grafana/grafana/tree/main/pkg/services/sqlstore/migrator>, `pkg/util/sqlite/sqlite_nocgo.go`
- <https://codeberg.org/superseriousbusiness/gotosocial> — `internal/db/`, `internal/db/bundb/`
- <https://github.com/pocketbase/pocketbase/blob/master/core/db_connect.go>, `core/base.go`, `core/db_builder.go`
- <https://github.com/mattn/go-sqlite3/blob/master/sqlite3.go> — DSN parameters
- <https://github.com/pressly/goose/blob/main/dialect.go>, `register.go`
- <https://github.com/benbjohnson/wtf> — README, `sqlite/`
- <https://github.com/photoprism/photoprism/tree/develop/internal/entity/migrate>
- <https://github.com/miniflux/v2/tree/main/internal/storage>, <https://github.com/mattermost/mattermost/blob/master/server/channels/store/store.go>
- <https://github.com/DATA-DOG/go-sqlmock> — README and its caveats

SQLite primary documentation:
- <https://www.sqlite.org/wal.html> — concurrency, persistence, limits
- <https://www.sqlite.org/pragma.html> — busy_timeout, foreign_keys, synchronous
- <https://www.sqlite.org/lang_transaction.html> — DEFERRED vs IMMEDIATE
- <https://www.sqlite.org/c3ref/busy_handler.html> — the handler is not always invoked
- <https://www.sqlite.org/rescode.html> — SQLITE_BUSY, SQLITE_BUSY_SNAPSHOT and its remedy
- <https://www.sqlite.org/lang_altertable.html> — what ALTER TABLE supports
- <https://www.sqlite.org/datatype3.html> — storage classes, no DATE type, type affinity

On SQLite in production (lower authority — read the framing):
- <https://litestream.io/tips/> — Ben Johnson's project; "must be set on each database connection"
- <https://docs.djangoproject.com/en/5.1/ref/databases/> — a framework that shipped `IMMEDIATE`
- <https://berthub.eu/articles/posts/a-brief-post-on-sqlite3-database-locked-despite-timeout/> — Bert Hubert
- <https://kerkour.com/sqlite-for-servers> — Sylvain Kerkour; well-informed blog, not an authority
- <https://github.com/mattn/go-sqlite3/issues/255> — the real per-connection pragma bug report
- <https://github.com/mattn/go-sqlite3/issues/274> — community discussion of `SetMaxOpenConns(1)`
- <https://neon.com/blog/testing-sqlite-postgres> — **vendor blog**, PostgreSQL company

Tools:
- <https://gorm.io/docs/migration.html>, `transactions.html`, `context.html`, `create.html`, `connecting_to_the_database.html`
- <https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md>
- <https://atlasgo.io/atlas-schema/hcl> — engine-specific by design
- <https://docs.sqlc.dev/en/latest/reference/language-support.html> — SQLite still Beta

## Things this document could not verify

- A measured benchmark of the throughput cost of `SetMaxOpenConns(1)`.
- Whether SQLite's busy handler is invoked specifically on
  `SQLITE_BUSY_SNAPSHOT`. The general rule (the handler may be skipped when
  retrying could deadlock) is verified; the WAL-specific case is inferred.
- Any substantial named blog post or forum thread attacking Ben Johnson's
  layout. The citable pushback is in his own repo's discussions.
- Any r/golang thread. Reddit was not reachable during this research, so no
  claim here rests on one.
- Whether an unrecognised SQLite URI parameter is *documented* as ignored.
  sqlite.org does not say so; the "silently ignored" conclusion comes from the
  driver source and from measurement.
