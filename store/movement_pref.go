package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

// SetMovementPref records a person's own numbers for one movement. A pref holding no numbers
// at all is deleted, so "back to the default" is the same call with everything cleared.
func (s *Store) SetMovementPref(ctx context.Context, accountID, movementID int64, d Dose) error {
	return s.WithTx(ctx, func(tx *gorm.DB) error {
		var m Content
		if err := visible(tx.Where("id = ? AND kind = ?", movementID, KindMovement), accountID).
			First(&m).Error; err != nil {
			return err
		}

		if emptyDose(d) {
			return tx.Where("account_id = ? AND movement_id = ?", accountID, movementID).
				Delete(&MovementPref{}).Error
		}

		p := MovementPref{
			AccountID: accountID, MovementID: movementID, MovementKind: KindMovement,
			Sets: d.Sets, Reps: d.Reps, WeightKg: d.WeightKg,
			RepSeconds: d.RepSeconds, RepRestSeconds: d.RepRestSeconds,
			SetRestSeconds: d.SetRestSeconds, PrepSeconds: d.PrepSeconds, Seconds: d.Seconds,
			UpdatedAt: time.Now(),
		}

		var have MovementPref
		err := tx.Where("account_id = ? AND movement_id = ?", accountID, movementID).
			First(&have).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			p.CreatedAt = p.UpdatedAt
			return tx.Create(&p).Error
		}
		if err != nil {
			return err
		}
		return tx.Model(&MovementPref{}).
			Where("account_id = ? AND movement_id = ?", accountID, movementID).
			Select("sets", "reps", "weight_kg", "rep_seconds", "rep_rest_seconds",
				"set_rest_seconds", "prep_seconds", "seconds", "updated_at").
			Updates(&p).Error
	})
}

// MovementPrefs returns an account's prefs keyed by movement id, so a tree read resolves
// without a query per movement.
func (s *Store) MovementPrefs(ctx context.Context, accountID int64) (map[int64]MovementPref, error) {
	var rows []MovementPref
	if err := s.read(ctx).Where("account_id = ?", accountID).Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[int64]MovementPref, len(rows))
	for _, r := range rows {
		out[r.MovementID] = r
	}
	return out, nil
}

// ResolveDose is the one place the order is written down: the movement's own default, then
// the person's pref, then the slot. What the runner types beats all three and never reaches
// here.
//
// Field by field. A pref that sets only weight_kg must not wipe the movement's sets.
func ResolveDose(m Content, pref *MovementPref, slot *Dose) Dose {
	d := Dose{
		Sets: m.DSets, Reps: m.DReps, WeightKg: m.DWeightKg,
		RepSeconds: m.DRepSeconds, RepRestSeconds: m.DRepRestSeconds,
		SetRestSeconds: m.DSetRestSeconds, PrepSeconds: m.DPrepSeconds, Seconds: m.DSeconds,
	}
	if pref != nil {
		over(&d.Sets, pref.Sets)
		over(&d.Reps, pref.Reps)
		over(&d.WeightKg, pref.WeightKg)
		over(&d.RepSeconds, pref.RepSeconds)
		over(&d.RepRestSeconds, pref.RepRestSeconds)
		over(&d.SetRestSeconds, pref.SetRestSeconds)
		over(&d.PrepSeconds, pref.PrepSeconds)
		over(&d.Seconds, pref.Seconds)
	}
	if slot != nil {
		over(&d.Sets, slot.Sets)
		over(&d.Reps, slot.Reps)
		over(&d.WeightKg, slot.WeightKg)
		over(&d.RepSeconds, slot.RepSeconds)
		over(&d.RepRestSeconds, slot.RepRestSeconds)
		over(&d.SetRestSeconds, slot.SetRestSeconds)
		over(&d.PrepSeconds, slot.PrepSeconds)
		over(&d.Seconds, slot.Seconds)
		if len(slot.PerSet) > 0 {
			d.PerSet = slot.PerSet
			d.Reps = nil
		}
	}
	return d
}

func over[T any](dst **T, v *T) {
	if v != nil {
		*dst = v
	}
}

func emptyDose(d Dose) bool {
	return d.Sets == nil && d.Reps == nil && d.WeightKg == nil && d.RepSeconds == nil &&
		d.RepRestSeconds == nil && d.SetRestSeconds == nil && d.PrepSeconds == nil &&
		d.Seconds == nil
}
