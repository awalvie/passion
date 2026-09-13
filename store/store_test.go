package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Every store test runs against both engines. A test that only runs on SQLite is not
// done: with ON DELETE RESTRICT, deleting an account succeeded on SQLite and was refused
// on Postgres from identical DDL, and a SQLite-only suite would never have seen it.
//
// Postgres runs when PASSION_TEST_POSTGRES holds a DSN. When it is set and the server is
// unreachable the suite FAILS rather than skipping — otherwise "runs on both" quietly
// degrades to "runs on SQLite".
func eachEngine(t *testing.T, fn func(t *testing.T, s *Store)) {
	t.Helper()

	t.Run("sqlite", func(t *testing.T) {
		s := open(t, Config{
			Engine:   EngineSQLite,
			DSN:      filepath.Join(t.TempDir(), "test.db"),
			LogLevel: logger.Silent,
		})
		fn(t, s)
	})

	dsn := os.Getenv("PASSION_TEST_POSTGRES")
	if dsn == "" {
		t.Log("PASSION_TEST_POSTGRES not set: skipping the Postgres half of this test")
		return
	}
	t.Run("postgres", func(t *testing.T) {
		s := open(t, Config{Engine: EnginePostgres, DSN: dsn, LogLevel: logger.Silent})
		// Each test gets a clean schema. Postgres is a shared server, unlike a temp file.
		if err := s.read(context.Background()).Exec(
			`DROP SCHEMA public CASCADE; CREATE SCHEMA public`).Error; err != nil {
			t.Fatalf("resetting the postgres schema: %v", err)
		}
		if err := s.Migrate(context.Background()); err != nil {
			t.Fatalf("migrate after reset: %v", err)
		}
		fn(t, s)
	})
}

func open(t *testing.T, cfg Config) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatalf("Open(%s): %v", cfg.Engine, err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("Migrate(%s): %v", cfg.Engine, err)
	}
	return s
}

func exec(t *testing.T, s *Store, q string, args ...any) error {
	t.Helper()
	return s.read(context.Background()).Exec(q, args...).Error
}

func mustExec(t *testing.T, s *Store, q string, args ...any) {
	t.Helper()
	if err := exec(t, s, q, args...); err != nil {
		t.Fatalf("%v\n  %s", err, q)
	}
}

func count(t *testing.T, s *Store, q string, args ...any) int64 {
	t.Helper()
	var n int64
	if err := s.read(context.Background()).Raw(q, args...).Scan(&n).Error; err != nil {
		t.Fatalf("%v\n  %s", err, q)
	}
	return n
}

// ---------------------------------------------------------------------------
// The schema itself
// ---------------------------------------------------------------------------

func TestMigrateCreatesEveryTable(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		want := []string{
			"account", "body_measurement", "grade_milestone",
			"content", "content_set", "content_item", "content_item_set", "content_media",
			"tag", "content_tag",
			"plan", "plan_slot", "plan_target", "scheduled", "calendar_event",
			"place",
			"log", "log_entry", "log_set", "log_climb",
		}
		if len(want) != len(Tables()) {
			t.Fatalf("this test names %d tables but Tables() has %d", len(want), len(Tables()))
		}
		for _, tbl := range want {
			if !s.read(context.Background()).Migrator().HasTable(tbl) {
				t.Errorf("table %q is missing", tbl)
			}
		}
	})
}

func TestMigrateIsIdempotent(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		before, err := s.SchemaVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := s.Migrate(ctx); err != nil {
			t.Fatalf("second Migrate: %v", err)
		}
		after, err := s.SchemaVersion(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if before != after {
			t.Errorf("version moved on a second migrate: %d then %d", before, after)
		}
	})
}

