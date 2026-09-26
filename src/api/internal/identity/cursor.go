package identity

import (
	"encoding/base64"
	"encoding/json"
	"errors"
)

func encodeCursor(c cursor) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func decodeCursor(s string) (cursor, error) {
	var c cursor
	if len(s) > 256 {
		return c, errors.New("cursor too long")
	}
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return c, err
	}
	if err := json.Unmarshal(b, &c); err != nil {
		return c, err
	}
	if !isUUID(c.ID) || !isUUID(c.Branch) || c.CreatedAt.IsZero() {
		return c, errors.New("malformed cursor")
	}
	return c, nil
}

// isUUID checks canonical 8-4-4-4-12 hex form so malformed path/cursor IDs
// are rejected before reaching PostgreSQL.
func isUUID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		default:
			if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
				return false
			}
		}
	}
	return true
}
