package insights

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/model"
)

// MacroSummary gives per-day averages: TargetAvg over the TargetDays days that
// have a target, ActualAvg over the ActualDays days with something eaten.
type MacroSummary struct {
	TargetAvg, ActualAvg   model.Macros
	TargetDays, ActualDays int
}

// WeightSummary is body weight over the week; nil without check-ins. Avg7 is
// the 7-day moving average at the last check-in.
type WeightSummary struct{ Start, End, Avg7 *model.Weight }

// Summary is one week of training, nutrition and body weight.
type Summary struct {
	Adherence Counts
	// SetsPerMuscle counts each completed working set once for every primary
	// muscle group of its exercise, so a set can count under several groups; a
	// region counts for its group here and for itself in SetsPerRegion (R83).
	SetsPerMuscle map[string]int
	SetsPerRegion map[string]int
	E1RM          map[string]model.Weight
	Macros        MacroSummary
	Weight        WeightSummary
}

// WeeklySummary summarises the seven days from weekStart, which must be a
// Monday or the call fails with ErrInvalid. Dates are the user's local calendar
// dates as stored, so the caller converts; today is the user's local today and
// decides which unlogged planned days are skipped or still pending.
func (s *Service) WeeklySummary(ctx context.Context, userID uuid.UUID, weekStart, today time.Time) (Summary, error) {
	if weekStart.Weekday() != time.Monday {
		return Summary{}, fmt.Errorf("%w: week start %s is not a Monday", ErrInvalid, weekStart.Format(time.DateOnly))
	}
	end := weekStart.AddDate(0, 0, 6)
	sum := Summary{SetsPerMuscle: map[string]int{}, SetsPerRegion: map[string]int{}, E1RM: map[string]model.Weight{}}
	var err error
	if sum.Adherence, err = s.workoutCounts(ctx, userID, weekStart, end, today); err != nil {
		return sum, err
	}
	if err = s.trainingStats(ctx, userID, weekStart, end, &sum); err != nil {
		return sum, err
	}
	days, err := s.mealDays(ctx, userID, weekStart, end)
	if err != nil {
		return sum, err
	}
	var target, actual model.Macros
	for _, d := range days {
		if d.target != (model.Macros{}) {
			target = target.Add(d.target)
			sum.Macros.TargetDays++
		}
		if d.eaten {
			actual = actual.Add(d.actual)
			sum.Macros.ActualDays++
		}
	}
	if n := sum.Macros.TargetDays; n > 0 {
		sum.Macros.TargetAvg = target.Scale(1 / float64(n))
	}
	if n := sum.Macros.ActualDays; n > 0 {
		sum.Macros.ActualAvg = actual.Scale(1 / float64(n))
	}
	sum.Weight, err = s.weekWeight(ctx, userID, weekStart, end)
	return sum, err
}

func (s *Service) trainingStats(ctx context.Context, userID uuid.UUID, from, to time.Time, sum *Summary) error {
	recs, err := s.records(ctx, userID, from, to)
	if err != nil {
		return err
	}
	exs, err := s.q.ListExercises(ctx, &userID)
	if err != nil {
		return err
	}
	// Keyed by slug and by lowercased name and alias: records written before a
	// name resolved, or by a client that sent no slug, still count (R80).
	muscles := make(map[string][]string, len(exs))
	for _, e := range exs {
		muscles[e.Slug], muscles[strings.ToLower(e.Name)] = e.PrimaryMuscles, e.PrimaryMuscles
		for _, a := range e.Aliases {
			if a = strings.ToLower(a); len(muscles[a]) == 0 { // a name match outranks an alias
				muscles[a] = e.PrimaryMuscles
			}
		}
	}
	for _, r := range recs {
		for _, e := range r.entries {
			ms := muscles[e.Slug]
			for _, n := range append([]string{e.Name}, e.Alternates...) {
				if len(ms) > 0 {
					break
				}
				ms = muscles[strings.ToLower(n)]
			}
			groups, regions := rollUp(ms)
			for _, st := range e.Sets {
				if isWorking(st) {
					for _, m := range groups {
						sum.SetsPerMuscle[m]++
					}
					for _, m := range regions {
						sum.SetsPerRegion[m]++
					}
				}
			}
			if p, ok := bestSet(e.Sets); ok && e.Slug != "" && p.E1RM.Value > sum.E1RM[e.Slug].Value {
				sum.E1RM[e.Slug] = p.E1RM
			}
		}
	}
	return nil
}

// rollUp maps muscle names to deduplicated groups and regions; a name the
// taxonomy does not know stays as stored, and none at all is "unknown".
func rollUp(ms []string) (groups, regions []string) {
	if len(ms) == 0 {
		return []string{"unknown"}, nil
	}
	for _, m := range ms {
		n, ok := catalog.Normalize(m)
		if !ok {
			n = m
		}
		if g, isRegion := catalog.Regions[n]; isRegion {
			regions = append(regions, n)
			n = g
		}
		if !slices.Contains(groups, n) {
			groups = append(groups, n)
		}
	}
	return groups, regions
}

func (s *Service) weekWeight(ctx context.Context, userID uuid.UUID, from, to time.Time) (WeightSummary, error) {
	ws, avg, first, err := s.weighIns(ctx, userID, from, to)
	if err != nil || first == len(ws) {
		return WeightSummary{}, err
	}
	last := len(ws) - 1
	start, end, mean := inKg(ws[first].kg), inKg(ws[last].kg), inKg(avg[last])
	return WeightSummary{&start, &end, &mean}, nil
}
