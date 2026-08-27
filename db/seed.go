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
	Label                  string
	Source                 string
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
	Label      string
	Source     string
	Needs      string
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
				Label:                  e.Label,
				Source:                 e.Source,
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
				Label:   t.Label,
				Source:  t.Source,
				Needs:   t.Needs,
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

		if err := seedActivityTemplates(tx, ownerID); err != nil {
			return err
		}

		boardIDs, err := seedVenuesAndBoards(tx, ownerID)
		if err != nil {
			return err
		}

		// Seed historical sessions + runs for the last ~3 months
		if err := seedHistoricalRuns(tx, ownerID, templateIDs); err != nil {
			return err
		}

		if err := seedTrainingCycleWithSchedule(tx, ownerID, templateIDs); err != nil {
			return err
		}

		if err := seedRunVariants(tx, ownerID, templateIDs, boardIDs); err != nil {
			return err
		}

		if err := seedCalendarEvents(tx, ownerID); err != nil {
			return err
		}

		if err := seedAwkwardFixtures(tx, ownerID); err != nil {
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
		{Kind: "timed_reps", Name: "Max Hangs", Label: "hangboard, fingers, strength", Source: "Logical Progression", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge — demo library item", Sets: 6, Reps: 1, RepSeconds: 10, SetRestSeconds: 120, WeightKg: 5},
		{Kind: "reps_and_sets", Name: "Weighted Pull-ups", Label: "strength, pull", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, SetRestSeconds: 120, WeightKg: 10},
		{Kind: "session", SessionDurationSeconds: 600, Name: "10m Open Mobility", Label: "warmup, mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1518611012118-696072aa579a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Session-style block example"},
	}
}