// The pragma test. Every cascade and every composite kind check in the schema rests on
// foreign keys being on, and an unrecognised SQLite DSN parameter is accepted SILENTLY —
// so without this, a typo in sqliteDSN would leave the whole schema inert and green.
func TestForeignKeysAreEnforced(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		err := exec(t, s,
			`INSERT INTO content_media (content_id, url) VALUES (999999, 'x')`)
		if err == nil {
			t.Fatal("a child row with a nonexistent parent was accepted: foreign keys are off")
		}
	})
}

func TestMigrationDirectoriesAgree(t *testing.T) {
	sq, err := MigrationVersions(EngineSQLite)
	if err != nil {
		t.Fatal(err)
	}
	pg, err := MigrationVersions(EnginePostgres)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(sq, ",") != strings.Join(pg, ",") {
		t.Errorf("the two dialects have drifted apart:\n  sqlite:   %v\n  postgres: %v", sq, pg)
	}
	if len(sq) == 0 {
		t.Fatal("no migrations are embedded")
	}
}

// Goose owns the schema. AutoMigrate would silently reshape a table out from under a
// migration, so its absence is asserted rather than promised in a comment.
func TestNoAutoMigrate(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		name := e.Name()
		// Test files are excluded: this one names AutoMigrate in its own message, and a
		// check that matches its own text tests itself rather than the code.
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		// A call, not the word. Comments and strings may mention it.
		for i, line := range strings.Split(string(src), "\n") {
			code := line
			if j := strings.Index(code, "//"); j >= 0 {
				code = code[:j]
			}
			if strings.Contains(code, ".AutoMigrate(") {
				t.Errorf("%s:%d calls AutoMigrate; goose owns the schema", name, i+1)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Ownership and the catalog
// ---------------------------------------------------------------------------

func seedAccount(t *testing.T, s *Store, id int64, email string) {
	t.Helper()
	mustExec(t, s, `INSERT INTO account (id, email, password_hash, created_at, updated_at)
	                VALUES (?, ?, 'x', ?, ?)`, id, email, time.Now(), time.Now())
}

func seedContent(t *testing.T, s *Store, id int64, kind, slug string, author *int64) {
	t.Helper()
	var pick *int
	if kind == KindMenu {
		one := 1
		pick = &one
	}
	mustExec(t, s, `INSERT INTO content
	     (uuid, family, id, kind, slug, name, pick_count, author_id, created_at, updated_at)
	     VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		testUUID(id), testUUID(id), id, kind, slug, slug, pick, author, time.Now(), time.Now())
}

// testUUID derives a row's identity from the id a test gives it. These tests insert content
// directly because the subject is a constraint or a cascade rather than the importer, but
// every row still carries an identity. Both columns take the same value: a row with no
// family of its own heads its own series.
func testUUID(id int64) string {
	return fmt.Sprintf("00000000-0000-4000-8000-%012d", id)
}

// Two partial unique indexes, not one, and this is what they buy. Two shipped rows cannot
// share a name. One account cannot hold a name twice. But a person's copy of a shipped row
// KEEPS the shipped name, because the two rows live in different indexes and never meet.
//
// Keeping the name is what keeps a person's history in one series across a copy. An earlier
// design gave the copy a new name and needed a hidden lineage id to stitch the two halves
// back together.
func TestACopyOfShippedContentKeepsItsName(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 10, KindSession, "boulder", nil)

		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,created_at,updated_at)
		                      VALUES (?,?,11,'session','boulder','dup',?,?)`,
			testUUID(11), testUUID(11), time.Now(), time.Now()); err == nil {
			t.Error("two shipped rows shared a kind and slug")
		}

		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
		                      VALUES (?,?,12,'session','boulder','mine',?,?,?)`,
			testUUID(12), testUUID(12), one, time.Now(), time.Now()); err != nil {
			t.Errorf("a copy of shipped content could not keep its name: %v", err)
		}

		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
		                      VALUES (?,?,13,'session','boulder','again',?,?,?)`,
			testUUID(13), testUUID(13), one, time.Now(), time.Now()); err == nil {
			t.Error("one account held the same kind and slug twice")
		}
	})
}

