package logs

import (
	"context"
	"net/http"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

// MaxPhotoBytes is the largest progress photo accepted.
const MaxPhotoBytes = 2 << 20

// storePhoto saves b, which checkPhoto has accepted.
func storePhoto(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, b []byte) (uuid.UUID, error) {
	p, err := q.CreatePhoto(ctx, sqlc.CreatePhotoParams{UserID: userID, Bytes: b, ContentType: http.DetectContentType(b)})
	return p.ID, err
}

// checkPhoto accepts an empty b (no photo, or keep the stored one), else a PNG or JPEG of at most
// MaxPhotoBytes. The type is sniffed from the bytes, not trusted from the client.
func checkPhoto(b []byte) error {
	if len(b) == 0 {
		return nil
	}
	if len(b) > MaxPhotoBytes {
		return invalid("photo larger than %d bytes", MaxPhotoBytes)
	}
	if ct := http.DetectContentType(b); ct != "image/png" && ct != "image/jpeg" {
		return invalid("photo must be PNG or JPEG")
	}
	return nil
}
