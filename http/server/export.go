package web

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
	"gorm.io/gorm"

	"passion/db"
)

// slugifyName converts a display name into a safe filename stem.
var nonAlphanumRe = regexp.MustCompile(`[^a-z0-9]+`)

func slugifyName(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = nonAlphanumRe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		return "export"
	}
	return s
}

func exportMedia(media []db.ExerciseMedia) []exportMediaItem {
	if len(media) == 0 {
		return nil
	}
	out := make([]exportMediaItem, 0, len(media))
	for _, m := range media {
		if m.VideoURL == "" && m.ThumbnailURL == "" {
			continue
		}
		out = append(out, exportMediaItem{
			VideoURL:     m.VideoURL,
			ThumbnailURL: m.ThumbnailURL,
		})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ── Library exercise export ──────────────────────────────────────────────────

type exportMediaItem struct {
	VideoURL     string `yaml:"video_url,omitempty"`
	ThumbnailURL string `yaml:"thumbnail_url,omitempty"`
}

type exportExercise struct {
	Name                   string            `yaml:"name"`
	Kind                   string            `yaml:"kind,omitempty"`
	SessionDurationSeconds int               `yaml:"session_duration_seconds,omitempty"`
	Media                  []exportMediaItem `yaml:"media,omitempty"`
	Notes                  string            `yaml:"notes,omitempty"`
	Sets                   int               `yaml:"sets,omitempty"`
	Reps                   int               `yaml:"reps,omitempty"`
	RepSeconds             int               `yaml:"rep_seconds,omitempty"`
	RepRestSeconds         int               `yaml:"rep_rest_seconds,omitempty"`
	SetRestSeconds         int               `yaml:"set_rest_seconds,omitempty"`
	PrepSeconds            int               `yaml:"prep_seconds,omitempty"`
	RungSeconds            string            `yaml:"rung_seconds,omitempty"`
	WeightKg               float64           `yaml:"weight_kg,omitempty"`
}

func (s *Server) handleExportLibraryExercise(w http.ResponseWriter, r *http.Request, ownerID uint, id uint) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var row db.LibraryExercise
	if err := s.store.DB.Where("owner_id = ? AND id = ?", ownerID, id).Preload("Media").First(&row).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	out := exportExercise{
		Name:                   row.Name,
		Kind:                   row.Kind,
		SessionDurationSeconds: row.SessionDurationSeconds,
		Media:                  exportMedia(row.Media),
		Notes:                  row.Notes,
		Sets:                   row.Sets,
		Reps:                   row.Reps,
		RepSeconds:             row.RepSeconds,
		RepRestSeconds:         row.RepRestSeconds,
		SetRestSeconds:         row.SetRestSeconds,
		PrepSeconds:            row.PrepSeconds,
		RungSeconds:            row.RungSeconds,
		WeightKg:               row.WeightKg,
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := slugifyName(row.Name) + ".yaml"
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// handleExportLibraryExercisesBulk exports a selection of library exercises as
// a multi-document YAML file (one document per exercise, separated by ---).
// It accepts POST with form field ids[] containing selected exercise IDs.
func (s *Server) handleExportLibraryExercisesBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ownerID, ok := s.currentUserID(r)
	if !ok {
		s.unauthorizedRedirect(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	rawIDs := r.Form["ids[]"]
	if len(rawIDs) == 0 {
		http.Error(w, "no exercises selected", http.StatusBadRequest)
		return
	}
	ids := make([]uint, 0, len(rawIDs))
	for _, raw := range rawIDs {
		n, err := strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
		if err != nil {
			http.Error(w, "invalid id: "+raw, http.StatusBadRequest)
			return
		}
		ids = append(ids, uint(n))
	}

	var rows []db.LibraryExercise
	if err := s.store.DB.Where("owner_id = ? AND id IN ?", ownerID, ids).Preload("Media").Find(&rows).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var buf strings.Builder
	for _, row := range rows {
		out := exportExercise{
			Name:                   row.Name,
			Kind:                   row.Kind,
			SessionDurationSeconds: row.SessionDurationSeconds,
			Media:                  exportMedia(row.Media),
			Notes:                  row.Notes,
			Sets:                   row.Sets,
			Reps:                   row.Reps,
			RepSeconds:             row.RepSeconds,
			RepRestSeconds:         row.RepRestSeconds,
			SetRestSeconds:         row.SetRestSeconds,
			PrepSeconds:            row.PrepSeconds,
			RungSeconds:            row.RungSeconds,
			WeightKg:               row.WeightKg,
		}
		chunk, err := yaml.Marshal(out)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		buf.WriteString("---\n")
		buf.Write(chunk)
	}

	filename := fmt.Sprintf("exercises_%d.yaml", len(rows))
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, buf.String())
}

// ── Activity template export ─────────────────────────────────────────────────

type exportActivityTemplate struct {
	Name      string                   `yaml:"name"`
	Exercises []exportTemplateExercise `yaml:"exercises"`
}

func (s *Server) handleExportActivityTemplate(w http.ResponseWriter, r *http.Request, ownerID uint, templateID uint) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var at db.ActivityTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		Preload("Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Exercises.Media").
		First(&at).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	childrenByParent := map[uint][]db.Exercise{}
	for _, ex := range at.Exercises {
		if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
			childrenByParent[*ex.ParentExerciseID] = append(childrenByParent[*ex.ParentExerciseID], ex)
		}
	}

	out := exportActivityTemplate{Name: at.Name}
	for _, ex := range at.Exercises {
		if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
			continue
		}
		ete := exportTemplateExercise{
			Name:                   ex.Name,
			Kind:                   ex.Kind,
			SessionDurationSeconds: ex.SessionDurationSeconds,
			Media:                  exportMedia(ex.Media),
			Notes:                  ex.Notes,
			Sets:                   ex.Sets,
			Reps:                   ex.Reps,
			RepSeconds:             ex.RepSeconds,
			RepRestSeconds:         ex.RepRestSeconds,
			SetRestSeconds:         ex.SetRestSeconds,
			PrepSeconds:            ex.PrepSeconds,
			RungSeconds:            ex.RungSeconds,
			WeightKg:               ex.WeightKg,
		}
		if children, ok := childrenByParent[ex.ID]; ok {
			for _, ch := range children {
				ete.Children = append(ete.Children, exportTemplateExercise{
					Name:                   ch.Name,
					Kind:                   ch.Kind,
					SessionDurationSeconds: ch.SessionDurationSeconds,
					Media:                  exportMedia(ch.Media),
					Notes:                  ch.Notes,
					Sets:                   ch.Sets,
					Reps:                   ch.Reps,
					RepSeconds:             ch.RepSeconds,
					RepRestSeconds:         ch.RepRestSeconds,
					SetRestSeconds:         ch.SetRestSeconds,
					PrepSeconds:            ch.PrepSeconds,
					RungSeconds:            ch.RungSeconds,
					WeightKg:               ch.WeightKg,
				})
			}
		}
		out.Exercises = append(out.Exercises, ete)
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := slugifyName(at.Name) + ".yaml"
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// ── Session template export ──────────────────────────────────────────────────

type exportTemplateExercise struct {
	Name                   string                   `yaml:"name,omitempty"`
	Kind                   string                   `yaml:"kind,omitempty"`
	SessionDurationSeconds int                      `yaml:"session_duration_seconds,omitempty"`
	Media                  []exportMediaItem        `yaml:"media,omitempty"`
	Notes                  string                   `yaml:"notes,omitempty"`
	Sets                   int                      `yaml:"sets,omitempty"`
	Reps                   int                      `yaml:"reps,omitempty"`
	RepSeconds             int                      `yaml:"rep_seconds,omitempty"`
	RepRestSeconds         int                      `yaml:"rep_rest_seconds,omitempty"`
	SetRestSeconds         int                      `yaml:"set_rest_seconds,omitempty"`
	PrepSeconds            int                      `yaml:"prep_seconds,omitempty"`
	RungSeconds            string                   `yaml:"rung_seconds,omitempty"`
	WeightKg               float64                  `yaml:"weight_kg,omitempty"`
	Children               []exportTemplateExercise `yaml:"children,omitempty"`
}

type exportActivity struct {
	Type      string                   `yaml:"type"`
	Exercises []exportTemplateExercise `yaml:"exercises"`
}

type exportTemplate struct {
	Name       string           `yaml:"name"`
	Color      string           `yaml:"color,omitempty"`
	Activities []exportActivity `yaml:"activities"`
}

func (s *Server) handleExportTemplate(w http.ResponseWriter, r *http.Request, ownerID uint, templateID uint) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var tpl db.SessionTemplate
	if err := s.store.DB.
		Where("owner_id = ? AND id = ?", ownerID, templateID).
		Preload("Activities", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises", func(tx *gorm.DB) *gorm.DB {
			return tx.Where("owner_id = ?", ownerID).Order("order_index asc")
		}).
		Preload("Activities.Exercises.Media").
		First(&tpl).Error; err != nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	out := exportTemplate{
		Name:  tpl.Name,
		Color: tpl.Color,
	}
	for _, act := range tpl.Activities {
		// Build a map of parent ID → children for exercise_catalog nesting.
		childrenByParent := map[uint][]db.Exercise{}
		for _, ex := range act.Exercises {
			if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
				childrenByParent[*ex.ParentExerciseID] = append(childrenByParent[*ex.ParentExerciseID], ex)
			}
		}

		ea := exportActivity{Type: act.Type}
		for _, ex := range act.Exercises {
			// Skip child exercises at the top level; they're nested under their parent.
			if ex.ParentExerciseID != nil && *ex.ParentExerciseID != 0 {
				continue
			}
			ete := exportTemplateExercise{
				Name:                   ex.Name,
				Kind:                   ex.Kind,
				SessionDurationSeconds: ex.SessionDurationSeconds,
				Media:                  exportMedia(ex.Media),
				Notes:                  ex.Notes,
				Sets:                   ex.Sets,
				Reps:                   ex.Reps,
				RepSeconds:             ex.RepSeconds,
				RepRestSeconds:         ex.RepRestSeconds,
				SetRestSeconds:         ex.SetRestSeconds,
				PrepSeconds:            ex.PrepSeconds,
				RungSeconds:            ex.RungSeconds,
				WeightKg:               ex.WeightKg,
			}
			if children, ok := childrenByParent[ex.ID]; ok {
				for _, ch := range children {
					ete.Children = append(ete.Children, exportTemplateExercise{
						Name:                   ch.Name,
						Kind:                   ch.Kind,
						SessionDurationSeconds: ch.SessionDurationSeconds,
						Media:                  exportMedia(ch.Media),
						Notes:                  ch.Notes,
						Sets:                   ch.Sets,
						Reps:                   ch.Reps,
						RepSeconds:             ch.RepSeconds,
						RepRestSeconds:         ch.RepRestSeconds,
						SetRestSeconds:         ch.SetRestSeconds,
						PrepSeconds:            ch.PrepSeconds,
						RungSeconds:            ch.RungSeconds,
						WeightKg:               ch.WeightKg,
					})
				}
			}
			ea.Exercises = append(ea.Exercises, ete)
		}
		out.Activities = append(out.Activities, ea)
	}

	data, err := yaml.Marshal(out)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	filename := slugifyName(tpl.Name) + ".yaml"
	w.Header().Set("Content-Type", "application/x-yaml")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}