// One name, held four times at once: by the app, by two people who copied the app's row and
// edited it, and by a third who made hers from scratch and never saw the app's.
//
// This is what the two partial unique indexes are for. A name is unique among the app's rows,
// and unique among ONE account's rows, and that is the whole rule. Nothing compares one
// account's names to another's, so two people choosing the same word never meet.
func TestOneNameCanBeHeldByTheAppAndEveryAccount(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		for i, who := range []string{"alice", "bob", "carol"} {
			seedAccount(t, s, int64(i+1), who+"@example.com")
		}
		now := time.Now()
		mk := func(id int64, author *int64, name string) {
			mustExec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
			                VALUES (?,?,?,'block','drills',?,?,?,?)`,
				testUUID(id), testUUID(id), id, name, author, now, now)
		}
		a, b, c := int64(1), int64(2), int64(3)
		mk(10, nil, "Drills")                 // the app ships it
		mk(11, &a, "Drills, Alice's version") // Alice copied it and edited
		mk(12, &b, "Drills, Bob's version")   // Bob copied it and edited
		mk(13, &c, "Carol's own drills")      // Carol made hers from scratch

		var rows []Content
		if err := s.read(ctx).Where("slug = ?", "drills").Order("id").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if len(rows) != 4 {
			t.Fatalf("%d rows called drills, want 4", len(rows))
		}
		for _, r := range rows {
			owner := "the app"
			if r.AuthorID != nil {
				owner = fmt.Sprintf("account %d", *r.AuthorID)
			}
			t.Logf("id=%d slug=%q owned by %s — %q", r.ID, r.Slug, owner, r.Name)
		}

		// What each person can see. Nobody sees anybody else's.
		for _, id := range []int64{1, 2, 3} {
			n := count(t, s, `SELECT count(*) FROM content
			                  WHERE slug='drills' AND (author_id IS NULL OR author_id = ?)`, id)
			if n != 2 {
				t.Errorf("account %d can see %d rows called drills, want 2 (the app's and its own)", id, n)
			}
		}

		// Nobody can hold two. That is the only rule.
		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
		                      VALUES (?,?,14,'block','drills','Second one',?,?,?)`,
			testUUID(14), testUUID(14), a, now, now); err == nil {
			t.Error("one account held two blocks called drills")
		}
		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,created_at,updated_at)
		                      VALUES (?,?,15,'block','drills','Second shipped',?,?)`,
			testUUID(15), testUUID(15), now, now); err == nil {
			t.Error("the app shipped two blocks called drills")
		}
	})
}

// A name only has to be unique within its kind, which is what lets every reference name the
// kind it expects. Both indexes include the kind, so this needs no extra rule.
func TestTwoKindsCanHoldOneName(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedContent(t, s, 10, KindBlock, "drills", nil)
		if err := exec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,pick_count,created_at,updated_at)
		                      VALUES (?,?,11,'menu','drills','Drills',1,?,?)`,
			testUUID(11), testUUID(11), time.Now(), time.Now()); err != nil {
			t.Errorf("a block and a menu could not share a name: %v", err)
		}
	})
}

