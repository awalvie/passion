package db

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

type YAMLImportOptions struct {
	OwnerID              uint
	ExercisesDir         []string
	SessionTemplatesDir  []string
	ActivityTemplatesDir []string
}

type yamlExerciseDoc struct {
	Exercises []yamlExercise `yaml:"exercises"`
}

type yamlMediaItem struct {
	VideoURL     string `yaml:"video_url"`
	ThumbnailURL string `yaml:"thumbnail_url"`
}

type yamlExercise struct {
	Name                   string          `yaml:"name"`
	Label                  string          `yaml:"label"`
	Source                 string          `yaml:"source"`
	Kind                   string          `yaml:"kind"`
	SessionDurationSeconds int             `yaml:"session_duration_seconds"`
	Media                  []yamlMediaItem `yaml:"media"`
	Notes                  string          `yaml:"notes"`
	Sets                   int             `yaml:"sets"`
	Reps                   int             `yaml:"reps"`
	RepSeconds             int             `yaml:"rep_seconds"`
	RepRestSeconds         int             `yaml:"rep_rest_seconds"`
	SetRestSeconds         int             `yaml:"set_rest_seconds"`
	PrepSeconds            int             `yaml:"prep_seconds"`
	RungSeconds            string          `yaml:"rung_seconds"`
	WeightKg               float64         `yaml:"weight_kg"`
}

type yamlSessionTemplateDoc struct {
	Templates []yamlSessionTemplate `yaml:"templates"`
}

type yamlSessionTemplate struct {
	Name       string                `yaml:"name"`
	Color      string                `yaml:"color"`
	Label      string                `yaml:"label"`
	Source     string                `yaml:"source"`
	Needs      string                `yaml:"needs"`
	Activities []yamlSessionActivity `yaml:"activities"`
}

type yamlSessionActivity struct {
	Ref       string                `yaml:"ref"`
	Type      string                `yaml:"type"`
	Name      string                `yaml:"name"`
	Exercises []yamlSessionExercise `yaml:"exercises"`
}

type yamlActivityTemplateDoc struct {
	ActivityTemplates []yamlActivityTemplate `yaml:"activity_templates"`
}

type yamlActivityTemplate struct {
	Name      string                `yaml:"name"`
	Label     string                `yaml:"label"`
	Source    string                `yaml:"source"`
	Exercises []yamlSessionExercise `yaml:"exercises"`
}

type yamlSessionExercise struct {
	Ref                    string                `yaml:"ref"`
	Name                   string                `yaml:"name"`
	Kind                   string                `yaml:"kind"`
	SessionDurationSeconds int                   `yaml:"session_duration_seconds"`
	Media                  []yamlMediaItem       `yaml:"media"`
	Notes                  string                `yaml:"notes"`
	Sets                   int                   `yaml:"sets"`
	Reps                   int                   `yaml:"reps"`
	RepSeconds             int                   `yaml:"rep_seconds"`
	RepRestSeconds         int                   `yaml:"rep_rest_seconds"`
	SetRestSeconds         int                   `yaml:"set_rest_seconds"`
	PrepSeconds            int                   `yaml:"prep_seconds"`
	RungSeconds            string                `yaml:"rung_seconds"`
	WeightKg               float64               `yaml:"weight_kg"`
	Children               []yamlSessionExercise `yaml:"children"`
}

func (s *Store) ImportYAML(opts YAMLImportOptions) error {
	if opts.OwnerID == 0 {
		return errors.New("yaml import owner id must be positive")
	}
	exercisesDirs := trimDirs(opts.ExercisesDir)
	templatesDirs := trimDirs(opts.SessionTemplatesDir)
	if len(exercisesDirs) == 0 || len(templatesDirs) == 0 {
		return errors.New("yaml import directories are required")
	}

	libraryExercises, err := loadExerciseYAML(exercisesDirs)
	if err != nil {
		return err
	}
	if err := validateNoDuplicateExerciseNames(libraryExercises); err != nil {
		return err
	}

	resolvedExerciseMap := make(map[string]yamlExercise, len(libraryExercises))
	for _, ex := range libraryExercises {
		resolvedExerciseMap[strings.TrimSpace(ex.Name)] = ex
	}

	// Activity templates are optional — skip if dir not configured.
	var activityTemplates []yamlActivityTemplate
	if activityTemplatesDirs := trimDirs(opts.ActivityTemplatesDir); len(activityTemplatesDirs) > 0 {
		activityTemplates, err = loadActivityTemplateYAML(activityTemplatesDirs)
		if err != nil {
			return err
		}
		if err := validateNoDuplicateActivityTemplateNames(activityTemplates); err != nil {
			return err
		}
	}

	resolvedActivityTemplateMap := make(map[string]yamlActivityTemplate, len(activityTemplates))
	for _, at := range activityTemplates {
		resolvedActivityTemplateMap[strings.TrimSpace(at.Name)] = at
	}

	sessionTemplates, err := loadSessionTemplateYAML(templatesDirs)
	if err != nil {
		return err
	}
	if err := validateNoDuplicateTemplateNames(sessionTemplates); err != nil {
		return err
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		for _, ex := range libraryExercises {
			if err := upsertLibraryExercise(tx, opts.OwnerID, ex); err != nil {
				return err
			}
		}
		for _, at := range activityTemplates {
			if err := upsertActivityTemplate(tx, opts.OwnerID, at, resolvedExerciseMap); err != nil {
				return err
			}
		}
		for _, tpl := range sessionTemplates {
			if err := upsertSessionTemplate(tx, opts.OwnerID, tpl, resolvedExerciseMap, resolvedActivityTemplateMap); err != nil {
				return err
			}
		}
		return pruneCatalogOrphans(tx, opts.OwnerID, libraryExercises, activityTemplates, sessionTemplates)
	})
}

