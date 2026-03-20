package db

import (
	"log/slog"
	"time"

	"gorm.io/gorm"
)

type seedMedia struct {
	VideoURL     string
	ThumbnailURL string
}

type seedExercise struct {
	Kind                   string
	SessionDurationSeconds int
	Name                   string
	Media                  []seedMedia
	Notes                  string
	Sets                   int
	Reps                   int
	RepSeconds             int
	RepRestSeconds         int
	SetRestSeconds         int
	WeightKg               float64
}

type seedActivity struct {
	Type      string
	Exercises []seedExercise
}

type seedTemplate struct {
	Name       string
	Color      string // optional hex, e.g. #eab308
	Activities []seedActivity
}

func (s *Store) SeedDevIfEmpty(ownerID uint) error {
	if err := s.EnsureSeedUser(ownerID, "demo@passion.local", ""); err != nil {
		return err
	}

	var count int64
	if err := s.DB.Model(&SessionTemplate{}).Where("owner_id = ?", ownerID).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		slog.Info("skipping dev seed: existing templates found", "owner_id", ownerID, "template_count", count)
		return nil
	}
	slog.Info("starting dev seed", "owner_id", ownerID)

	return s.DB.Transaction(func(tx *gorm.DB) error {
		templates := defaultSeedTemplates()
		var templateIDs []uint

		for _, e := range defaultSeedLibraryExercises() {
			lib := LibraryExercise{
				OwnerID:                ownerID,
				Kind:                   NormalizeKind(e.Kind),
				SessionDurationSeconds: e.SessionDurationSeconds,
				Name:                   e.Name,
				Notes:                  e.Notes,
				Sets:                   e.Sets,
				Reps:                   e.Reps,
				RepSeconds:             e.RepSeconds,
				RepRestSeconds:         e.RepRestSeconds,
				SetRestSeconds:         e.SetRestSeconds,
				WeightKg:               e.WeightKg,
			}
			if err := tx.Create(&lib).Error; err != nil {
				return err
			}
			for i, m := range e.Media {
				em := ExerciseMedia{
					OwnerID:           ownerID,
					LibraryExerciseID: &lib.ID,
					VideoURL:          m.VideoURL,
					ThumbnailURL:      m.ThumbnailURL,
					OrderIndex:        i,
				}
				if err := tx.Create(&em).Error; err != nil {
					return err
				}
			}
		}

		for _, t := range templates {
			template := SessionTemplate{
				OwnerID: ownerID,
				Name:    t.Name,
				Color:   t.Color,
			}
			if err := tx.Create(&template).Error; err != nil {
				return err
			}
			templateIDs = append(templateIDs, template.ID)

			for aIdx, a := range t.Activities {
				activity := Activity{
					OwnerID:           ownerID,
					SessionTemplateID: template.ID,
					Type:              a.Type,
					OrderIndex:        aIdx,
				}
				if err := tx.Create(&activity).Error; err != nil {
					return err
				}

				for eIdx, e := range a.Exercises {
					actID := activity.ID
					exercise := Exercise{
						OwnerID:                ownerID,
						ActivityID:             &actID,
						Kind:                   NormalizeKind(e.Kind),
						SessionDurationSeconds: e.SessionDurationSeconds,
						Name:                   e.Name,
						Notes:                  e.Notes,
						Sets:                   e.Sets,
						Reps:                   e.Reps,
						RepSeconds:             e.RepSeconds,
						RepRestSeconds:         e.RepRestSeconds,
						SetRestSeconds:         e.SetRestSeconds,
						WeightKg:               e.WeightKg,
						OrderIndex:             eIdx,
					}
					if err := tx.Create(&exercise).Error; err != nil {
						return err
					}
					for i, m := range e.Media {
						em := ExerciseMedia{
							OwnerID:      ownerID,
							ExerciseID:   &exercise.ID,
							VideoURL:     m.VideoURL,
							ThumbnailURL: m.ThumbnailURL,
							OrderIndex:   i,
						}
						if err := tx.Create(&em).Error; err != nil {
							return err
						}
					}
				}
			}
		}

		// Seed historical sessions + runs for the last ~3 months
		if err := seedHistoricalRuns(tx, ownerID, templateIDs); err != nil {
			return err
		}

		slog.Info(
			"completed dev seed",
			"owner_id", ownerID,
			"templates_created", len(templates),
		)
		return nil
	})
}

func (s *Store) EnsureSeedUser(ownerID uint, email string, passwordHash string) error {
	var count int64
	if err := s.DB.Model(&User{}).Where("id = ?", ownerID).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		u := User{
			Model: gorm.Model{
				ID: ownerID,
			},
			Email:        email,
			PasswordHash: passwordHash,
		}
		return s.DB.Create(&u).Error
	}
	return nil
}

