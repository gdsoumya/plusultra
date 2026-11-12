package api

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
)

// Day log statuses by enum value.
var dayStatusNames = [...]string{
	pb.DayStatus_DAY_STATUS_PLANNED: logs.StatusPlanned,
	pb.DayStatus_DAY_STATUS_DONE:    logs.StatusDone,
	pb.DayStatus_DAY_STATUS_PARTIAL: logs.StatusPartial,
	pb.DayStatus_DAY_STATUS_SKIPPED: logs.StatusSkipped,
	pb.DayStatus_DAY_STATUS_REST:    logs.StatusRest,
}

var logsCodes = map[error]connect.Code{
	logs.ErrNotFound: connect.CodeNotFound, logs.ErrInvalid: connect.CodeInvalidArgument,
	logs.ErrConflict: connect.CodeAlreadyExists, logs.ErrFinalized: connect.CodeFailedPrecondition,
}

// logsError maps the logs sentinels to Connect codes; other errors pass through.
func logsError(err error) error {
	for sentinel, code := range logsCodes {
		if errors.Is(err, sentinel) {
			return connect.NewError(code, err)
		}
	}
	return err
}

// userUnit is the caller's weight unit, which fills in unspecified units.
func (d Deps) userUnit(ctx context.Context) (model.Unit, error) {
	id, err := callerID(ctx)
	if err != nil {
		return "", err
	}
	return userUnit(ctx, d.Queries, id)
}

// userUnit is the unit userID sees weights in.
func userUnit(ctx context.Context, q *sqlc.Queries, userID uuid.UUID) (model.Unit, error) {
	u, err := q.GetUser(ctx, userID)
	return model.Unit(u.Units), err
}

// kgOf reads a request weight as kilograms, in the unit it states or def when unspecified; nil stays nil.
func kgOf(w *pb.Weight, def model.Unit) *float64 {
	if w == nil {
		return nil
	}
	kg := weightDB(w, def).Kg()
	return &kg
}

func parseDate(s string) (time.Time, error) {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return t, connect.NewError(connect.CodeInvalidArgument, errors.New("date must be YYYY-MM-DD"))
	}
	return t, nil
}

func macrosOf(m *pb.Macros) model.Macros {
	return model.Macros{Kcal: m.GetKcal(), Protein: m.GetProtein(), Carbs: m.GetCarbs(), Fat: m.GetFat(), Fiber: m.GetFiber(), WaterML: m.GetWaterMl()}
}

func roundingPB(r model.Rounding) *pb.Rounding {
	return &pb.Rounding{Kind: r.Kind, Increment: r.Increment, Bar: r.Bar, Plates: r.Plates}
}

func roundingOf(r *pb.Rounding) *model.Rounding {
	if r == nil {
		return nil
	}
	return &model.Rounding{Kind: r.GetKind(), Increment: r.GetIncrement(), Bar: r.GetBar(), Plates: r.GetPlates()}
}

func statusPB(s string) pb.DayStatus {
	return pb.DayStatus(max(slices.Index(dayStatusNames[:], s), 0)) //nolint:gosec // index into a 6-element array
}

func weightDB(w *pb.Weight, def model.Unit) model.Weight {
	out := model.Weight{Value: w.GetValue(), Unit: def}
	if w.GetUnit() != pb.Unit_UNIT_UNSPECIFIED {
		out.Unit = model.Unit(unitNames[w.GetUnit()])
	}
	return out
}

// fromKg converts a stored kilogram value to the user's unit.
func fromKg(kg float64, unit model.Unit) model.Weight {
	if unit == model.UnitLb {
		return model.Weight{Value: kg / model.Weight{Value: 1, Unit: model.UnitLb}.Kg(), Unit: unit}
	}
	return model.Weight{Value: kg, Unit: model.UnitKg}
}

func mealsPB(ms []model.Meal) []*pb.Meal {
	out := make([]*pb.Meal, len(ms))
	for i, m := range ms {
		out[i] = &pb.Meal{Name: m.Name, Time: m.Time, Desc: m.Desc, Items: make([]*pb.MealItem, len(m.Items))}
		for j, it := range m.Items {
			f := &pb.FrozenFood{Name: it.Food.Name, Ref: it.Food.Ref, Per100G: macrosPB(it.Food.Per100g), Unit: it.Food.Unit}
			if it.Food.PerUnit != nil {
				f.PerUnit = macrosPB(*it.Food.PerUnit)
			}
			out[i].Items[j] = &pb.MealItem{Food: f, Qty: it.Qty, Unit: it.Unit, Note: it.Note, Macros: macrosPB(it.Macros), Planned: it.Planned, Edited: it.Edited}
		}
	}
	return out
}

