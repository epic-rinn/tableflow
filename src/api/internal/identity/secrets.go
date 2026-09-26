package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters: OWASP's minimum recommended configuration.
const (
	argonMemoryKiB = 19 * 1024
	argonTime      = 2
	argonThreads   = 1
	argonKeyLen    = 32
	argonSaltLen   = 16

	minPasswordRunes = 12
	maxPasswordBytes = 128 * 4
	maxPasswordRunes = 128
)

// Hasher bounds concurrent argon2id work so login bursts cannot exhaust
// memory (each hash allocates ~19 MiB).
type Hasher struct {
	slots chan struct{}
	dummy string
}

// NewHasher allows at most concurrency simultaneous hash operations.
func NewHasher(concurrency int) *Hasher {
	h := &Hasher{slots: make(chan struct{}, concurrency)}
	h.dummy = h.encode([]byte("timing-equalisation-dummy-password"), mustSalt())
	return h
}

func mustSalt() []byte {
	salt := make([]byte, argonSaltLen)
	_, _ = rand.Read(salt)
	return salt
}

func (h *Hasher) acquire(ctx context.Context) error {
	select {
	case h.slots <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Hasher) release() { <-h.slots }

func (h *Hasher) encode(password, salt []byte) string {
	key := argon2.IDKey(password, salt, argonTime, argonMemoryKiB, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemoryKiB, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// Hash returns a PHC-encoded argon2id hash.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := h.acquire(ctx); err != nil {
		return "", err
	}
	defer h.release()
	return h.encode([]byte(password), mustSalt()), nil
}

// Verify reports whether password matches encoded. An empty encoded value
// (unknown or not-yet-activated account) is checked against a dummy hash so
// response time does not reveal whether the account exists.
func (h *Hasher) Verify(ctx context.Context, password, encoded string) (bool, error) {
	if err := h.acquire(ctx); err != nil {
		return false, err
	}
	defer h.release()
	target, known := encoded, encoded != ""
	if !known {
		target = h.dummy
	}
	params, salt, want, err := decodeHash(target)
	if err != nil {
		return false, err
	}
	got := argon2.IDKey([]byte(password), salt, params.t, params.m, params.p, uint32(len(want)))
	return known && subtle.ConstantTimeCompare(got, want) == 1, nil
}

type argonParams struct {
	m, t uint32
	p    uint8
}

func decodeHash(encoded string) (argonParams, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	var p argonParams
	var version int
	if len(parts) != 6 || parts[1] != "argon2id" {
		return p, nil, nil, errors.New("unsupported password hash")
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return p, nil, nil, errors.New("unsupported argon2 version")
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.m, &p.t, &p.p); err != nil {
		return p, nil, nil, errors.New("malformed argon2 parameters")
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return p, nil, nil, errors.New("malformed argon2 salt")
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) == 0 {
		return p, nil, nil, errors.New("malformed argon2 key")
	}
	return p, salt, key, nil
}

// validatePassword applies length-only rules (NIST SP 800-63B style).
func validatePassword(pw string) string {
	switch {
	case !utf8.ValidString(pw):
		return "must be valid text"
	case utf8.RuneCountInString(pw) < minPasswordRunes:
		return fmt.Sprintf("must be at least %d characters", minPasswordRunes)
	case utf8.RuneCountInString(pw) > maxPasswordRunes || len(pw) > maxPasswordBytes:
		return fmt.Sprintf("must be at most %d characters", maxPasswordRunes)
	}
	return ""
}

// tokenBytes is the entropy of session and activation secrets (256 bits).
const tokenBytes = 32

// newToken returns a random URL-safe secret and its SHA-256 digest. Only the
// digest is stored; the raw value is shown or set as a cookie exactly once.
func newToken() (string, []byte) {
	b := make([]byte, tokenBytes)
	_, _ = rand.Read(b)
	raw := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:]
}

// hashToken digests a presented secret, rejecting anything that could not
// have been issued (wrong length or alphabet) without touching the database.
func hashToken(raw string) ([]byte, bool) {
	if len(raw) != base64.RawURLEncoding.EncodedLen(tokenBytes) {
		return nil, false
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw); err != nil {
		return nil, false
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], true
}
