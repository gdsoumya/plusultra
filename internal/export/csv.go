package export

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"slices"
	"strconv"

	"github.com/gdsoumya/plusultra/internal/model"
)

// csvHeader is the columns of the day CSV: one row per set, meal item and
// check-in, told apart by row, with the day's columns repeated on each.
var csvHeader = []string{"date", "status", "notes", "readiness", "row",
	"program", "week", "day", "day_name", "minutes", "finalized", "modified", "exercise", "label", "set", "warmup",
	"planned_reps", "planned_min_reps", "planned_weight", "planned_unit", "planned_rpe", "planned_rest", "planned_amrap", "planned_ask", "set_label",
	"completed", "reps", "weight", "unit", "rpe", "work_time", "reps_left", "edited",
	"meal", "ad_hoc", "food", "ref", "qty", "qty_unit", "note", "kcal", "protein", "carbs", "fat", "fiber", "water_ml", "planned_item",
	"target_kcal", "target_protein", "target_carbs", "target_fat",
	"weight_kg", "body_fat", "measurements"}

type cells map[string]string

func formatCSV(days []Day) []byte {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	_ = w.Write(csvHeader)
	for _, d := range days {
		n := 0
		row := func(kind string, c cells) {
			r := make([]string, len(csvHeader))
			r[0], r[1], r[2], r[3], r[4] = d.Date, d.Status, d.Notes, string(d.Readiness), kind
			for k, v := range c {
				r[slices.Index(csvHeader, k)] = v
			}
			_ = w.Write(r)
			n++
		}
		for _, wk := range d.workouts() {
			for _, e := range wk.Entries {
				for i, s := range e.Sets {
					row("set", setCells(wk, e, i, s))
				}
			}
		}
		if m := d.Meals; m != nil {
			for _, meal := range m.Meals {
				adHoc := !slices.ContainsFunc(meal.Items, func(it model.Item) bool { return it.Planned })
				for _, it := range meal.Items {
					row("item", itemCells(m, meal, adHoc, it))
				}
			}
		}
		if c := d.Checkin; c != nil {
			m, _ := json.Marshal(c.Measurements)
			row("checkin", cells{"weight_kg": optNum(c.WeightKg), "body_fat": optNum(c.BodyFat), "measurements": string(m)})
		}
		if n == 0 {
			row("day", nil)
		}
	}
	w.Flush()
	return buf.Bytes()
}

func plain(x float64) string { return strconv.FormatFloat(x, 'f', -1, 64) }

func setCells(w *Workout, e Entry, i int, s model.RecordSet) cells {
	p := s.Planned
	return cells{"program": w.Program, "week": strconv.Itoa(w.Week), "day": strconv.Itoa(w.Day), "day_name": w.DayName,
		"minutes": strconv.Itoa(w.minutes()), "finalized": strconv.FormatBool(w.Finalized), "modified": strconv.FormatBool(w.Modified), "exercise": e.Name, "label": e.Label,
		"set": strconv.Itoa(i + 1), "warmup": strconv.FormatBool(s.Warmup || p.Warmup),
		"planned_reps": strconv.Itoa(p.Reps), "planned_min_reps": strconv.Itoa(p.MinReps), "planned_weight": plain(p.Weight.Value), "planned_unit": string(p.Weight.Unit),
		"planned_rpe": plain(p.RPE), "planned_rest": strconv.Itoa(p.Rest), "planned_amrap": strconv.FormatBool(p.Amrap), "planned_ask": strconv.FormatBool(p.Ask), "set_label": p.Label,
		"completed": strconv.FormatBool(s.Completed), "reps": strconv.Itoa(s.Reps), "weight": plain(s.Weight.Value), "unit": string(s.Weight.Unit),
		"rpe": plain(s.RPE), "work_time": strconv.Itoa(s.WorkTime), "reps_left": strconv.Itoa(s.RepsLeft), "edited": strconv.FormatBool(s.Edited)}
}

func itemCells(m *Meals, meal model.Meal, adHoc bool, it model.Item) cells {
	return cells{"meal": meal.Name, "ad_hoc": strconv.FormatBool(adHoc), "food": it.Food.Name, "ref": it.Food.Ref, "qty": plain(it.Qty), "qty_unit": it.Unit, "note": it.Note,
		"kcal": plain(it.Macros.Kcal), "protein": plain(it.Macros.Protein), "carbs": plain(it.Macros.Carbs), "fat": plain(it.Macros.Fat), "fiber": plain(it.Macros.Fiber),
		"water_ml": plain(it.Macros.WaterML), "planned_item": strconv.FormatBool(it.Planned),
		"target_kcal": plain(m.Target.Kcal), "target_protein": plain(m.Target.Protein), "target_carbs": plain(m.Target.Carbs), "target_fat": plain(m.Target.Fat)}
}
