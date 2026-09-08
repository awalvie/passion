// Package store is the only package that touches the database. It exports no database
// handle: a caller gets a *Store and can reach the data only through its methods.
//
// Two conventions hold throughout, and both are load-bearing:
//
//   - A nullable number is a pointer. NULL means "not asked for"; zero is a real value,
//     because 0 kg is bodyweight and 0 reps is not the same as no target.
//   - Goose owns the schema. These structs mirror the migrations and are never used with
//     AutoMigrate, so a struct that drifts from a migration is a bug in the struct.
package store

import "time"

// Date is a calendar day with no time and no zone, held as "YYYY-MM-DD". SQLite has no
// DATE type, and a fixed-width ISO string sorts and compares correctly on both engines.
// All date arithmetic happens in Go.
type Date = string

// ---------------------------------------------------------------------------
// Accounts
// ---------------------------------------------------------------------------

type Account struct {
	ID                 int64  `gorm:"primaryKey"`
	Email              string `gorm:"size:255;not null;uniqueIndex:ux_account_email"`
	PasswordHash       string `gorm:"size:255;not null"`
	HeightCm           *int
	ApeIndexCm         *int
	BoulderGradeSystem string `gorm:"size:32;not null;default:font"`
	RouteGradeSystem   string `gorm:"size:32;not null;default:french"`
	MaxPullUps         *int
	MaxHangKg          *float64 `gorm:"type:numeric(5,2)"`

	// TimeZone decides which day "today" is. Every heatmap, streak and calendar boundary
	// reads it, so it belongs to the athlete and not to the server.
	TimeZone string `gorm:"size:64;not null;default:UTC"`

	// TokenEpoch is a JWT claim, compared on every request. Changing a password bumps it,
	// which is the only way a stateless token is revoked before it expires.
	TokenEpoch int `gorm:"not null;default:0"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

type BodyMeasurement struct {
	ID        int64    `gorm:"primaryKey"`
	AccountID int64    `gorm:"not null;uniqueIndex:ux_body_measurement,priority:1"`
	TakenOn   Date     `gorm:"type:varchar(10);not null;uniqueIndex:ux_body_measurement,priority:2"`
	WeightKg  *float64 `gorm:"type:numeric(5,2)"`
}

// GradeMilestone stores the grade as written and a number beside it. Without the ordinal a
// grade cannot be sorted, so progression is not expressible in SQL at all.
type GradeMilestone struct {
	ID           int64  `gorm:"primaryKey"`
	AccountID    int64  `gorm:"not null;uniqueIndex:ux_grade_milestone,priority:1"`
	Discipline   string `gorm:"size:16;not null;uniqueIndex:ux_grade_milestone,priority:2"`
	TakenOn      Date   `gorm:"type:varchar(10);not null;uniqueIndex:ux_grade_milestone,priority:3"`
	Grade        string `gorm:"size:32;not null"`
	GradeOrdinal int    `gorm:"not null"`
}

// ---------------------------------------------------------------------------
// Content — one table for what the app ships and what a user writes
// ---------------------------------------------------------------------------

// Content kinds. A menu is its own kind rather than a movement holding movements, which is
// what makes a cycle in the tree structurally impossible: no legal edge leaves a movement.
const (
	KindMovement = "movement"
	KindMenu     = "menu"
	KindBlock    = "block"
	KindSession  = "session"
)

const (
	BlockWarmup   = "warmup"
	BlockMain     = "main"
	BlockCooldown = "cooldown"
)

type Content struct {
	ID   int64  `gorm:"primaryKey"`
	Kind string `gorm:"size:16;not null"`
	Slug string `gorm:"size:128;not null"`
	Name string `gorm:"size:255;not null"`

	// ContentKey is what a finished run points at. It is NOT unique: a fork inherits its
	// parent's value, and that inheritance is what holds one progression together across an
	// edit. Never put a unique index on it.
	//
	// Not named UUID for that reason. A column called uuid invites the constraint that would
	// break forking.
	ContentKey string `gorm:"type:char(36);not null;index"`

	// SourceTree is the name of the catalog tree that owns this row, or nil when a person
	// made it in the app. Every fork is nil. A re-import rewrites exactly the rows whose
	// SourceTree matches the tree being imported, which is what keeps a YAML file
	// meaningful after its first import instead of dead.
	//
	// A tree name, never a path: paths differ between machines.
	SourceTree *string `gorm:"size:64"`

	// AuthorID nil means the app ships this row. No account holds it, so no account's
	// deletion can reach it. Nothing in the schema sets this column to NULL on delete —
	// that would silently publish a user's private content into the shipped catalog.
	AuthorID *int64 `gorm:"index"`

	// ForkedFromID names the row this was copied from. The copy takes a NEW slug and
	// inherits ContentKey.
	//
	// It has to take a new slug. A fork of a row from a private tree has the same author as
	// its source, and the per-author unique index is (author_id, kind, slug) — so a copy
	// that kept the slug could never insert. History is safe anyway, because ContentKey is
	// the durable link now, not the slug.
	//
	// This is also why "show my version, hide the one it replaced" resolves through this
	// column and not by matching slugs.
	ForkedFromID *int64 `gorm:"index"`

	Notes  string `gorm:"type:text;not null;default:''"`
	Source string `gorm:"size:64;not null;default:''"`

	// RetiredOn is set by the importer when a shipped slug leaves the YAML tree. The
	// library hides these; plans and logs that reference them keep resolving.
	RetiredOn *Date `gorm:"type:varchar(10)"`

	// Movement columns. Defaults only — what a session asks for lives on ContentItem.
	MovementKind *string `gorm:"size:32"`

	// PerSide is true when the numbers are per side. Without it, "1 set of 6, per side" can
	// only be said in prose, and the player counts 6 when the athlete owes 12.
	PerSide bool `gorm:"not null;default:false"`

	DSets           *int
	DReps           *int
	DWeightKg       *float64 `gorm:"type:numeric(6,2)"`
	DRepSeconds     *int
	DRepRestSeconds *int
	DSetRestSeconds *int
	DPrepSeconds    *int
	DSeconds        *int

	// Block columns.
	BlockKind *string `gorm:"size:16"`

	// Menu columns. PickCount is the fewest options you must choose, not the most.
	PickCount *int

	// Session columns.
	Color string `gorm:"size:16;not null;default:''"`
	Needs string `gorm:"size:255;not null;default:''"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// Shipped reports whether the app ships this row rather than a user having written it.
func (c Content) Shipped() bool { return c.AuthorID == nil }

// FromAFile reports whether a catalog tree owns this row. Such a row is read-only in the
// app for the same reason shipped content is: a file is the source, and an edit made here
// would be overwritten by the next import with no message. Editing one forks it instead.
func (c Content) FromAFile() bool { return c.SourceTree != nil }

// Editable is the one question a handler should ask before letting a write through. It is
// not the same as "mine": your imported private content is yours and still read-only.
func (c Content) Editable() bool { return !c.Shipped() && !c.FromAFile() }

// ContentItem is one edge in the content tree, and the numbers for that use.
//
// The numbers live here and not on the movement. That is what lets one movement row be
// referenced by every session that uses it, instead of being copied into each.
//
// ParentKind and ChildKind duplicate Content.Kind, but a composite foreign key keeps each
// honest, so they cannot disagree with the rows they point at.
type ContentItem struct {
	ID         int64  `gorm:"primaryKey"`
	ParentID   int64  `gorm:"not null"`
	ParentKind string `gorm:"size:16;not null"`
	ChildID    int64  `gorm:"not null"`
	ChildKind  string `gorm:"size:16;not null"`
	Position   int    `gorm:"not null"`

	Sets           *int
	Reps           *int
	WeightKg       *float64 `gorm:"type:numeric(6,2)"`
	RepSeconds     *int
	RepRestSeconds *int
	SetRestSeconds *int
	PrepSeconds    *int
	Seconds        *int
	Notes          string `gorm:"type:text;not null;default:''"`
}

// ContentItemSet is one planned set, for when the sets differ from each other.
//
// RepIndex is why the key has three columns. A ladder's rungs are reps inside one set, not
// sets of their own, so (item, set) cannot hold 3s / 6s / 9s. An ordinary set is RepIndex
// 0; a three-rung ladder is set 1 with RepIndex 1, 2 and 3.
// ContentSet is a movement's own per-rep numbers, for a movement whose reps are not all the
// same. The mirror of ContentItemSet one level up: that table holds what a block asks for,
// this one holds what the movement is.
//
// It exists because a movement can BE a ladder — a 3-second hang, then 6, then 9, is not a
// choice a block makes about a plain hang, it is the movement itself. Without this table
// such a shape could only be written inside one block, so no library could hold one.
//
// Most movements need no row here.
type ContentSet struct {
	ContentID int64 `gorm:"primaryKey"`
	SetIndex  int   `gorm:"primaryKey"`
	RepIndex  int   `gorm:"primaryKey;default:0"`
	Reps      *int
	WeightKg  *float64 `gorm:"type:numeric(6,2)"`
	Seconds   *int
}

type ContentItemSet struct {
	ContentItemID int64 `gorm:"primaryKey"`
	SetIndex      int   `gorm:"primaryKey"`
	RepIndex      int   `gorm:"primaryKey;default:0"`
	Reps          *int
	WeightKg      *float64 `gorm:"type:numeric(6,2)"`
	Seconds       *int
}

type ContentMedia struct {
	ID        int64  `gorm:"primaryKey"`
	ContentID int64  `gorm:"not null;index"`
	URL       string `gorm:"size:512;not null"`
	ThumbURL  string `gorm:"size:512;not null;default:''"`
	Position  int    `gorm:"not null;default:0"`
}

type Tag struct {
	ID   int64  `gorm:"primaryKey"`
	Slug string `gorm:"size:64;not null;uniqueIndex:ux_tag_slug"`
	Name string `gorm:"size:64;not null"`
}

type ContentTag struct {
	ContentID int64 `gorm:"primaryKey"`
	TagID     int64 `gorm:"primaryKey;index"`
}

// ---------------------------------------------------------------------------
// Planning
// ---------------------------------------------------------------------------

type Plan struct {
	ID        int64  `gorm:"primaryKey"`
	AccountID int64  `gorm:"not null;index"`
	Name      string `gorm:"size:255;not null"`
	StartsOn  Date   `gorm:"type:varchar(10);not null"`
	Weeks     int    `gorm:"not null"`
	Goal      string `gorm:"size:255;not null;default:''"`
	Focus     string `gorm:"size:32;not null;default:''"`
	Notes     string `gorm:"type:text;not null;default:''"`

	// GoalsJSON holds the before / after / how goals. They are only ever read and written
	// with the plan and never queried across plans, so they do not earn a table.
	GoalsJSON string `gorm:"type:text;not null;default:'[]'"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// PlanSlot is one rule: on this weekday, do this session. Week 0 means every week.
//
// Week is NOT NULL deliberately. A nullable column in a unique key enforces nothing,
// because every engine treats NULLs in a unique index as distinct from each other.
type PlanSlot struct {
	ID          int64  `gorm:"primaryKey"`
	PlanID      int64  `gorm:"not null;uniqueIndex:ux_slot,priority:1"`
	Week        int    `gorm:"not null;default:0;uniqueIndex:ux_slot,priority:2"`
	Weekday     int    `gorm:"not null;uniqueIndex:ux_slot,priority:3"`
	SessionID   int64  `gorm:"not null;uniqueIndex:ux_slot,priority:4"`
	SessionKind string `gorm:"size:16;not null;default:session"`
}

// PlanTarget is what a cycle asks of one movement — the Exercise targets page.
//
// Week 0 is the whole cycle; 1..n is that week only. "Varies by week" is not a stored
// flag: it is true when a week > 0 row exists for that plan and movement. A flag could
// disagree with the rows; a derived answer cannot.
type PlanTarget struct {
	ID           int64  `gorm:"primaryKey"`
	PlanID       int64  `gorm:"not null;uniqueIndex:ux_target,priority:1"`
	MovementID   int64  `gorm:"not null;uniqueIndex:ux_target,priority:2"`
	Week         int    `gorm:"not null;default:0;uniqueIndex:ux_target,priority:3"`
	MovementKind string `gorm:"size:16;not null;default:movement"`

	Sets       *int
	Reps       *int
	WeightKg   *float64 `gorm:"type:numeric(6,2)"`
	RepSeconds *int
}

// Scheduled is one session on one date.
//
// PlanID is nullable because a session can be scheduled by hand with no cycle behind it.
// No natural key is possible as a result — the same session twice on one day is legal and
// Position tells them apart — so preventing accidental duplicates is the app's job.
type Scheduled struct {
	ID          int64  `gorm:"primaryKey"`
	AccountID   int64  `gorm:"not null;index:ix_sched_account,priority:1"`
	OnDate      Date   `gorm:"type:varchar(10);not null;index:ix_sched_account,priority:2"`
	PlanID      *int64 `gorm:"index"`
	SessionID   int64  `gorm:"not null"`
	SessionKind string `gorm:"size:16;not null;default:session"`
	Position    int    `gorm:"not null;default:0"`

	CreatedAt time.Time `gorm:"not null"`
}

type CalendarEvent struct {
	ID        int64  `gorm:"primaryKey"`
	AccountID int64  `gorm:"not null;index:ix_event_account,priority:1"`
	FromDate  Date   `gorm:"type:varchar(10);not null;index:ix_event_account,priority:2"`
	ToDate    Date   `gorm:"type:varchar(10);not null"`
	PlanID    *int64 `gorm:"index"`
	Title     string `gorm:"size:128;not null"`
	Kind      string `gorm:"size:32;not null"`
	Blocks    bool   `gorm:"not null;default:true"`
	Notes     string `gorm:"type:text;not null;default:''"`
}

// Place is somewhere you train — a gym, a crag, or a board. One table with a kind,
// because they were the same shape wearing two names.
type Place struct {
	ID        int64  `gorm:"primaryKey"`
	AccountID int64  `gorm:"not null;index"`
	Kind      string `gorm:"size:16;not null"`
	Name      string `gorm:"size:128;not null"`
	Location  string `gorm:"size:128;not null;default:''"`
	BoardType string `gorm:"size:32;not null;default:''"`
}

// ---------------------------------------------------------------------------
// The log — frozen. It reads no content table, ever.
// ---------------------------------------------------------------------------
//
// UUID keys on all four, because a phone with no signal creates these rows and has to
// invent their ids. UpdatedAt on all four, because sync has to know what changed.
//
// Every name here is this table's own copy. Drop every content table and the whole of
// history still renders — that is the rule, and it is what the phase 3 gate tests.

const (
	LogDraft     = "draft" // a manual entry still being typed in; hidden from history
	LogActive    = "active"
	LogDone      = "done"
	LogAbandoned = "abandoned"
)

const (
	EntryPending = "pending"
	EntryDone    = "done"
	EntrySkipped = "skipped"
)

// Log is one session on one date. A log with no entries is a journal entry, not a second
// table.
type Log struct {
	ID        string `gorm:"type:char(36);primaryKey"`
	AccountID int64  `gorm:"not null;index:ix_log_account,priority:1;index:ix_log_state,priority:1;index:ix_log_sync,priority:1"`
	OnDate    Date   `gorm:"type:varchar(10);not null;index:ix_log_account,priority:2"`

	// Both nullable and both for grouping only. What renders is the frozen text below.
	ScheduledID *int64 `gorm:"index"`
	SessionID   *int64 `gorm:"index"`

	SessionSlug string `gorm:"size:128;not null;default:''"`
	SessionName string `gorm:"size:255;not null;default:''"`
	Color       string `gorm:"size:16;not null;default:''"`

	// PlaceName is frozen for the same reason as SessionName: reading the venue live would
	// mean renaming a gym rewrites every session you ever did there.
	PlaceID   *int64 `gorm:"index"`
	PlaceName string `gorm:"size:128;not null;default:''"`
	PlaceKind string `gorm:"size:16;not null;default:''"`

	State     string `gorm:"size:16;not null;index:ix_log_state,priority:2"`
	StartedAt *time.Time
	EndedAt   *time.Time

	Sleep   *int
	Energy  *int
	RPE     *int
	Focus   string `gorm:"size:32;not null;default:''"`
	Setting string `gorm:"size:16;not null;default:''"`

	WentWell  string `gorm:"type:text;not null;default:''"`
	NextFocus string `gorm:"type:text;not null;default:''"`
	Notes     string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null;index:ix_log_sync,priority:2"`
}

// LogEntry is one movement inside a logged session.
//
// AccountID and OnDate are frozen copies of the parent's, so the progression query — "the
// last five times you did this" — is answered by one index without joining Log. A
// composite foreign key with ON UPDATE CASCADE is what stops them ever diverging.
type LogEntry struct {
	ID        string `gorm:"type:char(36);primaryKey"`
	LogID     string `gorm:"type:char(36);not null;index:ix_entry_log,priority:1"`
	AccountID int64  `gorm:"not null;index:ix_entry_progression,priority:1"`
	OnDate    Date   `gorm:"type:varchar(10);not null;index:ix_entry_progression,priority:3,sort:desc"`

	// Position is not unique. Resolving a menu replaces one row with several at the same
	// position, so every read is ORDER BY position, id.
	Position int `gorm:"not null;index:ix_entry_log,priority:2"`

	BlockName string `gorm:"size:255;not null;default:''"`
	BlockKind string `gorm:"size:16;not null;default:''"`

	// MovementKey replaces the pair this used to carry — a row pointer nulled on delete,
	// and the slug as text. No foreign key: the log is frozen and has to survive its
	// content row being deleted outright, and a fork shares this value, so it could never
	// be a key to exactly one row.
	MovementKey  string `gorm:"type:char(36);not null;index:ix_entry_progression,priority:2"`
	MovementName string `gorm:"size:255;not null"`
	MovementKind string `gorm:"size:32;not null;default:''"`

	// What the plan asked of you that day, resolved once and never recomputed.
	TSets           *int
	TReps           *int
	TWeightKg       *float64 `gorm:"type:numeric(6,2)"`
	TRepSeconds     *int
	TRepRestSeconds *int
	TSetRestSeconds *int
	TPrepSeconds    *int
	TSeconds        *int

	State          string `gorm:"size:16;not null;default:pending"`
	ElapsedSeconds int    `gorm:"not null;default:0"`

	// SetMode "simple" implies zero LogSet rows. The one write path enforces it.
	SetMode      string `gorm:"size:16;not null;default:simple"`
	ClimbKind    string `gorm:"size:32;not null;default:''"`
	ClimbSubtype string `gorm:"size:32;not null;default:''"`
	Notes        string `gorm:"type:text;not null;default:''"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// LogSet is one set: what was asked of it, and what you did. Both on the row, so an open
// session with per-set planning needs nothing extra.
type LogSet struct {
	ID             string `gorm:"type:char(36);primaryKey"`
	LogEntryID     string `gorm:"type:char(36);not null;uniqueIndex:ux_log_set,priority:1"`
	SetIndex       int    `gorm:"not null;uniqueIndex:ux_log_set,priority:2"`
	RepIndex       int    `gorm:"not null;default:0;uniqueIndex:ux_log_set,priority:3"`
	TargetReps     *int
	TargetWeightKg *float64 `gorm:"type:numeric(6,2)"`
	TargetSeconds  *int
	Reps           *int
	WeightKg       *float64 `gorm:"type:numeric(6,2)"`
	Seconds        *int
	UpdatedAt      time.Time `gorm:"not null"`
}

// LogClimb is one boulder or route attempt. GradeOrdinal is nullable because an ARC lap or
// a traverse has no grade; where there is one, the number is what makes a pyramid sortable.
type LogClimb struct {
	ID           string `gorm:"type:char(36);primaryKey"`
	LogEntryID   string `gorm:"type:char(36);not null;index:ix_climb_entry,priority:1"`
	Position     int    `gorm:"not null;default:0;index:ix_climb_entry,priority:2"`
	Kind         string `gorm:"size:16;not null"`
	Setting      string `gorm:"size:16;not null;default:''"`
	Subtype      string `gorm:"size:32;not null;default:''"`
	Grade        string `gorm:"size:32;not null;default:''"`
	GradeOrdinal *int
	Style        string    `gorm:"size:32;not null;default:''"`
	RopeStyle    string    `gorm:"size:32;not null;default:''"`
	Attempts     int       `gorm:"not null;default:0"`
	Sent         bool      `gorm:"not null;default:false"`
	Seconds      int       `gorm:"not null;default:0"`
	Stars        int       `gorm:"not null;default:0"`
	Focus        string    `gorm:"type:text;not null;default:''"`
	Thoughts     string    `gorm:"type:text;not null;default:''"`
	UpdatedAt    time.Time `gorm:"not null"`
}

// Tables is every model, in dependency order. It exists so a test can assert the structs
// and the migrations describe the same 19 tables, and it is deliberately not passed to
// AutoMigrate anywhere.
func Tables() []any {
	return []any{
		&Account{}, &BodyMeasurement{}, &GradeMilestone{},
		&Content{}, &ContentSet{}, &ContentItem{}, &ContentItemSet{}, &ContentMedia{},
		&Tag{}, &ContentTag{},
		&Plan{}, &PlanSlot{}, &PlanTarget{}, &Scheduled{}, &CalendarEvent{},
		&Place{},
		&Log{}, &LogEntry{}, &LogSet{}, &LogClimb{},
	}
}