func defaultSeedTemplates() []seedTemplate {
	return []seedTemplate{
		{
			Name:   "Bouldering Power Session",
			Color:  "#eab308",
			Label:  "boulder, power, fingers",
			Source: "Power Company Climbing",
			Needs:  "hangboard, 20mm edge",
			Activities: []seedActivity{
				{
					Type: "warmup",
					Exercises: []seedExercise{
						{Kind: "session", SessionDurationSeconds: 600, Name: "Mobility Flow", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1545205597-3d9d02c29597?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Shoulders and wrists"},
						{Kind: "session", SessionDurationSeconds: 600, Name: "10m Open Mobility", Label: "warmup, mobility", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1518611012118-696072aa579a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Move freely; jot cues below"},
						{Kind: "timed_reps", Name: "Easy Traverses", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1522163182402-834f871fd851?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Smooth movement, light grip", Sets: 3, Reps: 1, RepSeconds: 120, SetRestSeconds: 60},
					},
				},
				{
					Type: "activity",
					Exercises: []seedExercise{
						{Kind: "climbing", Name: "Limit Boulders", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1549576490-b0b4831ef60a?auto=format&fit=crop&w=1200&q=80"}}, Notes: "3-5 hard moves — log each attempt as a tick"},
						{Kind: "timed_reps", Name: "Max Hangs", Label: "hangboard, fingers, strength", Source: "Logical Progression", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1571008887538-b36bb32f4571?auto=format&fit=crop&w=1200&q=80", VideoURL: "https://www.w3schools.com/html/mov_bbb.mp4"}}, Notes: "20mm edge", Sets: 6, Reps: 1, RepSeconds: 10, SetRestSeconds: 120, WeightKg: 5},
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
			Name:   "Strength Base Session",
			Color:  "#ef4444",
			Label:  "strength, antagonist",
			Source: "Logical Progression",
			Needs:  "barbell, rings",
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
						{Kind: "reps_and_sets", Name: "Weighted Pull-ups", Label: "strength, pull", Media: []seedMedia{{ThumbnailURL: "https://images.unsplash.com/photo-1517838277536-f5f99be501cd?auto=format&fit=crop&w=1200&q=80"}}, Notes: "Controlled tempo", Sets: 5, Reps: 5, SetRestSeconds: 120, WeightKg: 10},
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
			Label: "technique, endurance, footwork",
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
							Setting:    "indoor",
							Subtype:    "commercial",
							Grade:      pick(grades),
							Style:      pick(tickStyles),
							Attempts:   between(1, 5),
							Sent:       sent,
							Stars:      rng.Intn(4), // 0–3
							OrderIndex: t,
						}
						if tplIdx == 2 {
							tick.Kind = "sport"
							tick.RopeStyle = "lead"
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

// ─── Dev seed fixtures ──────────────────────────────────────────────────────
//
// Everything below exists so a freshly reseeded dev database has enough shape to
// exercise every screen: activity templates, a live training cycle with sessions
// in the future, runs in each state the app can produce, climbing venues/boards,
// calendar events, and deliberately awkward content (very long names, many
// labels, missing sources, empty states) that is what actually surfaces layout
// bugs. Reseeding is destructive by design — see `make reseed`.

// seedLocalMidnight normalises a timestamp to local midnight, matching how the app
// stores ScheduledSession.ScheduledDate and CalendarEvent bounds.
func seedLocalMidnight(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// seedISOWeekday returns Mon=1..Sun=7, the convention TrainingCycleWeekdayMapping uses.
func seedISOWeekday(t time.Time) int {
	wd := int(t.Weekday())
	if wd == 0 {
		return 7
	}
	return wd
}

type seedActivityTemplateDef struct {
	Name      string
	Label     string
	Source    string
	Exercises []seedExercise
}

func defaultSeedActivityTemplates() []seedActivityTemplateDef {
	return []seedActivityTemplateDef{
		{
			Name:   "General Warm-Up",
			Label:  "warmup, mobility",
			Source: "Power Company Climbing",
			Exercises: []seedExercise{
				{Kind: "session", SessionDurationSeconds: 480, Name: "Pulse Raiser", Notes: "Easy movement until warm."},
				{Kind: "reps_and_sets", Name: "Shoulder Circles", Sets: 2, Reps: 10, SetRestSeconds: 30},
			},
		},
		{
			Name:   "Antagonist & Prehab",
			Label:  "antagonist, strength, prehab",
			Source: "Logical Progression",
			Exercises: []seedExercise{
				{Kind: "reps_and_sets", Name: "External Rotations", Sets: 3, Reps: 12, SetRestSeconds: 60},
				{Kind: "reps_and_sets", Name: "Overhead Press", Sets: 3, Reps: 6, SetRestSeconds: 90},
			},
		},
		{
			Name:  "Cooldown Stretch",
			Label: "cooldown, mobility",
			Exercises: []seedExercise{
				{Kind: "session", SessionDurationSeconds: 300, Name: "Forearm & Lat Stretch"},
			},
		},
	}
}

func seedActivityTemplates(tx *gorm.DB, ownerID uint) error {
	for _, at := range defaultSeedActivityTemplates() {
		row := ActivityTemplate{
			OwnerID: ownerID,
			Name:    at.Name,
			Label:   at.Label,
			Source:  at.Source,
		}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		for i, e := range at.Exercises {
			templateID := row.ID
			ex := Exercise{
				OwnerID:                ownerID,
				ActivityTemplateID:     &templateID,
				Kind:                   NormalizeKind(e.Kind),
				SessionDurationSeconds: e.SessionDurationSeconds,
				Name:                   e.Name,
				Notes:                  e.Notes,
				Sets:                   e.Sets,
				Reps:                   e.Reps,
				RepSeconds:             e.RepSeconds,
				SetRestSeconds:         e.SetRestSeconds,
				OrderIndex:             i,
			}
			if err := tx.Create(&ex).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// seedVenuesAndBoards creates the venues and boards the tick logger offers, and
// returns the board IDs so runs can reference one.
func seedVenuesAndBoards(tx *gorm.DB, ownerID uint) ([]uint, error) {
	venues := []ClimbingVenue{
		{OwnerID: ownerID, Name: "The Castle", Kind: "commercial", Location: "London"},
		{OwnerID: ownerID, Name: "VauxWall East", Kind: "commercial", Location: "London"},
		{OwnerID: ownerID, Name: "Rocher Canon", Kind: "outdoor", Location: "Fontainebleau"},
		{OwnerID: ownerID, Name: "Portland — The Cuttings", Kind: "outdoor", Location: "Dorset"},
	}
	for i := range venues {
		if err := tx.Create(&venues[i]).Error; err != nil {
			return nil, err
		}
	}

	boards := []ClimbingBoard{
		{OwnerID: ownerID, BoardType: "tension", Name: "Tension Board 2 — 40°"},
		{OwnerID: ownerID, BoardType: "kilter", Name: "Home Kilter"},
		{OwnerID: ownerID, BoardType: "moon", Name: "Moon 2019"},
	}
	ids := make([]uint, 0, len(boards))
	for i := range boards {
		if err := tx.Create(&boards[i]).Error; err != nil {
			return nil, err
		}
		ids = append(ids, boards[i].ID)
	}
	return ids, nil
}

func seedCalendarEvents(tx *gorm.DB, ownerID uint) error {
	today := seedLocalMidnight(time.Now())
	events := []CalendarEvent{
		{
			OwnerID: ownerID, Title: "Fontainebleau trip", Kind: "trip",
			StartDate: today.AddDate(0, 0, 12), EndDate: today.AddDate(0, 0, 19),
			Notes: "Font — expect no gym sessions this week.", Blocks: true,
		},
		{
			OwnerID: ownerID, Title: "Finger tweak — easy week", Kind: "injury",
			StartDate: today.AddDate(0, 0, -30), EndDate: today.AddDate(0, 0, -24),
			Notes: "Left ring finger, A2. No crimping.", Blocks: true,
		},
		{
			OwnerID: ownerID, Title: "Deload", Kind: "rest",
			StartDate: today.AddDate(0, 0, 21), EndDate: today.AddDate(0, 0, 27),
			Blocks: false,
		},
		{
			OwnerID: ownerID, Title: "Bloc Fest qualifiers", Kind: "competition",
			StartDate: today.AddDate(0, 0, 33), EndDate: today.AddDate(0, 0, 33),
			Blocks: false,
		},
	}
	for i := range events {
		if err := tx.Create(&events[i]).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedTrainingCycleWithSchedule creates a 4-week cycle that is already mid-flight
// (it started last week) with goals, weekday mappings, and scheduled sessions from
// today to the cycle's end. Past dates are left to seedHistoricalRuns so the two
// do not both claim the same days.
func seedTrainingCycleWithSchedule(tx *gorm.DB, ownerID uint, templateIDs []uint) error {
	if len(templateIDs) == 0 {
		return nil
	}

	today := seedLocalMidnight(time.Now())
	monday := today.AddDate(0, 0, -(seedISOWeekday(today) - 1))
	start := monday.AddDate(0, 0, -7)

	cycle := TrainingCycle{
		OwnerID:   ownerID,
		Name:      "Autumn Power Block",
		StartDate: start,
		Weeks:     4,
		Focus:     "projects",
		Label:     "boulder, power",
		Goal:      "Flash 6c on the board by the end of the block.",
		Notes:     "Three load weeks, one deload. Skill work first in every session.",
	}
	if err := tx.Create(&cycle).Error; err != nil {
		return err
	}

	goals := []CycleGoal{
		{Before: "Fall off 6b+ board problems", After: "Flash 6c consistently", How: "Two board sessions a week, max power first", OrderIndex: 0},
		{Before: "Pumped after 3 routes", After: "Ten routes without failing", How: "One endurance day, traverse circuits", OrderIndex: 1},
		{Before: "Avoiding lead", After: "Comfortable falling above the bolt", How: "Fall practice opens every lead day", OrderIndex: 2},
	}
	for i := range goals {
		goals[i].OwnerID = ownerID
		goals[i].TrainingCycleID = cycle.ID
		if err := tx.Create(&goals[i]).Error; err != nil {
			return err
		}
	}

	// Mon/Wed/Fri/Sun, cycling through whichever templates exist.
	weekdays := []int{1, 3, 5, 7}
	byWeekday := map[int]uint{}
	for i, wd := range weekdays {
		tplID := templateIDs[i%len(templateIDs)]
		byWeekday[wd] = tplID
		mapping := TrainingCycleWeekdayMapping{
			OwnerID:           ownerID,
			TrainingCycleID:   cycle.ID,
			Weekday:           wd,
			SessionTemplateID: tplID,
		}
		if err := tx.Create(&mapping).Error; err != nil {
			return err
		}
	}

	end := start.AddDate(0, 0, cycle.Weeks*7-1)
	cycleID := cycle.ID
	for d := start; !d.After(end); d = d.AddDate(0, 0, 1) {
		if d.Before(today) {
			continue
		}
		tplID, ok := byWeekday[seedISOWeekday(d)]
		if !ok {
			continue
		}
		ss := ScheduledSession{
			OwnerID:           ownerID,
			TrainingCycleID:   &cycleID,
			ScheduledDate:     d,
			SessionTemplateID: tplID,
		}
		if err := tx.Create(&ss).Error; err != nil {
			return err
		}
	}
	return nil
}

// seedRunVariants creates one run in each state the app can produce, so screens that
// only appear for an unusual state (an open session mid-flight, a manual draft, a run
// finished early with skipped steps) have something to render.
func seedRunVariants(tx *gorm.DB, ownerID uint, templateIDs []uint, boardIDs []uint) error {
	if len(templateIDs) == 0 {
		return nil
	}
	now := time.Now()
	today := seedLocalMidnight(now)

	// The hidden per-user anchor template that open and manual runs hang off, created
	// the same way handleStartOpenSession does.
	anchor := SessionTemplate{OwnerID: ownerID, Name: "Open Session", IsSystem: true}
	if err := tx.Create(&anchor).Error; err != nil {
		return err
	}

	newScheduled := func(tplID uint, date time.Time, trial bool) (*ScheduledSession, error) {
		ss := &ScheduledSession{
			OwnerID:           ownerID,
			IsTrial:           trial,
			ScheduledDate:     date,
			SessionTemplateID: tplID,
		}
		return ss, tx.Create(ss).Error
	}

	// 1. Open session, still in progress today.
	ss, err := newScheduled(anchor.ID, today, true)
	if err != nil {
		return err
	}
	openRun := SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsOpen:             true,
		CustomName:         "Evening board session",
		Status:             RunStatusRunning,
		StartedAt:          now.Add(-40 * time.Minute),
	}
	if err := tx.Create(&openRun).Error; err != nil {
		return err
	}

	// 2. Manual draft — the resume-draft path in the training log.
	ss, err = newScheduled(anchor.ID, today, true)
	if err != nil {
		return err
	}
	draft := SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsManual:           true,
		IsDraft:            true,
		Status:             RunStatusRunning,
		StartedAt:          now.Add(-3 * time.Hour),
	}
	if err := tx.Create(&draft).Error; err != nil {
		return err
	}

	// 3. Manual saved entry, backdated — a session logged after the fact.
	ss, err = newScheduled(anchor.ID, today.AddDate(0, 0, -2), true)
	if err != nil {
		return err
	}
	manualStart := today.AddDate(0, 0, -2).Add(18 * time.Hour)
	manualDone := manualStart.Add(75 * time.Minute)
	manual := SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		IsTrial:            true,
		IsManual:           true,
		CustomName:         "Logged from memory — outdoor bouldering",
		Status:             RunStatusCompleted,
		StartedAt:          manualStart,
		CompletedAt:        &manualDone,
	}
	if err := tx.Create(&manual).Error; err != nil {
		return err
	}
	if err := tx.Create(&SessionJournal{
		OwnerID:      ownerID,
		RunID:        &manual.ID,
		Date:         manualStart,
		SleepScore:   4,
		Energy:       4,
		RPE:          8,
		Focus:        "projects",
		Location:     "outdoor",
		WentWell:     "Sent the project second go after a season on it.",
		NextFocus:    "Find a new line at the same grade.",
		SessionNotes: "Cold, dry conditions. Skin held up.",
	}).Error; err != nil {
		return err
	}

	// 4. A guided run finished early: the steps after the stopping point are recorded
	// as skipped, so the summary accounts for every step rather than leaving holes.
	tplID := templateIDs[0]
	var exercises []Exercise
	err = tx.
		Joins("JOIN activities ON activities.id = exercises.activity_id").
		Where("activities.session_template_id = ? AND exercises.parent_exercise_id IS NULL", tplID).
		Order("exercises.order_index").
		Find(&exercises).Error
	if err != nil {
		return err
	}

	ss, err = newScheduled(tplID, today.AddDate(0, 0, -4), false)
	if err != nil {
		return err
	}
	earlyStart := today.AddDate(0, 0, -4).Add(19 * time.Hour)
	earlyDone := earlyStart.Add(28 * time.Minute)
	early := SessionRun{
		OwnerID:            ownerID,
		ScheduledSessionID: ss.ID,
		Status:             RunStatusCompleted,
		StartedAt:          earlyStart,
		CompletedAt:        &earlyDone,
	}
	if err := tx.Create(&early).Error; err != nil {
		return err
	}
	for i, ex := range exercises {
		status := RunStatusCompleted
		if i > 0 {
			status = RunStatusSkipped
		}
		comp := RunExerciseCompletion{
			OwnerID:        ownerID,
			RunID:          early.ID,
			ExerciseID:     ex.ID,
			Status:         status,
			CompletedAt:    earlyStart.Add(time.Duration(10*(i+1)) * time.Minute),
			ElapsedSeconds: 600,
			RunNotes:       "Left early — finger felt tender.",
		}
		if err := tx.Create(&comp).Error; err != nil {
			return err
		}
	}

	// 5. Board context and per-set logs on the exercises of that template, so the
	// climbing meta and set-log editors have rows to show.
	for _, ex := range exercises {
		if ex.Kind == "climbing" && len(boardIDs) > 0 {
			boardID := boardIDs[0]
			if err := tx.Create(&ClimbingExerciseMeta{
				OwnerID:    ownerID,
				RunID:      early.ID,
				ExerciseID: ex.ID,
				Type:       "board",
				BoardKind:  "tension",
				BoardID:    &boardID,
			}).Error; err != nil {
				return err
			}
		}
	}

	// Per-set targets and logs need a weighted exercise, which the first template may
	// not have — look across every seeded template rather than assuming.
	var lifts []Exercise
	if err := tx.
		Where("owner_id = ? AND kind = ? AND sets > 0 AND parent_exercise_id IS NULL", ownerID, "reps_and_sets").
		Order("id").
		Limit(3).
		Find(&lifts).Error; err != nil {
		return err
	}
	for _, ex := range lifts {
		for setIdx := 1; setIdx <= ex.Sets && setIdx <= 4; setIdx++ {
			if err := tx.Create(&ExercisePlannedSet{
				OwnerID:    ownerID,
				ExerciseID: ex.ID,
				SetIndex:   setIdx,
				Reps:       ex.Reps,
				WeightKg:   ex.WeightKg + float64(setIdx-1)*2.5,
			}).Error; err != nil {
				return err
			}
			if err := tx.Create(&ManualExerciseSetLog{
				OwnerID:    ownerID,
				RunID:      manual.ID,
				ExerciseID: ex.ID,
				SetIndex:   setIdx,
				Reps:       ex.Reps,
				WeightKg:   ex.WeightKg + float64(setIdx-1)*2.5,
			}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// seedAwkwardFixtures adds content chosen to break layouts: names far longer than any
// column, more labels than fit on one line, missing sources, and an empty template.
// Every mobile overflow bug found so far only showed up with content like this.
func seedAwkwardFixtures(tx *gorm.DB, ownerID uint) error {
	longName := "Low Intensity Endurance on Continuous Boulders (45+ Minutes, No Hangs)"
	manyLabels := "technique, footwork, tension, balance, endurance, boulder, warmup"

	libs := []LibraryExercise{
		{
			OwnerID: ownerID, Name: longName, Label: manyLabels,
			Source: "Power Company Climbing", Kind: NormalizeKind("climbing"),
			Notes: "Deliberately long name — checks truncation in the library table.",
		},
		{
			OwnerID: ownerID, Name: "Unsourced Drill", Kind: NormalizeKind("session"),
			SessionDurationSeconds: 900,
			Notes:                  "No label, no source — checks the empty-cell case.",
		},
		{
			OwnerID: ownerID, Name: "Heel-Hook Isometric Pull 60, 90, 120 Degrees",
			Label: "strength, tension, core", Source: "Self-Coached Climber",
			Kind: NormalizeKind("timed_reps"), Sets: 3, Reps: 3, RepSeconds: 8, SetRestSeconds: 120,
		},
	}
	for i := range libs {
		if err := tx.Create(&libs[i]).Error; err != nil {
			return err
		}
	}

	// A template with no activities at all: the empty-state branch of the editor.
	empty := SessionTemplate{
		OwnerID: ownerID,
		Name:    "Empty Template (no activities yet)",
		Label:   "draft",
	}
	if err := tx.Create(&empty).Error; err != nil {
		return err
	}

	// A template whose name, labels and needs all overflow their columns.
	wide := SessionTemplate{
		OwnerID: ownerID,
		Name:    "Board Session — Limit Bouldering, Sub-Limit Execution and Optional Strength",
		Color:   "#84cc16",
		Label:   manyLabels,
		Needs:   "Tension Board 2, 20mm edge, hangboard, kettlebell, rings, resistance bands",
	}
	if err := tx.Create(&wide).Error; err != nil {
		return err
	}
	activity := Activity{OwnerID: ownerID, SessionTemplateID: wide.ID, Type: "activity", OrderIndex: 0}
	if err := tx.Create(&activity).Error; err != nil {
		return err
	}
	actID := activity.ID
	if err := tx.Create(&Exercise{
		OwnerID:    ownerID,
		ActivityID: &actID,
		Kind:       NormalizeKind("climbing"),
		Name:       longName,
		Notes:      "Long exercise name inside a long template name.",
		OrderIndex: 0,
	}).Error; err != nil {
		return err
	}

	return tx.Create(&ActivityTemplate{
		OwnerID: ownerID,
		Name:    "Movement Practice: Board — Tension, Commitment and Deadpointing",
		Label:   manyLabels,
		Source:  "Paradigm Climbing",
	}).Error
}
