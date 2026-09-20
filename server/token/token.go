// Package token mints the bearer tokens that identify a signed-in device.
//
// Only the hash is ever stored, so a database dump does not hand over live
// sessions.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// Size is 32 bytes, which is 256 bits of entropy. The current standard's floor
// is 128.
const Size = 32

// New returns a fresh token and the hash to store. The token is returned once
// and never again.
func New() (raw string, hash []byte, err error) {
	b := make([]byte, Size)
	if _, err := rand.Read(b); err != nil {
		return "", nil, fmt.Errorf("read random bytes: %w", err)
	}

	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, Hash(raw), nil
}

// Hash is what goes in the database, and what a lookup compares against.
func Hash(raw string) []byte {
	sum := sha256.Sum256([]byte(raw))
	return sum[:]
}
