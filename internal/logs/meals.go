package logs

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

// MealLog is what the user ate on one date. Totals are always the sum of the items.
type MealLog struct {
	ID        uuid.UUID
	Date      string
	ProgramID *uuid.UUID
	Target    model.Macros
	Meals     []model.Meal
	Totals    model.Macros
	Version   int32
	DeviceID  string
	UpdatedAt time.Time
	DeletedAt *time.Time // set only on tombstones, which only sync returns
}

// MealLogInput is the client-supplied part of a meal log. A zero ID is replaced by a new one.
type MealLogInput struct {
	ID        uuid.UUID
	Date      string
	ProgramID *uuid.UUID
	Target    *model.Macros
	Meals     []model.Meal
	DeviceID  string
}

// AdHocItem is a food added to a log outside the plan: a catalog food by FoodID, or Food
// with macros typed in by hand.
type AdHocItem struct {
	FoodID   uuid.UUID
	Food     *model.Food
	Qty      float64
	Unit     string // "g" or one of the catalog food's units; empty means grams
	Note     string
	DeviceID string
}

// MealService reads and writes meal logs.
type MealService struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewMealService returns a MealService on q; pool is used for the transactions that
// enforce one log per date.
func NewMealService(q *sqlc.Queries, pool *pgxpool.Pool) *MealService {
	return &MealService{q: q, pool: pool}
}

