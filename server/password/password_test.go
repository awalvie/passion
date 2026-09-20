package password

import (
	"errors"
	"strings"
	"testing"
)

func TestHashAndVerify(t *testing.T) {
	encoded, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if err := Verify(encoded, "correct horse battery"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

func TestVerifyRejectsTheWrongPassword(t *testing.T) {
	encoded, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if err := Verify(encoded, "correct horse batterz"); !errors.Is(err, ErrMismatch) {
		t.Fatalf("got %v, want ErrMismatch", err)
	}
}

func TestHashIsSaltedPerCall(t *testing.T) {
	a, _ := Hash("correct horse battery")
	b, _ := Hash("correct horse battery")
	if a == b {
		t.Fatal("two hashes of one password are identical, so the salt is not random")
	}
}

func TestHashLooksLikeAPHCString(t *testing.T) {
	encoded, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if !strings.HasPrefix(encoded, "$argon2id$v=19$m=65536,t=3,p=4$") {
		t.Fatalf("unexpected shape: %s", encoded)
	}
	if strings.Contains(encoded, "correct horse battery") {
		t.Fatal("the hash contains the password")
	}
}

func TestLengthLimits(t *testing.T) {
	if _, err := Hash("short"); !errors.Is(err, ErrTooShort) {
		t.Fatalf("got %v, want ErrTooShort", err)
	}
	if _, err := Hash(strings.Repeat("a", MaxLength+1)); !errors.Is(err, ErrTooLong) {
		t.Fatalf("got %v, want ErrTooLong", err)
	}
	if _, err := Hash(strings.Repeat("a", MaxLength)); err != nil {
		t.Fatalf("a password at the limit was refused: %v", err)
	}
}

// Every one of these must fail. A parser that accepts any of them is an
// authentication bypass.
func TestVerifyRefusesMalformedHashes(t *testing.T) {
	good, err := Hash("correct horse battery")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	parts := strings.Split(good, "$")

	cases := map[string]string{
		"empty":             "",
		"not a phc string":  "hunter2",
		"no leading dollar": strings.TrimPrefix(good, "$"),
		"too few fields":    "$argon2id$v=19$m=65536,t=3,p=4$" + parts[4],
		"wrong algorithm":   strings.Replace(good, "argon2id", "argon2i", 1),
		"wrong version":     strings.Replace(good, "v=19", "v=18", 1),
		"no parameters":     strings.Replace(good, "m=65536,t=3,p=4", "", 1),
		"zero memory":       strings.Replace(good, "m=65536", "m=0", 1),
		"zero time":         strings.Replace(good, "t=3", "t=0", 1),
		"zero threads":      strings.Replace(good, "p=4", "p=0", 1),
		"salt not base64":   strings.Replace(good, parts[4], "!!!!", 1),
		"key not base64":    strings.Replace(good, parts[5], "!!!!", 1),
		"empty key":         strings.Replace(good, parts[5], "", 1),
	}

	for name, encoded := range cases {
		t.Run(name, func(t *testing.T) {
			if err := Verify(encoded, "correct horse battery"); err == nil {
				t.Fatal("accepted a malformed hash")
			}
		})
	}
}

// Verify must read the parameters out of the stored hash, not assume the ones
// the app writes today. Otherwise raising them locks everybody out.
func TestVerifyUsesTheStoredParameters(t *testing.T) {
	old := params{memory: 32 * 1024, time: 2, threads: 2, saltLen: 16, keyLen: 32}
	salt := make([]byte, old.saltLen)
	encoded := encode(old, salt, derive(old, "correct horse battery", salt))

	if err := Verify(encoded, "correct horse battery"); err != nil {
		t.Fatalf("verify: %v", err)
	}
}
