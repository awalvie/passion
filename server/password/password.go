// Package password hashes and checks passwords with argon2id.
//
// A hash is stored as one PHC string, which carries the algorithm, the
// version, every parameter and the salt inside the value. Raising the work
// factor is then a change here and never a migration.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
)

// OWASP's floor is 46 MiB. RFC 9106, echoed by x/crypto/argon2, asks for
// 64 MiB when 2 GiB is not available. 64 MiB satisfies both.
var current = params{memory: 64 * 1024, time: 3, threads: 4, saltLen: 16, keyLen: 32}

const (
	// MinLength follows the current guidance. MaxLength exists because every
	// hash costs 64 MiB, so an unbounded body would be a denial-of-service.
	MinLength = 8
	MaxLength = 128
)

var (
	ErrTooShort = fmt.Errorf("password must be at least %d characters", MinLength)
	ErrTooLong  = fmt.Errorf("password must be at most %d characters", MaxLength)

	// ErrMismatch means the password is wrong. It is deliberately the same
	// error whatever was wrong, so it tells an attacker nothing.
	ErrMismatch = errors.New("password does not match")

	errMalformed = errors.New("stored password hash is malformed")
)

type params struct {
	memory  uint32
	time    uint32
	threads uint8
	saltLen uint32
	keyLen  uint32
}

// Hash returns a PHC string for plain.
func Hash(plain string) (string, error) {
	switch {
	case len(plain) < MinLength:
		return "", ErrTooShort
	case len(plain) > MaxLength:
		return "", ErrTooLong
	}

	salt := make([]byte, current.saltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("read random salt: %w", err)
	}

	return encode(current, salt, derive(current, plain, salt)), nil
}

// Verify reports whether plain matches encoded.
func Verify(encoded, plain string) error {
	stored, salt, key, err := decode(encoded)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare(derive(stored, plain, salt), key) != 1 {
		return ErrMismatch
	}
	return nil
}

func derive(p params, plain string, salt []byte) []byte {
	return argon2.IDKey([]byte(plain), salt, p.time, p.memory, p.threads, p.keyLen)
}

func encode(p params, salt, key []byte) string {
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.memory, p.time, p.threads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key))
}

// decode refuses anything it does not fully understand. A parser that guesses
// here is an authentication bypass.
func decode(encoded string) (p params, salt, key []byte, err error) {
	fields := strings.Split(encoded, "$")
	if len(fields) != 6 || fields[0] != "" || fields[1] != "argon2id" {
		return p, nil, nil, errMalformed
	}

	// Sscanf stops when the format runs out and ignores whatever follows, so
	// each field is rendered back and compared. Without that, "v=19junk" reads
	// as 19.
	var version int
	if _, err := fmt.Sscanf(fields[2], "v=%d", &version); err != nil {
		return p, nil, nil, errMalformed
	}
	if version != argon2.Version || fields[2] != fmt.Sprintf("v=%d", version) {
		return p, nil, nil, errMalformed
	}

	if _, err := fmt.Sscanf(fields[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil {
		return p, nil, nil, errMalformed
	}
	if p.memory == 0 || p.time == 0 || p.threads == 0 {
		return p, nil, nil, errMalformed
	}
	if fields[3] != fmt.Sprintf("m=%d,t=%d,p=%d", p.memory, p.time, p.threads) {
		return p, nil, nil, errMalformed
	}

	if salt, err = base64.RawStdEncoding.DecodeString(fields[4]); err != nil {
		return p, nil, nil, errMalformed
	}
	if key, err = base64.RawStdEncoding.DecodeString(fields[5]); err != nil {
		return p, nil, nil, errMalformed
	}
	if len(salt) == 0 || len(key) == 0 {
		return p, nil, nil, errMalformed
	}

	p.saltLen = uint32(len(salt))
	p.keyLen = uint32(len(key))
	return p, salt, key, nil
}
