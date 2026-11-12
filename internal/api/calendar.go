package api

import (
	"context"

	"connectrpc.com/connect"
	"github.com/google/uuid"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/internal/auth"
	"github.com/gdsoumya/plusultra/internal/logs"
)

type calendarServer struct{ Deps }

func (s *calendarServer) GetCalendar(ctx context.Context, req *connect.Request[pb.GetCalendarRequest]) (*connect.Response[pb.GetCalendarResponse], error) {
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	days, err := s.Logs.GetCalendar(ctx, id, req.Msg.GetDates().GetFrom(), req.Msg.GetDates().GetTo())
	if err != nil {
		return nil, logsError(err)
	}
	res := &pb.GetCalendarResponse{Days: make([]*pb.CalendarDay, len(days))}
	for i, d := range days {
		res.Days[i] = &pb.CalendarDay{Date: d.Date, Kind: d.Kind, WorkoutWeek: i32(d.WorkoutWeek), WorkoutDay: i32(d.WorkoutDay),
			WorkoutName: d.WorkoutName, MealKind: d.MealKind, LogStatus: statusPB(d.LogStatus)}
	}
	return connect.NewResponse(res), nil
}

func (s *calendarServer) GetDay(ctx context.Context, req *connect.Request[pb.GetDayRequest]) (*connect.Response[pb.GetDayResponse], error) {
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	day, err := s.dayView(ctx, id, req.Msg.GetDate())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.GetDayResponse{Day: day}), nil
}

func (s *calendarServer) dayView(ctx context.Context, id uuid.UUID, date string) (*pb.DayView, error) {
	unit, err := s.userUnit(ctx)
	if err != nil {
		return nil, err
	}
	v, err := s.Logs.GetDay(ctx, id, date)
	if err != nil {
		return nil, logsError(err)
	}
	return dayViewPB(v, unit), nil
}

func (s *calendarServer) UpdateDay(ctx context.Context, req *connect.Request[pb.UpdateDayRequest]) (*connect.Response[pb.UpdateDayResponse], error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return nil, err
	}
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	in := logs.DayLogInput{Date: m.GetDate(), Notes: m.GetNotes(), Readiness: readinessDB(m.GetReadiness())}
	if m.Status != nil {
		in.Status = dayStatusNames[m.GetStatus()] // protovalidate allows only defined, non-zero statuses
	}
	log, err := s.Logs.UpdateDay(ctx, id, in)
	if err != nil {
		return nil, logsError(err)
	}
	return connect.NewResponse(&pb.UpdateDayResponse{Log: dayLogPB(log)}), nil
}

func (s *calendarServer) QuickFillDay(ctx context.Context, req *connect.Request[pb.QuickFillDayRequest]) (*connect.Response[pb.QuickFillDayResponse], error) {
	if err := auth.RequireWrite(ctx); err != nil {
		return nil, err
	}
	id, err := callerID(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if err := s.Logs.QuickFill(ctx, id, m.GetDate(), logs.QuickFill{Workout: m.GetWorkout(), Meals: m.GetMeals(), Finalize: m.GetFinalize()}); err != nil {
		return nil, logsError(err)
	}
	day, err := s.dayView(ctx, id, m.GetDate())
	if err != nil {
		return nil, err
	}
	return connect.NewResponse(&pb.QuickFillDayResponse{Day: day}), nil
}
