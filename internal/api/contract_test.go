package api

import (
	"errors"
	"strings"
	"testing"
	"time"

	"buf.build/go/protovalidate"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/timestamppb"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
)

func TestMessagesRoundTripProtoJSON(t *testing.T) {
	kg := func(v float64) *pb.Weight { return &pb.Weight{Value: v, Unit: pb.Unit_UNIT_KG} }
	now := timestamppb.New(time.Date(2026, 11, 11, 7, 30, 0, 0, time.UTC))
	day := &pb.ResolvedDay{
		ProgramId: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d", Date: "2026-11-11", Week: 1, Day: 3,
		Name: "Upper A", Kind: "training",
		Exercises: []*pb.ResolvedExercise{{
			Name: "Bench Press", Equipment: "barbell", Alternates: []string{"Bench Press"}, Current: 1, Unilateral: true,
			Progression: "Linear: +2.5 kg after a successful workout",
			Sets:        []*pb.ResolvedSet{{Reps: 5, MinReps: 5, Weight: kg(75), Rest: 180, Label: "Work"}},
		}},
	}
	meal := &pb.Meal{Name: "Lunch", Items: []*pb.MealItem{{
		Food: &pb.FrozenFood{Name: "Chicken breast, raw", Per100G: &pb.Macros{Kcal: 120, Protein: 22.5}},
		Qty:  200, Unit: "g", Macros: &pb.Macros{Kcal: 240, Protein: 45}, Planned: true,
	}}}
	record := &pb.WorkoutRecord{
		Id: "7d1c2b3a-4e5f-4a6b-8c7d-9e0f1a2b3c4d", Date: "2026-11-11", Week: 1, Day: 3,
		Entries: []*pb.ExerciseEntry{{
			Name: "Bench Press", Current: 1, Unilateral: true,
			Sets:         []*pb.SetEntry{{Planned: day.Exercises[0].Sets[0], Completed: true, Reps: 5, RepsLeft: 4, Weight: kg(75), Rpe: 8}},
			PromptValues: map[string]float64{"rir": 2},
		}},
		StartedAt: now, Version: 3, DeviceId: "pixel-8",
	}
	mealLog := &pb.MealLog{Date: "2026-11-11", Target: &pb.Macros{Kcal: 2600}, Meals: []*pb.Meal{meal}, Totals: &pb.Macros{Kcal: 240}}
	bodyFat := 18.5
	checkin := &pb.Checkin{Id: "5a4b3c2d-1e0f-4a9b-8c7d-6e5f4a3b2c1d", Date: "2026-11-11", Weight: kg(82.4), BodyFat: &bodyFat,
		Measurements: map[string]float64{"waist": 82}, PhotoId: "9f8e7d6c-5b4a-4392-8170-6f5e4d3c2b1a"}
	adHoc := &pb.CreateWorkoutRecordRequest{Id: record.Id, Date: "2026-11-11", Source: &pb.CreateWorkoutRecordRequest_AdHoc{AdHoc: &pb.AdHoc{
		DayName: "Hotel gym", Entries: []*pb.ExerciseEntry{{Name: "Push Up", Slug: "push-up", Sets: []*pb.SetEntry{{Planned: &pb.ResolvedSet{Reps: 20}}}}},
	}}}

	msgs := []proto.Message{
		&pb.RegisterResponse{User: &pb.User{Username: "alice", Units: pb.Unit_UNIT_KG, CreatedAt: now}, Token: "t"},
		&pb.CreateApiKeyResponse{Key: "lsk_abcd1234_secret", ApiKey: &pb.ApiKey{Name: "claude", Scopes: []pb.ApiKeyScope{pb.ApiKeyScope_API_KEY_SCOPE_READ}, ExpiresAt: now}},
		&pb.Program{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Status: pb.ProgramStatus_PROGRAM_STATUS_ACTIVE, Script: "program(...)",
			Structure: &pb.ProgramStructure{Weeks: 16, Days: []*pb.StructureDay{{Week: 1, Day: 2, Name: "Upper A"}}},
			Cursor:    &pb.ProgramCursor{NextWeek: 1, NextDay: 2}},
		&pb.ValidateProgramResponse{Errors: []*pb.ScriptError{{Line: 3, Col: 5, Message: "missing weight"}}, NameErrors: []*pb.NameError{{Line: 4, Name: "Bench", Suggestions: []string{"Bench Press"}}}},
		&pb.SimulateProgramRequest{Script: "program(...)", Weeks: 2, Assumptions: &pb.SimulateAssumptions{CompleteAll: true, Reps: map[string]int32{"Squat": 3}}},
		&pb.GetDayResponse{Day: &pb.DayView{Date: "2026-11-11", Plan: day, Workout: record, Meal: mealLog, Checkin: checkin,
			Log: &pb.DayLog{Status: pb.DayStatus_DAY_STATUS_DONE, Readiness: &pb.Readiness{Sleep: proto.Int32(4)}}}},
		&pb.GetCalendarResponse{Days: []*pb.CalendarDay{{Date: "2026-11-11", Kind: "training", WorkoutWeek: 1, WorkoutDay: 3, LogStatus: pb.DayStatus_DAY_STATUS_PLANNED}}},
		&pb.SearchExercisesResponse{Exercises: []*pb.Exercise{{Slug: "bench-press-barbell", Name: "Bench Press", PrimaryMuscles: []string{"chest"}}}},
		&pb.SetExerciseDataRequest{Data: &pb.ExerciseData{Slug: "bench-press-barbell", Rm1: kg(100), Rounding: &pb.Rounding{Kind: "plates", Bar: 20, Plates: []float64{25, 20, 10, 5, 2.5, 1.25}}}},
		&pb.SearchFoodsResponse{Foods: []*pb.Food{{Name: "Egg", Per100G: &pb.Macros{Kcal: 143}, Units: map[string]float64{"egg": 50}}}},
		&pb.ListConceptMembersResponse{Foods: []*pb.Food{{Id: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d", Source: "ifct", SourceId: "A015",
			Name: "Rice, raw, milled", ConceptId: "rice-white-raw", ConceptName: "Rice, white, raw"}}, PreferredFoodId: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d"},
		&pb.WeeklySummaryResponse{WeekStart: "2026-11-09", Adherence: &pb.AdherenceCounts{Planned: 4, Done: 3}, SetsPerMuscle: map[string]int32{"chest": 12},
			E1Rm: []*pb.LiftEstimate{{Slug: "bench-press-barbell", Name: "Bench Press", E1Rm: kg(98)}}},
		adHoc,
		&pb.UpdateCheckinRequest{Checkin: &pb.Checkin{Id: checkin.Id, Date: "2026-11-11", Photo: []byte{0xff, 0xd8}, PhotoContentType: "image/jpeg"}},
		&pb.ListExerciseDataResponse{Items: []*pb.ExerciseData{{Slug: "squat-barbell", Rm1: kg(140)}}},
		&pb.ListProgramsResponse{Programs: []*pb.Program{{Mode: pb.ProgramMode_PROGRAM_MODE_FLOATING}}, NextCursor: "c2", HasMore: true},
		&pb.ExportDaysRequest{Dates: &pb.DateRange{From: "2026-11-01", To: "2026-11-11"}, Format: pb.ExportFormat_EXPORT_FORMAT_CSV},
		&pb.PushResponse{AppliedIds: []string{record.Id}, Conflicts: &pb.SyncRows{MealLogs: []*pb.MealLog{mealLog}},
			Reasons: map[string]string{mealLog.Id: "date_taken", "9f8e7d6c-5b4a-4392-8170-6f5e4d3c2b1a": "invalid"}},
		&pb.PullResponse{Rows: &pb.SyncRows{WorkoutRecords: []*pb.WorkoutRecord{record}, MealLogs: []*pb.MealLog{mealLog}, Checkins: []*pb.Checkin{checkin}}, ResolvedDays: []*pb.ResolvedDay{day}, Cursor: "c1", HasMore: true},
	}
	for _, m := range msgs {
		t.Run(string(m.ProtoReflect().Descriptor().Name()), func(t *testing.T) {
			b, err := protojson.Marshal(m)
			if err != nil {
				t.Fatal(err)
			}
			got := m.ProtoReflect().Type().New().Interface()
			if err := protojson.Unmarshal(b, got); err != nil {
				t.Fatal(err)
			}
			if !proto.Equal(m, got) {
				t.Fatalf("round trip changed the message:\nwant %v\ngot  %v", m, got)
			}
		})
	}
}

func TestValidationRules(t *testing.T) {
	inMonth := timestamppb.New(time.Now().Add(30 * 24 * time.Hour))
	logWith := func(id string) *pb.MealLog { return &pb.MealLog{Id: id, Date: "2026-11-11"} }
	tests := []struct {
		name string
		msg  proto.Message
		rule string // expected violated rule id; empty means valid
	}{
		{"good register", &pb.RegisterRequest{Username: "alice_1", Password: "correct horse"}, ""},
		{"bad username", &pb.RegisterRequest{Username: "Alice!", Password: "correct horse"}, "string.pattern"},
		{"short password", &pb.RegisterRequest{Username: "alice", Password: "short"}, "string.min_len"},
		{"api key without expiry", &pb.CreateApiKeyRequest{Name: "k", Scopes: []pb.ApiKeyScope{pb.ApiKeyScope_API_KEY_SCOPE_READ}}, "required"},
		{"api key expiring in two years", &pb.CreateApiKeyRequest{Name: "k", Scopes: []pb.ApiKeyScope{pb.ApiKeyScope_API_KEY_SCOPE_READ},
			ExpiresAt: timestamppb.New(time.Now().Add(2 * 365 * 24 * time.Hour))}, "timestamp.within"},
		{"good api key", &pb.CreateApiKeyRequest{Name: "k", Scopes: []pb.ApiKeyScope{pb.ApiKeyScope_API_KEY_SCOPE_WRITE}, ExpiresAt: inMonth}, ""},
		{"limit over 500", &pb.ListWorkoutRecordsRequest{Dates: &pb.DateRange{From: "2026-11-01", To: "2026-11-11"}, Limit: 501}, "int32.gte_lte"},
		{"reversed dates", &pb.ListCheckinsRequest{Dates: &pb.DateRange{From: "2026-11-11", To: "2026-11-01"}}, "date_range.order"},
		{"script too large", &pb.CreateProgramRequest{Kind: pb.ProgramKind_PROGRAM_KIND_WORKOUT, Script: strings.Repeat("x", 262145)}, "string.max_bytes"},
		{"good date", &pb.GetDayRequest{Date: "2026-11-11"}, ""},
		{"bad date", &pb.GetDayRequest{Date: "2026-13-01"}, "string.date"},
		{"meal log with client id", &pb.CreateMealLogRequest{MealLog: logWith("0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d")}, ""},
		{"meal log without id", &pb.CreateMealLogRequest{MealLog: logWith("")}, "string.uuid_empty"},
		{"planned item not eaten", &pb.MealItem{Food: &pb.FrozenFood{Name: "Rice"}, Qty: 0}, ""},
		{"negative qty", &pb.MealItem{Food: &pb.FrozenFood{Name: "Rice"}, Qty: -1}, "double.gte_lte"},
		{"concept members without id", &pb.ListConceptMembersRequest{}, "string.min_len"},
		{"search one source", &pb.SearchFoodsRequest{Query: "rice", Source: "ifct"}, ""},
		{"search reference seed", &pb.SearchFoodsRequest{Query: "rice", Source: "reference"}, ""},
		{"search bare usda", &pb.SearchFoodsRequest{Query: "rice", Source: "usda"}, "string.in"},
		{"day update without status", &pb.UpdateDayRequest{Date: "2026-11-11", Notes: "slept badly"}, ""},
		{"photo and clear_photo", &pb.UpdateCheckinRequest{Checkin: &pb.Checkin{Id: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d", Date: "2026-11-11", Photo: []byte{1}}, ClearPhoto: true}, "update_checkin.photo"},
		{"photo in sync push", &pb.PushRequest{DeviceId: "pixel-8", Rows: &pb.SyncRows{Checkins: []*pb.Checkin{{Id: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d", Date: "2026-11-11", Photo: []byte{1}}}}}, "sync_rows.photo"},
		{"program day week 0", &pb.CreateWorkoutRecordRequest{Id: "0b6f3c1e-6a43-4f7e-9a51-1d2f3a4b5c6d", Date: "2026-11-11",
			Source: &pb.CreateWorkoutRecordRequest_ProgramDay{ProgramDay: &pb.ProgramDay{Week: 0, Day: 2}}}, "int32.gte"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := protovalidate.Validate(tt.msg)
			if tt.rule == "" {
				if err != nil {
					t.Fatalf("want valid, got %v", err)
				}
				return
			}
			var verr *protovalidate.ValidationError
			if !errors.As(err, &verr) {
				t.Fatalf("want a validation error, got %v", err)
			}
			for _, v := range verr.Violations {
				if v.Proto.GetRuleId() == tt.rule {
					return
				}
			}
			t.Fatalf("want rule %s, got %v", tt.rule, err)
		})
	}
}

// Rules are CEL compiled lazily, so a broken rule only fails when its message is validated.
func TestEveryMessageRuleCompiles(t *testing.T) {
	n := 0
	protoregistry.GlobalTypes.RangeMessages(func(mt protoreflect.MessageType) bool {
		if mt.Descriptor().ParentFile().Package() != "plusultra.v1" {
			return true
		}
		n++
		err := protovalidate.Validate(mt.New().Interface())
		var verr *protovalidate.ValidationError
		if err != nil && !errors.As(err, &verr) {
			t.Errorf("%s: %v", mt.Descriptor().FullName(), err)
		}
		return true
	})
	if n < 100 {
		t.Fatalf("checked only %d messages", n)
	}
}
