// Package store is the only package that touches the database. Two conventions hold
// throughout:
//
//   - A nullable number is a pointer. NULL means "not asked for", and zero is a real value:
//     0 kg is bodyweight.
//   - Goose owns the schema. These structs mirror the migrations and are never given to
//     AutoMigrate, so a struct that drifts from a migration is a bug in the struct.
package store

import "time"

// Date is a calendar day held as "YYYY-MM-DD". SQLite has no DATE type, and a fixed-width
// ISO string sorts correctly on both engines. All date arithmetic happens in Go.
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

	// TimeZone decides which day "today" is, for every heatmap, streak and calendar edge.
	TimeZone string `gorm:"size:64;not null;default:UTC"`

	// TokenEpoch is a JWT claim, compared on every request. A password change bumps it,
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
// grade cannot be sorted in SQL.
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

// Content kinds. A menu is its own kind rather than a movement holding movements, so no
// legal edge leaves a movement and a cycle is impossible.
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
	// ID is the local key. Every foreign key in this schema points here, and nothing else
	// does. It may differ between two installs and it never leaves the machine.
	ID int64 `gorm:"primaryKey"`

	// UUID is the global key, written in the YAML file, and how the importer matches a file
	// to this row across a rename. Never a foreign key target.
	//
	// Unique per owner, not globally: two accounts may add the same published tree. The two
	// partial unique indexes are in the migration, because a WHERE clause has no struct tag.
	UUID string `gorm:"size:36;not null"`

	// Family groups a row with the rows it was copied from and into. Read by history, and by
	// nothing else. A new row's family is its own UUID; a copy inherits its parent's.
	Family string `gorm:"size:36;not null;index:ix_content_family"`

	Kind string `gorm:"size:16;not null"`
	Slug string `gorm:"size:128;not null"`
	Name string `gorm:"size:255;not null"`

	// SourceTree is the name of the catalog tree that owns this row, or nil when nothing
	// does. Editing a file-owned row in the app sets this to nil, which detaches the row
	// from its file for good. A tree name, never a path: paths differ between machines.
	SourceTree *string `gorm:"size:64"`

	// AuthorID nil means the app ships this row. Nothing in the schema sets this column to
	// NULL on delete: that would publish a user's private content into the shipped catalog.
	AuthorID *int64 `gorm:"index"`

	Notes  string `gorm:"type:text;not null;default:''"`
	Source string `gorm:"size:64;not null;default:''"`

	// RetiredOn is set by the importer when a file leaves the tree. The library hides these.
	// Plans and logs that reference them keep resolving.
	RetiredOn *Date `gorm:"type:varchar(10)"`

	// Movement columns. Defaults only — what a session asks for lives on ContentItem.
	//
	// MovementStyle decides the run screen. Not PlanTarget.MovementKind, which holds a
	// content kind.
	MovementStyle *string `gorm:"size:32"`

	// PerSide is true when the numbers are per side, so the player counts 12 and not 6.
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

	// Menu columns. PickCount is the fewest options you must choose. 0 means skippable.
	PickCount *int

	// Session columns.
	Color string `gorm:"size:16;not null;default:''"`
	Needs string `gorm:"size:255;not null;default:''"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// Shipped reports whether the app ships this row rather than a user having written it.
func (c Content) Shipped() bool { return c.AuthorID == nil }

// FromAFile reports whether a catalog tree still owns this row. Editing such a row detaches
// it, and the file stops having any effect. Callers show that consequence before writing.
func (c Content) FromAFile() bool { return c.SourceTree != nil }

// Editable is the question a handler asks before letting a write through. A row the app
// ships is never edited; the app offers a copy instead.
func (c Content) Editable() bool { return !c.Shipped() }

// ContentItem is one edge in the content tree, and the numbers for that use. The numbers
// live here, so one movement row can be referenced by every session that uses it.
//
// ParentKind and ChildKind duplicate Content.Kind so a composite foreign key can keep each
// edge pointing at a row of the right kind.
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

// ContentSet is a movement's own per-rep numbers, for a movement that IS a ladder: a hang
// of 3 seconds, then 6, then 9. Most movements need no row here.
//
// RepIndex is why the key has three columns. A ladder's rungs are reps inside one set, not
// sets of their own, so (content, set) cannot hold 3s / 6s / 9s.
type ContentSet struct {
	ContentID int64 `gorm:"primaryKey"`
	SetIndex  int   `gorm:"primaryKey"`
	RepIndex  int   `gorm:"primaryKey;default:0"`
	Reps      *int
	WeightKg  *float64 `gorm:"type:numeric(6,2)"`
	Seconds   *int
}

// ContentItemSet is ContentSet one level up: what a block asks for on one slot.
type ContentItemSet struct {
	ContentItemID int64 `gorm:"primaryKey"`
	SetIndex      int   `gorm:"primaryKey"`
	RepIndex      int   `gorm:"primaryKey;default:0"`
	Reps          *int
	WeightKg      *float64 `gorm:"type:numeric(6,2)"`
	Seconds       *int
}

// MovementPref is a person's own numbers for a movement they do not own: the app suggests
// 20 kg on its Half Crimp Hang, you use 25. An override rather than a copy, so a later
// release improving the app's row still reaches you.
//
// Resolution order for a number, weakest first: the movement's own default, then this, then
// the ContentItem override on the slot being run, then what the person enters while running.
// The slot wins over this, because a session that ramps 60/70/80% must not collapse into one
// number.
//
// MovementKind is always "movement", so a composite foreign key can say so.
type MovementPref struct {
	AccountID    int64  `gorm:"primaryKey"`
	MovementID   int64  `gorm:"primaryKey"`
	MovementKind string `gorm:"size:16;not null;default:movement"`

	Sets           *int
	Reps           *int
	WeightKg       *float64 `gorm:"type:numeric(6,2)"`
	RepSeconds     *int
	RepRestSeconds *int
	SetRestSeconds *int
	PrepSeconds    *int
	Seconds        *int

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
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

	// GoalsJSON holds the before / after / how goals. Never queried across plans, so they do
	// not earn a table.
	GoalsJSON string `gorm:"type:text;not null;default:'[]'"`

	CreatedAt time.Time `gorm:"not null"`
	UpdatedAt time.Time `gorm:"not null"`
}

// PlanSlot is one rule: on this weekday, do this session. Week 0 means every week.
//
// Week is NOT NULL because every engine treats NULLs in a unique index as distinct, so a
// nullable column in a unique key enforces nothing.
type PlanSlot struct {
	ID          int64  `gorm:"primaryKey"`
	PlanID      int64  `gorm:"not null;uniqueIndex:ux_slot,priority:1"`
	Week        int    `gorm:"not null;default:0;uniqueIndex:ux_slot,priority:2"`
	Weekday     int    `gorm:"not null;uniqueIndex:ux_slot,priority:3"`
	SessionID   int64  `gorm:"not null;uniqueIndex:ux_slot,priority:4"`
	SessionKind string `gorm:"size:16;not null;default:session"`
}

// PlanTarget is what a cycle asks of one movement — the Exercise targets page. Week 0 is
// the whole cycle, 1..n is that week only. "Varies by week" is derived, not stored: it is
// true when a week > 0 row exists for that plan and movement.
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

// Scheduled is one session on one date. PlanID is nullable: a session can be scheduled by
// hand. The same session twice on one day is legal, and Position tells them apart, so there
// is no natural key and the app has to prevent accidental duplicates itself.
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

// Place is somewhere you train — a gym, a crag, or a board.
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
// Every name here is this table's own copy: drop every content table and history still
// renders.

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

// Log is one session on one date. A log with no entries is a journal entry.
type Log struct {
	ID        string `gorm:"type:varchar(36);primaryKey;not null"`
	AccountID int64  `gorm:"not null;index:ix_log_account,priority:1;index:ix_log_state,priority:1;index:ix_log_sync,priority:1"`
	OnDate    Date   `gorm:"type:varchar(10);not null;index:ix_log_account,priority:2"`

	// Both nullable and both for grouping only. What renders is the frozen text below.
	ScheduledID *int64 `gorm:"index"`
	SessionID   *int64 `gorm:"index"`

	// SessionFamily is what "times completed" counts. Not the name: a person's session
	// called Power and the app's session called Power are two workouts. Empty for an open
	// session, which had none behind it.
	SessionFamily string `gorm:"size:36;not null;default:''"`
	SessionSlug   string `gorm:"size:128;not null;default:''"`
	SessionName   string `gorm:"size:255;not null;default:''"`
	Color         string `gorm:"size:16;not null;default:''"`

	// PlaceName is frozen, so renaming a gym does not rewrite every session you did there.
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

// LogEntry is one movement inside a logged session. The account and the date live on Log
// and are read through the join.
type LogEntry struct {
	ID    string `gorm:"type:varchar(36);primaryKey;not null"`
	LogID string `gorm:"type:varchar(36);not null;index:ix_entry_log,priority:1;index:ix_entry_movement,priority:2"`

	// Position is not unique. Resolving a menu replaces one row with several at the same
	// position, so every read is ORDER BY position, id.
	Position int `gorm:"not null;index:ix_entry_log,priority:2"`

	BlockName string `gorm:"size:255;not null;default:''"`
	BlockKind string `gorm:"size:16;not null;default:''"`

	// MovementID points at the row that was run, and goes nil if that row is deleted.
	// MovementFamily is the identity history groups by, held as text so it survives that
	// delete. MovementSlug and MovementName are what the movement was called that day.
	//
	// History groups on the family and never on the slug: two unrelated movements can share
	// a slug, and a rename would otherwise have to rewrite these rows.
	MovementID     *int64
	MovementFamily string `gorm:"size:36;not null;index:ix_entry_movement,priority:1"`
	MovementSlug   string `gorm:"size:128;not null"`
	MovementName   string `gorm:"size:255;not null"`

	// Frozen, so changing how a movement is performed does not redraw an old run.
	MovementStyle string `gorm:"size:32;not null;default:''"`

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

// LogSet is one set: what was asked of it, and what you did.
type LogSet struct {
	ID             string `gorm:"type:varchar(36);primaryKey;not null"`
	LogEntryID     string `gorm:"type:varchar(36);not null;uniqueIndex:ux_log_set,priority:1"`
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
	ID           string `gorm:"type:varchar(36);primaryKey;not null"`
	LogEntryID   string `gorm:"type:varchar(36);not null;index:ix_climb_entry,priority:1"`
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

// Tables is every model, in dependency order, so a test can assert that the structs and the
// migrations describe the same tables. Never passed to AutoMigrate.
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