// What the family is for. Runs from before and after a copy answer as one series, and the
// query needs nothing but the family and a join to log.
//
// Two movements sharing a SLUG do not merge, which is the fault this replaced: a person's
// own movement and one the app ships can be called the same thing and be different
// exercises.
func TestProgressionSpansACopyAsOneSeries(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 10, KindMovement, "wpu", nil) // the app's
		// The person's copy: a new id, the app row's family, and the same slug.
		mustExec(t, s, `INSERT INTO content
		     (uuid,family,id,kind,slug,name,author_id,created_at,updated_at)
		     VALUES (?,?,11,'movement','wpu','Weighted Pull-Ups',?,?,?)`,
			testUUID(11), testUUID(10), one, time.Now(), time.Now())

		for i, d := range []string{"2026-01-01", "2026-02-01"} {
			ranFrom := int64(10)
			if i == 1 {
				ranFrom = 11 // after the copy, the run came from their own row
			}
			logID, entryID := uuid.NewString(), uuid.NewString()
			mustExec(t, s, `INSERT INTO log (id,account_id,on_date,state,created_at,updated_at)
			                VALUES (?,1,?,'done',?,?)`, logID, d, time.Now(), time.Now())
			mustExec(t, s, `INSERT INTO log_entry
			     (id,log_id,position,movement_id,movement_family,movement_slug,movement_name,
			      created_at,updated_at)
			     VALUES (?,?,?,?,?,'wpu','Weighted Pull-Ups',?,?)`,
				entryID, logID, i, ranFrom, testUUID(10), time.Now(), time.Now())
		}

		// The real progression query: ix_entry_movement by its leading column, then log by
		// its primary key for the account.
		if n := count(t, s, `SELECT count(*) FROM log_entry e JOIN log l ON l.id = e.log_id
		                     WHERE l.account_id = 1 AND e.movement_family = ?`,
			testUUID(10)); n != 2 {
			t.Errorf("progression across the copy returned %d rows, want 2 as one series", n)
		}

		// Another account's own movement, also called wpu, and not a copy of anything. Same
		// slug, different family, so it shares none of this history.
		two := int64(2)
		seedAccount(t, s, 2, "b@b.c")
		seedContent(t, s, 12, KindMovement, "wpu", &two)
		if n := count(t, s, `SELECT count(*) FROM log_entry WHERE movement_family = ?`,
			testUUID(12)); n != 0 {
			t.Error("history merged two movements that only share a name")
		}
	})
}

// source_tree is the row's own answer to "does a file still own me".
//
// Editable is a different question and a simpler one: everything you own is editable.
// Editing a row a file owns is allowed and detaches it, which is what FromAFile warns a
// caller to say before writing. Only what the app ships is never edited; there the app
// offers a copy instead.
func TestSourceTreeSaysWhetherAFileOwnsARow(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		one := int64(1)
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 10, KindMovement, "shipped_one", nil)
		seedContent(t, s, 11, KindMovement, "typed_by_hand", &one)
		mustExec(t, s, `UPDATE content SET source_tree='shipped' WHERE id=10`)
		mustExec(t, s, `INSERT INTO content (uuid,family,id,kind,slug,name,author_id,source_tree,created_at,updated_at)
		                VALUES (?,?,12,'movement','from_my_tree','Mine',?,'private',?,?)`,
			testUUID(12), testUUID(12), one, time.Now(), time.Now())

		var rows []Content
		if err := s.read(context.Background()).Order("id").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		want := []struct {
			shipped, fromFile, editable bool
		}{
			{true, true, false},  // 10: the app ships it, so it is copied rather than edited
			{false, false, true}, // 11: you typed it here
			{false, true, true},  // 12: yours, a file owns it, editing detaches it
		}
		for i, w := range want {
			c := rows[i]
			if c.Shipped() != w.shipped || c.FromAFile() != w.fromFile || c.Editable() != w.editable {
				t.Errorf("%s: shipped=%v fromAFile=%v editable=%v, want %v/%v/%v",
					c.Slug, c.Shipped(), c.FromAFile(), c.Editable(), w.shipped, w.fromFile, w.editable)
			}
		}
	})
}