func mealsOf(ms []*pb.Meal) []model.Meal {
	out := make([]model.Meal, len(ms))
	for i, m := range ms {
		out[i] = model.Meal{Name: m.GetName(), Time: m.GetTime(), Desc: m.GetDesc(), Items: make([]model.Item, len(m.GetItems()))}
		for j, it := range m.GetItems() {
			f := it.GetFood()
			food := model.Food{Name: f.GetName(), Ref: f.GetRef(), Per100g: macrosOf(f.GetPer100G()), Unit: f.GetUnit()}
			if f.GetPerUnit() != nil {
				pu := macrosOf(f.GetPerUnit())
				food.PerUnit = &pu
			}
			out[i].Items[j] = model.Item{Food: food, Qty: it.GetQty(), Unit: it.GetUnit(), Note: it.GetNote(), Planned: it.GetPlanned(), Edited: it.GetEdited()}
		}
	}
	return out
}

func mealLogInput(m *pb.MealLog) (logs.MealLogInput, error) {
	id, err := parseID(m.GetId())
	if err != nil {
		return logs.MealLogInput{}, err
	}
	in := logs.MealLogInput{ID: id, Date: m.GetDate(), Meals: mealsOf(m.GetMeals()), DeviceID: m.GetDeviceId()}
	if m.GetProgramId() != "" {
		pid, err := parseID(m.GetProgramId())
		if err != nil {
			return in, err
		}
		in.ProgramID = &pid
	}
	if m.GetTarget() != nil {
		t := macrosOf(m.GetTarget())
		in.Target = &t
	}
	return in, nil
}

func checkinInput(m *pb.Checkin, u model.Unit) (logs.CheckinInput, error) {
	id, err := parseID(m.GetId())
	return logs.CheckinInput{ID: id, Date: m.GetDate(), WeightKg: kgOf(m.GetWeight(), u), BodyFat: m.BodyFat,
		Measurements: m.GetMeasurements(), Photo: m.GetPhoto(), DeviceID: m.GetDeviceId()}, err
}

func mealLogPB(l logs.MealLog) *pb.MealLog {
	out := &pb.MealLog{Id: l.ID.String(), Date: l.Date, Target: macrosPB(l.Target), Meals: mealsPB(l.Meals), Totals: macrosPB(l.Totals),
		Version: l.Version, DeviceId: l.DeviceID, UpdatedAt: timestamppb.New(l.UpdatedAt), DeletedAt: optTime(l.DeletedAt)}
	if l.ProgramID != nil {
		out.ProgramId = l.ProgramID.String()
	}
	return out
}

// checkinPB maps a check-in with whatever photo bytes the service attached.
func checkinPB(c logs.Checkin, unit model.Unit) *pb.Checkin {
	out := &pb.Checkin{Id: c.ID.String(), Date: c.Date, BodyFat: c.BodyFat, Measurements: c.Measurements, Photo: c.Photo,
		PhotoContentType: c.PhotoContentType, Version: c.Version, DeviceId: c.DeviceID, UpdatedAt: timestamppb.New(c.UpdatedAt),
		DeletedAt: optTime(c.DeletedAt)}
	if c.WeightKg != nil {
		out.Weight = weightPB(fromKg(*c.WeightKg, unit))
	}
	if c.PhotoID != nil {
		out.PhotoId = c.PhotoID.String()
	}
	return out
}

func dayLogPB(l logs.DayLog) *pb.DayLog {
	out := &pb.DayLog{Id: l.ID.String(), Date: l.Date, Status: statusPB(l.Status), Notes: l.Notes,
		Version: l.Version, DeviceId: l.DeviceID, UpdatedAt: timestamppb.New(l.UpdatedAt), DeletedAt: optTime(l.DeletedAt)}
	if r := l.Readiness; r != nil {
		out.Readiness = &pb.Readiness{Sleep: optInt32(r.Sleep), Soreness: optInt32(r.Soreness), Stress: optInt32(r.Stress)}
	}
	return out
}

func readinessDB(r *pb.Readiness) *logs.Readiness {
	if r == nil {
		return nil
	}
	return &logs.Readiness{Sleep: optInt(r.Sleep), Soreness: optInt(r.Soreness), Stress: optInt(r.Stress)}
}

func optInt32(v *int) *int32 {
	if v == nil {
		return nil
	}
	n := i32(*v)
	return &n
}

func optInt(v *int32) *int {
	if v == nil {
		return nil
	}
	n := int(*v)
	return &n
}

func resolvedSetDB(s *pb.ResolvedSet, unit model.Unit) model.ResolvedSet {
	return model.ResolvedSet{Reps: int(s.GetReps()), MinReps: int(s.GetMinReps()), Weight: weightDB(s.GetWeight(), unit), RPE: s.GetRpe(),
		Rest: int(s.GetRest()), Work: int(s.GetWork()), Amrap: s.GetAmrap(), Ask: s.GetAsk(), LogRPE: s.GetLogRpe(), Auto: s.GetAuto(),
		CountUp: s.GetCountUp(), Warmup: s.GetWarmup(), Label: s.GetLabel()}
}

