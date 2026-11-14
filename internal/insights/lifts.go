package insights

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// BestSet is the set with the highest estimated 1RM.
type BestSet struct {
	Reps   int
	Weight model.Weight
}

// Point is an exercise's performance on one date. Volume sums weight times
// reps over the completed working sets, bodyweight sets counting 0; RecordID
// is the workout record the best set is from.
type Point struct {
	Date     time.Time
	E1RM     model.Weight
	BestSet  BestSet
	Volume   model.Weight
	Sets     int
	RecordID uuid.UUID
}

type record struct {
	id      uuid.UUID
	date    time.Time
	entries []model.RecordEntry
}

// records returns the finalized workout records from..to, oldest first.
func (s *Service) records(ctx context.Context, userID uuid.UUID, from, to time.Time) ([]record, error) {
	rows, err := s.q.ListFinalizedWorkoutRecords(ctx, sqlc.ListFinalizedWorkoutRecordsParams{UserID: userID, FromDate: from, ToDate: to})
	if err != nil {
		return nil, err
	}
	var out []record
	for _, r := range rows {
		var es []model.RecordEntry
		if err := json.Unmarshal(r.Entries, &es); err != nil {
			return nil, fmt.Errorf("workout record %s entries: %w", r.ID, err)
		}
		out = append(out, record{r.ID, r.Date, es})
	}
	return out, nil
}

// LiftHistory returns the exercise's best set for every date it was done, oldest
// first. A zero from or to leaves that side of the range open.
func (s *Service) LiftHistory(ctx context.Context, userID uuid.UUID, slug string, from, to time.Time) ([]Point, error) {
	from, to = span(from, to)
	recs, err := s.records(ctx, userID, from, to)
	if err != nil {
		return nil, err
	}
	var pts []Point
	for _, r := range recs {
		for _, e := range r.entries {
			if e.Slug != slug {
				continue
			}
			p, ok := bestSet(e.Sets)
			if !ok {
				continue
			}
			p.Date, p.RecordID = r.date, r.id
			if n := len(pts); n > 0 && pts[n-1].Date.Equal(p.Date) {
				last := &pts[n-1]
				last.Sets += p.Sets
				last.Volume.Value += p.Volume.Value
				if p.E1RM.Value > last.E1RM.Value {
					last.E1RM, last.BestSet, last.RecordID = p.E1RM, p.BestSet, p.RecordID
				}
				continue
			}
			pts = append(pts, p)
		}
	}
	return pts, nil
}

func isWorking(s model.RecordSet) bool { return s.Completed && !s.Warmup && !s.Planned.Warmup }

// bestSet picks the working set with the highest Epley estimate and tallies
// the working sets and their volume. A set without logged reps or weight falls
// back to its plan; found is false without a weighted set.
func bestSet(sets []model.RecordSet) (Point, bool) {
	best := Point{Volume: inKg(0)}
	var found bool
	for _, st := range sets {
		if !isWorking(st) {
			continue
		}
		best.Sets++
		reps, w := st.Reps, model.Weight(st.Weight).Kg()
		if reps == 0 {
			reps = st.Planned.Reps
		}
		if w == 0 {
			w = model.Weight(st.Planned.Weight).Kg()
		}
		if reps <= 0 || w <= 0 {
			continue
		}
		best.Volume.Value += w * float64(reps)
		if e := w * (1 + float64(reps)/30); !found || e > best.E1RM.Value {
			best.E1RM, best.BestSet, found = inKg(e), BestSet{reps, inKg(w)}, true
		}
	}
	return best, found
}