// defaultSeedLibraryExercises creates reusable presets for the exercise library (same shape as template exercises).
func defaultSeedLibraryExercises() []seedExercise {
	return []seedExercise{
		{Name: "Max Hangs", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge — demo library item", Sets: 6, Reps: 1, RepSeconds: 10, RepRestSeconds: 0, SetRestSeconds: 120, WeightKg: 5},
		{Name: "Weighted Pull-ups", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, RepSeconds: 5, RepRestSeconds: 0, SetRestSeconds: 120, WeightKg: 10},
		{Kind: "session", SessionDurationSeconds: 600, Name: "10m Open Mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1518611012118-696072aa579a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Session-style block example"},
	}
}

func defaultSeedTemplates() []seedTemplate {
	return []seedTemplate{
		{
			Name:  "Bouldering Power Session",
			Color: "#eab308",
			Activities: []seedActivity{
				{
					Type: "warmup",
					Exercises: []seedExercise{
						{Name: "Mobility Flow", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1545205597-3d9d02c29597?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Shoulders and wrists", Sets: 1, Reps: 1, RepSeconds: 600, RepRestSeconds: 0, SetRestSeconds: 0},
						{Kind: "session", SessionDurationSeconds: 600, Name: "10m Open Mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1518611012118-696072aa579a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Move freely; jot cues below"},
						{Name: "Easy Traverses", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1522163182402-834f871fd851?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Smooth movement, light grip", Sets: 3, Reps: 1, RepSeconds: 120, RepRestSeconds: 0, SetRestSeconds: 60},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Name: "Limit Boulders", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1549576490-b0b4831ef60a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "3-5 hard moves", Sets: 5, Reps: 1, RepSeconds: 180, RepRestSeconds: 0, SetRestSeconds: 180},
						{Name: "Max Hangs", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge", Sets: 6, Reps: 1, RepSeconds: 10, RepRestSeconds: 0, SetRestSeconds: 120, WeightKg: 5},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Name: "Forearm Recovery", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517836357463-d25dfeac3438?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Light band + stretch", Sets: 1, Reps: 1, RepSeconds: 420, RepRestSeconds: 0, SetRestSeconds: 0},
					},
				},
			},
		},
		{
			Name:  "Strength Base Session",
			Color: "#ef4444",
			Activities: []seedActivity{
				{
					Type: "warmup",
					Exercises: []seedExercise{
						{Name: "Row + Band Prep", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1534368786749-b63e8f22a541?auto=format&fit=crop&w=1200&q=80"}}, Notes: "2 rounds easy pace", Sets: 2, Reps: 12, RepSeconds: 4, RepRestSeconds: 30, SetRestSeconds: 30},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Name: "Weighted Pull-ups", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, RepSeconds: 5, RepRestSeconds: 0, SetRestSeconds: 120, WeightKg: 10},
						{Name: "Ring Rows", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1579758629938-03607ccdbaba?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Full range", Sets: 4, Reps: 10, RepSeconds: 4, RepRestSeconds: 0, SetRestSeconds: 90},
						{Name: "Core Hollow Holds", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1598971639058-a67f6b7b0ef4?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Brace and breathe", Sets: 4, Reps: 1, RepSeconds: 30, RepRestSeconds: 0, SetRestSeconds: 45},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Name: "Lat + Pec Stretch", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1506126613408-eca07ce68773?auto=format&fit=crop&w=1200&q=80"}}, Sets: 1, Reps: 1, RepSeconds: 300, RepRestSeconds: 0, SetRestSeconds: 0},
					},
				},
			},
		},
		{
			Name:  "Technique + Volume",
			Color: "#3b82f6",
			Activities: []seedActivity{
				{
					Type: "warmup",
					Exercises: []seedExercise{
						{Name: "Easy Routes", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1564769662533-4f00a87b4056?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Progressively harder", Sets: 4, Reps: 1, RepSeconds: 180, RepRestSeconds: 0, SetRestSeconds: 60},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Name: "Footwork Drills", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1526401485004-2fda9f2f1b0d?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Silent feet focus", Sets: 6, Reps: 1, RepSeconds: 120, RepRestSeconds: 0, SetRestSeconds: 45},
						{Name: "ARC Climbing", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1523419409543-a5e549c1f7eb?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Continuous easy climbing", Sets: 2, Reps: 1, RepSeconds: 900, RepRestSeconds: 0, SetRestSeconds: 180},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Name: "Breathing + Mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1490645935967-10de6ba17061?auto=format&fit=crop&w=1200&q=80"}}, Sets: 1, Reps: 1, RepSeconds: 360, RepRestSeconds: 0, SetRestSeconds: 0},
					},
				},
			},
		},
	}
}

func localDate(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// seedHistoricalRuns creates ~40 completed session runs spread across the last 3 months
// plus one "running" session for today, to populate the history page.
func seedHistoricalRuns(tx *gorm.DB, ownerID uint, templateIDs []uint) error {
	if len(templateIDs) == 0 {
		return nil
	}

	now := time.Now()

	// Load root exercises for each template so we can create completions
	exercisesByTemplate := map[uint][]uint{}
	for _, tplID := range templateIDs {
		var exercises []Exercise
		err := tx.
			Joins("JOIN activities ON activities.id = exercises.activity_id").
			Where("activities.session_template_id = ? AND exercises.parent_exercise_id IS NULL", tplID).
			Select("exercises.id").
			Find(&exercises).Error
		if err != nil {
			return err
		}
		ids := make([]uint, len(exercises))
		for i, e := range exercises {
			ids[i] = e.ID
		}
		exercisesByTemplate[tplID] = ids
	}

	// Training days: offsets from today (negative = past).
	// One session per day, ~3-4 days per week across 12 weeks.
	dayOffsets := []int{
		-84, -82, -80, -78, // week 12 ago
		-77, -75, -73,      // week 11
		-70, -68, -66,      // week 10
		-63, -61, -59, -57, // week 9
		-56, -54, -52,      // week 8
		-49, -47, -45, -43, // week 7
		-42, -40, -38,      // week 6
		-35, -33, -31,      // week 5
		-28, -26, -24, -22, // week 4
		-21, -19, -17,      // week 3
		-14, -12, -10, -9,  // week 2
		-7, -5, -3, -1,     // last week
		0,                  // today (will be "running")
	}

	// Session durations (minutes) to vary completed times
	durations := []int{55, 70, 45, 80, 60, 75, 50, 65, 90, 55}

	for i, offset := range dayOffsets {
		tplID := templateIDs[i%len(templateIDs)]
		sessionDate := localDate(now.AddDate(0, 0, offset))
		startHour := 7 + (i % 4) // vary start between 7-10am
		startedAt := sessionDate.Add(time.Duration(startHour) * time.Hour)

		ss := ScheduledSession{
			OwnerID:           ownerID,
			TrainingCycleID:   nil,
			IsTrial:           false,
			ScheduledDate:     sessionDate,
			SessionTemplateID: tplID,
		}
		if err := tx.Create(&ss).Error; err != nil {
			return err
		}

		isToday := offset == 0
		status := "completed"
		if isToday {
			status = "running"
		}

		var completedAt *time.Time
		if status == "completed" {
			dur := time.Duration(durations[i%len(durations)]) * time.Minute
			t := startedAt.Add(dur)
			completedAt = &t
		}

		run := SessionRun{
			OwnerID:            ownerID,
			ScheduledSessionID: ss.ID,
			Status:             status,
			StartedAt:          startedAt,
			CompletedAt:        completedAt,
		}
		if err := tx.Create(&run).Error; err != nil {
			return err
		}

		// Create exercise completions
		exerciseIDs := exercisesByTemplate[tplID]
		if status == "completed" && len(exerciseIDs) > 0 {
			// Complete all exercises for most sessions, skip last one occasionally
			completeCount := len(exerciseIDs)
			if i%5 == 0 && completeCount > 1 {
				completeCount-- // skip one exercise every 5th session
			}
			for j := 0; j < completeCount; j++ {
				comp := RunExerciseCompletion{
					OwnerID:     ownerID,
					RunID:       run.ID,
					ExerciseID:  exerciseIDs[j],
					Status:      "completed",
					CompletedAt: startedAt.Add(time.Duration(10*(j+1)) * time.Minute),
				}
				if err := tx.Create(&comp).Error; err != nil {
					return err
				}
			}
		} else if isToday && len(exerciseIDs) > 1 {
			// Today's running session: complete first 2 exercises
			for j := 0; j < 2 && j < len(exerciseIDs); j++ {
				comp := RunExerciseCompletion{
					OwnerID:     ownerID,
					RunID:       run.ID,
					ExerciseID:  exerciseIDs[j],
					Status:      "completed",
					CompletedAt: startedAt.Add(time.Duration(10*(j+1)) * time.Minute),
				}
				if err := tx.Create(&comp).Error; err != nil {
					return err
				}
			}
		}
	}

	return nil
}
