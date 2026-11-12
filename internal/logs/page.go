package logs

import (
	"context"
	"encoding/base64"
	"hash/fnv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

const (
	defaultLimit = 100
	maxLimit     = 500
)

// Page is one page of a cursor-paginated list.
type Page[T any] struct {
	Items      []T
	NextCursor string // empty on the last page
	HasMore    bool
}

// pageQuery is a validated date range and cursor position.
type pageQuery struct {
	from, to, afterDate time.Time
	afterID             uuid.UUID
	limit               int
}

func newPageQuery(from, to, cursor string, limit int) (pageQuery, error) {
	var q pageQuery
	var err error
	if q.from, err = parseDate(from); err != nil {
		return q, err
	}
	if q.to, err = parseDate(to); err != nil {
		return q, err
	}
	if limit < 0 || limit > maxLimit {
		return q, invalid("limit must be 0-%d", maxLimit)
	}
	q.limit = limit
	if limit == 0 {
		q.limit = defaultLimit
	}
	q.afterDate = q.from.AddDate(0, 0, -1) // no cursor: every row on from sorts after this
	if cursor != "" {
		if q.afterDate, q.afterID, err = decodeCursor(cursor); err != nil {
			return q, err
		}
	}
	return q, nil
}

// fetch is the query's row limit: one more than a page, to learn whether another follows.
func (q pageQuery) fetch() int32 { return int32(q.limit + 1) } //nolint:gosec // limit <= maxLimit

// paginate trims rows (fetched with fetch()) to a page and builds the next cursor from key.
func paginate[R, T any](q pageQuery, rows []R, key func(R) (time.Time, uuid.UUID), conv func(R) (T, error)) (Page[T], error) {
	p := Page[T]{Items: make([]T, 0, min(len(rows), q.limit))}
	if p.HasMore = len(rows) > q.limit; p.HasMore {
		rows = rows[:q.limit]
		p.NextCursor = encodeCursor(key(rows[len(rows)-1]))
	}
	for _, r := range rows {
		it, err := conv(r)
		if err != nil {
			return p, err
		}
		p.Items = append(p.Items, it)
	}
	return p, nil
}

func encodeCursor(date time.Time, id uuid.UUID) string {
	return base64.RawURLEncoding.EncodeToString([]byte(date.Format(time.DateOnly) + "|" + id.String()))
}

func decodeCursor(s string) (time.Time, uuid.UUID, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	d, id, ok := strings.Cut(string(raw), "|")
	if err != nil || !ok {
		return time.Time{}, uuid.Nil, invalid("bad cursor")
	}
	date, derr := time.Parse(time.DateOnly, d)
	uid, uerr := uuid.Parse(id)
	if derr != nil || uerr != nil {
		return time.Time{}, uuid.Nil, invalid("bad cursor")
	}
	return date, uid, nil
}

// inDayTx runs fn in a transaction holding a lock for (user, date, kind), so concurrent
// creates for one date cannot both pass the "one per date" check.
func inDayTx(ctx context.Context, pool *pgxpool.Pool, q *sqlc.Queries, userID uuid.UUID, date time.Time, kind string, fn func(*sqlc.Queries) error) error {
	h := fnv.New64a()
	_, _ = h.Write([]byte(kind + userID.String() + date.Format(time.DateOnly)))
	return pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, int64(h.Sum64())); err != nil { //nolint:gosec // hash, wraparound is fine
			return err
		}
		return fn(q.WithTx(tx))
	})
}

func parseDate(s string) (time.Time, error) {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil || d.Format(time.DateOnly) != s {
		return time.Time{}, invalid("date %q is not YYYY-MM-DD", s)
	}
	return d, nil
}
