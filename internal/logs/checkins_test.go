package logs_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/logs"
)

var (
	pngHeader  = []byte("\x89PNG\r\n\x1a\n")
	jpegHeader = []byte("\xff\xd8\xff\xe0")
)

func photo(header []byte, size int) []byte {
	return append(bytes.Clone(header), make([]byte, size-len(header))...)
}

func TestCheckinPhotoTooLarge(t *testing.T) {
	e := setup(t)
	in := logs.CheckinInput{Date: "2026-03-02", Photo: photo(pngHeader, logs.MaxPhotoBytes+1)}
	if _, err := e.checkins.Create(e.ctx, e.user, in); !errors.Is(err, logs.ErrInvalid) {
		t.Fatalf("oversize: %v", err)
	}
	in.Photo = photo(pngHeader, logs.MaxPhotoBytes)
	c, err := e.checkins.Create(e.ctx, e.user, in)
	if err != nil {
		t.Fatalf("exactly 2 MiB: %v", err)
	}
	if c.PhotoID == nil {
		t.Fatal("no photo id")
	}
	n, err := e.q.ListPhotosByID(e.ctx, sqlcPhotosParams(e.user, *c.PhotoID))
	if err != nil || len(n) != 1 {
		t.Fatalf("photos %d %v", len(n), err)
	}
}

func TestOneCheckinPerDateUpsert(t *testing.T) {
	e := setup(t)
	first, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{ID: uuid.New(), Date: "2026-03-02", WeightKg: ptr(82.5), DeviceID: "phone"})
	if err != nil {
		t.Fatal(err)
	}
	near(t, "weight", *first.WeightKg, 82.5)
	second, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{ID: uuid.New(), Date: "2026-03-02", WeightKg: ptr(82.1), BodyFat: ptr(14.5),
		Measurements: map[string]float64{"waist": 81.5}, DeviceID: "tablet"})
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Version != 2 || second.DeviceID != "tablet" {
		t.Fatalf("second %+v", second)
	}
	near(t, "weight", *second.WeightKg, 82.1)
	near(t, "body fat", *second.BodyFat, 14.5)
	near(t, "waist", second.Measurements["waist"], 81.5)
	p, err := e.checkins.List(e.ctx, e.user, "2026-03-01", "2026-03-31", "", 10, false)
	if err != nil || len(p.Items) != 1 {
		t.Fatalf("list: %d %v", len(p.Items), err)
	}

	// Update onto another check-in's date conflicts.
	other, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{Date: "2026-03-03"})
	if err != nil {
		t.Fatal(err)
	}
	if other.WeightKg != nil || other.BodyFat != nil {
		t.Fatalf("unset values came back set: %+v", other)
	}
	if _, err := e.checkins.Update(e.ctx, e.user, other.ID, logs.CheckinInput{Date: "2026-03-02"}); !errors.Is(err, logs.ErrConflict) {
		t.Fatalf("update onto taken date: %v", err)
	}
	if _, err := e.checkins.Update(e.ctx, e.user, uuid.New(), logs.CheckinInput{Date: "2026-03-09"}); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("update missing: %v", err)
	}

	if err := e.checkins.Delete(e.ctx, e.user, first.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := e.checkins.Get(e.ctx, e.user, first.ID, false); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if err := e.checkins.Delete(e.ctx, e.user, first.ID); !errors.Is(err, logs.ErrNotFound) {
		t.Fatalf("delete twice: %v", err)
	}
	again, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{Date: "2026-03-02"})
	if err != nil || again.ID == first.ID {
		t.Fatalf("recreate after delete: %v id %v", err, again.ID)
	}
}

func TestCheckinPhotoKeepAndClear(t *testing.T) {
	e := setup(t)
	img := photo(jpegHeader, 64)
	c, err := e.checkins.Create(e.ctx, e.user, logs.CheckinInput{Date: "2026-03-02", WeightKg: ptr(80.0), Photo: img})
	if err != nil {
		t.Fatal(err)
	}
	if c.Photo != nil {
		t.Fatal("photo bytes returned without include")
	}

	// An update without a photo keeps the stored one.
	u, err := e.checkins.Update(e.ctx, e.user, c.ID, logs.CheckinInput{Date: "2026-03-02", WeightKg: ptr(79.0)})
	if err != nil {
		t.Fatal(err)
	}
	if u.PhotoID == nil || *u.PhotoID != *c.PhotoID {
		t.Fatalf("photo dropped: %v", u.PhotoID)
	}
	g, err := e.checkins.Get(e.ctx, e.user, c.ID, true)
	if err != nil || !bytes.Equal(g.Photo, img) || g.PhotoContentType != "image/jpeg" {
		t.Fatalf("get with photo: %v ct=%q", err, g.PhotoContentType)
	}
	g, err = e.checkins.Get(e.ctx, e.user, c.ID, false)
	if err != nil || g.Photo != nil || g.PhotoID == nil {
		t.Fatalf("get without photo: %+v %v", g, err)
	}
	p, err := e.checkins.List(e.ctx, e.user, "2026-03-01", "2026-03-31", "", 10, true)
	if err != nil || len(p.Items) != 1 || !bytes.Equal(p.Items[0].Photo, img) {
		t.Fatalf("list with photos: %v", err)
	}

	// A new photo replaces the old one, which is deleted.
	png := photo(pngHeader, 32)
	r, err := e.checkins.Update(e.ctx, e.user, c.ID, logs.CheckinInput{Date: "2026-03-02", Photo: png})
	if err != nil {
		t.Fatal(err)
	}
	if *r.PhotoID == *c.PhotoID {
		t.Fatal("photo id not replaced")
	}
	if n, _ := e.q.ListPhotosByID(e.ctx, sqlcPhotosParams(e.user, *c.PhotoID)); len(n) != 0 {
		t.Fatal("old photo kept")
	}

	cleared, err := e.checkins.ClearPhoto(e.ctx, e.user, c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if cleared.PhotoID != nil || cleared.Version != r.Version+1 {
		t.Fatalf("cleared %+v", cleared)
	}
	if n, _ := e.q.ListPhotosByID(e.ctx, sqlcPhotosParams(e.user, *r.PhotoID)); len(n) != 0 {
		t.Fatal("cleared photo kept")
	}
	if _, err := e.checkins.ClearPhoto(e.ctx, e.user, c.ID); err != nil {
		t.Fatalf("clearing twice: %v", err)
	}
}
