package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrEmailTaken is returned rather than the driver's duplicate-key error, so the
	// handler does not have to know which column collided.
	ErrEmailTaken = errors.New("store: that email already has an account")
	// ErrBadCredentials covers both a missing account and a wrong password. One error for
	// both, so the response cannot be used to discover which emails exist.
	ErrBadCredentials = errors.New("store: email or password is wrong")
)

// normalizeEmail folds case and trims. Done here and not in the database, whose collations
// disagree about case between engines.
func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// CreateAccount registers an account and returns it. Signup is open; there is no invite.
func (s *Store) CreateAccount(ctx context.Context, email, password, timeZone string) (Account, error) {
	email = normalizeEmail(email)
	if email == "" {
		return Account{}, fmt.Errorf("store: email must not be empty")
	}
	if len(password) < 8 {
		return Account{}, fmt.Errorf("store: password must be at least 8 characters")
	}
	if timeZone == "" {
		timeZone = "UTC"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return Account{}, fmt.Errorf("store: hashing password: %w", err)
	}

	a := Account{
		Email:        email,
		PasswordHash: string(hash),
		TimeZone:     timeZone,
	}
	if err := s.read(ctx).Create(&a).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return Account{}, ErrEmailTaken
		}
		return Account{}, err
	}
	return a, nil
}

// Authenticate checks an email and password. It returns ErrBadCredentials for both a
// missing account and a wrong password, and compares a hash either way so the response
// time does not reveal which emails exist.
func (s *Store) Authenticate(ctx context.Context, email, password string) (Account, error) {
	var a Account
	err := s.read(ctx).Where("email = ?", normalizeEmail(email)).First(&a).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Compare against a throwaway hash so a missing account costs the same as a wrong
		// password.
		_ = bcrypt.CompareHashAndPassword(
			[]byte("$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"),
			[]byte(password))
		return Account{}, ErrBadCredentials
	}
	if err != nil {
		return Account{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(password)) != nil {
		return Account{}, ErrBadCredentials
	}
	return a, nil
}

// AccountByID is what the auth middleware calls on every request. It returns the account
// so the caller can compare TokenEpoch against the token's claim.
func (s *Store) AccountByID(ctx context.Context, id int64) (Account, error) {
	var a Account
	if err := s.read(ctx).First(&a, id).Error; err != nil {
		return Account{}, err
	}
	return a, nil
}

// ChangePassword replaces the hash and bumps TokenEpoch in one transaction. Tokens are
// stateless, so the bump is the only thing that revokes a cookie issued before the change.
func (s *Store) ChangePassword(ctx context.Context, id int64, current, next string) error {
	if len(next) < 8 {
		return fmt.Errorf("store: password must be at least 8 characters")
	}
	return s.WithTx(ctx, func(tx *gorm.DB) error {
		var a Account
		if err := tx.First(&a, id).Error; err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(a.PasswordHash), []byte(current)) != nil {
			return ErrBadCredentials
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("store: hashing password: %w", err)
		}
		return tx.Model(&Account{}).Where("id = ?", id).Updates(map[string]any{
			"password_hash": string(hash),
			"token_epoch":   a.TokenEpoch + 1,
			"updated_at":    time.Now(),
		}).Error
	})
}

// UpdateProfile changes the fields the profile page owns. Nil leaves a value alone, which
// is why the numbers are pointers: zero is a real height and a real max hang.
func (s *Store) UpdateProfile(ctx context.Context, id int64, p ProfileUpdate) error {
	fields := map[string]any{"updated_at": time.Now()}
	if p.HeightCm != nil {
		fields["height_cm"] = *p.HeightCm
	}
	if p.ApeIndexCm != nil {
		fields["ape_index_cm"] = *p.ApeIndexCm
	}
	if p.MaxPullUps != nil {
		fields["max_pull_ups"] = *p.MaxPullUps
	}
	if p.MaxHangKg != nil {
		fields["max_hang_kg"] = *p.MaxHangKg
	}
	if p.BoulderGradeSystem != "" {
		fields["boulder_grade_system"] = p.BoulderGradeSystem
	}
	if p.RouteGradeSystem != "" {
		fields["route_grade_system"] = p.RouteGradeSystem
	}
	if p.TimeZone != "" {
		if _, err := time.LoadLocation(p.TimeZone); err != nil {
			return fmt.Errorf("store: %q is not a known time zone", p.TimeZone)
		}
		fields["time_zone"] = p.TimeZone
	}
	return s.read(ctx).Model(&Account{}).Where("id = ?", id).Updates(fields).Error
}

type ProfileUpdate struct {
	HeightCm           *int
	ApeIndexCm         *int
	MaxPullUps         *int
	MaxHangKg          *float64
	BoulderGradeSystem string
	RouteGradeSystem   string
	TimeZone           string
}

// DeleteAccount removes an account and everything it owns, by cascade. Shipped content has
// no author, so there is no row for the cascade to follow.
func (s *Store) DeleteAccount(ctx context.Context, id int64) error {
	return s.read(ctx).Delete(&Account{}, id).Error
}

// AccountCount exists so the first-run path can tell an empty install from a populated one.
func (s *Store) AccountCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.read(ctx).Model(&Account{}).Count(&n).Error
	return n, err
}

// FirstAccount resolves the lowest account id, for the dev auth bypass. Id 1 may never have
// existed, or may have been deleted since.
func (s *Store) FirstAccount(ctx context.Context) (Account, error) {
	var a Account
	if err := s.read(ctx).Order("id").First(&a).Error; err != nil {
		return Account{}, err
	}
	return a, nil
}
