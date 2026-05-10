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
	); err != nil {
		return err
	}

	return nil
}