func TestContentTreeRejectsIllegalEdges(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedContent(t, s, 10, KindSession, "sess", nil)
		seedContent(t, s, 20, KindBlock, "blk", nil)
		seedContent(t, s, 30, KindMovement, "mv", nil)
		seedContent(t, s, 40, KindMenu, "menu", nil)

		legal := [][4]any{
			{10, KindSession, 20, KindBlock},
			{20, KindBlock, 30, KindMovement},
			{20, KindBlock, 40, KindMenu},
			{40, KindMenu, 30, KindMovement},
		}
		for i, e := range legal {
			if err := exec(t, s, `INSERT INTO content_item (parent_id,parent_kind,child_id,child_kind,position)
			                      VALUES (?,?,?,?,?)`, e[0], e[1], e[2], e[3], i); err != nil {
				t.Errorf("legal edge %v %v rejected: %v", e[1], e[3], err)
			}
		}

		illegal := [][4]any{
			{10, KindSession, 30, KindMovement},  // a session cannot hold a movement
			{30, KindMovement, 30, KindMovement}, // nothing leaves a movement, so no cycles
			{20, KindBlock, 10, KindSession},     // a block cannot hold a session
			{10, KindSession, 30, KindBlock},     // the kind lies about the row
		}
		for _, e := range illegal {
			if err := exec(t, s, `INSERT INTO content_item (parent_id,parent_kind,child_id,child_kind,position)
			                      VALUES (?,?,?,?,99)`, e[0], e[1], e[2], e[3]); err == nil {
				t.Errorf("illegal edge %v -> %v was accepted", e[1], e[3])
			}
		}
	})
}

