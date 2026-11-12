// Package insights summarises training, nutrition and body weight from the
// stored logs. Every weight it returns is in kilograms.
package insights

import (
	"errors"
	"time"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// ErrInvalid marks a request the caller got wrong.
var ErrInvalid = errors.New("insights: invalid")

// Service computes insights for one user at a time.
type Service struct{ q *sqlc.Queries }

// New returns a Service reading through db.
func New(db sqlc.DBTX) *Service { return &Service{q: sqlc.New(db)} }

// Counts tallies planned days by how they went. A planned day is Pending until
// today has passed it; after that it is Done, Partial or Skipped. Planned is
// the sum of the four. Ratio is Done over the days that are no longer pending,
// 0 when there are none.
type Counts struct {
	Planned, Done, Partial, Skipped, Pending int
	Ratio                                    float64
}

func (c *Counts) add(o outcome) {
	c.Planned++
	switch o {
	case outDone:
		c.Done++
	case outPartial:
		c.Partial++
	case outSkipped:
		c.Skipped++
	case outPending:
		c.Pending++
	}
}

func (c *Counts) finish() {
	if n := c.Planned - c.Pending; n > 0 {
		c.Ratio = float64(c.Done) / float64(n)
	}
}

type outcome int

const (
	outPending outcome = iota
	outDone
	outPartial
	outSkipped
)

// settle is the outcome of a planned day with no logged result: skipped once
// it is before today, pending otherwise.
func settle(date, today time.Time) outcome {
	if date.Before(today) {
		return outSkipped
	}
	return outPending
}

func inKg(v float64) model.Weight { return model.Weight{Value: v, Unit: model.UnitKg} }

// span turns an optional date range into query bounds; a zero time is open-ended.
func span(from, to time.Time) (time.Time, time.Time) {
	if from.IsZero() {
		from = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if to.IsZero() {
		to = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
	}
	return from, to
}
