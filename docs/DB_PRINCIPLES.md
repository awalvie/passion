# Database design principles and portable-SQL facts

A reference for designing one schema that runs on PostgreSQL, MySQL/MariaDB and SQLite,
in Go with GORM.

**How to read this.** Every portability claim carries a documentation URL. Where a claim
could not be confirmed in primary documentation it is marked `unverified`. Nothing here
is quoted unless the quoted page was actually fetched. Version numbers refer to the
documentation set named in the link.

Two words are used precisely:

- **Supported** — the construct parses and does what the standard says.
- **Enforced** — the database actually rejects a violating row.

---

## Part 1 — Portability facts

### 1.1 Summary table

| Construct | PostgreSQL | MySQL 8.4 | MariaDB | SQLite |
|---|---|---|---|---|
| Partial / filtered unique index | Yes | No | No | Yes (3.8.0+) |
| Multiple NULLs allowed in UNIQUE | Yes | Yes | Yes | Yes |
| `NULLS NOT DISTINCT` | Yes (15+) | No | No | No |
| CHECK enforced | Yes | Yes (8.0.16+) | Yes (10.2.1+) | Yes (unless PRAGMA off) |
| Generated column STORED | Yes (12+) | Yes | Yes (`PERSISTENT`) | Yes (3.31.0+) |
| Generated column VIRTUAL | Yes (18+, now default) | Yes (default) | Yes | Yes |
| Index on a generated column | Yes | Yes | Yes | Yes |
| Native `DATE` type | Yes | Yes | Yes | No |
| Instant-with-zone type | `timestamptz` | `TIMESTAMP` (to 2038) | `TIMESTAMP` (to 2038 pre-11.5) | No |
| `ON DELETE CASCADE` / `SET NULL` | Yes | Yes | Yes | Yes, but FKs off by default |
| Deferrable constraints | Yes (UNIQUE, PK, FK, EXCLUDE) | No | No (`unverified`) | FKs only |
| Identity / auto-increment | `IDENTITY` / `serial` | `AUTO_INCREMENT` | `AUTO_INCREMENT` | `INTEGER PRIMARY KEY` |
| Native JSON type | `json` / `jsonb` | `JSON` (binary) | alias for `LONGTEXT` | No (text or JSONB blob) |
| Index a value inside JSON | Yes (GIN / expression) | Via generated column | Via generated column | Via expression index |
| Default string comparison | Case-**sensitive** | Case-**insensitive** | Case-**insensitive** | Case-**sensitive** |
| Max identifier length | 63 bytes | 64 chars | 64 chars | Not documented |
| Max columns in one index | 32 | 16 | 16 (`unverified`) | 2000 |
| Native ENUM | Yes | Yes | Yes | No |
| Native BOOLEAN | Yes | `TINYINT(1)` synonym | `TINYINT(1)` synonym | Integer 0/1 |
| Index on TEXT/CLOB | Yes | Prefix length required | Prefix length required | Yes |
| Transactional DDL (rollback) | Yes | No — implicit commit | No — implicit commit | Yes |

### 1.2 Partial / filtered unique indexes

| DB | Support | Source |
|---|---|---|
| PostgreSQL | Yes. "This enforces uniqueness among the rows that satisfy the index predicate, without constraining those that do not." | https://www.postgresql.org/docs/current/indexes-partial.html |
| SQLite | Yes. "Partial indexes have been supported in SQLite since version 3.8.0 (2013-08-26)." and "A partial index definition may include the UNIQUE keyword." | https://sqlite.org/partialindex.html |
| MySQL | No. The `CREATE INDEX` grammar has no `WHERE` clause. Functional key parts (expression indexes) *are* supported. | https://dev.mysql.com/doc/refman/8.4/en/create-index.html |
| MariaDB | No `WHERE` clause in the `CREATE INDEX` grammar. | https://mariadb.com/docs/server/reference/sql-statements/data-definition/create/create-index |

**Portable alternative.** Do not use a predicate. Put a `NOT NULL` discriminator column
into the unique key so that the key is total, not partial. `UNIQUE (owner_id, slug)`
with `owner_id NOT NULL` replaces `UNIQUE (slug) WHERE owner_id IS NULL`.

### 1.3 NULL in UNIQUE constraints

All four permit multiple NULLs.

| DB | Wording | Source |
|---|---|---|
| PostgreSQL | "By default, two null values are not considered equal in this comparison." | https://www.postgresql.org/docs/current/ddl-constraints.html |
| PostgreSQL | "The default is that they are distinct, so that a unique index could contain multiple null values in a column." | https://www.postgresql.org/docs/current/sql-createindex.html |
| MySQL | "A `UNIQUE` index permits multiple `NULL` values for columns that can contain `NULL`." | https://dev.mysql.com/doc/refman/8.4/en/create-table.html |
| SQLite | "For the purposes of UNIQUE constraints, NULL values are considered distinct from all other values, including other NULLs." | https://sqlite.org/lang_createtable.html |

