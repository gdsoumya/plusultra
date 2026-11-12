package api

import (
	"encoding/json"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
	"github.com/gdsoumya/plusultra/internal/programs"
)

// Database spellings of program enums, indexed by enum value.
var (
	kindNames = [...]string{
		pb.ProgramKind_PROGRAM_KIND_WORKOUT: programs.KindWorkout,
		pb.ProgramKind_PROGRAM_KIND_MEAL:    programs.KindMeal,
	}
	statusNames = [...]string{
		pb.ProgramStatus_PROGRAM_STATUS_DRAFT:    programs.StatusDraft,
		pb.ProgramStatus_PROGRAM_STATUS_ACTIVE:   programs.StatusActive,
		pb.ProgramStatus_PROGRAM_STATUS_FINISHED: programs.StatusFinished,
		pb.ProgramStatus_PROGRAM_STATUS_ARCHIVED: programs.StatusArchived,
	}
)

// i32 narrows counts that the engine bounds (weeks, days, reps, seconds, lines).
func i32(n int) int32 { return int32(n) } //nolint:gosec // bounded by the engine's limits

func dateString(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

func programPB(p sqlc.Program, script bool) (*pb.Program, error) {
	st, err := structurePB(p.Structure)
	out := &pb.Program{
		Id: p.ID.String(), Name: p.Name, Revision: p.Revision, Structure: st,
		Kind:      pb.ProgramKind(slices.Index(kindNames[:], p.Kind)),       //nolint:gosec // index into a 3-element array
		Status:    pb.ProgramStatus(slices.Index(statusNames[:], p.Status)), //nolint:gosec // index into a 5-element array
		StartDate: dateString(p.StartDate), EndDate: dateString(p.EndDate),
		CreatedAt: timestamppb.New(p.CreatedAt), UpdatedAt: timestamppb.New(p.UpdatedAt),
	}
	if script {
		out.Script = p.Script
	}
	switch {
	case p.Status == programs.StatusDraft:
	case p.StartDate != nil:
		out.Mode = pb.ProgramMode_PROGRAM_MODE_ANCHORED
	default:
		out.Mode = pb.ProgramMode_PROGRAM_MODE_FLOATING
	}
	return out, err
}

func structurePB(raw []byte) (*pb.ProgramStructure, error) {
	var st programs.Structure
	if err := json.Unmarshal(raw, &st); err != nil {
		return nil, err
	}
	return structureOf(&st), nil
}

func structureOf(st *programs.Structure) *pb.ProgramStructure {
	out := &pb.ProgramStructure{Weeks: i32(st.Weeks), Loop: st.Loop}
	for _, d := range st.Days {
		out.Days = append(out.Days, &pb.StructureDay{Week: i32(d.Week), Day: i32(d.Day), Name: d.Name, Kind: d.Kind})
	}
	return out
}

func revisionPB(r sqlc.ProgramRevision, script bool) (*pb.ProgramRevision, error) {
	st, err := structurePB(r.Structure)
	out := &pb.ProgramRevision{Revision: r.Revision, CreatedBy: r.CreatedBy, CreatedAt: timestamppb.New(r.CreatedAt), Structure: st}
	if script {
		out.Script = r.Script
	}
	return out, err
}

// scriptErrorPB describes a script or resolution error; only script errors have a position.
func scriptErrorPB(err error) *pb.ScriptError {
	if ee, ok := errors.AsType[*engine.EvalError](err); ok {
		return &pb.ScriptError{Line: i32(ee.Line), Col: i32(ee.Col), Message: ee.Msg}
	}
	if ce, ok := errors.AsType[*connect.Error](err); ok {
		return &pb.ScriptError{Message: ce.Message()}
	}
	return &pb.ScriptError{Message: err.Error()}
}

func nameErrorPB(n catalog.NameError) *pb.NameError {
	return &pb.NameError{Line: i32(n.Line), Name: n.Name, Suggestions: n.Suggestions}
}

func weightPB(w model.Weight) *pb.Weight {
	return &pb.Weight{Value: w.Value, Unit: pb.Unit(slices.Index(unitNames[:], string(w.Unit)))} //nolint:gosec // index into a 3-element array
}

func resolvedDayPB(d *model.ResolvedDay) *pb.ResolvedDay {
	out := &pb.ResolvedDay{Week: i32(d.Week), Day: i32(d.Day), Name: d.Name, Desc: d.Desc, Kind: d.Kind}
	for _, e := range d.Exercises {
		x := &pb.ResolvedExercise{Name: e.Name, Equipment: e.Equipment, Label: e.Label, Alternates: e.Alternates,
			Current: i32(e.Current), Desc: e.Desc, Superset: e.Superset, Slug: e.Slug, Unilateral: e.Unilateral, Progression: e.Progression}
		for _, s := range e.Sets {
			x.Sets = append(x.Sets, &pb.ResolvedSet{Reps: i32(s.Reps), MinReps: i32(s.MinReps), Weight: weightPB(s.Weight),
				Rpe: s.RPE, Rest: i32(s.Rest), Work: i32(s.Work), Amrap: s.Amrap, Ask: s.Ask, LogRpe: s.LogRPE,
				Auto: s.Auto, CountUp: s.CountUp, Warmup: s.Warmup, Label: s.Label})
		}
		out.Exercises = append(out.Exercises, x)
	}
	return out
}

func macrosPB(m model.Macros) *pb.Macros {
	return &pb.Macros{Kcal: m.Kcal, Protein: m.Protein, Carbs: m.Carbs, Fat: m.Fat, Fiber: m.Fiber, WaterMl: m.WaterML}
}

// mealDayPB maps d; without a target of its own, the day's target is the sum of its items.
func mealDayPB(d *model.MealDay) *pb.ResolvedMealDay {
	totals := d.Totals()
	out := &pb.ResolvedMealDay{Name: d.Name, Desc: d.Desc, Target: macrosPB(totals), Totals: macrosPB(totals), Meals: mealsPB(d.Meals)}
	if d.Target != nil {
		out.Target = macrosPB(*d.Target)
	}
	return out
}