// Create adds the log for in.Date, or returns ErrConflict when the date already has one.
func (s *MealService) Create(ctx context.Context, userID uuid.UUID, in MealLogInput) (MealLog, error) {
	p, date, err := encodeMealLog(userID, in)
	if err != nil {
		return MealLog{}, err
	}
	if err := CheckProgram(ctx, s.q, userID, p.ProgramID, nil); err != nil {
		return MealLog{}, err
	}
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	var row sqlc.MealLog
	err = inDayTx(ctx, s.pool, s.q, userID, date, "meal", func(q *sqlc.Queries) error {
		if _, err := q.GetMealLogByDate(ctx, sqlc.GetMealLogByDateParams{UserID: userID, Date: date}); err == nil {
			return ErrConflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		// A live log with this id would be moved to the new date by the upsert.
		if _, err := q.GetMealLog(ctx, sqlc.GetMealLogParams{ID: p.ID, UserID: userID}); err == nil {
			return ErrConflict
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		row, err = q.UpsertMealLog(ctx, p)
		if errors.Is(err, pgx.ErrNoRows) { // the id belongs to another user
			return ErrConflict
		}
		return err
	})
	if err != nil {
		return MealLog{}, err
	}
	return DecodeMealLog(row)
}

// FillFromPlan creates the log for date from a planned meal day, marking every item planned.
// It returns ErrConflict when the date already has a log.
func (s *MealService) FillFromPlan(ctx context.Context, userID uuid.UUID, date string, day *model.MealDay, programID uuid.UUID) (MealLog, error) {
	in := MealLogInput{Date: date, Target: day.Target, Meals: make([]model.Meal, len(day.Meals))}
	if programID != uuid.Nil {
		in.ProgramID = &programID
	}
	for i, m := range day.Meals {
		in.Meals[i] = m
		in.Meals[i].Items = make([]model.Item, len(m.Items))
		for j, it := range m.Items {
			it.Planned = true
			in.Meals[i].Items[j] = it
		}
	}
	return s.Create(ctx, userID, in)
}

// Update replaces the log's target, meals and program. The date cannot change.
func (s *MealService) Update(ctx context.Context, userID, id uuid.UUID, in MealLogInput) (MealLog, error) {
	return update(ctx, s.q, userID, id, in)
}

func update(ctx context.Context, q *sqlc.Queries, userID, id uuid.UUID, in MealLogInput) (MealLog, error) {
	p, date, err := encodeMealLog(userID, in)
	if err != nil {
		return MealLog{}, err
	}
	old, err := q.GetMealLog(ctx, sqlc.GetMealLogParams{ID: id, UserID: userID})
	if err != nil {
		return MealLog{}, notFound(err)
	}
	if !old.Date.Equal(date) {
		return MealLog{}, invalid("a meal log's date cannot change")
	}
	if err := CheckProgram(ctx, q, userID, p.ProgramID, old.ProgramID); err != nil {
		return MealLog{}, err
	}
	row, err := q.UpdateMealLog(ctx, sqlc.UpdateMealLogParams{
		ID: id, UserID: userID, Target: p.Target, Meals: p.Meals, ProgramID: p.ProgramID, DeviceID: p.DeviceID,
	})
	if err != nil {
		return MealLog{}, notFound(err)
	}
	return DecodeMealLog(row)
}

// Delete soft-deletes the log, leaving a tombstone for sync.
func (s *MealService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	n, err := s.q.DeleteMealLog(ctx, sqlc.DeleteMealLogParams{ID: id, UserID: userID})
	if err == nil && n == 0 {
		return ErrNotFound
	}
	return err
}

// Get returns one log.
func (s *MealService) Get(ctx context.Context, userID, id uuid.UUID) (MealLog, error) {
	row, err := s.q.GetMealLog(ctx, sqlc.GetMealLogParams{ID: id, UserID: userID})
	if err != nil {
		return MealLog{}, notFound(err)
	}
	return DecodeMealLog(row)
}

// ByDate returns the log for a date.
func (s *MealService) ByDate(ctx context.Context, userID uuid.UUID, date string) (MealLog, error) {
	d, err := parseDate(date)
	if err != nil {
		return MealLog{}, err
	}
	row, err := s.q.GetMealLogByDate(ctx, sqlc.GetMealLogByDateParams{UserID: userID, Date: d})
	if err != nil {
		return MealLog{}, notFound(err)
	}
	return DecodeMealLog(row)
}

// List returns the logs from..to (inclusive), oldest first, one page at a time. An empty
// cursor starts at the beginning; limit 0 means 100.
func (s *MealService) List(ctx context.Context, userID uuid.UUID, from, to, cursor string, limit int) (Page[MealLog], error) {
	pq, err := newPageQuery(from, to, cursor, limit)
	if err != nil {
		return Page[MealLog]{}, err
	}
	rows, err := s.q.ListMealLogsPage(ctx, sqlc.ListMealLogsPageParams{
		UserID: userID, FromDate: pq.from, ToDate: pq.to, AfterDate: pq.afterDate, AfterID: pq.afterID, MaxRows: pq.fetch(),
	})
	if err != nil {
		return Page[MealLog]{}, err
	}
	return paginate(pq, rows, func(r sqlc.MealLog) (time.Time, uuid.UUID) { return r.Date, r.ID }, DecodeMealLog)
}

// AddAdHocItem appends an item to the named meal of a log, creating the meal when it is
// missing. Catalog macros are frozen into the item.
func (s *MealService) AddAdHocItem(ctx context.Context, userID, logID uuid.UUID, meal string, in AdHocItem) (MealLog, error) {
	food, unit, err := s.adHocFood(ctx, userID, in)
	if err != nil {
		return MealLog{}, err
	}
	l, err := s.Get(ctx, userID, logID)
	if err != nil {
		return MealLog{}, err
	}
	date, _ := parseDate(l.Date)
	var out MealLog
	// Re-read under the date lock so concurrent adds do not overwrite each other.
	err = inDayTx(ctx, s.pool, s.q, userID, date, "meal", func(q *sqlc.Queries) error {
		row, err := q.GetMealLog(ctx, sqlc.GetMealLogParams{ID: logID, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		if l, err = DecodeMealLog(row); err != nil {
			return err
		}
		i := 0
		for i < len(l.Meals) && l.Meals[i].Name != meal {
			i++
		}
		if i == len(l.Meals) {
			l.Meals = append(l.Meals, model.Meal{Name: meal})
		}
		l.Meals[i].Items = append(l.Meals[i].Items, model.Item{Food: food, Qty: in.Qty, Unit: unit, Note: in.Note})
		out, err = update(ctx, q, userID, logID, MealLogInput{
			Date: l.Date, ProgramID: l.ProgramID, Target: &l.Target, Meals: l.Meals, DeviceID: in.DeviceID,
		})
		return err
	})
	return out, err
}

// adHocFood returns the frozen food for in and the unit its quantity is in.
func (s *MealService) adHocFood(ctx context.Context, userID uuid.UUID, in AdHocItem) (model.Food, string, error) {
	unit := in.Unit
	if unit == "" {
		unit = gram
	}
	if in.Food != nil {
		return *in.Food, unit, nil
	}
	if in.FoodID == uuid.Nil {
		return model.Food{}, "", invalid("a food id or a custom food is required")
	}
	f, err := catalog.GetFood(ctx, s.q, userID, in.FoodID)
	if errors.Is(err, catalog.ErrNotFound) {
		return model.Food{}, "", ErrNotFound
	}
	if err != nil {
		return model.Food{}, "", err
	}
	food := model.Food{Name: f.Name, Ref: f.ID.String()}
	var units []catalog.Unit
	if err := json.Unmarshal(f.Per100g, &food.Per100g); err != nil {
		return model.Food{}, "", err
	}
	if err := json.Unmarshal(f.Units, &units); err != nil {
		return model.Food{}, "", err
	}
	if unit == gram {
		return food, unit, nil
	}
	for _, u := range units {
		if u.Name == unit {
			pu := food.Per100g.Scale(u.Grams / 100)
			food.PerUnit, food.Unit = &pu, u.Name
			return food, unit, nil
		}
	}
	return model.Food{}, "", invalid("%s has no unit %q", f.Name, unit)
}

// CheckProgram returns ErrInvalid when programID is set, differs from the row's current
// program old, and is not a live program of userID. A row keeps a program deleted after it.
func CheckProgram(ctx context.Context, q *sqlc.Queries, userID uuid.UUID, programID, old *uuid.UUID) error {
	if programID == nil || (old != nil && *old == *programID) {
		return nil
	}
	_, err := q.GetProgram(ctx, sqlc.GetProgramParams{ID: *programID, UserID: userID})
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrUnknownProgram
	}
	return err
}

// encodeMealLog validates in and returns the upsert params for it.
func encodeMealLog(userID uuid.UUID, in MealLogInput) (sqlc.UpsertMealLogParams, time.Time, error) {
	date, err := parseDate(in.Date)
	if err != nil {
		return sqlc.UpsertMealLogParams{}, date, err
	}
	var target model.Macros
	if in.Target != nil {
		target = *in.Target
	}
	if err := checkTarget(target); err != nil {
		return sqlc.UpsertMealLogParams{}, date, err
	}
	meals, err := normalizeMeals(in.Meals)
	if err != nil {
		return sqlc.UpsertMealLogParams{}, date, err
	}
	p := sqlc.UpsertMealLogParams{ID: in.ID, UserID: userID, Date: date, ProgramID: in.ProgramID, DeviceID: in.DeviceID}
	if p.Target, err = json.Marshal(target); err != nil {
		return p, date, err
	}
	p.Meals, err = json.Marshal(meals)
	return p, date, err
}

// DecodeMealLog converts a stored row, computing its totals.
func DecodeMealLog(r sqlc.MealLog) (MealLog, error) {
	l := MealLog{ID: r.ID, Date: r.Date.Format(time.DateOnly), ProgramID: r.ProgramID, Version: r.Version, DeviceID: r.DeviceID, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt}
	if err := json.Unmarshal(r.Target, &l.Target); err != nil {
		return l, err
	}
	if err := json.Unmarshal(r.Meals, &l.Meals); err != nil {
		return l, err
	}
	l.Totals = (&model.MealDay{Meals: l.Meals}).Totals()
	return l, nil
}

func notFound(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
