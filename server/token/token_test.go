package token

import (
	"bytes"
	"crypto/sha256"
	"strings"
	"testing"
)

func TestNewIsDifferentEveryTime(t *testing.T) {
	seen := make(map[string]bool, 100)
	for range 100 {
		raw, _, err := New()
		if err != nil {
			t.Fatalf("new: %v", err)
		}
		if seen[raw] {
			t.Fatal("the same token came back twice")
		}
		seen[raw] = true
	}
}

func TestHashMatchesTheToken(t *testing.T) {
	raw, hash, err := New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if !bytes.Equal(hash, Hash(raw)) {
		t.Fatal("the returned hash does not match hashing the token again")
	}
}

func TestHashIsNotTheToken(t *testing.T) {
	raw, hash, err := New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if strings.Contains(string(hash), raw) {
		t.Fatal("the stored hash contains the token")
	}
	if len(hash) != sha256.Size {
		t.Fatalf("hash is %d bytes, want %d", len(hash), sha256.Size)
	}
}

func TestTokenIsUrlSafe(t *testing.T) {
	raw, _, err := New()
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// It travels in an Authorization header, so it must carry no padding and
	// nothing that needs escaping.
	if strings.ContainsAny(raw, "+/= ") {
		t.Fatalf("token is not url safe: %q", raw)
	}
}
