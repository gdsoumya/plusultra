package api

import (
	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/model"
	syncsvc "github.com/gdsoumya/plusultra/internal/sync"
)

// syncPullPB maps a pull, with check-in weights in the user's unit.
func syncPullPB(c syncsvc.Changes, unit model.Unit) (*pb.PullResponse, error) {
	out := &pb.PullResponse{Rows: syncRowsPB(c.Rows, unit), Cursor: c.Cursor, HasMore: c.HasMore}
	for _, p := range c.Programs {
		x, err := programPB(p, false)
		if err != nil {
			return nil, err
		}
		if cur, ok := c.Cursors[p.ID]; ok {
			x.Cursor = &pb.ProgramCursor{NextWeek: cur.NextWeek, NextDay: cur.NextDay}
		}
		out.Programs = append(out.Programs, x)
	}
	for _, d := range c.ResolvedDays {
		x := resolvedDayPB(d.Day.Workout)
		x.ProgramId, x.Date = d.ProgramID.String(), dateString(d.Day.Date)
		out.ResolvedDays = append(out.ResolvedDays, x)
	}
	for _, d := range c.ResolvedMealDays {
		day := d.Day
		x := mealDayPB(day.Meal)
		x.ProgramId, x.Date, x.Week, x.Day, x.Kind = d.ProgramID.String(), dateString(day.Date), i32(day.Week), i32(day.Day), day.Kind
		out.ResolvedMealDays = append(out.ResolvedMealDays, x)
	}
	return out, nil
}

func syncRowsPB(r syncsvc.Rows, unit model.Unit) *pb.SyncRows {
	out := &pb.SyncRows{}
	for _, d := range r.DayLogs {
		out.DayLogs = append(out.DayLogs, dayLogPB(d))
	}
	for _, w := range r.WorkoutRecords {
		out.WorkoutRecords = append(out.WorkoutRecords, recordPB(w))
	}
	for _, m := range r.MealLogs {
		out.MealLogs = append(out.MealLogs, mealLogPB(m))
	}
	for _, c := range r.Checkins {
		out.Checkins = append(out.Checkins, checkinPB(c, unit))
	}
	return out
}
