package db

import (
	"log/slog"
	"math/rand"
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
	PrepSeconds            int
	RungSeconds            string
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
				PrepSeconds:            e.PrepSeconds,
				RungSeconds:            e.RungSeconds,
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
						PrepSeconds:            e.PrepSeconds,
						RungSeconds:            e.RungSeconds,
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
		{Kind: "timed_reps", Name: "Max Hangs", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge — demo library item", Sets: 6, Reps: 1, RepSeconds: 10, SetRestSeconds: 120, WeightKg: 5},
		{Kind: "reps_and_sets", Name: "Weighted Pull-ups", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, SetRestSeconds: 120, WeightKg: 10},
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
						{Kind: "session", SessionDurationSeconds: 600, Name: "Mobility Flow", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1545205597-3d9d02c29597?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Shoulders and wrists"},
						{Kind: "session", SessionDurationSeconds: 600, Name: "10m Open Mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1518611012118-696072aa579a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Move freely; jot cues below"},
						{Kind: "timed_reps", Name: "Easy Traverses", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1522163182402-834f871fd851?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Smooth movement, light grip", Sets: 3, Reps: 1, RepSeconds: 120, SetRestSeconds: 60},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Kind: "climbing", Name: "Limit Boulders", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1549576490-b0b4831ef60a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "3-5 hard moves — log each attempt as a tick"},
						{Kind: "timed_reps", Name: "Max Hangs", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge", Sets: 6, Reps: 1, RepSeconds: 10, SetRestSeconds: 120, WeightKg: 5},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Kind: "session", SessionDurationSeconds: 420, Name: "Forearm Recovery", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517836357463-d25dfeac3438?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Light band + stretch"},
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
						{Kind: "reps_and_sets", Name: "Row + Band Prep", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1534368786749-b63e8f22a541?auto=format&fit=crop&w=1200&q=80"}}, Notes: "2 rounds easy pace", Sets: 2, Reps: 12, SetRestSeconds: 30},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Kind: "reps_and_sets", Name: "Weighted Pull-ups", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, SetRestSeconds: 120, WeightKg: 10},
						{Kind: "reps_and_sets", Name: "Ring Rows", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1579758629938-03607ccdbaba?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Full range", Sets: 4, Reps: 10, SetRestSeconds: 90},
						{Kind: "timed_reps", Name: "Core Hollow Holds", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1598971639058-a67f6b7b0ef4?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Brace and breathe", Sets: 4, Reps: 1, RepSeconds: 30, SetRestSeconds: 45},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Kind: "session", SessionDurationSeconds: 300, Name: "Lat + Pec Stretch", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1506126613408-eca07ce68773?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Doorframe + floor stretch"},
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
						{Kind: "climbing", Name: "Easy Routes", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1564769662533-4f00a87b4056?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Progressively harder — log each route as a tick"},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Kind: "timed_reps", Name: "Footwork Drills", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1526401485004-2fda9f2f1b0d?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Silent feet focus", Sets: 6, Reps: 1, RepSeconds: 120, SetRestSeconds: 45},
						{Kind: "timed_reps", Name: "ARC Climbing", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1523419409543-a5e549c1f7eb?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Continuous easy climbing", Sets: 2, Reps: 1, RepSeconds: 900, SetRestSeconds: 180},
					},
				},
				{
					Type: "cooldown",
					Exercises: []seedExercise{
						{Kind: "session", SessionDurationSeconds: 360, Name: "Breathing + Mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1490645935967-10de6ba17061?auto=format&fit=crop&w=1200&q=80"}}},
					},
				},
			},
		},
	}
}

type seedExerciseInfo struct {
	ID   uint
	Kind string
}

