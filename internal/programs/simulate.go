package programs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// simulateTimeout bounds a whole simulation; each script run inside it has
// its own one-second limit.
const simulateTimeout = 10 * time.Second

// SimulateInput says what to simulate: Script, or ProgramID with Script as
// an optional replacement, from From, FromDate or the program's position,
// for Weeks weeks, dated from AnchorStart or the program's start date.
// Weights without a unit are in the user's units.
type SimulateInput struct {
	Script      string
	ProgramID   uuid.UUID
	Weeks       int
	From        Position
	FromDate    *time.Time
	AnchorStart *time.Time
	Overrides   Overrides
	// Reps done, and completed, on every set of the exercises named by catalog slug.
	Reps     map[string]int
	Scenario []engine.Logged
	Trace    bool
	Filter   Filter
	Expect   []Expectation
	// CompareScript or CompareScenario run a second simulation for Diff.
	CompareScript   string
	CompareScenario []engine.Logged
	// MealProgramID or MealScript map a meal program onto the run's dates.
	MealProgramID uuid.UUID
	MealScript    string
}

// Position is a week and day of a program; zero means unset.
type Position struct{ Week, Day int }

// Overrides replace parts of the user's state for one simulation.
type Overrides struct {
	Rounding   map[string]model.Rounding
	RM1        map[string]model.Weight
	Bodyweight *model.Weight
}

// SimulateOutput is what Simulate produced. Days are workout days, dated
// when anchored, or the days of a meal script.
type SimulateOutput struct {
	Days       []Day
	Output     []string
	Trace      []engine.SessionTrace
	Warnings   []engine.Warning
	Compact    []string
	Assertions []Assertion
	Diff       []DiffCell
	Meals      []MealMapping
}

// Simulate runs in without saving anything. Script and resolution errors,
// and running out of time, are InvalidArgument; the output then still holds
// the print output and warnings.
func (s *Service) Simulate(ctx context.Context, userID uuid.UUID, in SimulateInput) (SimulateOutput, error) {
	var out SimulateOutput
	src := in.Script
	var prog *sqlc.Program
	if in.ProgramID != uuid.Nil {
		p, err := get(ctx, s.Q, userID, in.ProgramID)
		if err != nil {
			return out, err
		}
		prog, src = &p, cmp.Or(src, p.Script)
	}
	if _, meal := engine.SplitExport(src); meal {
		sc, err := checked(ctx, s.Q, userID, KindMeal, src, nil)
		if err != nil {
			return out, err
		}
		out.Days = slices.DeleteFunc(mealDays(sc.meal), func(d Day) bool { return d.Week > in.Weeks })
		return out, nil
	}
	ctx, cancel := context.WithTimeout(ctx, simulateTimeout)
	defer cancel()
	r, err := s.simRun(ctx, userID, src, prog, &in)
	if err != nil {
		return out, err
	}
	res, err := engine.Simulate(ctx, r.p, r.st, r.opts)
	out.Output, out.Warnings = res.Output, res.Warnings
	if err != nil {
		return out, invalidArgument(err)
	}
	days := r.days(res.Days)
	out.Assertions = assert(in.Expect, days)
	if in.CompareScript != "" || len(in.CompareScenario) > 0 {
		b := r
		if in.CompareScript != "" {
			if b, err = s.simRun(ctx, userID, in.CompareScript, prog, &in); err != nil {
				return out, err
			}
		}
		if len(in.CompareScenario) > 0 {
			b.opts.Scenario = withUnits(in.CompareScenario, b.st.Units)
		}
		resB, err := engine.Simulate(ctx, b.p, b.st, b.opts)
		if err != nil {
			return out, invalidArgument(fmt.Errorf("compare: %w", err))
		}
		out.Diff = diff(days, b.days(resB.Days))
	}
	if in.MealProgramID != uuid.Nil || in.MealScript != "" {
		if out.Meals, err = s.mealMapping(ctx, userID, &in, r.start, days); err != nil {
			return out, err
		}
	}
	out.Days, out.Trace = in.Filter.apply(days, res.Trace)
	out.Compact = make([]string, len(out.Days))
	for i, d := range out.Days {
		out.Compact[i] = compactLine(d)
	}
	return out, nil
}

// simRun is a simulation ready for the engine: the program, the user's state
// with the overrides, and the dates when the sessions have them.
type simRun struct {
	p     *model.Program
	st    *model.UserState
	sl    slugs
	start *time.Time // Monday of week 1
	week  int        // week of the first session counted from start, not wrapped by looping
	opts  engine.SimOptions
}

