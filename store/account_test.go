package store

import (
	"context"
	"errors"
	"testing"
)

func TestSignupAndAuthenticate(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()

		a, err := s.CreateAccount(ctx, "  Awalvie@Example.COM ", "correct horse", "Europe/Berlin")
		if err != nil {
			t.Fatal(err)
		}
		if a.Email != "awalvie@example.com" {
			t.Errorf("email was not folded and trimmed: %q", a.Email)
		}
		if a.TimeZone != "Europe/Berlin" {
			t.Errorf("time zone not stored: %q", a.TimeZone)
		}

		// Case-folded on the way in, so the same address cannot be registered twice with
		// different capitals. Left to the collation this would differ per engine.
		if _, err := s.CreateAccount(ctx, "AWALVIE@example.com", "another one", ""); !errors.Is(err, ErrEmailTaken) {
			t.Errorf("a duplicate email gave %v, want ErrEmailTaken", err)
		}

		if _, err := s.Authenticate(ctx, "AWALVIE@EXAMPLE.COM", "correct horse"); err != nil {
			t.Errorf("could not log in with a differently-cased email: %v", err)
		}
		if _, err := s.Authenticate(ctx, "awalvie@example.com", "wrong"); !errors.Is(err, ErrBadCredentials) {
			t.Errorf("wrong password gave %v, want ErrBadCredentials", err)
		}
		// A missing account and a wrong password must be indistinguishable.
		if _, err := s.Authenticate(ctx, "nobody@example.com", "whatever"); !errors.Is(err, ErrBadCredentials) {
			t.Errorf("unknown email gave %v, want ErrBadCredentials", err)
		}

		if _, err := s.CreateAccount(ctx, "short@example.com", "seven77", ""); err == nil {
			t.Error("a 7-character password was accepted")
		}
	})
}

// Changing a password must end sessions issued before it. Tokens are stateless, so the
// epoch is the only mechanism that can do it.
func TestChangePasswordBumpsTokenEpoch(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		a, err := s.CreateAccount(ctx, "a@b.c", "first password", "")
		if err != nil {
			t.Fatal(err)
		}
		if a.TokenEpoch != 0 {
			t.Fatalf("a new account started at epoch %d, want 0", a.TokenEpoch)
		}

		if err := s.ChangePassword(ctx, a.ID, "wrong", "second password"); !errors.Is(err, ErrBadCredentials) {
			t.Errorf("changing with the wrong current password gave %v", err)
		}
		if err := s.ChangePassword(ctx, a.ID, "first password", "second password"); err != nil {
			t.Fatal(err)
		}

		got, err := s.AccountByID(ctx, a.ID)
		if err != nil {
			t.Fatal(err)
		}
		if got.TokenEpoch != 1 {
			t.Errorf("epoch is %d after a password change, want 1", got.TokenEpoch)
		}
		if _, err := s.Authenticate(ctx, "a@b.c", "second password"); err != nil {
			t.Errorf("the new password does not work: %v", err)
		}
		if _, err := s.Authenticate(ctx, "a@b.c", "first password"); !errors.Is(err, ErrBadCredentials) {
			t.Error("the old password still works")
		}
	})
}

func TestUpdateProfileLeavesUnsetFieldsAlone(t *testing.T) {
	eachEngine(t, func(t *testing.T, s *Store) {
		ctx := context.Background()
		a, err := s.CreateAccount(ctx, "a@b.c", "a password", "")
		if err != nil {
			t.Fatal(err)
		}

		h, hang := 178, 42.5
		if err := s.UpdateProfile(ctx, a.ID, ProfileUpdate{HeightCm: &h, MaxHangKg: &hang}); err != nil {
			t.Fatal(err)
		}
		got, _ := s.AccountByID(ctx, a.ID)
		if got.HeightCm == nil || *got.HeightCm != 178 {
			t.Errorf("height not stored: %v", got.HeightCm)
		}
		if got.MaxHangKg == nil || *got.MaxHangKg != 42.5 {
			t.Errorf("max hang not stored: %v", got.MaxHangKg)
		}

		// Only the ape index this time. Height must survive, which is what pointers buy.
		ape := -2
		if err := s.UpdateProfile(ctx, a.ID, ProfileUpdate{ApeIndexCm: &ape}); err != nil {
			t.Fatal(err)
		}
		got, _ = s.AccountByID(ctx, a.ID)
		if got.HeightCm == nil || *got.HeightCm != 178 {
			t.Errorf("height was cleared by an unrelated update: %v", got.HeightCm)
		}
		if got.ApeIndexCm == nil || *got.ApeIndexCm != -2 {
			t.Errorf("a negative ape index did not store: %v", got.ApeIndexCm)
		}

		if err := s.UpdateProfile(ctx, a.ID, ProfileUpdate{TimeZone: "Mars/Olympus"}); err == nil {
			t.Error("an unknown time zone was accepted")
		}
	})
}