func plannedPB(p model.RecordPlanned) *pb.ResolvedSet {
	return &pb.ResolvedSet{Reps: i32(p.Reps), MinReps: i32(p.MinReps), Weight: weightPB(model.Weight(p.Weight)), Rpe: p.RPE,
		Rest: i32(p.Rest), Work: i32(p.Work), Amrap: p.Amrap, Ask: p.Ask, LogRpe: p.LogRPE, Auto: p.Auto, CountUp: p.CountUp,
		Warmup: p.Warmup, Label: p.Label}
}

func entryPB(e model.RecordEntry) *pb.ExerciseEntry {
	out := &pb.ExerciseEntry{Name: e.Name, Equipment: e.Equipment, Label: e.Label, Alternates: e.Alternates, Current: i32(e.Current),
		PromptValues: e.PromptValues, Slug: e.Slug, Desc: e.Desc, Superset: e.Superset, Variation: i32(e.Variation),
		Unilateral: e.Unilateral, Sets: make([]*pb.SetEntry, len(e.Sets))}
	for i, s := range e.Sets {
		set := &pb.SetEntry{Planned: plannedPB(s.Planned), Completed: s.Completed, Reps: i32(s.Reps), Rpe: s.RPE,
			WorkTime: i32(s.WorkTime), RepsLeft: i32(s.RepsLeft), Edited: s.Edited}
		if s.Weight.Unit != "" { // a weight was logged
			set.Weight = weightPB(model.Weight(s.Weight))
		}
		out.Sets[i] = set
	}
	return out
}

func entriesDB(es []*pb.ExerciseEntry, unit model.Unit) []model.RecordEntry {
	out := make([]model.RecordEntry, len(es))
	for i, e := range es {
		out[i] = model.RecordEntry{Name: e.GetName(), Slug: e.GetSlug(), Equipment: e.GetEquipment(), Label: e.GetLabel(), Alternates: e.GetAlternates(),
			Current: int(e.GetCurrent()), Desc: e.GetDesc(), Superset: e.GetSuperset(), Variation: int(e.GetVariation()), PromptValues: e.GetPromptValues(),
			Unilateral: e.GetUnilateral(), Sets: make([]model.RecordSet, len(e.GetSets()))}
		for j, s := range e.GetSets() {
			planned := resolvedSetDB(s.GetPlanned(), unit)
			out[i].Sets[j] = model.RecordSet{Planned: model.PlanSet(planned), Completed: s.GetCompleted(), Reps: int(s.GetReps()),
				Weight: model.RecordWeight(weightDB(s.GetWeight(), unit)), RPE: s.GetRpe(), WorkTime: int(s.GetWorkTime()),
				RepsLeft: int(s.GetRepsLeft()), Warmup: planned.Warmup, Edited: s.GetEdited()}
		}
	}
	return out
}

func recordPB(r logs.WorkoutRecord) *pb.WorkoutRecord {
	out := &pb.WorkoutRecord{Id: r.ID.String(), Date: r.Date, Week: i32(r.Day.Week), Day: i32(r.Day.Day), DayName: r.Day.Name,
		DayDesc: r.Day.Desc, Kind: r.Day.Kind, Entries: make([]*pb.ExerciseEntry, len(r.Entries)), StartedAt: optTime(r.StartedAt),
		FinishedAt: optTime(r.FinishedAt), Finalized: r.Finalized, Version: r.Version, DeviceId: r.DeviceID, UpdatedAt: timestamppb.New(r.UpdatedAt),
		DeletedAt: optTime(r.DeletedAt), Modified: r.Modified}
	if r.ProgramID != nil {
		out.ProgramId = r.ProgramID.String()
	}
	plan := logs.PlanEntries(&r.Day, r.Entries)
	for i, e := range r.Entries {
		out.Entries[i] = entryPB(e)
		out.Entries[i].FromProgram = plan[i]
	}
	return out
}

func dayViewPB(v logs.DayView, unit model.Unit) *pb.DayView {
	out := &pb.DayView{Date: v.Date}
	if v.Log != nil {
		out.Log = dayLogPB(*v.Log)
	}
	if v.Plan != nil {
		out.Plan = resolvedDayPB(v.Plan)
		out.Plan.ProgramId, out.Plan.Date = v.PlanProgram.String(), v.Date
	}
	if mp := v.MealPlan; mp != nil {
		out.MealPlan = mealDayPB(mp.Meal)
		out.MealPlan.ProgramId, out.MealPlan.Date = v.MealProgram.String(), v.Date
		out.MealPlan.Week, out.MealPlan.Day, out.MealPlan.Kind = i32(mp.Week), i32(mp.Day), mp.Kind
	}
	if v.Workout != nil {
		out.Workout = recordPB(*v.Workout)
	}
	if v.Meal != nil {
		out.Meal = mealLogPB(*v.Meal)
	}
	if v.Checkin != nil {
		out.Checkin = checkinPB(*v.Checkin, unit)
	}
	return out
}

func timePtr(t *timestamppb.Timestamp) *time.Time {
	if t == nil {
		return nil
	}
	v := t.AsTime()
	return &v
}