func (s *Service) simRun(ctx context.Context, userID uuid.UUID, src string, prog *sqlc.Program, in *SimulateInput) (simRun, error) {
	r := simRun{week: 1, opts: engine.SimOptions{Weeks: in.Weeks, Trace: in.Trace, Reps: map[string]int{}}}
	p, err := engine.Evaluate(src, scriptFile)
	if err != nil {
		return r, invalidArgument(err)
	}
	r.p = p
	if r.st, r.sl, err = userState(ctx, s.Q, userID, p); err != nil {
		return r, err
	}
	if prog != nil {
		if r.st.Overlay, err = LoadOverlay(ctx, s.Q, prog.ID); err != nil {
			return r, err
		}
		if r.week, r.opts.FromDay, err = s.position(ctx, *prog); err != nil {
			return r, err
		}
		if prog.StartDate != nil {
			m := monday(*prog.StartDate)
			r.start = &m
		}
	}
	if in.AnchorStart != nil {
		m := monday(*in.AnchorStart)
		r.start = &m
	}
	if in.From.Week > 0 {
		r.week, r.opts.FromDay = in.From.Week, in.From.Day
	}
	if in.FromDate != nil {
		if r.start == nil {
			return r, invalidArgument(errors.New("from_date needs anchor_start_date or an anchored program"))
		}
		r.week, r.opts.FromDay = weekDay(*r.start, *in.FromDate)
	}
	if r.week > p.Weeks && !p.Loop {
		return r, invalidArgument(fmt.Errorf("week %d is past the %d-week program", r.week, p.Weeks))
	}
	r.opts.FromWeek = (r.week-1)%p.Weeks + 1
	for name, slug := range r.sl { // the engine matches exercises by name
		if n, ok := in.Reps[slug]; ok && slug != "" {
			r.opts.Reps[name] = n
		}
	}
	r.opts.Scenario = withUnits(in.Scenario, r.st.Units)
	for name, rd := range in.Overrides.Rounding {
		r.st.Rounding[name] = rd
	}
	for name, w := range in.Overrides.RM1 {
		r.st.RM1[name] = withUnit(w, r.st.Units)
	}
	if in.Overrides.Bodyweight != nil {
		r.st.Bodyweight = withUnit(*in.Overrides.Bodyweight, r.st.Units)
	}
	return r, nil
}

// position is where program p stands: the cursor of a floating program,
// today's week and day of an anchored one, else its start.
func (s *Service) position(ctx context.Context, p sqlc.Program) (int, int, error) {
	if p.StartDate == nil {
		c, ok, err := s.Cursor(ctx, p)
		if !ok || err != nil {
			return 1, 0, err
		}
		return int(c.NextWeek), int(c.NextDay), nil
	}
	today, err := s.today(ctx, s.Q, p.UserID)
	if err != nil || today.Before(*p.StartDate) {
		return 1, 0, err
	}
	w, d := weekDay(monday(*p.StartDate), today)
	return w, d, nil
}

// weekDay is the week and day of date counted from monday, week 1 day 1.
func weekDay(monday, date time.Time) (int, int) {
	off := max(int(date.Sub(monday).Hours()/24), 0)
	return off/7 + 1, off%7 + 1
}

// days dates the engine's days from the run's start, counting weeks on past
// the program's last week when it loops.
func (r simRun) days(rds []model.ResolvedDay) []Day {
	days := make([]Day, len(rds))
	week, prevW, prevD := r.week, r.opts.FromWeek, max(r.opts.FromDay, 1)-1
	for i := range rds {
		rd := &rds[i]
		r.sl.set(rd)
		if delta := rd.Week - prevW; delta < 0 || delta == 0 && rd.Day <= prevD {
			week += delta + r.p.Weeks
		} else {
			week += delta
		}
		prevW, prevD = rd.Week, rd.Day
		days[i] = Day{Week: rd.Week, Day: rd.Day, Kind: rd.Kind, Workout: rd}
		if r.start != nil {
			d := r.start.AddDate(0, 0, (week-1)*7+rd.Day-1)
			days[i].Date = &d
		}
	}
	return days
}

func withUnit(w model.Weight, u model.Unit) model.Weight {
	w.Unit = cmp.Or(w.Unit, u)
	return w
}

// withUnits fills the user's unit into the scenario's weights.
func withUnits(sc []engine.Logged, u model.Unit) []engine.Logged {
	out := slices.Clone(sc)
	for i := range out {
		l := &out[i]
		if l.Weight != nil {
			w := withUnit(*l.Weight, u)
			l.Weight = &w
		}
		if l.WeightDelta != nil {
			w := withUnit(*l.WeightDelta, u)
			l.WeightDelta = &w
		}
		l.Sets = slices.Clone(l.Sets)
		for j := range l.Sets {
			l.Sets[j].Weight = withUnit(l.Sets[j].Weight, u)
		}
	}
	return out
}