func nameSet[T any](items []T, name func(T) string) map[string]struct{} {
	out := make(map[string]struct{}, len(items))
	for _, it := range items {
		out[name(it)] = struct{}{}
	}
	return out
}

// pruneCatalogOrphans removes catalog-managed rows for the owner whose names are no
// longer present in the YAML — the stale rows left behind when a catalog entry is
// renamed or dropped. It only touches rows flagged ManagedByCatalog (never UI-created
// data), skips the system open-session template, and never deletes a session template
// that has scheduled sessions or cycle mappings (which would orphan logged runs) or a
// library exercise still referenced by an exercise copy or cycle override.
//
// As a safety guard against a misconfigured/empty import directory wiping a whole
// category, a category is only pruned when the YAML actually defined entries for it.
func pruneCatalogOrphans(tx *gorm.DB, ownerID uint, exs []yamlExercise, ats []yamlActivityTemplate, sts []yamlSessionTemplate) error {
	if len(exs) > 0 {
		keep := nameSet(exs, func(e yamlExercise) string { return e.Name })
		var rows []LibraryExercise
		if err := tx.Where("owner_id = ? AND managed_by_catalog = ?", ownerID, true).Find(&rows).Error; err != nil {
			return err
		}
		for _, le := range rows {
			if _, ok := keep[le.Name]; ok {
				continue
			}
			var used int64
			if err := tx.Model(&Exercise{}).Where("library_exercise_id = ?", le.ID).Count(&used).Error; err != nil {
				return err
			}
			if used == 0 {
				var ov int64
				if err := tx.Model(&CycleExerciseOverride{}).Where("library_exercise_id = ?", le.ID).Count(&ov).Error; err != nil {
					return err
				}
				used += ov
			}
			if used == 0 {
				var wov int64
				if err := tx.Model(&CycleExerciseWeekOverride{}).Where("library_exercise_id = ?", le.ID).Count(&wov).Error; err != nil {
					return err
				}
				used += wov
			}
			if used == 0 {
				// A catalog parent may own UI-created child options; keep it so they
				// aren't left with a dangling parent_library_exercise_id.
				var kids int64
				if err := tx.Model(&LibraryExercise{}).Where("parent_library_exercise_id = ?", le.ID).Count(&kids).Error; err != nil {
					return err
				}
				used += kids
			}
			if used > 0 {
				continue
			}
			if err := tx.Unscoped().Where("library_exercise_id = ?", le.ID).Delete(&ExerciseMedia{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&LibraryExercise{}, le.ID).Error; err != nil {
				return err
			}
		}
	}

	if len(ats) > 0 {
		keep := nameSet(ats, func(a yamlActivityTemplate) string { return a.Name })
		var rows []ActivityTemplate
		if err := tx.Where("owner_id = ? AND managed_by_catalog = ?", ownerID, true).Find(&rows).Error; err != nil {
			return err
		}
		for _, at := range rows {
			if _, ok := keep[at.Name]; ok {
				continue
			}
			if err := deleteExercisesAndMedia(tx, "activity_template_id = ?", at.ID); err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&ActivityTemplate{}, at.ID).Error; err != nil {
				return err
			}
		}
	}

	if len(sts) > 0 {
		keep := nameSet(sts, func(t yamlSessionTemplate) string { return t.Name })
		var rows []SessionTemplate
		if err := tx.Where("owner_id = ? AND managed_by_catalog = ?", ownerID, true).Find(&rows).Error; err != nil {
			return err
		}
		for _, st := range rows {
			if _, ok := keep[st.Name]; ok {
				continue
			}
			if st.IsSystem {
				continue
			}
			// Unscoped: a soft-deleted scheduled session can still have a live run
			// whose completions reference this template's exercises, so it must count
			// as in-use and block the delete.
			var refs int64
			if err := tx.Unscoped().Model(&ScheduledSession{}).Where("session_template_id = ?", st.ID).Count(&refs).Error; err != nil {
				return err
			}
			if refs == 0 {
				var cyc int64
				if err := tx.Unscoped().Model(&TrainingCycleWeekdayMapping{}).Where("session_template_id = ?", st.ID).Count(&cyc).Error; err != nil {
					return err
				}
				refs += cyc
			}
			if refs > 0 {
				continue
			}
			var acts []Activity
			if err := tx.Where("session_template_id = ?", st.ID).Find(&acts).Error; err != nil {
				return err
			}
			for _, a := range acts {
				if err := deleteExercisesAndMedia(tx, "activity_id = ?", a.ID); err != nil {
					return err
				}
			}
			if err := tx.Unscoped().Where("session_template_id = ?", st.ID).Delete(&Activity{}).Error; err != nil {
				return err
			}
			if err := tx.Unscoped().Delete(&SessionTemplate{}, st.ID).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

// deleteExercisesAndMedia hard-deletes the exercises matched by cond (plus their
// media) so pruned templates leave no orphaned child rows behind. The step-by-step
// child deletion is required, not redundant: SQLite foreign keys are not enabled on
// this connection, so the OnDelete:CASCADE tags in the models never fire.
func deleteExercisesAndMedia(tx *gorm.DB, cond string, arg any) error {
	var exs []Exercise
	if err := tx.Where(cond, arg).Find(&exs).Error; err != nil {
		return err
	}
	for _, e := range exs {
		if err := tx.Unscoped().Where("exercise_id = ?", e.ID).Delete(&ExerciseMedia{}).Error; err != nil {
			return err
		}
	}
	return tx.Unscoped().Where(cond, arg).Delete(&Exercise{}).Error
}

func loadExerciseYAML(dirs []string) ([]yamlExercise, error) {
	paths, err := listYAMLFiles(dirs)
	if err != nil {
		return nil, err
	}
	var out []yamlExercise
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read exercise yaml %q: %w", path, err)
		}
		items, err := parseExercisesYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parse exercise yaml %q: %w", path, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

func loadSessionTemplateYAML(dirs []string) ([]yamlSessionTemplate, error) {
	paths, err := listYAMLFiles(dirs)
	if err != nil {
		return nil, err
	}
	var out []yamlSessionTemplate
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read session template yaml %q: %w", path, err)
		}
		items, err := parseSessionTemplatesYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parse session template yaml %q: %w", path, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

// trimDirs drops blank entries so a stray comma in configuration is not read as a
// directory named "".
func trimDirs(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// listYAMLFiles returns every .yaml/.yml file under each dir, walking subdirectories so
// the catalog can be organised into folders (by program, say). Layout carries no meaning
// to the importer — an entry's identity is its name field, not its path — so files can be
// moved freely, including between directories, without triggering a rename or a prune.
// Hidden directories are skipped.
//
// Several directories may be given: the published catalog plus a private one, say. They
// are concatenated before validation, so a ref in one resolves against every other, and
// a name defined twice is caught by the duplicate-name check as if it came from one tree.
// Note that filepath.WalkDir does not follow symlinks, so a symlinked directory is
// skipped silently — list the real path instead.
func listYAMLFiles(dirs []string) ([]string, error) {
	var files []string
	for _, dir := range dirs {
		err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() {
				if p != dir && strings.HasPrefix(d.Name(), ".") {
					return fs.SkipDir
				}
				return nil
			}
			switch strings.ToLower(filepath.Ext(d.Name())) {
			case ".yaml", ".yml":
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("read yaml dir %q: %w", dir, err)
		}
	}
	sort.Strings(files)
	return files, nil
}

func parseExercisesYAML(data []byte) ([]yamlExercise, error) {
	var wrapped yamlExerciseDoc
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Exercises) > 0 {
		return validateExercises(wrapped.Exercises)
	}
	var single yamlExercise
	if err := yaml.Unmarshal(data, &single); err == nil && strings.TrimSpace(single.Name) != "" {
		return validateExercises([]yamlExercise{single})
	}
	return nil, errors.New("expected exercise document with exercises list or single exercise")
}

func parseSessionTemplatesYAML(data []byte) ([]yamlSessionTemplate, error) {
	var wrapped yamlSessionTemplateDoc
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.Templates) > 0 {
		return validateSessionTemplates(wrapped.Templates)
	}
	var single yamlSessionTemplate
	if err := yaml.Unmarshal(data, &single); err == nil && strings.TrimSpace(single.Name) != "" {
		return validateSessionTemplates([]yamlSessionTemplate{single})
	}
	return nil, errors.New("expected template document with templates list or single template")
}

func validateExercises(items []yamlExercise) ([]yamlExercise, error) {
	for i := range items {
		items[i].Name = strings.TrimSpace(items[i].Name)
		if items[i].Name == "" {
			return nil, fmt.Errorf("exercise[%d]: name is required", i)
		}
		items[i].Kind = NormalizeKind(items[i].Kind)
	}
	return items, nil
}

func validateSessionTemplates(items []yamlSessionTemplate) ([]yamlSessionTemplate, error) {
	for tIdx := range items {
		items[tIdx].Name = strings.TrimSpace(items[tIdx].Name)
		if items[tIdx].Name == "" {
			return nil, fmt.Errorf("template[%d]: name is required", tIdx)
		}
		for aIdx := range items[tIdx].Activities {
			act := &items[tIdx].Activities[aIdx]
			act.Ref = strings.TrimSpace(act.Ref)
			act.Type = strings.TrimSpace(act.Type)
			if act.Ref != "" && act.Type != "" {
				return nil, fmt.Errorf("template[%d].activities[%d]: use either ref or type, not both", tIdx, aIdx)
			}
			if act.Ref != "" && len(act.Exercises) > 0 {
				return nil, fmt.Errorf("template[%d].activities[%d]: ref activities cannot define inline exercises", tIdx, aIdx)
			}
			if act.Ref == "" && act.Type == "" {
				return nil, fmt.Errorf("template[%d].activities[%d]: type is required", tIdx, aIdx)
			}
			if act.Ref != "" {
				continue
			}
			for eIdx := range act.Exercises {
				ex := &act.Exercises[eIdx]
				ex.Ref = strings.TrimSpace(ex.Ref)
				ex.Name = strings.TrimSpace(ex.Name)
				if ex.Ref == "" && ex.Name == "" {
					return nil, fmt.Errorf("template[%d].activities[%d].exercises[%d]: name is required for inline exercise", tIdx, aIdx, eIdx)
				}
				if ex.Ref != "" && ex.Name != "" {
					return nil, fmt.Errorf("template[%d].activities[%d].exercises[%d]: use either ref or inline exercise fields, not both", tIdx, aIdx, eIdx)
				}
				ex.Kind = NormalizeKind(ex.Kind)
			}
		}
	}
	return items, nil
}

func validateNoDuplicateExerciseNames(items []yamlExercise) error {
	seen := map[string]struct{}{}
	for _, ex := range items {
		name := strings.TrimSpace(ex.Name)
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate exercise name in yaml import: %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func validateNoDuplicateTemplateNames(items []yamlSessionTemplate) error {
	seen := map[string]struct{}{}
	for _, tpl := range items {
		name := strings.TrimSpace(tpl.Name)
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate session template name in yaml import: %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func NormalizeKind(raw string) string {
	s := strings.TrimSpace(strings.ToLower(raw))
	switch s {
	case "session":
		return "session"
	case "exercise_catalog":
		return "exercise_catalog"
	case "timed_reps":
		return "timed_reps"
	case "climbing":
		return "climbing"
	default:
		return "reps_and_sets"
	}
}

func createExerciseMedia(tx *gorm.DB, ownerID uint, exerciseID *uint, libraryExerciseID *uint, items []yamlMediaItem) error {
	for i, m := range items {
		v := strings.TrimSpace(m.VideoURL)
		t := strings.TrimSpace(m.ThumbnailURL)
		if v == "" && t == "" {
			continue
		}
		em := ExerciseMedia{
			OwnerID:           ownerID,
			ExerciseID:        exerciseID,
			LibraryExerciseID: libraryExerciseID,
			VideoURL:          v,
			ThumbnailURL:      t,
			OrderIndex:        i,
		}
		if err := tx.Create(&em).Error; err != nil {
			return err
		}
	}
	return nil
}

func loadActivityTemplateYAML(dirs []string) ([]yamlActivityTemplate, error) {
	paths, err := listYAMLFiles(dirs)
	if err != nil {
		return nil, err
	}
	var out []yamlActivityTemplate
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read activity template yaml %q: %w", path, err)
		}
		items, err := parseActivityTemplatesYAML(data)
		if err != nil {
			return nil, fmt.Errorf("parse activity template yaml %q: %w", path, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

func parseActivityTemplatesYAML(data []byte) ([]yamlActivityTemplate, error) {
	var wrapped yamlActivityTemplateDoc
	if err := yaml.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	if len(wrapped.ActivityTemplates) > 0 {
		return validateActivityTemplates(wrapped.ActivityTemplates)
	}
	var single yamlActivityTemplate
	if err := yaml.Unmarshal(data, &single); err == nil && strings.TrimSpace(single.Name) != "" {
		return validateActivityTemplates([]yamlActivityTemplate{single})
	}
	return nil, errors.New("expected activity template document with activity_templates list or single template")
}

func validateActivityTemplates(items []yamlActivityTemplate) ([]yamlActivityTemplate, error) {
	for i := range items {
		items[i].Name = strings.TrimSpace(items[i].Name)
		if items[i].Name == "" {
			return nil, fmt.Errorf("activity_template[%d]: name is required", i)
		}
		for eIdx := range items[i].Exercises {
			ex := &items[i].Exercises[eIdx]
			ex.Ref = strings.TrimSpace(ex.Ref)
			ex.Name = strings.TrimSpace(ex.Name)
			if ex.Ref == "" && ex.Name == "" {
				return nil, fmt.Errorf("activity_template[%d] %q exercises[%d]: name is required for inline exercise", i, items[i].Name, eIdx)
			}
			if ex.Ref != "" && ex.Name != "" {
				return nil, fmt.Errorf("activity_template[%d] %q exercises[%d]: use either ref or inline exercise fields, not both", i, items[i].Name, eIdx)
			}
			ex.Kind = NormalizeKind(ex.Kind)
		}
	}
	return items, nil
}

func validateNoDuplicateActivityTemplateNames(items []yamlActivityTemplate) error {
	seen := map[string]struct{}{}
	for _, at := range items {
		name := strings.TrimSpace(at.Name)
		if _, ok := seen[name]; ok {
			return fmt.Errorf("duplicate activity template name in yaml import: %q", name)
		}
		seen[name] = struct{}{}
	}
	return nil
}

func upsertActivityTemplate(tx *gorm.DB, ownerID uint, at yamlActivityTemplate, byExerciseName map[string]yamlExercise) error {
	var row ActivityTemplate
	res := tx.Where("owner_id = ? AND name = ?", ownerID, at.Name).Limit(1).Find(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		row = ActivityTemplate{OwnerID: ownerID, Name: at.Name}
	}
	row.Label = strings.TrimSpace(at.Label)
	row.Source = strings.TrimSpace(at.Source)
	row.ManagedByCatalog = true
	if err := tx.Save(&row).Error; err != nil {
		return err
	}

	// Replace all exercises for this template (children share the same ActivityTemplateID).
	if err := tx.Where("owner_id = ? AND activity_template_id = ?", ownerID, row.ID).
		Delete(&Exercise{}).Error; err != nil {
		return err
	}

	tplID := row.ID
	for eIdx, ex := range at.Exercises {
		ate := Exercise{
			OwnerID:            ownerID,
			ActivityTemplateID: &tplID,
			OrderIndex:         eIdx,
		}
		if ex.Ref != "" {
			ref, ok := byExerciseName[ex.Ref]
			if !ok {
				return fmt.Errorf("activity template %q references unknown exercise %q", at.Name, ex.Ref)
			}
			ate.Name = ref.Name
			ate.Kind = ref.Kind
			ate.SessionDurationSeconds = ref.SessionDurationSeconds
			ate.Notes = ref.Notes
			ate.Sets = ref.Sets
			ate.Reps = ref.Reps
			ate.RepSeconds = ref.RepSeconds
			ate.RepRestSeconds = ref.RepRestSeconds
			ate.SetRestSeconds = ref.SetRestSeconds
			ate.PrepSeconds = ref.PrepSeconds
			ate.RungSeconds = ref.RungSeconds
			ate.WeightKg = ref.WeightKg
		} else {
			ate.Name = ex.Name
			ate.Kind = ex.Kind
			ate.SessionDurationSeconds = ex.SessionDurationSeconds
			ate.Notes = ex.Notes
			ate.Sets = ex.Sets
			ate.Reps = ex.Reps
			ate.RepSeconds = ex.RepSeconds
			ate.RepRestSeconds = ex.RepRestSeconds
			ate.SetRestSeconds = ex.SetRestSeconds
			ate.PrepSeconds = ex.PrepSeconds
			ate.RungSeconds = ex.RungSeconds
			ate.WeightKg = ex.WeightKg
		}
		if err := tx.Create(&ate).Error; err != nil {
			return err
		}

		if ate.Kind == "exercise_catalog" && len(ex.Children) > 0 {
			for cIdx, child := range ex.Children {
				child.Ref = strings.TrimSpace(child.Ref)
				child.Name = strings.TrimSpace(child.Name)
				childKind := NormalizeKind(child.Kind)
				if childKind == "exercise_catalog" {
					return fmt.Errorf("activity template %q: nested exercise_catalog not allowed (child %q)", at.Name, child.Name)
				}
				pid := ate.ID
				childEx := Exercise{
					OwnerID:            ownerID,
					ActivityTemplateID: &tplID,
					OrderIndex:         eIdx*1000 + cIdx + 1,
					Kind:               childKind,
					ParentExerciseID:   &pid,
				}
				if child.Ref != "" {
					ref, ok := byExerciseName[child.Ref]
					if !ok {
						return fmt.Errorf("activity template %q exercise_catalog %q: unknown child exercise %q", at.Name, ate.Name, child.Ref)
					}
					childEx.Name = ref.Name
					childEx.Kind = NormalizeKind(ref.Kind)
					childEx.SessionDurationSeconds = ref.SessionDurationSeconds
					childEx.Notes = ref.Notes
					childEx.Sets = ref.Sets
					childEx.Reps = ref.Reps
					childEx.RepSeconds = ref.RepSeconds
					childEx.RepRestSeconds = ref.RepRestSeconds
					childEx.SetRestSeconds = ref.SetRestSeconds
					childEx.PrepSeconds = ref.PrepSeconds
					childEx.RungSeconds = ref.RungSeconds
					childEx.WeightKg = ref.WeightKg
				} else {
					childEx.Name = child.Name
					childEx.SessionDurationSeconds = child.SessionDurationSeconds
					childEx.Notes = child.Notes
					childEx.Sets = child.Sets
					childEx.Reps = child.Reps
					childEx.RepSeconds = child.RepSeconds
					childEx.RepRestSeconds = child.RepRestSeconds
					childEx.SetRestSeconds = child.SetRestSeconds
					childEx.PrepSeconds = child.PrepSeconds
					childEx.RungSeconds = child.RungSeconds
					childEx.WeightKg = child.WeightKg
				}
				if err := tx.Create(&childEx).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func upsertLibraryExercise(tx *gorm.DB, ownerID uint, ex yamlExercise) error {
	var row LibraryExercise
	res := tx.Where("owner_id = ? AND name = ?", ownerID, ex.Name).Limit(1).Find(&row)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		row = LibraryExercise{
			OwnerID: ownerID,
			Name:    ex.Name,
		}
	}
	row.Label = strings.TrimSpace(ex.Label)
	row.Source = strings.TrimSpace(ex.Source)
	row.ManagedByCatalog = true
	row.Kind = ex.Kind
	row.SessionDurationSeconds = ex.SessionDurationSeconds
	row.Notes = ex.Notes
	row.Sets = ex.Sets
	row.Reps = ex.Reps
	row.RepSeconds = ex.RepSeconds
	row.RepRestSeconds = ex.RepRestSeconds
	row.SetRestSeconds = ex.SetRestSeconds
	row.PrepSeconds = ex.PrepSeconds
	row.RungSeconds = ex.RungSeconds
	row.WeightKg = ex.WeightKg
	if err := tx.Save(&row).Error; err != nil {
		return err
	}

	// Replace media rows.
	if err := tx.Where("library_exercise_id = ?", row.ID).Delete(&ExerciseMedia{}).Error; err != nil {
		return err
	}
	return createExerciseMedia(tx, ownerID, nil, &row.ID, ex.Media)
}

func upsertSessionTemplate(tx *gorm.DB, ownerID uint, tpl yamlSessionTemplate, byName map[string]yamlExercise, byActivityTemplate map[string]yamlActivityTemplate) error {
	var template SessionTemplate
	res := tx.Where("owner_id = ? AND name = ?", ownerID, tpl.Name).Limit(1).Find(&template)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		template = SessionTemplate{
			OwnerID: ownerID,
			Name:    tpl.Name,
		}
	}
	template.Color = strings.TrimSpace(tpl.Color)
	template.Label = strings.TrimSpace(tpl.Label)
	template.Source = strings.TrimSpace(tpl.Source)
	template.Needs = strings.TrimSpace(tpl.Needs)
	template.ManagedByCatalog = true
	if err := tx.Save(&template).Error; err != nil {
		return err
	}

	if err := tx.Where("owner_id = ? AND session_template_id = ?", ownerID, template.ID).Delete(&Activity{}).Error; err != nil {
		return err
	}

	for aIdx, a := range tpl.Activities {
		// Activity-template ref: copy type + exercises from the named activity template.
		if a.Ref != "" {
			atpl, ok := byActivityTemplate[a.Ref]
			if !ok {
				return fmt.Errorf("session template %q references unknown activity template %q", tpl.Name, a.Ref)
			}
			activityName := a.Name
			if activityName == "" {
				activityName = atpl.Name
			}
			activity := Activity{
				OwnerID:           ownerID,
				SessionTemplateID: template.ID,
				Type:              a.Type,
				Name:              activityName,
				OrderIndex:        aIdx,
			}
			if err := tx.Create(&activity).Error; err != nil {
				return err
			}
			for eIdx, ex := range atpl.Exercises {
				actID := activity.ID
				exercise := Exercise{
					OwnerID:    ownerID,
					ActivityID: &actID,
					OrderIndex: eIdx,
				}
				var media []yamlMediaItem
				if ex.Ref != "" {
					ref, ok := byName[ex.Ref]
					if !ok {
						return fmt.Errorf("activity template %q references unknown exercise %q", a.Ref, ex.Ref)
					}
					exercise.Name = ref.Name
					exercise.Kind = ref.Kind
					exercise.SessionDurationSeconds = ref.SessionDurationSeconds
					exercise.Notes = ref.Notes
					exercise.Sets = ref.Sets
					exercise.Reps = ref.Reps
					exercise.RepSeconds = ref.RepSeconds
					exercise.RepRestSeconds = ref.RepRestSeconds
					exercise.SetRestSeconds = ref.SetRestSeconds
					exercise.PrepSeconds = ref.PrepSeconds
					exercise.RungSeconds = ref.RungSeconds
					exercise.WeightKg = ref.WeightKg
					media = ref.Media
				} else {
					exercise.Name = ex.Name
					exercise.Kind = ex.Kind
					exercise.SessionDurationSeconds = ex.SessionDurationSeconds
					exercise.Notes = ex.Notes
					exercise.Sets = ex.Sets
					exercise.Reps = ex.Reps
					exercise.RepSeconds = ex.RepSeconds
					exercise.RepRestSeconds = ex.RepRestSeconds
					exercise.SetRestSeconds = ex.SetRestSeconds
					exercise.PrepSeconds = ex.PrepSeconds
					exercise.RungSeconds = ex.RungSeconds
					exercise.WeightKg = ex.WeightKg
					media = ex.Media
				}
				if err := tx.Create(&exercise).Error; err != nil {
					return err
				}
				if err := createExerciseMedia(tx, ownerID, &exercise.ID, nil, media); err != nil {
					return err
				}
				if exercise.Kind == "exercise_catalog" && len(ex.Children) > 0 {
					for cIdx, child := range ex.Children {
						pid := exercise.ID
						childEx := Exercise{
							OwnerID:          ownerID,
							ActivityID:       &actID,
							OrderIndex:       eIdx*1000 + cIdx + 1,
							ParentExerciseID: &pid,
						}
						var childMedia []yamlMediaItem
						if child.Ref != "" {
							ref, ok := byName[child.Ref]
							if !ok {
								return fmt.Errorf("activity template %q exercise_catalog %q: unknown child %q", a.Ref, exercise.Name, child.Ref)
							}
							childEx.Name = ref.Name
							childEx.Kind = NormalizeKind(ref.Kind)
							childEx.SessionDurationSeconds = ref.SessionDurationSeconds
							childEx.Notes = ref.Notes
							childEx.Sets = ref.Sets
							childEx.Reps = ref.Reps
							childEx.RepSeconds = ref.RepSeconds
							childEx.RepRestSeconds = ref.RepRestSeconds
							childEx.SetRestSeconds = ref.SetRestSeconds
							childEx.PrepSeconds = ref.PrepSeconds
							childEx.RungSeconds = ref.RungSeconds
							childEx.WeightKg = ref.WeightKg
							childMedia = ref.Media
						} else {
							childEx.Name = child.Name
							childEx.Kind = NormalizeKind(child.Kind)
							childEx.SessionDurationSeconds = child.SessionDurationSeconds
							childEx.Notes = child.Notes
							childEx.Sets = child.Sets
							childEx.Reps = child.Reps
							childEx.RepSeconds = child.RepSeconds
							childEx.RepRestSeconds = child.RepRestSeconds
							childEx.SetRestSeconds = child.SetRestSeconds
							childEx.PrepSeconds = child.PrepSeconds
							childEx.RungSeconds = child.RungSeconds
							childEx.WeightKg = child.WeightKg
							childMedia = child.Media
						}
						if err := tx.Create(&childEx).Error; err != nil {
							return err
						}
						if err := createExerciseMedia(tx, ownerID, &childEx.ID, nil, childMedia); err != nil {
							return err
						}
					}
				}
			}
			continue
		}

		activity := Activity{
			OwnerID:           ownerID,
			SessionTemplateID: template.ID,
			Type:              a.Type,
			Name:              a.Name,
			OrderIndex:        aIdx,
		}
		if err := tx.Create(&activity).Error; err != nil {
			return err
		}
		for eIdx, ex := range a.Exercises {
			actID := activity.ID
			exercise := Exercise{
				OwnerID:    ownerID,
				ActivityID: &actID,
				OrderIndex: eIdx,
			}
			var media []yamlMediaItem
			if ex.Ref != "" {
				ref, ok := byName[ex.Ref]
				if !ok {
					return fmt.Errorf("template %q activity %q references unknown exercise %q", tpl.Name, a.Type, ex.Ref)
				}
				exercise.Name = ref.Name
				exercise.Kind = ref.Kind
				exercise.SessionDurationSeconds = ref.SessionDurationSeconds
				exercise.Notes = ref.Notes
				exercise.Sets = ref.Sets
				exercise.Reps = ref.Reps
				exercise.RepSeconds = ref.RepSeconds
				exercise.RepRestSeconds = ref.RepRestSeconds
				exercise.SetRestSeconds = ref.SetRestSeconds
				exercise.PrepSeconds = ref.PrepSeconds
				exercise.RungSeconds = ref.RungSeconds
				exercise.WeightKg = ref.WeightKg
				media = ref.Media
			} else {
				exercise.Name = ex.Name
				exercise.Kind = ex.Kind
				exercise.SessionDurationSeconds = ex.SessionDurationSeconds
				exercise.Notes = ex.Notes
				exercise.Sets = ex.Sets
				exercise.Reps = ex.Reps
				exercise.RepSeconds = ex.RepSeconds
				exercise.RepRestSeconds = ex.RepRestSeconds
				exercise.SetRestSeconds = ex.SetRestSeconds
				exercise.PrepSeconds = ex.PrepSeconds
				exercise.RungSeconds = ex.RungSeconds
				exercise.WeightKg = ex.WeightKg
				media = ex.Media
			}
			if err := tx.Create(&exercise).Error; err != nil {
				return err
			}
			if err := createExerciseMedia(tx, ownerID, &exercise.ID, nil, media); err != nil {
				return err
			}
			// Create children for exercise_catalog exercises.
			if exercise.Kind == "exercise_catalog" && len(ex.Children) > 0 {
				for cIdx, child := range ex.Children {
					var childName, childNotes, childRungSeconds string
					var childKind string
					var childSessionDur, childSets, childReps, childRepSec, childRepRest, childSetRest, childPrepSec int
					var childWeight float64
					var childMedia []yamlMediaItem

					if child.Ref != "" {
						ref, ok := byName[child.Ref]
						if !ok {
							return fmt.Errorf("template %q exercise_catalog %q references unknown child exercise %q", tpl.Name, exercise.Name, child.Ref)
						}
						childName = ref.Name
						childKind = NormalizeKind(ref.Kind)
						childSessionDur = ref.SessionDurationSeconds
						childNotes = ref.Notes
						childSets = ref.Sets
						childReps = ref.Reps
						childRepSec = ref.RepSeconds
						childRepRest = ref.RepRestSeconds
						childSetRest = ref.SetRestSeconds
						childPrepSec = ref.PrepSeconds
						childRungSeconds = ref.RungSeconds
						childWeight = ref.WeightKg
						childMedia = ref.Media
					} else {
						childName = child.Name
						childKind = NormalizeKind(child.Kind)
						childSessionDur = child.SessionDurationSeconds
						childNotes = child.Notes
						childSets = child.Sets
						childReps = child.Reps
						childRepSec = child.RepSeconds
						childRepRest = child.RepRestSeconds
						childSetRest = child.SetRestSeconds
						childPrepSec = child.PrepSeconds
						childRungSeconds = child.RungSeconds
						childWeight = child.WeightKg
						childMedia = child.Media
					}

					if childKind == "exercise_catalog" {
						return fmt.Errorf("template %q: nested exercise_catalog not allowed (child %q)", tpl.Name, childName)
					}
					pid := exercise.ID
					childExercise := Exercise{
						OwnerID:                ownerID,
						ActivityID:             &actID,
						OrderIndex:             eIdx*1000 + cIdx + 1,
						Name:                   childName,
						Kind:                   childKind,
						SessionDurationSeconds: childSessionDur,
						Notes:                  childNotes,
						Sets:                   childSets,
						Reps:                   childReps,
						RepSeconds:             childRepSec,
						RepRestSeconds:         childRepRest,
						SetRestSeconds:         childSetRest,
						PrepSeconds:            childPrepSec,
						RungSeconds:            childRungSeconds,
						WeightKg:               childWeight,
						ParentExerciseID:       &pid,
					}
					if err := tx.Create(&childExercise).Error; err != nil {
						return err
					}
					if err := createExerciseMedia(tx, ownerID, &childExercise.ID, nil, childMedia); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}