`NULLS NOT DISTINCT` exists only in PostgreSQL, added in 15: "Previously `NULL` entries
were always treated as distinct values, but this can now be changed by creating
constraints and indexes using `UNIQUE NULLS NOT DISTINCT`."
(https://www.postgresql.org/docs/release/15.0/)

**Portable rule.** No column that participates in a unique key may be nullable. This is
the single most important portability rule in this document, and it decides Problem A.

### 1.4 CHECK constraints

| DB | Enforced | Disallowed in the expression | Source |
|---|---|---|---|
| PostgreSQL | Yes | "Currently, `CHECK` expressions cannot contain subqueries nor refer to variables other than columns of the current row" | https://www.postgresql.org/docs/current/sql-createtable.html |
| MySQL | 8.0.16+ | AUTO_INCREMENT columns, columns of other tables, nondeterministic built-ins (`NOW()`, `CURRENT_USER()`, `CONNECTION_ID()`), stored/loadable functions, parameters, variables, subqueries | https://dev.mysql.com/doc/refman/8.4/en/create-table-check-constraints.html |
| MariaDB | 10.2.1+ | AUTO_INCREMENT columns. Can be globally switched off with `check_constraint_checks=OFF` | https://mariadb.com/docs/server/reference/sql-statements/data-definition/constraint |
| SQLite | Yes on write | Subqueries. "CHECK constraints are only verified when the table is written, not when it is read." Can be switched off with `PRAGMA ignore_check_constraints=ON` | https://sqlite.org/lang_createtable.html |

Two facts that catch people out:

1. **Before MySQL 8.0.16 a CHECK clause was accepted and silently ignored.** The 8.0.16
   release note: "Previously, MySQL permitted a limited form of `CHECK` constraint syntax,
   but parsed and ignored it. MySQL now implements the core features of table and column
   `CHECK` constraints, for all storage engines."
   (https://dev.mysql.com/doc/relnotes/mysql/8.0/en/news-8-0-16.html) MariaDB behaved the
   same way before 10.2.1. The 10.2.1 version boundary is reported by MariaDB
   documentation search results but is not printed on the CONSTRAINT reference page —
   treat the exact patch number as `unverified`.
2. **A CHECK that evaluates to NULL passes.** PostgreSQL: "It should be noted that a check
   constraint is satisfied if the check expression evaluates to true or the null value…
   Since most expressions will evaluate to the null value if any operand is null, they
   will not prevent null values in the constrained columns."
   (https://www.postgresql.org/docs/current/ddl-constraints.html) So
   `CHECK (owner_id > 0)` does not stop `owner_id` being NULL. You need `NOT NULL` as well.

**Portable subset.** Single-row, single-table, deterministic, no subqueries, no function
calls beyond arithmetic and comparison. Always pair a CHECK with `NOT NULL`.

### 1.5 Generated / computed columns

| DB | Keywords | Indexable | Notes | Source |
|---|---|---|---|---|
| PostgreSQL | `STORED` (12+), `VIRTUAL` (18+) | Stored: yes | 18 made VIRTUAL the default: "Allow generated columns to be virtual, and make them the default" | https://www.postgresql.org/docs/current/ddl-generated-columns.html · https://www.postgresql.org/docs/release/18.0/ |
| MySQL | `VIRTUAL` (default), `STORED` | Both. InnoDB supports secondary indexes on virtual columns | FK cannot reference a virtual generated column; stored generated columns cannot use `CASCADE`/`SET NULL`/`SET DEFAULT` | https://dev.mysql.com/doc/refman/8.4/en/create-table-generated-columns.html |
| MariaDB | `VIRTUAL`, `PERSISTENT` (a.k.a. `STORED`) | "Defining indexes on both `VIRTUAL` and `PERSISTENT` generated columns is supported." | Cannot be a primary key; PERSISTENT columns in FKs do not support `ON UPDATE CASCADE`, `ON UPDATE SET NULL`, `ON DELETE SET NULL` | https://mariadb.com/docs/server/reference/sql-statements/data-definition/create/generated-columns |
| SQLite | `VIRTUAL`, `STORED`, 3.31.0+ | Yes, and may carry `NOT NULL`, `CHECK`, `UNIQUE` and FK constraints | Cannot be part of a PRIMARY KEY; `STORED` columns cannot be added by `ALTER TABLE ADD COLUMN`; no subqueries, aggregates, window functions or nondeterministic functions | https://sqlite.org/gencol.html |

**Landmines.** The default kind flipped in PostgreSQL 18, so *always write `STORED` or
`VIRTUAL` explicitly*. Whether a PostgreSQL 18 **virtual** generated column can be
indexed is `unverified` here — assume it cannot and use `STORED` when you need an index.
`STORED` is the only kind that is indexable on every engine listed above.

### 1.6 DATE and TIMESTAMP

| Type | PostgreSQL | MySQL / MariaDB | SQLite |
|---|---|---|---|
| Calendar date, no zone | `date`, 4 bytes, 4713 BC–5874897 AD | `DATE`, `'1000-01-01'`–`'9999-12-31'` | No type. TEXT/REAL/INTEGER |
| Local wall time | `timestamp without time zone` | `DATETIME`, `'1000-01-01 00:00:00'`–`'9999-12-31 23:59:59'` | No type |
| Instant | `timestamp with time zone` | `TIMESTAMP`, `'1970-01-01 00:00:01'` UTC–`'2038-01-19 03:14:07'` UTC | No type |

Sources: https://www.postgresql.org/docs/current/datatype-datetime.html ·
https://dev.mysql.com/doc/refman/8.4/en/datetime.html ·
https://mariadb.com/docs/server/reference/data-types/date-and-time-data-types/timestamp ·
https://sqlite.org/datatype3.html

How zones are handled:

- PostgreSQL: "All timezone-aware dates and times are stored internally in UTC… the value
  is stored internally as UTC, and the originally stated or assumed time zone is not
  retained."
- MySQL: "MySQL converts `TIMESTAMP` values from the current time zone to UTC for storage,
  and back from UTC to the current time zone for retrieval. (This does not occur for other
  types such as `DATETIME`.)"
- MariaDB: TIMESTAMP values "are converted from the session's time zone to Coordinated
  Universal Time (UTC) when stored, and converted back to the session's time zone when
  retrieved." Range reaches 2106 only from 11.5; earlier versions stop at 2038.
- SQLite: "SQLite does not have a storage class set aside for storing dates and/or times."

**A date without a time zone should be a `DATE` column** (or an ISO-8601 `YYYY-MM-DD`
TEXT column on SQLite). It is a calendar fact, not an instant. Never store a training
day as a timestamp: converting it through a zone will move it by a day.

**Portable rule for instants.** Convert to UTC in Go and store in a
zone-*unaware* column — `timestamp` on PostgreSQL, `DATETIME(6)` on MySQL/MariaDB,
TEXT or INTEGER on SQLite. Do not put the conversion in the database. Three engines
with three different session-zone rules, one of which silently ends in 2038, is not a
portable foundation.

### 1.7 ON DELETE CASCADE / SET NULL

All four support both actions.

| DB | Notes | Source |
|---|---|---|
| PostgreSQL | Standard behaviour | https://www.postgresql.org/docs/current/ddl-constraints.html |
| MySQL | "MySQL parses but ignores inline `REFERENCES` specifications" in a column definition — they act as comments with no enforcement. A table-level `FOREIGN KEY` clause is required. Cascades cannot nest more than 15 levels. `MATCH` is ignored. | https://dev.mysql.com/doc/refman/8.4/en/ansi-diff-foreign-keys.html |
| MariaDB | `RESTRICT`, `CASCADE`, `SET NULL`, `NO ACTION` on InnoDB | https://mariadb.com/docs/server/reference/sql-statements/data-definition/constraint |
| SQLite | "Foreign key constraints are disabled by default (for backwards compatibility), so must be enabled separately for each database connection." | https://sqlite.org/foreignkeys.html |

**Landmine.** `PRAGMA foreign_keys = ON` is per **connection**. A Go connection pool
opens many connections. If the pragma is not applied to every one of them, foreign keys
are enforced on some queries and not others, non-deterministically. Set it in the DSN or
in a connection hook, not once at startup.

### 1.8 Deferrable constraints

| DB | Support | Source |
|---|---|---|
| PostgreSQL | "Currently, only `UNIQUE`, `PRIMARY KEY`, `REFERENCES` (foreign key), and `EXCLUDE` constraints are affected by this setting." `NOT NULL` and `CHECK` are always immediate. | https://www.postgresql.org/docs/current/sql-set-constraints.html |
| MySQL | No. "MySQL checks foreign key constraints immediately; the check is not deferred to transaction commit." | https://dev.mysql.com/doc/refman/8.4/en/ansi-diff-foreign-keys.html |
| MariaDB | Not supported (`unverified` — no MariaDB page confirming this was located) | — |
| SQLite | Foreign keys only. "Deferred foreign key constraints are not checked until the transaction tries to COMMIT." Deferral of UNIQUE/PK is `unverified`. | https://sqlite.org/foreignkeys.html |

**Portable rule.** Design so that no operation ever needs deferral. This directly
forbids `UNIQUE (parent_id, position)` on an ordered list — see §3.5.

### 1.9 Identity / auto-increment

| DB | Syntax | Source |
|---|---|---|
| PostgreSQL | `GENERATED { ALWAYS \| BY DEFAULT } AS IDENTITY` — "It will have an implicit sequence attached to it… Such a column is implicitly `NOT NULL`." | https://www.postgresql.org/docs/current/sql-createtable.html |
| MySQL / MariaDB | `AUTO_INCREMENT` | https://dev.mysql.com/doc/refman/8.4/en/create-table.html |
| SQLite | "If a table contains a column of type INTEGER PRIMARY KEY, then that column becomes an alias for the ROWID." `AUTOINCREMENT` "is not allowed on WITHOUT ROWID tables or on any table column other than INTEGER PRIMARY KEY." | https://sqlite.org/autoinc.html |

**Landmine that matters for immutable history.** Without `AUTOINCREMENT`, SQLite reuses
rowids: "The purpose of AUTOINCREMENT is to prevent the reuse of ROWIDs from previously
deleted rows." A deleted session template can therefore have its id handed to a
*different* template later, and any historical row that kept only a numeric reference now
points at the wrong content. SQLite also warns that AUTOINCREMENT "imposes extra CPU,
memory, disk space, and disk I/O overhead and should be avoided if not strictly needed."
GORM's `autoIncrement` tag emits it. Decide deliberately: either turn it on for tables
whose ids are referenced by history, or (better) do not depend on id stability at all —
see Problem B.

### 1.10 JSON

| DB | Type | Indexing values inside | Source |
|---|---|---|---|
| PostgreSQL | `json` (text copy) and `jsonb` (binary). "In general, most applications should prefer to store JSON data as `jsonb`" | GIN (`jsonb_ops`, `jsonb_path_ops`), btree, hash, and expression indexes on extracted paths | https://www.postgresql.org/docs/current/datatype-json.html |
| MySQL | Native binary `JSON` | "`JSON` columns, like columns of other binary types, are not indexed directly; instead, you can create an index on a generated column that extracts a scalar value from the `JSON` column." InnoDB also supports multi-valued indexes on JSON arrays | https://dev.mysql.com/doc/refman/8.4/en/json.html |
| MariaDB | Not a real type: "`JSON` is an alias for `LONGTEXT COLLATE utf8mb4_bin` introduced for compatibility reasons with MySQL's JSON data type." A `JSON_VALID` CHECK is added automatically | Generated column + index | https://mariadb.com/docs/server/reference/data-types/string-data-types/json |
| SQLite | No JSON type. "SQLite stores JSON as ordinary text." A binary JSONB blob form exists from 3.45.0. Functions built in by default from 3.38.0 | Expression index on `json_extract(...)`, supported from 3.9.0 for deterministic expressions | https://sqlite.org/json1.html · https://sqlite.org/expridx.html |

**Portable rule.** JSON is an opaque blob. Anything you will filter, sort, join or
uniquely constrain on must be a real column. A stored generated column plus an index is
the closest thing to a portable JSON index, and it is still four different dialects of
DDL.

### 1.11 Case sensitivity of string comparison

This is the difference that breaks unique keys between MySQL and PostgreSQL.

| DB | Default | Effect on `UNIQUE (slug)` | Source |
|---|---|---|---|
| MySQL 8.4 | `utf8mb4` / `utf8mb4_0900_ai_ci` — accent-insensitive, **case-insensitive** | `Crimps` and `crimps` **collide** | https://dev.mysql.com/doc/refman/8.4/en/charset-mysql.html |
| MariaDB | MySQL-compatible `*_ci` collations. The default collation for a given MariaDB version was not confirmed here (`unverified`); the `JSON` alias explicitly pins `utf8mb4_bin` | Same as MySQL on any `*_ci` column | https://mariadb.com/docs/server/reference/data-types/string-data-types/json |
| PostgreSQL | All predefined collations are deterministic. "A deterministic collation uses deterministic comparisons, which means that it considers strings to be equal only if they consist of the same byte sequence." | `Crimps` and `crimps` **do not collide** | https://www.postgresql.org/docs/current/collation.html |
| SQLite | `BINARY`. `NOCASE` folds only "the 26 upper case characters of ASCII" | Case-sensitive by default | https://sqlite.org/datatype3.html |

Neither engine can be talked into the other cheaply. PostgreSQL's nondeterministic ICU
collations carry a documented cost: "their use leads to a performance penalty… B-tree
cannot use deduplication with indexes that use a nondeterministic collation. Also,
certain operations are not possible with nondeterministic collations, such as some
pattern matching operations." The `citext` extension exists but the documentation itself
"recommends using nondeterministic collations instead"
(https://www.postgresql.org/docs/current/citext.html).

**Portable rule.** Normalize in Go. Store a `slug` column that is already lowercase,
trimmed and ASCII-folded, and put the unique index on that. Never rely on the collation
to decide whether two names are the same. Keep the human-facing `name` in a separate,
unconstrained column.

Related: MySQL table and database names follow the filesystem — "such names are not
case-sensitive in Windows, but are case-sensitive in most varieties of Unix", governed by
`lower_case_table_names`. MySQL's own advice: "it is best to adopt a consistent
convention, such as always creating and referring to databases and tables using lowercase
names." (https://dev.mysql.com/doc/refman/8.4/en/identifier-case-sensitivity.html)

### 1.12 Length limits

| Limit | PostgreSQL | MySQL / MariaDB | SQLite |
|---|---|---|---|
| Identifier | 63 bytes | 64 characters | Not documented (`unverified`) |
| Columns per index | 32 | 16 (InnoDB, "Exceeding the limit returns an error") | 2000 default |
| Columns per table | 1600 | 1017 (InnoDB) | 2000 default |
| Index key prefix | Not stated on the limits page (`unverified`) | 3072 bytes (DYNAMIC/COMPRESSED), 767 bytes (REDUNDANT/COMPACT), 1000 bytes (MyISAM) | Not documented |
| Single value | 1 GB | 65,535 bytes total row; ~8000 bytes per row at 16K pages | 1,000,000,000 bytes default |

Sources: https://www.postgresql.org/docs/current/limits.html ·
https://dev.mysql.com/doc/refman/8.4/en/identifier-length.html ·
https://mariadb.com/docs/server/reference/sql-structure/sql-language-structure/identifier-names ·
https://dev.mysql.com/doc/refman/8.4/en/innodb-limits.html · https://sqlite.org/limits.html

**Portable budget.** Identifiers ≤ 63 characters. Composite indexes ≤ 16 columns. Any
indexed `VARCHAR` ≤ 191 characters: at 4 bytes per `utf8mb4` character that is 764 bytes,
which fits even the old 767-byte `REDUNDANT`/`COMPACT` limit, and leaves ample room under
3072 bytes for a composite key. MySQL generates foreign-key names itself and will overflow
64 characters if the table name is long, so name every constraint explicitly.

### 1.13 Enum representation

| Approach | Portable | Notes |
|---|---|---|
| Native `ENUM` | No | MySQL/MariaDB only. Max 65,535 elements. Values are stored as index numbers, and "`ENUM` values are sorted based on their index numbers, which depend on the order in which the enumeration members were listed" — so `ORDER BY` on an ENUM is not alphabetical. (https://dev.mysql.com/doc/refman/8.4/en/enum.html) |
| `CREATE TYPE … AS ENUM` | No | PostgreSQL only. "Existing values cannot be removed from an enum type, nor can the sort ordering of such values be changed, short of dropping and re-creating the enum type." Labels are case sensitive. (https://www.postgresql.org/docs/current/datatype-enum.html) |
| `VARCHAR` + `CHECK (col IN (…))` | **Yes** | Works on all four. Adding a value is one `ALTER TABLE`. Remember the NULL rule: pair with `NOT NULL` |
| Lookup table + FK | **Yes** | Correct when the set is data the app or a user may extend, or when the value carries attributes (a label, a sort order, an icon) |

**Recommendation.** `VARCHAR` + `CHECK` + `NOT NULL` for a closed set the code owns
(`origin`, `status`). A lookup table when the set has attributes or grows without a code
change. Never native ENUM.

### 1.14 Boolean representation

| DB | Native | Source |
|---|---|---|
| PostgreSQL | `boolean` | https://www.postgresql.org/docs/current/datatype-boolean.html |
| MySQL / MariaDB | No. `BOOL`/`BOOLEAN` "are synonyms for `TINYINT(1)`"; `TRUE`/`FALSE` are aliases for `1`/`0` | https://dev.mysql.com/doc/refman/8.4/en/numeric-type-syntax.html |
| SQLite | No. "SQLite does not have a separate Boolean storage class. Instead, Boolean values are stored as integers 0 (false) and 1 (true)." | https://sqlite.org/datatype3.html |

**Portable rule.** Use a Go `bool` and let the driver bind the parameter. In raw SQL
never write `WHERE flag = 'true'` or `WHERE flag IS TRUE`; write `WHERE flag = ?`.

### 1.15 Text / CLOB, and whether it can be indexed

| DB | Behaviour | Source |
|---|---|---|
| PostgreSQL | `text` has no length limit and indexes like `varchar` | https://www.postgresql.org/docs/current/limits.html |
| MySQL / MariaDB | "For indexes on `BLOB` and `TEXT` columns, you must specify an index prefix length." Also: "`BLOB` and `TEXT` columns cannot have `DEFAULT` values." | https://dev.mysql.com/doc/refman/8.4/en/blob.html |
| SQLite | `TEXT` indexes and uniques normally. Declared types are advisory: "the type is recommended, not required. Any column can still store any type of data." | https://sqlite.org/datatype3.html |

**Portable rule.** A column that is indexed, unique, or given a default must be
`VARCHAR(n)`. `TEXT` is only for free prose that is never a key. This has a direct GORM
consequence — see §1.17.

### 1.16 Transactional DDL

| DB | Can a `CREATE TABLE` be rolled back inside a transaction? | Source |
|---|---|---|
| PostgreSQL | Yes. "a regular `CREATE INDEX` command can be performed within a transaction block, but `CREATE INDEX CONCURRENTLY` cannot." | https://www.postgresql.org/docs/current/sql-createindex.html · https://wiki.postgresql.org/wiki/Transactional_DDL_in_PostgreSQL:_A_Competitive_Analysis |
| MySQL | No. "Atomic DDL is not transactional DDL. DDL statements, atomic or otherwise, implicitly end any transaction that is active in the current session, as if you had done a `COMMIT` before executing the statement." | https://dev.mysql.com/doc/refman/8.4/en/atomic-ddl.html · https://dev.mysql.com/doc/refman/8.4/en/implicit-commit.html |
| MariaDB | Atomic DDL from 10.6.1: "Atomic means that either the operation succeeds (and is logged to the binary log) or is completely reversed." That is crash safety per statement, not rollback inside a user transaction | https://mariadb.com/docs/server/reference/sql-statements/data-definition/atomic-ddl |
| SQLite | Yes — DDL is a write statement inside the transaction, and the documented table-rewrite procedure runs inside one | https://sqlite.org/lang_transaction.html · https://sqlite.org/lang_altertable.html |

SQLite has a second, larger constraint: `ALTER TABLE` supports only `RENAME TO`,
`RENAME COLUMN`, `ADD COLUMN` and `DROP COLUMN`. A new column cannot carry `PRIMARY KEY`
or `UNIQUE`, cannot be `NOT NULL` without a non-NULL default, and cannot be
`GENERATED … STORED`. A column cannot be dropped if it is indexed, part of a PK, unique,
or referenced by a partial index, CHECK, FK, generated column, trigger or view. Changing
a type or adding a constraint requires the documented 12-step table rewrite.
(https://sqlite.org/lang_altertable.html)

**Portable rule.** One DDL statement per migration step. Every step idempotent and
re-runnable. Never write a migration that assumes it can roll back — on MySQL it cannot,
and you will be left half-migrated.

### 1.17 What GORM can express, and where it leaks

Expressible through struct tags and `AutoMigrate`
(https://gorm.io/docs/models.html · https://gorm.io/docs/indexes.html ·
https://gorm.io/docs/constraints.html):

| Need | Tag |
|---|---|
| Column name, type, size, precision, scale | `column`, `type`, `size`, `precision`, `scale` |
| Primary key, auto-increment | `primaryKey`, `autoIncrement`, `autoIncrementIncrement` |
| NOT NULL, default, comment | `not null`, `default`, `comment` |
| Unique column / unique index | `unique`, `uniqueIndex` |
| Index, composite index with column order | `index`, with `priority` |
| Index options | `class`, `type`, `where`, `comment`, `expression`, `sort`, `collate`, `length`, `option` |
| CHECK | `check:name_checker,name <> 'jinzhu'` |
| FK actions | `constraint:OnUpdate:CASCADE,OnDelete:SET NULL;` |
| Read/write permissions, skip | `<-`, `->`, `-` |
| Timestamps, embedding, serializers | `autoCreateTime`, `autoUpdateTime`, `embedded`, `embeddedPrefix`, `serializer` |

Needs raw SQL migrations:

- **Partial indexes on PostgreSQL/SQLite only.** GORM has a `where` index option, shown in
  its own docs as `index:,sort:desc,collate:utf8,type:btree,length:10,where:name3 != 'jinzhu'`.
  MySQL and MariaDB have no `WHERE` in the `CREATE INDEX` grammar, so the emitted DDL will
  fail there. (The failure itself is an inference from the MySQL grammar, not a quoted
  GORM statement.) Either avoid `where:` entirely — which §1.2's portable alternative lets
  you do — or gate it per dialect in a raw migration.
- `NULLS NOT DISTINCT` — no tag; PostgreSQL only anyway.
- Generated / computed columns — no tag documented on the GORM models page.
- Deferrable constraints, PostgreSQL enum types, row-level security, triggers, and any
  multi-statement data backfill.

Where GORM's portability leaks, from GORM's own documentation:

1. **`string` becomes `longtext` on MySQL.** The driver README documents
   `DefaultStringSize` as "add default size for string fields, by default, will use db
   type `longtext`" for fields with no size, no primary key, no index and no default.
   (https://github.com/go-gorm/mysql) Combine that with MySQL's "For indexes on `BLOB` and
   `TEXT` columns, you must specify an index prefix length" and you get a schema whose
   unique keys work on SQLite and PostgreSQL and fail on MySQL. Set `DefaultStringSize`
   and put an explicit `size:` on every string that is indexed.
2. **`AutoMigrate` is not a migration tool.** GORM: it "will create tables, missing foreign
   keys, constraints, columns and indexes. It will change existing column's type if its
   size, precision changed, or if it's changing from non-nullable to nullable. It **WON'T**
   delete unused columns to protect your data."
   (https://gorm.io/docs/migration.html) It cannot rename, cannot narrow, cannot drop, and
   its behaviour depends on the server version — the MySQL driver ships
   `DontSupportRenameIndex` ("rename index not supported before MySQL 5.7, MariaDB"),
   `DontSupportRenameColumn` ("not supported before MySQL 8, MariaDB") and
   `DisableDatetimePrecision` ("not supported before MySQL 5.6"). Use `AutoMigrate` for
   development convenience and versioned SQL migrations for anything that ships.
3. **Soft delete quietly defeats unique keys.** `gorm.DeletedAt` is a nullable timestamp,
   and every engine treats multiple NULLs as distinct (§1.3), so `UNIQUE (owner_id, slug,
   deleted_at)` does not prevent two live rows — and dropping `deleted_at` from the key
   means a soft-deleted row keeps blocking the slug forever. GORM's own docs concede this:
   "when creating unique composite index for the DeletedAt field, you must use other data
   format like unix second/flag." (https://gorm.io/docs/delete.html) Either use the
   integer-flag soft-delete form, or do not soft-delete rows whose slug is unique.
4. **The `check` tag was a no-op on MySQL before 8.0.16 and MariaDB before 10.2.1**, and
   MariaDB can still be run with `check_constraint_checks=OFF`. A CHECK is a good second
   line of defence, never the only one.
5. **`time.Time` mapping differs per dialect.** Confirm what your driver version emits
   before relying on it; the precision default is itself a driver config option
   (`DefaultDatetimePrecision`). Treat the concrete mapping as `unverified` here.

---

## Part 2 — The two hard problems

### Problem A — shipped reference data alongside user-owned data

Requirements: the catalog belongs to the app; nobody may edit it; a user may copy an item
and own the copy; slugs unique among shipped items, and unique per user among that user's
items.

#### The candidate patterns

**1. Reserved system / sentinel owner row.** Create one real account row that represents
the application, and set `owner_id` to it for every shipped item. `owner_id` stays
`NOT NULL`.

*Verified real system:* Discourse defines `SYSTEM_USER_ID = -1` in `lib/discourse.rb` and
tests membership with `id == Discourse::SYSTEM_USER_ID`; its `human_users` and `real`
scopes filter on `users.id > 0`, so the reserved range is below zero.
(https://github.com/discourse/discourse/blob/main/lib/discourse.rb ·
https://github.com/discourse/discourse/blob/main/app/models/user.rb)

*Trade-offs:* one unique key `(owner_id, slug)` satisfies both slug rules at once, with no
predicate and no NULL. Every join and every scope works unchanged. The costs are that
"who may edit this" is now application logic, not a schema fact, and that a careless
`DELETE FROM accounts` can orphan the whole catalog — so the sentinel row needs its own
guard.

**2. `NOT NULL` discriminator column inside the unique key.** Add
`origin VARCHAR NOT NULL CHECK (origin IN ('catalog','user'))` and key on
`(origin, owner_id, slug)`, or simply keep `(owner_id, slug)` and use `origin` for
authorization and filtering.

This is Fowler's **Single Table Inheritance**: "Represents an inheritance hierarchy of
classes as a single table that has columns for all the fields of the various classes."
(https://martinfowler.com/eaaCatalog/singleTableInheritance.html)

*Trade-offs:* one table, one query path, cheap `UNION`-free reads, and the discriminator
gives you a clean predicate for "is this editable". The cost is columns that are only
meaningful for one variant, and a discriminator that the database will not keep honest for
you beyond the CHECK.

**3. Separate tables for system and user data.** Fowler's **Concrete Table Inheritance**:
"Represents an inheritance hierarchy of classes with one table per concrete class in the
hierarchy." (https://martinfowler.com/eaaCatalog/concreteTableInheritance.html) The middle
option is **Class Table Inheritance**: "Represents an inheritance hierarchy of classes with
one table for each class." (https://martinfowler.com/eaaCatalog/classTableInheritance.html)

*Trade-offs:* immutability becomes structural — grant no write path to `catalog_items` and
it cannot be edited. Each table gets exactly the unique key it needs:
`UNIQUE (slug)` on the shipped table, `UNIQUE (owner_id, slug)` on the user table. The
cost is that every read that spans both needs a `UNION` or two queries, every foreign key
that could point at either needs two nullable columns or a polymorphic pair, and
"copy a catalog item" crosses a table boundary. For a schema where most reads span both
sets, this cost is paid on every page.

**4. Schema-level separation.** Put shipped content in its own namespace.

*Verified real system:* PostgreSQL does exactly this for its own metadata.
"Schema names beginning with `pg_` are reserved for system purposes and cannot be created
by users", and "pg_catalog is always effectively part of the search path. If it is not
named explicitly in the path then it is implicitly searched before searching the path's
schemas." (https://www.postgresql.org/docs/current/ddl-schemas.html)

*Trade-offs:* the cleanest separation and real privilege enforcement — but not portable.
SQLite has no comparable schema namespace inside one database file, and MySQL's "schema"
is a database. Rules out for a three-engine schema.

**5. Row-level security.** PostgreSQL RLS restricts visible rows per role: enable with
`ALTER TABLE … ENABLE ROW LEVEL SECURITY`, then `CREATE POLICY … USING (…)`. With no
policy a default-deny applies. Table owners bypass RLS unless you add
`FORCE ROW LEVEL SECURITY`; superusers and roles with `BYPASSRLS` always bypass.
(https://www.postgresql.org/docs/current/ddl-rowsecurity.html)

*Trade-offs:* the only option that puts the rule in the database rather than the
application, and it composes with the sentinel-owner design. But it is PostgreSQL-only,
needs a per-request database role or session setting, and is invisible to SQLite and
MySQL/MariaDB. If you adopt it, it must be a hardening layer over an app-enforced rule,
never the rule itself.

#### The portable answer

Combine 1 and 2, and skip the rest.

| Element | Choice |
|---|---|
| Table layout | One table per content kind, shipped and user rows together (Single Table Inheritance) |
| Owner | `owner_id BIGINT NOT NULL` referencing accounts, with one reserved account row for the application |
| Discriminator | `origin VARCHAR(16) NOT NULL CHECK (origin IN ('catalog','user'))` |
| Slug uniqueness | `UNIQUE (owner_id, slug)` — one total key, no predicate, no NULL |
| Slug normalization | Lowercased and folded in Go before insert (§1.11) |
| Immutability | One write path in the app that refuses `origin = 'catalog'`; optional trigger as a backstop |
| Copying | `INSERT … SELECT` with a new id, `owner_id` = the copying user, `origin = 'user'` |
| Provenance | `copied_from_id BIGINT NULL REFERENCES … ON DELETE SET NULL` — never `CASCADE` |

Why this and not a partial unique index: `UNIQUE (slug) WHERE owner_id IS NULL` plus
`UNIQUE (owner_id, slug)` is the pattern most people reach for first. It works on
PostgreSQL and SQLite and cannot be expressed on MySQL or MariaDB (§1.2). Giving the
catalog a real owner row collapses two keys into one and needs no dialect-specific DDL at
all. A nullable `owner_id` also means the unique key contains a nullable column, which
§1.3 forbids: two catalog rows with the same slug would both be accepted, everywhere.

---

### Problem B — a historical record that must not change

Requirement: a finished workout renders identically forever, even if the session template
is renamed, restructured or deleted.

#### The candidate patterns

| Pattern | What it is | Named source | Fit here |
|---|---|---|---|
| **Event sourcing** | "Capture all changes to an application state as a sequence of events." State is rebuilt by replaying the log; snapshots are an optimisation and can be cached anywhere because the events are the record | Fowler, *Event Sourcing* (https://martinfowler.com/eaaDev/EventSourcing.html) | Overkill. Solves rebuild-and-replay, which is not the problem. It also does not by itself stop a *template* edit from changing what a replay renders, unless the events already carry the content |
| **Append-only ledger** | Rows are inserted, never updated or deleted | Accounting practice | Right *discipline* for the finished-session table, but not by itself an answer — an append-only row that holds only a template id still changes meaning when the template changes |
| **Snapshot / audit table** | A copy of the referenced state, taken at a point in time, stored beside the record | Named as *Snapshot* / *Audit Log* in Fowler's analysis-patterns catalog — attribution `unverified`, no page fetched | This is the answer |
| **Kimball SCD Type 1** | "the old attribute value in the dimension row is overwritten with the new value; type 1 attributes always reflects the most recent assignment, and therefore this technique destroys history" (https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/type-1/) | Kimball | Exactly what must **not** happen. This is the failure mode |
| **Kimball SCD Type 2** | "changes add a new row in the dimension with the updated attribute values… A minimum of three additional columns should be added…: 1) row effective date or date/time stamp; 2) row expiration date or date/time stamp; and 3) current row indicator" (https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/type-2/) | Kimball | Correct in spirit — the history is preserved by adding rows and pinning the fact to a version. Heavy if applied to every catalog table |
| **Kimball SCD Type 3** | "changes add a new attribute in the dimension to preserve the old attribute value; the new value overwrites the main attribute as in a type 1 change" — an "alternate reality" (https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/type-3/) | Kimball | Wrong shape. Preserves one prior value, not an unbounded history |
| **Copy the price onto the order line** | The order line stores the amount charged, not a reference to the current product price | Accounting practice | The answer, in its simplest and best-tested form |
| **SQL:2011 system-versioned tables** | The engine keeps history automatically: `WITH SYSTEM VERSIONING`, queried with `FOR SYSTEM_TIME AS OF` | MariaDB from 10.6 (https://mariadb.com/docs/server/reference/sql-structure/temporal-tables/system-versioned-tables) | Would be ideal, and is unusable — MariaDB only |

*Verified real system for the copy-on-commit pattern:* Stripe finalizes an invoice and then
freezes it. "Finalizing an invoice… makes certain properties immutable on the invoice",
"After you finalize an invoice, you can't change most of its details", and "Finalizing the
invoice copies the following customer fields to it and makes them immutable" — listing
`customer_address`, `customer_email`, `customer_name`, `customer_phone`,
`customer_shipping`, `customer_tax_exempt`, `customer_tax_ids`. Corrections are made by
issuing a credit note, not by editing the invoice.
(https://docs.stripe.com/invoicing/integration/workflow-transitions)

#### Recommendation: snapshot on commit, append-only, reference for provenance only

When a session finishes, resolve the content and write it into the history record. From
that instant the record depends on nothing else.

| Rule | Detail |
|---|---|
| Snapshot at finish, not at start | The record captures what the user actually did. A start-time snapshot invites edits mid-session |
| Copy every field the render reads | Names, cues, per-item dose numbers, sets, reps, rest, and **the order** |
| Store as child rows, not JSON | `session_log` + `session_log_item` with plain columns. JSON's indexing story is not portable (§1.10). Reserve JSON for a free-text payload nothing queries |
| Denormalize the item order into the child row | A `position INTEGER NOT NULL` on the log item, copied, not recomputed |
| Reference the source loosely | `source_template_id BIGINT NULL REFERENCES session_templates(id) ON DELETE SET NULL`. Provenance and "start again" only. Never `ON DELETE CASCADE` — that is a schema that deletes history |
| Also copy an immutable human label | `source_template_name VARCHAR NOT NULL` so the provenance line still reads correctly after the FK goes NULL |
| Append only | Insert once. No `UPDATE`, no `DELETE`, no soft delete on this table. Corrections are new rows (Stripe's credit note) |
| Version the snapshot shape | `snapshot_version INTEGER NOT NULL` so a future rendering change can read old rows correctly |
| Do not trust the source id to be stable | SQLite reuses rowids after delete (§1.9). This is precisely why the label is copied and not looked up |

**How much duplication is correct.** Copy the transitive closure of what rendering reads,
and nothing else. Concretely: everything shown on the finished-session screen, plus
anything a future summary must aggregate. Do **not** copy the whole catalog graph, the
author's profile, or fields the render never touches — those stay as references, and are
allowed to go stale or NULL.

The correctness test is a single question: **delete the template, then open the finished
session.** If anything is missing, blank, or renamed, you referenced where you should have
copied. Run it as an automated test, not as a thought experiment.

This duplication is not a normalization failure. It is the same reasoning that makes an
order line store the price it charged: the two values answer different questions —
"what does this cost now" versus "what did this cost then" — and are therefore different
facts, not a redundant copy of one fact.

---

## Part 3 — Principles, ranked

Ranked by how expensive the mistake is to fix after the data exists.

### 1. No column that participates in a unique key may be nullable

**Source:** the standard's treatment of NULL in UNIQUE, as documented identically by all
four engines (§1.3), plus Codd's original distinction between a value and the absence of
one — E. F. Codd, "A Relational Model of Data for Large Shared Data Banks",
*Communications of the ACM* 13(6), June 1970 (https://dl.acm.org/doi/10.1145/362384.362685).

**Test:** for every unique index in the schema, assert every column in it is `NOT NULL`.
This is a query against the catalog and should be a CI check. It is first on the list
because it is the mistake that cannot be fixed once duplicates exist.

### 2. Copy history; do not reference it

**Source:** Kimball's Type 1 is defined as the technique that "destroys history"
(https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/type-1/); Type 2 preserves it by adding rows
(https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/type-2/). Stripe's finalized-invoice rule is the same
principle in a production API (https://docs.stripe.com/invoicing/integration/workflow-transitions).

**Test:** delete the source row and re-render the historical record. Automate it.

### 3. Every fact lives in one place, unless rule 2 applies

**Source:** Codd's normalization work, 1970 above; Boyce–Codd normal form from E. F. Codd,
"Recent Investigations in Relational Data Base Systems" (1974) — cited by name, text not
fetched, so treat any wording beyond the title as `unverified`. Date and Fagin,
"Simple conditions for guaranteeing higher normal forms in relational databases",
*ACM TODS* 17(3), 1992, for the practical rule that a schema in 3NF with a single
candidate key is automatically in higher normal forms (title and venue cited; text not
fetched, `unverified`).

**Test:** to change one real-world fact, how many rows must you update? If more than one
and rule 2 does not apply, it is denormalized by accident.

### 4. Constraints belong in the database, not only in Go

**Source:** the referential-integrity requirement of the relational model (Codd, 1970);
practically, the CHECK/FK/NOT NULL support tables in §1.4 and §1.7.

**Test:** load the database in a `sqlite3` or `psql` shell and try to insert a row the app
would reject. If it succeeds, the invariant is a convention, not a constraint. Where the
portable subset genuinely cannot express it — cross-row rules, immutability, tenant
scoping — say so explicitly in a comment and name the single code path that enforces it.

### 5. Surrogate primary keys, natural keys as separate unique constraints

**Source:** Kimball, *Dimension Surrogate Keys*: "Create anonymous integer primary keys
for every dimension. These dimension surrogate keys are simple integers, assigned in
sequence, starting with the value 1", because "Natural keys for a dimension may be created
by more than one source system, and these natural keys may be incompatible or poorly
administered" and "The DW/BI system needs to claim control of the primary keys of all
dimensions."
(https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/dimension-surrogate-key/)

Authorities genuinely disagree here. Kimball is unambiguous in favour of surrogates. C. J.
Date has argued the opposite position — that a well-chosen natural key is preferable and
that surrogate keys hide modelling errors — a stance widely attributed to his writing;
because no Date text was fetched for this document, treat the attribution as
`unverified` and the argument as reported rather than sourced. The practical resolution for
this schema: surrogate `int64` PK on every table, plus a real unique constraint on the
natural key (`(owner_id, slug)`). You get stable foreign keys and an enforced business
rule, and you do not have to choose.

**Test:** can a user rename their slug without any foreign key needing to change? If not,
the natural key is doing a surrogate's job.

### 6. Choose an inheritance mapping deliberately and write down which one

**Source:** Fowler, *Patterns of Enterprise Application Architecture* — Single Table
Inheritance (https://martinfowler.com/eaaCatalog/singleTableInheritance.html), Class Table
Inheritance (https://martinfowler.com/eaaCatalog/classTableInheritance.html), Concrete
Table Inheritance (https://martinfowler.com/eaaCatalog/concreteTableInheritance.html).

**Test:** for every place two kinds of thing share a table or split across tables, name
which of the three patterns it is. If nobody can name it, it was not a decision.

### 7. Deliberate denormalization is correct practice, and needs a stated invariant

Denormalization is not a failure when it is chosen and documented. Legitimate cases:

| Case | Why it is correct |
|---|---|
| Snapshot columns on a historical record | The copy and the original answer different questions (§Problem B) |
| A flattened, denormalized dimension | Kimball: "you should avoid snowflakes because it is difficult for business users to understand and navigate snowflakes. They can also negatively impact query performance." (https://www.kimballgroup.com/data-warehouse-business-intelligence-resources/kimball-techniques/dimensional-modeling-techniques/snowflake-dimension/) — the denormalized form is the recommended one |
| A `position` column | Order is not derivable from the data; it must be stored |
| A cached count or `last_activity_at` | Read cost, with a documented rebuild path |
| A normalized `slug` beside a human `name` | The portable substitute for a case-insensitive collation (§1.11) |

**Test:** state the invariant that keeps the copy correct, and where it is maintained. If
you cannot, the copy is a bug that has not fired yet.

### 8. Model a shallow ordered hierarchy with the cheapest structure that fits

**Sources:** Bill Karwin, *SQL Antipatterns* (Pragmatic Bookshelf), chapter "Naive Trees",
which names **Adjacency List** as the naive pattern and presents **Path Enumeration**,
**Nested Sets** and **Closure Table** as alternatives
(publisher extract: https://media.pragprog.com/titles/bksqla/trees.pdf). Joe Celko,
*Joe Celko's Trees and Hierarchies in SQL for Smarties*, 2nd ed., Elsevier/Morgan Kaufmann,
2012, with chapters on the Adjacency List Model, Path Enumeration Models and the Nested Set
Model (https://shop.elsevier.com/books/joe-celkos-trees-and-hierarchies-in-sql-for-smarties/celko/978-0-12-387733-8).

| Structure | Use when | Cost |
|---|---|---|
| **Distinct tables per level** | The depth is fixed and known (session → block → item) | None. Prefer this |
| **Adjacency list** (`parent_id`) | Depth is variable but shallow, and you never query "all descendants" | Recursive queries. `WITH RECURSIVE` works on all four but is another portability surface |
| **Materialized path** | You need ancestor queries and prefix search, and depth is bounded | String parsing; moves rewrite subtrees; path length limits |
| **Nested sets** | Reads dominate overwhelmingly and the tree is nearly static | Every insert rewrites much of the table. Wrong for user-edited data |
| **Closure table** | Arbitrary-depth ancestor/descendant queries must be fast and cheap to write | An extra table with O(depth) rows per node, and triggers or app code to maintain it |

**Recommendation for this schema:** a fixed two- or three-level structure of distinct
tables. A generic tree for a hierarchy you know is three levels deep buys nothing and costs
recursive SQL on every read. If a variable depth later becomes real, add a closure table
beside the adjacency list rather than replacing it.

**Test:** what is the maximum depth the product allows? If the answer is a small number,
you should not have a tree.

### 9. Model ordered lists with a plain `position`, and never make it unique

`position INTEGER NOT NULL`, a **non-unique** index on `(parent_id, position)`, and
reordering done inside one transaction.

**The unique-constraint swap problem.** `UNIQUE (parent_id, position)` looks right and makes
reordering nearly impossible. Swapping two adjacent items means two `UPDATE`s, and after the
first one two rows share a position. The standard fix is a deferred constraint — which
exists only on PostgreSQL (and, for foreign keys, SQLite); MySQL "checks foreign key
constraints immediately; the check is not deferred to transaction commit" (§1.8). The
workarounds are all bad: a temporary negative sentinel position, or rewriting every row's
position on every move.

Portable options, in order of preference:

| Option | How | Trade-off |
|---|---|---|
| Non-unique `position`, full rewrite on reorder | On any reorder, `UPDATE` every sibling to `1..n` in one transaction | Simple, obviously correct, no deferral needed. Fine for lists of tens of items. **Choose this** |
| Gapped integers | 100, 200, 300; insert at the midpoint; renumber when gaps run out | Fewer writes per move, occasional renumber |
| Fractional / arbitrary-precision positions | Assign the average of the two neighbours' indices. Figma does this, using "arbitrary-precision fractions instead of 64-bit doubles so that we can't run out of precision after lots of edits" (https://www.figma.com/blog/realtime-editing-of-ordered-sequences/) | Indices grow as strings; Figma also notes "Merging new elements from multiple clients may interleave them". Worth it only for concurrent multi-writer editing |

**Test:** write a test that swaps the first two items of a list and asserts the result, and
run it on all three engines. If it needs a deferred constraint, the design is not portable.

### 10. Multi-tenant row scoping, when SQL cannot enforce it

PostgreSQL RLS can enforce it (§Problem A, option 5). MySQL, MariaDB and SQLite cannot.
For a schema that must run on all three, the enforcement lives in Go, so it needs the same
rigour a constraint would get:

| Rule | Detail |
|---|---|
| Every user-owned table has `owner_id NOT NULL` | No exceptions, no nullable "global" rows — that is what the sentinel account is for |
| One narrow data-access layer | Every read and write for owned tables goes through functions that take an owner id as a required argument |
| No ad-hoc query builders in handlers | A handler that can compose its own `WHERE` can forget the scope |
| A test that enumerates the tables | Assert every table in the schema either has `owner_id NOT NULL` or is on an explicit allow-list of unowned tables. New tables fail the test until classified |
| Cross-owner reads are one named function | Sharing is a feature with its own code path and its own tests, not a relaxed scope |
| RLS as optional hardening | If PostgreSQL is the production target, add RLS on top — with `FORCE ROW LEVEL SECURITY`, since "table owners typically bypass RLS" (https://www.postgresql.org/docs/current/ddl-rowsecurity.html). Never as the only layer |

**Test:** delete the `WHERE owner_id = ?` from one query and see whether any test fails. If
not, the scoping is not tested.

### 11. Portability is a property of the DDL, not of the runtime

**Test:** every construct in the schema appears in the "portable" column of §1.1. Anything
that does not is either removed or moved behind a per-dialect raw migration with a comment
saying why.

---

## Appendix — the portable subset, as a checklist

- Surrogate `int64` primary key on every table.
- Every column `NOT NULL` unless NULL genuinely means "unknown".
- Every unique-key column `NOT NULL`. No partial indexes.
- `VARCHAR(n)` with n ≤ 191 for anything indexed. `TEXT` only for unindexed prose.
- No native ENUM. `VARCHAR` + `CHECK` + `NOT NULL`, or a lookup table.
- `DATE` for calendar dates. UTC in a zone-unaware column for instants, converted in Go.
- Foreign keys declared at table level, with explicit constraint names ≤ 63 characters.
- `PRAGMA foreign_keys = ON` applied to every SQLite connection in the pool.
- `ON DELETE SET NULL` from history to source. `CASCADE` never touches a history table.
- No deferrable constraints. No construct that would need one.
- `STORED`, never bare `GENERATED`, if generated columns are used at all.
- JSON is opaque. Anything queried is a real column.
- Slugs normalized in Go, never left to the collation.
- Composite indexes ≤ 16 columns.
- One DDL statement per migration step; every step idempotent; no reliance on rollback.
- History tables are insert-only.
