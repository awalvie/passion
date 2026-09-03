package db

import (
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Errors returned by RedeemInviteCode. Handlers map these to a message for the visitor.
// They are deliberately distinct so the log can say what actually happened, while the
// signup page says the same thing for all of them — an attacker learns nothing about
// which codes exist.
var (
	ErrInviteRequired = errors.New("an invite code is required")
	ErrInviteUnknown  = errors.New("invite code not found")
	ErrInviteUsed     = errors.New("invite code already used")
	ErrInviteExpired  = errors.New("invite code expired")
)

// inviteAlphabet omits the characters people misread when copying a code by hand:
// 0/O, 1/I/L, and the vowels that let a random string spell something unfortunate.
const inviteAlphabet = "BCDFGHJKMNPQRSTVWXYZ23456789"

// inviteCodeLen is the number of characters in a generated code, before grouping.
// 28^12 is far beyond guessing at any signup rate this app will ever see.
const inviteCodeLen = 12

// NewInviteCodeString returns a fresh random code, grouped for reading aloud, e.g.
// "K7PM-3XQD-9RTB".
func NewInviteCodeString() (string, error) {
	buf := make([]byte, inviteCodeLen)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	out := make([]byte, 0, inviteCodeLen+2)
	for i, b := range buf {
		if i > 0 && i%4 == 0 {
			out = append(out, '-')
		}
		out = append(out, inviteAlphabet[int(b)%len(inviteAlphabet)])
	}
	return string(out), nil
}

// NormaliseInviteCode makes the lookup forgiving about how a code was typed. Case,
// surrounding space and the grouping dashes all carry no meaning.
func NormaliseInviteCode(s string) string {
	s = strings.ToUpper(strings.TrimSpace(s))
	return strings.ReplaceAll(s, "-", "")
}

// FormatInviteCode regroups a stored code for display, so a listing shows the same thing
// that was handed out rather than an unbroken run of characters.
func FormatInviteCode(s string) string {
	s = NormaliseInviteCode(s)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && i%4 == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// CreateInviteCode mints and stores one code. createdBy is nil when the command line
// minted it before any account existed. expiresAt nil means the code never expires.
func CreateInviteCode(gdb *gorm.DB, createdBy *uint, note string, expiresAt *time.Time) (InviteCode, error) {
	for attempt := 0; attempt < 5; attempt++ {
		raw, err := NewInviteCodeString()
		if err != nil {
			return InviteCode{}, err
		}
		row := InviteCode{
			Code:        NormaliseInviteCode(raw),
			CreatedByID: createdBy,
			Note:        strings.TrimSpace(note),
			ExpiresAt:   expiresAt,
		}
		err = gdb.Create(&row).Error
		if err == nil {
			// Hand back the grouped form. Only the normalised form is stored.
			row.Code = raw
			return row, nil
		}
		if !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return InviteCode{}, err
		}
	}
	return InviteCode{}, fmt.Errorf("could not generate an unused invite code after 5 attempts")
}

// SignupIsOpen reports whether signup should run without an invite code. It is true only
// while the instance has no accounts at all, so a fresh self-hosted install can create its
// first user. After that, every signup needs a code.
func SignupIsOpen(gdb *gorm.DB) (bool, error) {
	var n int64
	if err := gdb.Model(&User{}).Count(&n).Error; err != nil {
		return false, err
	}
	return n == 0, nil
}

// CheckInviteCode reports whether a code could be redeemed right now, without claiming
// it. Signup uses this before creating the account, so a bad code leaves no user row
// behind. It is not a reservation: RedeemInviteCode still has to win the race.
func CheckInviteCode(gdb *gorm.DB, code string, now time.Time) error {
	_, err := lookupUsableInvite(gdb, code, now)
	return err
}

// lookupUsableInvite finds a code and reports why it cannot be used, if it cannot.
func lookupUsableInvite(gdb *gorm.DB, code string, now time.Time) (InviteCode, error) {
	normalised := NormaliseInviteCode(code)
	if normalised == "" {
		return InviteCode{}, ErrInviteRequired
	}
	var row InviteCode
	err := gdb.Where("code = ?", normalised).First(&row).Error
	if isNotFound(err) {
		return InviteCode{}, ErrInviteUnknown
	}
	if err != nil {
		return InviteCode{}, err
	}
	if row.Redeemed() {
		return row, ErrInviteUsed
	}
	if row.Expired(now) {
		return row, ErrInviteExpired
	}
	return row, nil
}

// RedeemInviteCode claims a code for userID. It is safe against two people submitting the
// same code at once: the UPDATE matches only rows that are still unredeemed, so exactly
// one caller can win.
func RedeemInviteCode(gdb *gorm.DB, code string, userID uint, now time.Time) error {
	row, err := lookupUsableInvite(gdb, code, now)
	if err != nil {
		return err
	}

	res := gdb.Model(&InviteCode{}).
		Where("id = ? AND used_by_id IS NULL", row.ID).
		Updates(map[string]any{"used_by_id": userID, "used_at": now})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		// Someone else claimed it between the read and the write.
		return ErrInviteUsed
	}
	return nil
}

// ListInviteCodes returns every code, newest first, for the command-line listing.
func ListInviteCodes(gdb *gorm.DB) ([]InviteCode, error) {
	var rows []InviteCode
	err := gdb.Order("id desc").Find(&rows).Error
	return rows, err
}
