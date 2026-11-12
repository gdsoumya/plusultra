package api

import (
	"errors"
	"strconv"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"go.starlark.net/starlark"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

func simulateInput(m *pb.SimulateProgramRequest) (programs.SimulateInput, error) {
	in := programs.SimulateInput{Script: m.GetScript(), Weeks: int(m.GetWeeks()), Reps: map[string]int{},
		From:          programs.Position{Week: int(m.GetFrom().GetWeek()), Day: int(m.GetFrom().GetDay())},
		Scenario:      sessions(m.GetScenario()),
		Trace:         m.GetTrace(),
		CompareScript: m.GetCompareScript(), CompareScenario: sessions(m.GetCompareScenario()),
		MealScript: m.GetMealScript(),
		Filter:     programs.Filter{Exercises: m.GetFilter().GetExercises(), WorkingOnly: m.GetFilter().GetWorkingOnly()}}
	for slug, n := range m.GetAssumptions().GetReps() {
		in.Reps[slug] = int(n)
	}
	var err error
	for _, id := range []struct {
		s   string
		dst *uuid.UUID
	}{{m.GetProgramId(), &in.ProgramID}, {m.GetMealProgramId(), &in.MealProgramID}} {
		if id.s != "" {
			if *id.dst, err = parseID(id.s); err != nil {
				return in, err
			}
		}
	}
	for _, d := range []struct {
		s   string
		dst **time.Time
	}{{m.GetFromDate(), &in.FromDate}, {m.GetAnchorStartDate(), &in.AnchorStart}} {
		if d.s != "" {
			t, err := time.Parse(time.DateOnly, d.s)
			if err != nil {
				return in, connect.NewError(connect.CodeInvalidArgument, errors.New("dates must be YYYY-MM-DD"))
			}
			*d.dst = &t
		}
	}
	for _, w := range m.GetFilter().GetWeeks() {
		in.Filter.Weeks = append(in.Filter.Weeks, int(w))
	}
	for _, d := range m.GetFilter().GetDays() {
		in.Filter.Days = append(in.Filter.Days, int(d))
	}
	if o := m.GetOverrides(); o != nil {
		in.Overrides = programs.Overrides{Rounding: map[string]model.Rounding{}, RM1: map[string]model.Weight{}}
		for name, r := range o.GetRounding() {
			in.Overrides.Rounding[name] = *roundingOf(r)
		}
		for name, w := range o.GetRm1() {
			in.Overrides.RM1[name] = weightOf(w)
		}
		if o.GetBodyweight() != nil {
			bw := weightOf(o.GetBodyweight())
			in.Overrides.Bodyweight = &bw
		}
	}
	for _, x := range m.GetExpect() {
		e := programs.Expectation{Week: int(x.GetWeek()), Day: int(x.GetDay()), Exercise: x.GetExercise(), Set: int(x.GetSet()), Reps: int(x.GetReps())}
		if x.GetWeight() != nil {
			w := weightOf(x.GetWeight())
			e.Weight = &w
		}
		in.Expect = append(in.Expect, e)
	}
	return in, nil
}

// weightOf is w as the engine takes it; no unit stays empty for the user's.
func weightOf(w *pb.Weight) model.Weight {
	return model.Weight{Value: w.GetValue(), Unit: model.Unit(unitNames[w.GetUnit()])}
}

func sessions(ss []*pb.SimulateSession) []engine.Logged {
	var out []engine.Logged
	for _, s := range ss {
		l := engine.Logged{Week: int(s.GetWeek()), Day: int(s.GetDay()), Exercise: s.GetExercise(), Label: s.GetLabel(),
			MissLastSetBy: int(s.GetMissLastSetBy()), Skip: s.GetSkip()}
		if s.GetWeight() != nil {
			w := weightOf(s.GetWeight())
			l.Weight = &w
		}
		if s.GetWeightDelta() != nil {
			w := weightOf(s.GetWeightDelta())
			l.WeightDelta = &w
		}
		for _, x := range s.GetSets() {
			l.Sets = append(l.Sets, model.SetResult{Reps: int(x.GetReps()), Weight: weightOf(x.GetWeight()), RPE: x.GetRpe(), Completed: x.GetCompleted()})
		}
		if len(s.GetPrompts()) > 0 {
			l.Prompts = map[string]starlark.Value{}
			for name, v := range s.GetPrompts() {
				l.Prompts[name] = promptValue(v)
			}
		}
		out = append(out, l)
	}
	return out
}

// promptValue reads a prompt answer: an int, a float, a bool, else text.
func promptValue(s string) starlark.Value {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return starlark.MakeInt64(n)
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return starlark.Float(f)
	}
	if b, err := strconv.ParseBool(s); err == nil {
		return starlark.Bool(b)
	}
	return starlark.String(s)
}

func simulateResponse(m *pb.SimulateProgramRequest, out programs.SimulateOutput) *pb.SimulateProgramResponse {
	res := &pb.SimulateProgramResponse{Output: out.Output, Compact: out.Compact}
	for _, w := range out.Warnings {
		res.Warnings = append(res.Warnings, &pb.SimulateWarning{Code: w.Code, Week: i32(w.Week), Day: i32(w.Day), Exercise: w.Exercise, Message: w.Message})
	}
	for _, d := range out.Days {
		if d.Workout == nil {
			md := mealDayPB(d.Meal)
			md.Week, md.Day, md.Kind = i32(d.Week), i32(d.Day), d.Kind
			res.MealDays = append(res.MealDays, md)
			continue
		}
		if !m.GetCompact() && !m.GetTrace() || m.GetIncludeDays() {
			rd := resolvedDayPB(d.Workout)
			rd.Date = dateString(d.Date)
			res.Days = append(res.Days, rd)
		}
	}
	if !m.GetCompact() {
		res.Compact = nil
	}
	dates := map[[2]int]string{}
	for _, d := range out.Days {
		dates[[2]int{d.Week, d.Day}] = dateString(d.Date)
	}
	for _, t := range out.Trace {
		st := &pb.SessionTrace{Week: i32(t.Week), Day: i32(t.Day), Date: dates[[2]int{t.Week, t.Day}], Exercise: t.Exercise, Label: t.Label,
			Function: t.Function, StateBefore: t.StateBefore, StateAfter: t.StateAfter, Prints: t.Prints}
		for _, w := range t.Writes {
			st.Writes = append(st.Writes, &pb.OverlayWrite{Address: w.Address, Kind: w.Kind, Value: w.Value})
		}
		res.Trace = append(res.Trace, st)
	}
	for i, a := range out.Assertions {
		res.Assertions = append(res.Assertions, &pb.SimulateAssertion{Expect: m.GetExpect()[i], Ok: a.OK, Actual: a.Actual, Message: a.Message})
	}
	for _, c := range out.Diff {
		res.Diff = append(res.Diff, &pb.SimulateDiff{Week: i32(c.Week), Day: i32(c.Day), Exercise: c.Exercise, Set: i32(c.Set), Field: c.Field, A: c.A, B: c.B})
	}
	for _, mm := range out.Meals {
		res.Meals = append(res.Meals, &pb.SimulateMealDay{Date: mm.Date.Format(time.DateOnly), Name: mm.Name, Kind: mm.Kind,
			Totals: macrosPB(mm.Totals), Target: macrosPB(mm.Target), Missing: mm.Missing})
	}
	return res
}
