package sync

import (
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// encodeCursor returns the opaque cursor of the row changed at (at, id).
func encodeCursor(at time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id.String()))
}

// decodeCursor parses a cursor; the empty cursor sorts before every row.
func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	if s == "" {
		return time.Time{}, uuid.Nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(s)
	at, id, ok := strings.Cut(string(raw), "|")
	if err != nil || !ok {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad cursor", ErrInvalid)
	}
	t, terr := time.Parse(time.RFC3339Nano, at)
	u, uerr := uuid.Parse(id)
	if terr != nil || uerr != nil {
		return time.Time{}, uuid.Nil, fmt.Errorf("%w: bad cursor", ErrInvalid)
	}
	return t, u, nil
}