// seedHistoricalRuns creates ~45 completed session runs spread across the last 14 weeks
// plus one "running" session for today. It seeds journal entries, climbing ticks, and
// manual exercise completions to give the stats panel meaningful data.
func seedHistoricalRuns(tx *gorm.DB, ownerID uint, templateIDs []uint) error {
	if len(templateIDs) == 0 {
		return nil
	}

	rng := rand.New(rand.NewSource(42))
	now := time.Now()

	// Load root exercises for each template, including kind, so we can create
	// ticks for climbing exercises.
	exercisesByTemplate := map[uint][]seedExerciseInfo{}
	for _, tplID := range templateIDs {
		var exercises []Exercise
		err := tx.
			Joins("JOIN activities ON activities.id = exercises.activity_id").
			Where("activities.session_template_id = ? AND exercises.parent_exercise_id IS NULL", tplID).
			Select("exercises.id, exercises.kind").
			Find(&exercises).Error
		if err != nil {
			return err
		}
		infos := make([]seedExerciseInfo, len(exercises))
		for i, e := range exercises {
			infos[i] = seedExerciseInfo{ID: e.ID, Kind: e.Kind}
		}
		exercisesByTemplate[tplID] = infos
	}

	// Training schedule: irregular pattern across 14 weeks, 2–4 sessions/week.
	// Mix of morning (7–9 am) and evening (17–19) start hours.
	type sessionSlot struct {
		offset    int // days from today
		startHour int
	}
	slots := []sessionSlot{
		{-97, 7}, {-95, 18}, {-93, 7}, // week 14
		{-90, 17}, {-88, 7}, // week 13 (rest-ish)
		{-84, 8}, {-82, 18}, {-80, 7}, {-78, 17}, // week 12
		{-77, 7}, {-75, 18}, {-73, 8}, // week 11
		{-70, 7}, {-67, 18}, // week 10 (lighter)
		{-63, 8}, {-61, 7}, {-59, 18}, {-57, 7}, // week 9
		{-55, 18}, {-53, 7}, {-51, 8}, // week 8
		{-49, 7}, {-47, 17}, {-45, 7}, // week 7
		{-42, 8}, {-39, 18}, // week 6 (lighter — travel)
		{-35, 7}, {-33, 18}, {-31, 7}, {-29, 8}, // week 5
		{-28, 17}, {-26, 7}, {-24, 18}, // week 4
		{-21, 8}, {-19, 7}, {-17, 18}, {-15, 7}, // week 3
		{-14, 17}, {-12, 7}, {-10, 18}, {-9, 8}, // week 2
		{-7, 7}, {-5, 18}, {-3, 7}, {-1, 17}, // last week
		{0, 8}, // today (RunStatusRunning)
	}

	// Realistic session durations in minutes, by template index.
	durationRanges := [][2]int{
		{60, 90},  // bouldering power — longer
		{50, 75},  // strength base
		{70, 100}, // technique + volume — longest
	}

	// Journal data pools.
	focusByTemplate := [][]string{
		{"projects", "strength", "projects", "general"},
		{"strength", "general", "strength"},
		{"technique", "endurance", "technique", "general"},
	}
	wentWellPool := []string{
		"Movement felt crisp on the crux sequences.",
		"Footwork stayed precise throughout.",
		"Good tension through the core on overhang.",
		"Managed rest positions well on the route.",
		"Pull-up strength noticeably improved.",
		"Breathing stayed calm even on hard moves.",
		"Hit all planned sets without dropping reps.",
		"Shoulder felt stable, no niggles.",
		"Dialled the beta on the project third go.",
		"Energy was high the whole session.",
	}
	nextFocusPool := []string{
		"Work hip positioning on steep terrain.",
		"Keep rest intervals strict — tendency to cut them short.",
		"Focus on quiet feet next session.",
		"More volume on open-hand grip.",
		"Try the direct start on the project.",
		"Add antagonist work — fingers are fatiguing fast.",
		"Earlier session time to avoid evening crowds.",
		"Film a run on the project to spot beta.",
	}

	// Boulder grades (Font scale) and route grades (French sport).
	boulderGrades := []string{"5+", "6a", "6a+", "6b", "6b+", "6c", "6c+", "7a", "7a+", "7b"}
	routeGrades := []string{"6a", "6a+", "6b", "6b+", "6c", "6c+", "7a", "7a+", "7b", "7b+"}
	tickStyles := []string{"onsight", "flash", "redpoint", "redpoint", "redpoint", "project", "project", "repeat"}

	pick := func(pool []string) string { return pool[rng.Intn(len(pool))] }
	between := func(lo, hi int) int { return lo + rng.Intn(hi-lo+1) }

	for i, slot := range slots {
		tplIdx := i % len(templateIDs)
		tplID := templateIDs[tplIdx]
		sessionDate := LocalDate(now.AddDate(0, 0, slot.offset))
		startedAt := sessionDate.Add(time.Duration(slot.startHour) * time.Hour)

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

		isToday := slot.offset == 0
		status := RunStatusCompleted
		if isToday {
			status = RunStatusRunning
		}

		durRange := durationRanges[tplIdx]
		durMin := between(durRange[0], durRange[1])

		var completedAt *time.Time
		if status == RunStatusCompleted {
			t := startedAt.Add(time.Duration(durMin) * time.Minute)
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

		exercises := exercisesByTemplate[tplID]

		if status == RunStatusCompleted && len(exercises) > 0 {
			completeCount := len(exercises)
			if rng.Intn(6) == 0 && completeCount > 1 {
				completeCount-- // skip last exercise ~17% of sessions
			}

			for j := 0; j < completeCount; j++ {
				ex := exercises[j]
				comp := RunExerciseCompletion{
					OwnerID:     ownerID,
					RunID:       run.ID,
					ExerciseID:  ex.ID,
					Status:      RunStatusCompleted,
					CompletedAt: startedAt.Add(time.Duration(between(8, 18)*(j+1)) * time.Minute),
				}
				if err := tx.Create(&comp).Error; err != nil {
					return err
				}

				if ex.Kind == "climbing" {
					tickCount := between(3, 8)
					grades := boulderGrades
					if tplIdx == 2 { // Technique+Volume uses routes
						grades = routeGrades
					}
					for t := 0; t < tickCount; t++ {
						sent := rng.Intn(3) != 0 // ~67% send rate
						tick := ClimbingTick{
							OwnerID:    ownerID,
							RunID:      run.ID,
							ExerciseID: ex.ID,
							Kind:       "boulder",
							Grade:      pick(grades),
							Style:      pick(tickStyles),
							Attempts:   between(1, 5),
							Sent:       sent,
							Stars:      rng.Intn(4), // 0–3
							OrderIndex: t,
						}
						if tplIdx == 2 {
							tick.Kind = "route"
						}
						if err := tx.Create(&tick).Error; err != nil {
							return err
						}
					}
				}
			}

			// Add a journal entry for ~85% of completed sessions.
			if rng.Intn(20) < 17 {
				sleep := between(2, 5)
				energy := between(2, 5)
				rpe := between(5, 9)
				location := "indoor"
				if rng.Intn(8) == 0 { // ~12% outdoor
					location = "outdoor"
				}
				focusPool := focusByTemplate[tplIdx]
				journal := SessionJournal{
					OwnerID:    ownerID,
					RunID:      &run.ID,
					Date:       startedAt,
					SleepScore: sleep,
					Energy:     energy,
					RPE:        rpe,
					Focus:      pick(focusPool),
					Location:   location,
					WentWell:   pick(wentWellPool),
					NextFocus:  pick(nextFocusPool),
				}
				if err := tx.Create(&journal).Error; err != nil {
					return err
				}
			}

		} else if isToday && len(exercises) > 1 {
			for j := 0; j < 2 && j < len(exercises); j++ {
				comp := RunExerciseCompletion{
					OwnerID:     ownerID,
					RunID:       run.ID,
					ExerciseID:  exercises[j].ID,
					Status:      RunStatusCompleted,
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
