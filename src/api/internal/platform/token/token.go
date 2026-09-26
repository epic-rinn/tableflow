// Package token issues and digests 256-bit bearer secrets (sessions,
// activation links, QR capabilities). Only digests are ever stored.
package token

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

const bytesLen = 32

// New returns a random URL-safe secret and its SHA-256 digest.
func New() (string, []byte) {
	b := make([]byte, bytesLen)
	_, _ = rand.Read(b)
	raw := base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	return raw, sum[:]
}

// Hash digests a presented secret, rejecting values that could not have been
// issued (wrong length or alphabet) without touching the database.
func Hash(raw string) ([]byte, bool) {
	if len(raw) != base64.RawURLEncoding.EncodedLen(bytesLen) {
		return nil, false
	}
	if _, err := base64.RawURLEncoding.DecodeString(raw); err != nil {
		return nil, false
	}
	sum := sha256.Sum256([]byte(raw))
	return sum[:], true
}
