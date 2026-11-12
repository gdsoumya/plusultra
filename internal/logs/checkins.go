package logs

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

var measurementKey = regexp.MustCompile(`^[a-z_]{1,32}$`)

// Checkin is a body check-in. Weight is in kg; the API layer converts from the user's unit.
// Photo and PhotoContentType are set only when photos were asked for.
type Checkin struct {
	ID               uuid.UUID
	Date             string
	WeightKg         *float64
	BodyFat          *float64
	Measurements     map[string]float64
	PhotoID          *uuid.UUID
	Photo            []byte
	PhotoContentType string
	Version          int32
	DeviceID         string
	UpdatedAt        time.Time
	DeletedAt        *time.Time // set only on tombstones, which only sync returns
}

// CheckinInput is the client-supplied part of a check-in. An empty Photo keeps the stored one.
type CheckinInput struct {
	ID           uuid.UUID
	Date         string
	WeightKg     *float64
	BodyFat      *float64
	Measurements map[string]float64
	Photo        []byte
	DeviceID     string
}

// CheckinService reads and writes check-ins, one per date.
type CheckinService struct {
	q    *sqlc.Queries
	pool *pgxpool.Pool
}

// NewCheckinService returns a CheckinService on q; pool is used for transactions.
func NewCheckinService(q *sqlc.Queries, pool *pgxpool.Pool) *CheckinService {
	return &CheckinService{q: q, pool: pool}
}

// Create records the check-in for in.Date, replacing the one already there (it keeps its
// id and its version is bumped).
func (s *CheckinService) Create(ctx context.Context, userID uuid.UUID, in CheckinInput) (Checkin, error) {
	return s.save(ctx, userID, in, uuid.Nil)
}

// Update replaces the values of check-in id. It returns ErrConflict when in.Date belongs to another check-in.
func (s *CheckinService) Update(ctx context.Context, userID, id uuid.UUID, in CheckinInput) (Checkin, error) {
	return s.save(ctx, userID, in, id)
}

// save upserts a check-in; id is the one being updated, or Nil for a create.
func (s *CheckinService) save(ctx context.Context, userID uuid.UUID, in CheckinInput, id uuid.UUID) (Checkin, error) {
	date, params, err := encodeCheckin(userID, in)
	if err != nil {
		return Checkin{}, err
	}
	var row sqlc.Checkin
	err = inDayTx(ctx, s.pool, s.q, userID, date, "checkin", func(q *sqlc.Queries) error {
		old, err := q.GetCheckinByDate(ctx, sqlc.GetCheckinByDateParams{UserID: userID, Date: date})
		found := err == nil
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		if id != uuid.Nil {
			if found && old.ID != id {
				return ErrConflict
			}
			if old, err = q.GetCheckin(ctx, sqlc.GetCheckinParams{ID: id, UserID: userID}); err != nil {
				return notFound(err)
			}
			found = true
		}
		params.ID = in.ID
		if found {
			params.ID, params.PhotoID = old.ID, old.PhotoID
		} else if params.ID == uuid.Nil {
			params.ID = uuid.New()
		}
		if len(in.Photo) > 0 {
			pid, err := storePhoto(ctx, q, userID, in.Photo)
			if err != nil {
				return err
			}
			params.PhotoID = &pid
		}
		if row, err = q.UpsertCheckin(ctx, params); err != nil {
			if errors.Is(err, pgx.ErrNoRows) { // the id belongs to another user
				return ErrConflict
			}
			return err
		}
		if found && len(in.Photo) > 0 && old.PhotoID != nil {
			return q.DeletePhoto(ctx, sqlc.DeletePhotoParams{ID: *old.PhotoID, UserID: userID})
		}
		return nil
	})
	if err != nil {
		return Checkin{}, err
	}
	return s.one(ctx, userID, row, false)
}

// ClearPhoto removes the check-in's photo.
func (s *CheckinService) ClearPhoto(ctx context.Context, userID, id uuid.UUID) (Checkin, error) {
	var row sqlc.Checkin
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		old, err := q.GetCheckin(ctx, sqlc.GetCheckinParams{ID: id, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		if row = old; old.PhotoID == nil {
			return nil
		}
		if row, err = q.UpsertCheckin(ctx, upsertParams(old, nil)); err != nil {
			return err
		}
		return q.DeletePhoto(ctx, sqlc.DeletePhotoParams{ID: *old.PhotoID, UserID: userID})
	})
	if err != nil {
		return Checkin{}, err
	}
	return s.one(ctx, userID, row, false)
}

// Delete soft-deletes the check-in and removes its photo.
func (s *CheckinService) Delete(ctx context.Context, userID, id uuid.UUID) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		old, err := q.GetCheckin(ctx, sqlc.GetCheckinParams{ID: id, UserID: userID})
		if err != nil {
			return notFound(err)
		}
		if _, err := q.DeleteCheckin(ctx, sqlc.DeleteCheckinParams{ID: id, UserID: userID}); err != nil {
			return err
		}
		if old.PhotoID == nil {
			return nil
		}
		return q.DeletePhoto(ctx, sqlc.DeletePhotoParams{ID: *old.PhotoID, UserID: userID})
	})
}

// Get returns one check-in, with its photo bytes when includePhoto is set.
func (s *CheckinService) Get(ctx context.Context, userID, id uuid.UUID, includePhoto bool) (Checkin, error) {
	row, err := s.q.GetCheckin(ctx, sqlc.GetCheckinParams{ID: id, UserID: userID})
	if err != nil {
		return Checkin{}, notFound(err)
	}
	return s.one(ctx, userID, row, includePhoto)
}

