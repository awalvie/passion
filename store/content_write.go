package store

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// ContentUpdate is what a person may change about a row. Nil leaves a field alone, so a form
// that shows three fields cannot blank the rest.
type ContentUpdate struct {
	Name   *string
	Notes  *string
	Source *string
	Color  *string
	Needs  *string

	Style   *string
	PerSide *bool
	Dose    *Dose
}

// UpdateContent writes a row the account owns. Writing detaches the row from its file in the
// same statement: the file would otherwise put the old value back on the next boot.
func (s *Store) UpdateContent(ctx context.Context, accountID, id int64, u ContentUpdate) (Content, error) {
	var out Content
	err := s.WithTx(ctx, func(tx *gorm.DB) error {
		var c Content
		if err := visible(tx.Where("id = ?", id), accountID).First(&c).Error; err != nil {
			return err
		}
		if !c.Editable() {
			return ErrShipped
		}

		fields := map[string]any{"updated_at": time.Now(), "source_tree": nil}
		set(fields, "name", u.Name)
		set(fields, "notes", u.Notes)
		set(fields, "source", u.Source)
		set(fields, "color", u.Color)
		set(fields, "needs", u.Needs)
		set(fields, "movement_style", u.Style)
		set(fields, "per_side", u.PerSide)
		if u.Dose != nil {
			fields["d_sets"] = u.Dose.Sets
			fields["d_reps"] = u.Dose.Reps
			fields["d_weight_kg"] = u.Dose.WeightKg
			fields["d_rep_seconds"] = u.Dose.RepSeconds
			fields["d_rep_rest_seconds"] = u.Dose.RepRestSeconds
			fields["d_set_rest_seconds"] = u.Dose.SetRestSeconds
			fields["d_prep_seconds"] = u.Dose.PrepSeconds
			fields["d_seconds"] = u.Dose.Seconds
		}

		if err := tx.Model(&Content{}).Where("id = ?", id).Updates(fields).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", id).First(&out).Error
	})
	return out, err
}

func set[T any](fields map[string]any, column string, v *T) {
	if v != nil {
		fields[column] = *v
	}
}