// The cross-engine one. With ON DELETE RESTRICT this passed on SQLite and failed on
// Postgres from identical DDL.
func TestDeletingAnAccountTakesItsDataAndLeavesTheCatalog(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		one, two := int64(1), int64(2)
		seedAccount(t, s, 1, "mine@b.c")
		seedAccount(t, s, 2, "other@b.c")

		seedContent(t, s, 10, KindSession, "shipped", nil)
		seedContent(t, s, 11, KindSession, "mine", &one)
		seedContent(t, s, 12, KindSession, "theirs", &two)

		mustExec(t, s, `INSERT INTO plan (id,account_id,name,starts_on,weeks,created_at,updated_at)
		                VALUES (5,1,'C','2026-01-05',4,?,?)`, time.Now(), time.Now())
		mustExec(t, s, `INSERT INTO plan_slot (plan_id,session_id,weekday) VALUES (5,11,2)`)
		mustExec(t, s, `INSERT INTO scheduled (account_id,plan_id,session_id,on_date,created_at)
		                VALUES (1,5,11,'2026-01-06',?)`, time.Now())
		mustExec(t, s, `INSERT INTO place (account_id,kind,name) VALUES (1,'gym','The Castle')`)
		mustExec(t, s, `INSERT INTO body_measurement (account_id,taken_on) VALUES (1,'2026-01-01')`)

		logID, entryID := uuid.NewString(), uuid.NewString()
		mustExec(t, s, `INSERT INTO log (id,account_id,on_date,state,created_at,updated_at)
		                VALUES (?,1,'2026-01-06','done',?,?)`, logID, time.Now(), time.Now())
		mustExec(t, s, `INSERT INTO log_entry
		     (id,log_id,position,movement_family,movement_slug,movement_name,created_at,updated_at)
		     VALUES (?,?,0,?,'mv','MV',?,?)`,
			entryID, logID, testUUID(900), time.Now(), time.Now())
		mustExec(t, s, `INSERT INTO log_set (id,log_entry_id,set_index,updated_at)
		                VALUES (?,?,1,?)`, uuid.NewString(), entryID, time.Now())

		shipped := count(t, s, `SELECT count(*) FROM content WHERE author_id IS NULL`)

		if err := exec(t, s, `DELETE FROM account WHERE id=1`); err != nil {
			t.Fatalf("could not delete an account that owns a session used by its own plan: %v", err)
		}

		for _, c := range []struct{ what, q string }{
			{"their content", `SELECT count(*) FROM content WHERE author_id=1`},
			{"their plans", `SELECT count(*) FROM plan WHERE account_id=1`},
			{"their plan slots", `SELECT count(*) FROM plan_slot WHERE plan_id=5`},
			{"their scheduled rows", `SELECT count(*) FROM scheduled WHERE account_id=1`},
			{"their places", `SELECT count(*) FROM place WHERE account_id=1`},
			{"their measurements", `SELECT count(*) FROM body_measurement WHERE account_id=1`},
			{"their logs", `SELECT count(*) FROM log WHERE account_id=1`},
			{"their log entries", `SELECT count(*) FROM log_entry e JOIN log l ON l.id = e.log_id WHERE l.account_id=1`},
			{"their log sets", `SELECT count(*) FROM log_set`},
		} {
			if n := count(t, s, c.q); n != 0 {
				t.Errorf("%s survived the delete: %d rows", c.what, n)
			}
		}

		if n := count(t, s, `SELECT count(*) FROM content WHERE author_id IS NULL`); n != shipped {
			t.Errorf("the shipped catalog changed: %d before, %d after", shipped, n)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE author_id=2`); n != 1 {
			t.Errorf("the other account's content was touched: %d rows left", n)
		}
	})
}

// ---------------------------------------------------------------------------
// The log
// ---------------------------------------------------------------------------

// The rule the whole design rests on: a finished session renders with the entire catalog
// absent, from its own frozen columns.
//
// Note what the rule is and is not. The log does hold nullable foreign keys into content
// and scheduled, deliberately, so runs can be grouped by the session they came from. The
// rule is that nothing READS them to render history — every name and number shown is this
// table's own copy.
func TestHistorySurvivesTheCatalogBeingDropped(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 30, KindMovement, "wpu", nil)

		logID, entryID := uuid.NewString(), uuid.NewString()
		mustExec(t, s, `INSERT INTO log (id,account_id,on_date,session_name,state,created_at,updated_at)
		                VALUES (?,1,'2026-03-01','Boulder Session','done',?,?)`,
			logID, time.Now(), time.Now())
		mustExec(t, s, `INSERT INTO log_entry
		     (id,log_id,position,movement_id,movement_family,movement_slug,movement_name,
		      t_sets,t_reps,created_at,updated_at)
		     VALUES (?,?,0,30,?,'wpu','Weighted Pull-Ups',4,6,?,?)`,
			entryID, logID, testUUID(30), time.Now(), time.Now())

		// CASCADE, because the log holds nullable pointers back into content and
		// scheduled — for grouping only, never for rendering. Postgres refuses a bare
		// DROP while those constraints exist; SQLite does not check. Dropping the
		// constraints along with the tables is exactly the state being tested: the
		// catalog is gone and history has to stand on its own frozen columns.
		cascade := ""
		if s.Engine() == EnginePostgres {
			cascade = " CASCADE"
		}
		for _, tbl := range []string{
			"content_item_set", "content_item", "content_tag", "content_media",
			"plan_target", "plan_slot", "scheduled", "content",
		} {
			mustExec(t, s, "DROP TABLE "+tbl+cascade)
		}

		var slug, name, session string
		if err := s.read(context.Background()).Raw(
			`SELECT e.movement_slug, e.movement_name, l.session_name
			 FROM log_entry e JOIN log l ON l.id = e.log_id LIMIT 1`).
			Row().Scan(&slug, &name, &session); err != nil {
			t.Fatalf("history could not be read with the catalog gone: %v", err)
		}
		if slug != "wpu" || name != "Weighted Pull-Ups" || session != "Boulder Session" {
			t.Errorf("history lost its frozen copies: %q %q %q", slug, name, session)
		}
	})
}

func TestRenamingDoesNotRewriteHistory(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 10, KindSession, "boulder", nil)
		mustExec(t, s, `INSERT INTO place (id,account_id,kind,name) VALUES (1,1,'gym','The Castle')`)

		logID := uuid.NewString()
		mustExec(t, s, `INSERT INTO log
		     (id,account_id,on_date,session_id,session_name,place_id,place_name,state,created_at,updated_at)
		     VALUES (?,1,'2026-03-01',10,'Boulder Session',1,'The Castle','done',?,?)`,
			logID, time.Now(), time.Now())

		mustExec(t, s, `UPDATE content SET name='Bouldering (revised)' WHERE id=10`)
		mustExec(t, s, `UPDATE place SET name='The Castle Climbing Centre' WHERE id=1`)

		var session, place string
		if err := s.read(context.Background()).Raw(
			`SELECT session_name, place_name FROM log WHERE id=?`, logID).
			Row().Scan(&session, &place); err != nil {
			t.Fatal(err)
		}
		if session != "Boulder Session" {
			t.Errorf("renaming the session rewrote history: %q", session)
		}
		if place != "The Castle" {
			t.Errorf("renaming the gym rewrote history: %q", place)
		}
	})
}

// ---------------------------------------------------------------------------
// Targets and ladders
// ---------------------------------------------------------------------------

func TestPlanTargetRefusesTwoWholeCycleRows(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedAccount(t, s, 1, "a@b.c")
		seedContent(t, s, 30, KindMovement, "wpu", nil)
		mustExec(t, s, `INSERT INTO plan (id,account_id,name,starts_on,weeks,created_at,updated_at)
		                VALUES (5,1,'C','2026-01-05',12,?,?)`, time.Now(), time.Now())

		mustExec(t, s, `INSERT INTO plan_target (plan_id,movement_id,week,sets,reps) VALUES (5,30,0,5,5)`)
		if err := exec(t, s, `INSERT INTO plan_target (plan_id,movement_id,week,sets,reps)
		                      VALUES (5,30,0,3,3)`); err == nil {
			t.Error("a second whole-cycle target for one movement was accepted")
		}
		if err := exec(t, s, `INSERT INTO plan_target (plan_id,movement_id,week,sets,reps)
		                      VALUES (5,30,3,6,4)`); err != nil {
			t.Errorf("a week-3 target was rejected: %v", err)
		}
		if err := exec(t, s, `INSERT INTO plan_target (plan_id,movement_id,week) VALUES (5,30,-1)`); err == nil {
			t.Error("a negative week was accepted")
		}
	})
}

// A ladder's rungs are reps inside one set, not sets of their own. Without rep_index in
// the key, 3s / 6s / 9s could not be stored at all.
// The reason content_set exists. "Hangboard Ladder: Half Crimp" is a 3s, then 6s, then 9s
// hang, and that shape is what the movement IS -- its name, slug and notes all say so.
// Before this table a ladder could only be written inside one block, so the library could
// not hold one and three movements carried names describing a shape they did not have.
func TestAMovementCanBeALadder(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedContent(t, s, 10, KindMovement, "hangboard_ladder_half_crimp", nil)
		mustExec(t, s, `UPDATE content SET d_sets=3, movement_style='timed_reps' WHERE id=10`)

		// One set of three reps, each a different length. rep_index carries the rung.
		for i, secs := range []int{3, 6, 9} {
			mustExec(t, s, `INSERT INTO content_set (content_id,set_index,rep_index,seconds)
			                VALUES (10,1,?,?)`, i, secs)
		}

		rows, err := s.read(context.Background()).Raw(
			`SELECT seconds FROM content_set WHERE content_id=10 AND set_index=1
			 ORDER BY rep_index`).Rows()
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got []int
		for rows.Next() {
			var n int
			if err := rows.Scan(&n); err != nil {
				t.Fatal(err)
			}
			got = append(got, n)
		}
		if len(got) != 3 || got[0] != 3 || got[1] != 6 || got[2] != 9 {
			t.Fatalf("the ladder read back as %v, want [3 6 9]", got)
		}

		// A block that uses it carries no rows of its own; it inherits the shape.
		seedContent(t, s, 20, KindBlock, "integrated_strength_a", nil)
		mustExec(t, s, `INSERT INTO content_item (parent_id,parent_kind,child_id,child_kind,position)
		                VALUES (20,'block',10,'movement',0)`)
		if n := count(t, s, `SELECT count(*) FROM content_item_set`); n != 0 {
			t.Errorf("the block carried %d per-set rows; it should inherit the movement's", n)
		}

		// Deleting the movement takes its ladder with it.
		mustExec(t, s, `DELETE FROM content_item WHERE parent_id=20`)
		mustExec(t, s, `DELETE FROM content WHERE id=10`)
		if n := count(t, s, `SELECT count(*) FROM content_set WHERE content_id=10`); n != 0 {
			t.Errorf("%d ladder rows outlived their movement", n)
		}
	})
}

// per_side exists because 24 movements say "per side" in prose only, so the app undercounts
// them. It defaults to false, which is what every other movement needs.
func TestPerSideDefaultsToFalse(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedContent(t, s, 10, KindMovement, "bulgarian_split_squats", nil)
		mustExec(t, s, `UPDATE content SET d_sets=1, d_reps=6, per_side=true WHERE id=10`)
		seedContent(t, s, 11, KindMovement, "bench_press", nil)

		var rows []Content
		if err := s.read(context.Background()).Order("id").Find(&rows).Error; err != nil {
			t.Fatal(err)
		}
		if !rows[0].PerSide {
			t.Error("per_side did not survive a write")
		}
		if rows[1].PerSide {
			t.Error("per_side defaulted to true; it must default to false")
		}
	})
}

func TestLadderRungsFitInsideOneSet(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		seedContent(t, s, 20, KindBlock, "blk", nil)
		seedContent(t, s, 30, KindMovement, "ladder", nil)
		mustExec(t, s, `INSERT INTO content_item (id,parent_id,parent_kind,child_id,child_kind,position,sets)
		                VALUES (1,20,'block',30,'movement',0,1)`)

		for i, sec := range []int{3, 6, 9} {
			if err := exec(t, s, `INSERT INTO content_item_set (content_item_id,set_index,rep_index,seconds)
			                      VALUES (1,1,?,?)`, i+1, sec); err != nil {
				t.Fatalf("rung %d rejected: %v", i+1, err)
			}
		}
		if n := count(t, s, `SELECT count(*) FROM content_item_set WHERE content_item_id=1 AND set_index=1`); n != 3 {
			t.Errorf("expected 3 rungs in set 1, got %d", n)
		}
		if err := exec(t, s, `INSERT INTO content_item_set (content_item_id,set_index,rep_index,seconds)
		                      VALUES (1,1,2,99)`); err == nil {
			t.Error("a duplicate (set, rep) was accepted")
		}
	})
}

// ---------------------------------------------------------------------------
// Transactions
// ---------------------------------------------------------------------------

func TestWithTxRollsBackEverything(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		seedAccount(t, s, 1, "a@b.c")

		wantErr := fmt.Errorf("deliberate")
		err := s.WithTx(ctx, func(tx *gorm.DB) error {
			if err := tx.Exec(`INSERT INTO content (uuid,family,id,kind,slug,name,created_at,updated_at)
			                   VALUES (?,?,10,'session','a','A',?,?)`,
				testUUID(10), testUUID(10), time.Now(), time.Now()).Error; err != nil {
				return err
			}
			if err := tx.Exec(`INSERT INTO content_item (parent_id,parent_kind,child_id,child_kind,position)
			                   VALUES (10,'session',10,'session',0)`).Error; err == nil {
				t.Error("an illegal edge was accepted inside a transaction")
			}
			return wantErr
		})
		if err != wantErr {
			t.Fatalf("WithTx returned %v, want the callback's error", err)
		}
		if n := count(t, s, `SELECT count(*) FROM content WHERE id=10`); n != 0 {
			t.Errorf("the rolled-back insert survived: %d rows", n)
		}
	})
}