// List returns the check-ins from..to (inclusive), oldest first, one page at a time.
func (s *CheckinService) List(ctx context.Context, userID uuid.UUID, from, to, cursor string, limit int, includePhotos bool) (Page[Checkin], error) {
	pq, err := newPageQuery(from, to, cursor, limit)
	if err != nil {
		return Page[Checkin]{}, err
	}
	rows, err := s.q.ListCheckinsPage(ctx, sqlc.ListCheckinsPageParams{
		UserID: userID, FromDate: pq.from, ToDate: pq.to, AfterDate: pq.afterDate, AfterID: pq.afterID, MaxRows: pq.fetch(),
	})
	if err != nil {
		return Page[Checkin]{}, err
	}
	p, err := paginate(pq, rows, func(r sqlc.Checkin) (time.Time, uuid.UUID) { return r.Date, r.ID }, DecodeCheckin)
	if err == nil && includePhotos {
		err = s.attachPhotos(ctx, userID, p.Items)
	}
	return p, err
}

func (s *CheckinService) one(ctx context.Context, userID uuid.UUID, row sqlc.Checkin, includePhoto bool) (Checkin, error) {
	c, err := DecodeCheckin(row)
	if err == nil && includePhoto {
		items := []Checkin{c}
		err = s.attachPhotos(ctx, userID, items)
		c = items[0]
	}
	return c, err
}

// attachPhotos fills in the photo bytes of cs with one query.
func (s *CheckinService) attachPhotos(ctx context.Context, userID uuid.UUID, cs []Checkin) error {
	var ids []uuid.UUID
	for _, c := range cs {
		if c.PhotoID != nil {
			ids = append(ids, *c.PhotoID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	photos, err := s.q.ListPhotosByID(ctx, sqlc.ListPhotosByIDParams{UserID: userID, Ids: ids})
	if err != nil {
		return err
	}
	byID := make(map[uuid.UUID]sqlc.Photo, len(photos))
	for _, p := range photos {
		byID[p.ID] = p
	}
	for i, c := range cs {
		if c.PhotoID != nil {
			cs[i].Photo, cs[i].PhotoContentType = byID[*c.PhotoID].Bytes, byID[*c.PhotoID].ContentType
		}
	}
	return nil
}

// encodeCheckin validates in and returns its date and upsert params (without id and photo).
func encodeCheckin(userID uuid.UUID, in CheckinInput) (time.Time, sqlc.UpsertCheckinParams, error) {
	p := sqlc.UpsertCheckinParams{UserID: userID, DeviceID: in.DeviceID}
	date, err := parseDate(in.Date)
	if err != nil {
		return date, p, err
	}
	p.Date = date
	if in.WeightKg != nil && (math.IsNaN(*in.WeightKg) || *in.WeightKg < 20 || *in.WeightKg > 500) {
		return date, p, invalid("weight must be 20-500 kg")
	}
	if in.BodyFat != nil && (math.IsNaN(*in.BodyFat) || *in.BodyFat < 2 || *in.BodyFat > 70) {
		return date, p, invalid("body fat must be 2-70 percent")
	}
	if len(in.Measurements) > 50 {
		return date, p, invalid("at most 50 measurements")
	}
	for k, v := range in.Measurements {
		if !measurementKey.MatchString(k) {
			return date, p, invalid("measurement name %q must be 1-32 letters or underscores", k)
		}
		if err := checkNumber("measurement "+k, v, 500); err != nil {
			return date, p, err
		}
	}
	if err := checkPhoto(in.Photo); err != nil {
		return date, p, err
	}
	if p.Weight, err = numeric(in.WeightKg); err != nil {
		return date, p, err
	}
	if p.BodyFat, err = numeric(in.BodyFat); err != nil {
		return date, p, err
	}
	m := in.Measurements
	if m == nil {
		m = map[string]float64{}
	}
	p.Measurements, err = json.Marshal(m)
	return date, p, err
}

// upsertParams are the params that rewrite c unchanged apart from its photo.
func upsertParams(c sqlc.Checkin, photo *uuid.UUID) sqlc.UpsertCheckinParams {
	return sqlc.UpsertCheckinParams{
		ID: c.ID, UserID: c.UserID, Date: c.Date, Weight: c.Weight, BodyFat: c.BodyFat,
		Measurements: c.Measurements, PhotoID: photo, DeviceID: c.DeviceID,
	}
}

// DecodeCheckin converts a stored row, without its photo bytes.
func DecodeCheckin(r sqlc.Checkin) (Checkin, error) {
	c := Checkin{ID: r.ID, Date: r.Date.Format(time.DateOnly), PhotoID: r.PhotoID, Version: r.Version, DeviceID: r.DeviceID, UpdatedAt: r.UpdatedAt, DeletedAt: r.DeletedAt}
	var err error
	if c.WeightKg, err = float(r.Weight); err != nil {
		return c, err
	}
	if c.BodyFat, err = float(r.BodyFat); err != nil {
		return c, err
	}
	return c, json.Unmarshal(r.Measurements, &c.Measurements)
}

func numeric(v *float64) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if v == nil {
		return n, nil
	}
	return n, n.Scan(strconv.FormatFloat(*v, 'f', -1, 64))
}

func float(n pgtype.Numeric) (*float64, error) {
	if !n.Valid {
		return nil, nil
	}
	f, err := n.Float64Value()
	return &f.Float64, err
}
