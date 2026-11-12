package api

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/logs"
	"github.com/gdsoumya/plusultra/internal/model"
	syncsvc "github.com/gdsoumya/plusultra/internal/sync"
)

// syncRowsIn converts pushed rows; a weight with no unit is in the user's unit. Device ids on
// rows are ignored, as the push's device id is recorded.
func syncRowsIn(r *pb.SyncRows, unit model.Unit) (syncsvc.Rows, error) {
	var out syncsvc.Rows
	for _, d := range r.GetDayLogs() {
		id, err := syncID(d.GetId())
		if err != nil {
			return out, err
		}
		out.DayLogs = append(out.DayLogs, logs.DayLog{
			ID: id, Date: d.GetDate(), Status: dayStatusNames[d.GetStatus()], Notes: d.GetNotes(), Readiness: readinessDB(d.GetReadiness()),
			Version: d.GetVersion(), UpdatedAt: syncUpdated(d.GetUpdatedAt()), DeletedAt: timePtr(d.GetDeletedAt()),
		})
	}
	for _, w := range r.GetWorkoutRecords() {
		x, err := syncRecordIn(w, unit)
		if err != nil {
			return out, err
		}
		out.WorkoutRecords = append(out.WorkoutRecords, x)
	}
	for _, m := range r.GetMealLogs() {
		x, err := syncMealLogIn(m)
		if err != nil {
			return out, err
		}
		out.MealLogs = append(out.MealLogs, x)
	}
	for _, c := range r.GetCheckins() {
		id, err := syncID(c.GetId())
		if err != nil {
			return out, err
		}
		// Photo bytes are kept so the service rejects them; the pushed photo id is not used.
		x := logs.Checkin{
			ID: id, Date: c.GetDate(), BodyFat: c.BodyFat, Measurements: c.GetMeasurements(), Photo: c.GetPhoto(),
			Version: c.GetVersion(), UpdatedAt: syncUpdated(c.GetUpdatedAt()), DeletedAt: timePtr(c.GetDeletedAt()),
		}
		if c.GetWeight() != nil {
			x.WeightKg = new(weightDB(c.GetWeight(), unit).Kg())
		}
		out.Checkins = append(out.Checkins, x)
	}
	return out, nil
}

// syncRecordIn takes the planned day's header; the workout service keeps a stored record's own.
func syncRecordIn(w *pb.WorkoutRecord, unit model.Unit) (logs.WorkoutRecord, error) {
	id, err := syncID(w.GetId())
	if err != nil {
		return logs.WorkoutRecord{}, err
	}
	out := logs.WorkoutRecord{
		ID: id, Date: w.GetDate(), Entries: entriesDB(w.GetEntries(), unit),
		Day:       model.ResolvedDay{Week: int(w.GetWeek()), Day: int(w.GetDay()), Name: w.GetDayName(), Desc: w.GetDayDesc(), Kind: w.GetKind()},
		StartedAt: timePtr(w.GetStartedAt()), FinishedAt: timePtr(w.GetFinishedAt()), Finalized: w.GetFinalized(),
		Version: w.GetVersion(), UpdatedAt: syncUpdated(w.GetUpdatedAt()), DeletedAt: timePtr(w.GetDeletedAt()),
	}
	out.ProgramID, err = syncOptIDIn(w.GetProgramId())
	return out, err
}

// syncMealLogIn ignores item macros and totals; the meal service computes them.
func syncMealLogIn(m *pb.MealLog) (logs.MealLog, error) {
	id, err := syncID(m.GetId())
	if err != nil {
		return logs.MealLog{}, err
	}
	out := logs.MealLog{
		ID: id, Date: m.GetDate(), Target: syncMacrosIn(m.GetTarget()), Meals: make([]model.Meal, len(m.GetMeals())),
		Version: m.GetVersion(), UpdatedAt: syncUpdated(m.GetUpdatedAt()), DeletedAt: timePtr(m.GetDeletedAt()),
	}
	for i, meal := range m.GetMeals() {
		out.Meals[i] = model.Meal{Name: meal.GetName(), Time: meal.GetTime(), Desc: meal.GetDesc(), Items: make([]model.Item, len(meal.GetItems()))}
		for j, it := range meal.GetItems() {
			f := it.GetFood()
			food := model.Food{Name: f.GetName(), Ref: f.GetRef(), Per100g: syncMacrosIn(f.GetPer100G()), Unit: f.GetUnit()}
			if f.GetPerUnit() != nil {
				food.PerUnit = new(syncMacrosIn(f.GetPerUnit()))
			}
			out.Meals[i].Items[j] = model.Item{Food: food, Qty: it.GetQty(), Unit: it.GetUnit(), Note: it.GetNote(), Planned: it.GetPlanned(), Edited: it.GetEdited()}
		}
	}
	out.ProgramID, err = syncOptIDIn(m.GetProgramId())
	return out, err
}

func syncMacrosIn(m *pb.Macros) model.Macros {
	return model.Macros{Kcal: m.GetKcal(), Protein: m.GetProtein(), Carbs: m.GetCarbs(), Fat: m.GetFat(), Fiber: m.GetFiber(), WaterML: m.GetWaterMl()}
}

func syncID(s string) (uuid.UUID, error) {
	id, err := uuid.Parse(s)
	if err != nil {
		return id, fmt.Errorf("row id %q is not a UUID", s)
	}
	return id, nil
}

func syncOptIDIn(s string) (*uuid.UUID, error) {
	if s == "" {
		return nil, nil
	}
	id, err := syncID(s)
	return &id, err
}

// syncUpdated is a pushed row's updated_at; a new row may leave it unset.
func syncUpdated(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime()
}
