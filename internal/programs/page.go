package programs

import (
	"context"
	"encoding/base64"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// Page is one page of a list. NextCursor is empty on the last page.
type Page[T any] struct {
	Items      []T
	NextCursor string
	HasMore    bool
}

// pageOf trims rows, fetched with one row more than limit, to a page.
func pageOf[T any](rows []T, limit int, cursor func(T) string) Page[T] {
	p := Page[T]{Items: rows}
	if p.HasMore = len(rows) > limit; p.HasMore {
		p.Items = rows[:limit]
		p.NextCursor = cursor(p.Items[limit-1])
	}
	return p
}

// fetchLimit checks limit, 0 meaning the default, and returns it with the rows to fetch.
func fetchLimit(limit int) (int, int32, error) {
	if limit < 0 || limit > maxLimit {
		return 0, 0, invalidArgument(errors.New("limit must be 0-500"))
	}
	if limit == 0 {
		limit = defaultLimit
	}
	return limit, int32(limit + 1), nil //nolint:gosec // limit <= maxLimit
}

// badCursor is a new error each time, since Connect errors carry mutable metadata.
func badCursor() error { return invalidArgument(errors.New("bad cursor")) }

// List returns userID's programs of kind and status, empty for any, most
// recently changed first.
func (s *Service) List(ctx context.Context, userID uuid.UUID, kind, status string, limit int, cursor string) (Page[sqlc.Program], error) {
	limit, fetch, err := fetchLimit(limit)
	if err != nil {
		return Page[sqlc.Program]{}, err
	}
	params := sqlc.ListProgramsPageParams{UserID: userID, Kind: kind, Status: status, MaxRows: fetch,
		BeforeUpdated: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), BeforeID: uuid.Max}
	if cursor != "" {
		raw, err := base64.RawURLEncoding.DecodeString(cursor)
		at, id, ok := strings.Cut(string(raw), "|")
		if err != nil || !ok {
			return Page[sqlc.Program]{}, badCursor()
		}
		if params.BeforeUpdated, err = time.Parse(time.RFC3339Nano, at); err != nil {
			return Page[sqlc.Program]{}, badCursor()
		}
		if params.BeforeID, err = uuid.Parse(id); err != nil {
			return Page[sqlc.Program]{}, badCursor()
		}
	}
	rows, err := s.Q.ListProgramsPage(ctx, params)
	return pageOf(rows, limit, func(p sqlc.Program) string {
		return base64.RawURLEncoding.EncodeToString([]byte(p.UpdatedAt.Format(time.RFC3339Nano) + "|" + p.ID.String()))
	}), err
}

// ListRevisions returns program id's revisions, newest first.
func (s *Service) ListRevisions(ctx context.Context, userID, id uuid.UUID, limit int, cursor string) (Page[sqlc.ProgramRevision], error) {
	limit, fetch, err := fetchLimit(limit)
	if err != nil {
		return Page[sqlc.ProgramRevision]{}, err
	}
	if _, err := get(ctx, s.Q, userID, id); err != nil {
		return Page[sqlc.ProgramRevision]{}, err
	}
	before := int32(math.MaxInt32)
	if cursor != "" {
		n, err := strconv.ParseInt(cursor, 10, 32)
		if err != nil || n < 1 {
			return Page[sqlc.ProgramRevision]{}, badCursor()
		}
		before = int32(n)
	}
	rows, err := s.Q.ListRevisionsPage(ctx, sqlc.ListRevisionsPageParams{ProgramID: id, BeforeRevision: before, MaxRows: fetch})
	return pageOf(rows, limit, func(r sqlc.ProgramRevision) string { return strconv.Itoa(int(r.Revision)) }), err
}
