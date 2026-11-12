package api

import (
	"bytes"
	"math"
	"net/http"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"google.golang.org/protobuf/proto"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
)

// pngBytes is n bytes that sniff as a PNG.
func pngBytes(n int) []byte {
	b := make([]byte, n)
	copy(b, "\x89PNG\r\n\x1a\n")
	return b
}

func TestCheckinsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	e.setUnits(t, tok, pb.Unit_UNIT_LB)
	c := plusultrav1connect.NewCheckinServiceClient(http.DefaultClient, e.url)
	dates := &pb.DateRange{From: "2026-11-01", To: "2026-11-30"}

	// An unspecified unit is the user's (lb); the largest allowed photo needs the 4 MiB read limit.
	in := &pb.Checkin{Id: uuid.NewString(), Date: "2026-11-10", Weight: &pb.Weight{Value: 180}, BodyFat: proto.Float64(20),
		Measurements: map[string]float64{"waist": 32}, Photo: pngBytes(2 << 20), PhotoContentType: "image/png", DeviceId: "phone"}
	created, err := c.CreateCheckin(ctx, as(tok, &pb.CreateCheckinRequest{Checkin: in}))
	if err != nil {
		t.Fatal(err)
	}
	got := created.Msg.GetCheckin()
	if got.GetId() != in.GetId() || got.GetDate() != "2026-11-10" || got.GetWeight().GetUnit() != pb.Unit_UNIT_LB || math.Abs(got.GetWeight().GetValue()-180) > 1e-6 ||
		got.GetBodyFat() != 20 || got.GetMeasurements()["waist"] != 32 || got.GetPhotoId() == "" || len(got.GetPhoto()) != 0 ||
		got.GetVersion() != 1 || got.GetDeviceId() != "phone" || got.GetUpdatedAt() == nil {
		t.Fatalf("created %v", got)
	}
	photoID := got.GetPhotoId()

	list, err := c.ListCheckins(ctx, as(tok, &pb.ListCheckinsRequest{Dates: dates}))
	if err != nil || len(list.Msg.GetCheckins()) != 1 || len(list.Msg.GetCheckins()[0].GetPhoto()) != 0 || list.Msg.GetCheckins()[0].GetPhotoId() != photoID {
		t.Fatalf("list = %v, err %v", list, err)
	}
	withPhotos, err := c.ListCheckins(ctx, as(tok, &pb.ListCheckinsRequest{Dates: dates, IncludePhotos: true}))
	if err != nil || len(withPhotos.Msg.GetCheckins()) != 1 {
		t.Fatalf("list with photos = %v, err %v", withPhotos, err)
	}
	if p := withPhotos.Msg.GetCheckins()[0]; !bytes.Equal(p.GetPhoto(), in.GetPhoto()) || p.GetPhotoContentType() != "image/png" {
		t.Fatalf("photo = %d bytes %q", len(p.GetPhoto()), p.GetPhotoContentType())
	}

	// Stored in kg: the same check-in reads back converted once the user switches units.
	e.setUnits(t, tok, pb.Unit_UNIT_KG)
	inKg := call(c.ListCheckins(ctx, as(tok, &pb.ListCheckinsRequest{Dates: dates}))).ok(t).GetCheckins()[0].GetWeight()
	if inKg.GetUnit() != pb.Unit_UNIT_KG || math.Abs(inKg.GetValue()-81.6466266) > 1e-3 {
		t.Fatalf("weight in kg = %v", inKg)
	}

	// An explicit unit is honoured; an empty photo keeps the stored one.
	in.Weight, in.Photo, in.PhotoContentType = &pb.Weight{Value: 176.37, Unit: pb.Unit_UNIT_LB}, nil, ""
	up := call(c.UpdateCheckin(ctx, as(tok, &pb.UpdateCheckinRequest{Checkin: in}))).ok(t).GetCheckin()
	if math.Abs(up.GetWeight().GetValue()-80) > 0.01 || up.GetPhotoId() != photoID || up.GetVersion() != 2 {
		t.Fatalf("update = %v", up)
	}
	cleared := call(c.UpdateCheckin(ctx, as(tok, &pb.UpdateCheckinRequest{Checkin: in, ClearPhoto: true}))).ok(t).GetCheckin()
	if cleared.GetPhotoId() != "" {
		t.Fatalf("clear photo = %v", cleared)
	}
	both := &pb.Checkin{Id: in.GetId(), Date: in.GetDate(), Photo: pngBytes(16)}
	_, err = c.UpdateCheckin(ctx, as(tok, &pb.UpdateCheckinRequest{Checkin: both, ClearPhoto: true}))
	assertCode(t, err, connect.CodeInvalidArgument)

	// Creating for a date that has a check-in replaces its values and keeps its id.
	again := call(c.CreateCheckin(ctx, as(tok, &pb.CreateCheckinRequest{Checkin: &pb.Checkin{Id: uuid.NewString(), Date: "2026-11-10", Weight: &pb.Weight{Value: 79}}}))).ok(t).GetCheckin()
	if again.GetId() != in.GetId() || again.GetVersion() <= cleared.GetVersion() || again.BodyFat != nil {
		t.Fatalf("create again = %v", again)
	}

	for _, bad := range []*pb.Checkin{
		{Id: uuid.NewString(), Date: "2026-11-11", Weight: &pb.Weight{Value: 10, Unit: pb.Unit_UNIT_KG}},
		{Id: uuid.NewString(), Date: "2026-11-11", Photo: []byte("not an image at all")},
		{Id: uuid.NewString(), Date: "2026-11-11", Measurements: map[string]float64{"Waist Size": 32}},
	} {
		_, err = c.CreateCheckin(ctx, as(tok, &pb.CreateCheckinRequest{Checkin: bad}))
		assertCode(t, err, connect.CodeInvalidArgument)
	}

	bob := e.invitee(t, tok, "bob").GetToken()
	_, err = c.UpdateCheckin(ctx, as(bob, &pb.UpdateCheckinRequest{Checkin: in}))
	assertCode(t, err, connect.CodeNotFound)
	_, err = c.DeleteCheckin(ctx, as(bob, &pb.DeleteCheckinRequest{Id: in.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	if list, err := c.ListCheckins(ctx, as(bob, &pb.ListCheckinsRequest{Dates: dates, IncludePhotos: true})); err != nil || len(list.Msg.GetCheckins()) != 0 {
		t.Fatalf("bob sees alice's check-ins: %v, err %v", list, err)
	}
	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.DeleteCheckin(ctx, as(ro, &pb.DeleteCheckinRequest{Id: in.GetId()}))
	assertCode(t, err, connect.CodePermissionDenied)

	if _, err := c.DeleteCheckin(ctx, as(tok, &pb.DeleteCheckinRequest{Id: in.GetId()})); err != nil {
		t.Fatal(err)
	}
	if list, err := c.ListCheckins(ctx, as(tok, &pb.ListCheckinsRequest{Dates: dates})); err != nil || len(list.Msg.GetCheckins()) != 0 {
		t.Fatalf("after delete = %v, err %v", list, err)
	}
}
