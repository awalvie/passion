package db

import (
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type Store struct {
	DB *gorm.DB
}

func NewSqlite(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	s := &Store{DB: db}
	if err := s.AutoMigrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) AutoMigrate() error {
	if err := s.DB.AutoMigrate(
		&User{},
		&SessionTemplate{},
		&Activity{},
		&Exercise{},
		&ActivityTemplate{},
		&LibraryExercise{},
		&ExerciseMedia{},
		&TrainingCycle{},
		&TrainingCycleWeekdayMapping{},
		&ScheduledSession{},
		&SessionRun{},
		&RunExerciseCompletion{},
		&RunExerciseChoice{},
		&CycleExerciseOverride{},
		&CycleExerciseWeekOverride{},
		&SessionJournal{},
		&ClimbingVenue{},
		&ClimbingBoard{},
		&ClimbingTick{},
		&ManualExerciseSetLog{},
		&ClimbingExerciseMeta{},
		&CalendarEvent{},
	); err != nil {
		return err
	}

	if err := s.migrateTimedReps(); err != nil {
		return err
	}
	return s.migrateSessionJournals()
}

// migrateSessionJournals recreates the session_journals table if it was created
// with run_id as NOT NULL (the original schema). SQLite cannot ALTER a column's
// nullability, so we drop-and-recreate while the table is still empty in dev.
func (s *Store) migrateSessionJournals() error {
	type colInfo struct {
		CID       int    `gorm:"column:cid"`
		Name      string `gorm:"column:name"`
		NotNull   int    `gorm:"column:notnull"`
	}
	var cols []colInfo
	if err := s.DB.Raw("PRAGMA table_info(session_journals)").Scan(&cols).Error; err != nil {
		return err
	}
	for _, c := range cols {
		if c.Name == "run_id" && c.NotNull == 1 {
			// Old schema — drop and let AutoMigrate recreate with nullable run_id.
			if err := s.DB.Exec("DROP TABLE IF EXISTS session_journals").Error; err != nil {
				return err
			}
			return s.DB.AutoMigrate(&SessionJournal{})
		}
	}
	return nil
}

// migrateTimedReps is idempotent: promotes reps_and_sets exercises that use timer fields
// to the new timed_reps kind, then zeroes timer fields on remaining reps_and_sets rows.
func (s *Store) migrateTimedReps() error {
	tables := []string{"exercises", "library_exercises"}
	for _, t := range tables {
		if err := s.DB.Exec(`
			UPDATE `+t+`
			SET kind = 'timed_reps'
			WHERE kind = 'reps_and_sets'
			  AND (rep_seconds > 0 OR prep_seconds > 0 OR rep_rest_seconds > 0)
		`).Error; err != nil {
			return err
		}
		if err := s.DB.Exec(`
			UPDATE `+t+`
			SET rep_seconds = 0, rep_rest_seconds = 0, set_rest_seconds = 0, prep_seconds = 0
			WHERE kind = 'reps_and_sets'
		`).Error; err != nil {
			return err
		}
	}
	return nil
}
