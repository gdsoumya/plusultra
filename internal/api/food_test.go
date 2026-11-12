package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"connectrpc.com/connect"

	pb "github.com/gdsoumya/plusultra/gen/go/plusultra/v1"
	"github.com/gdsoumya/plusultra/gen/go/plusultra/v1/plusultrav1connect"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
)

type toServer struct{ base *url.URL }

func (r toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = r.base.Scheme, r.base.Host
	return http.DefaultTransport.RoundTrip(req)
}

// fakeOFF points barcode lookups at h for the rest of the test.
func fakeOFF(t *testing.T, h http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	old := offClient.Transport
	offClient.Transport = toServer{u}
	t.Cleanup(func() { offClient.Transport = old })
}

const offBody = `{"status":1,"product":{"product_name":"Choco Spread","nutriments":{
"energy-kcal_100g":539,"proteins_100g":6.3,"carbohydrates_100g":57.5,"fat_100g":30.9,"fiber_100g":0}}}`

func TestFoodsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	c := plusultrav1connect.NewFoodServiceClient(http.DefaultClient, e.url)

	in := &pb.Food{Name: "Paneer", Aliases: []string{"cottage cheese"}, Per100G: &pb.Macros{Kcal: 265, Protein: 18, Fat: 20}, Units: map[string]float64{"cube": 20}}
	created, err := c.CreateCustomFood(ctx, as(tok, &pb.CreateCustomFoodRequest{Food: in}))
	if err != nil {
		t.Fatal(err)
	}
	f := created.Msg.GetFood()
	if f.GetId() == "" || f.GetSource() != "custom" || f.GetSourceId() == "" || !f.GetCustom() || f.GetPer100G().GetKcal() != 265 ||
		f.GetUnits()["cube"] != 20 || len(f.GetAliases()) != 1 || f.GetBarcode() != "" {
		t.Fatalf("created %v", f)
	}
	got, err := c.GetFood(ctx, as(tok, &pb.GetFoodRequest{Id: f.GetId()}))
	if err != nil || got.Msg.GetFood().GetName() != "Paneer" || got.Msg.GetFood().GetUnits()["cube"] != 20 {
		t.Fatalf("get = %v, err %v", got, err)
	}
	hits, err := c.SearchFoods(ctx, as(tok, &pb.SearchFoodsRequest{Query: "pan"}))
	if err != nil || len(hits.Msg.GetFoods()) != 1 || hits.Msg.GetFoods()[0].GetId() != f.GetId() {
		t.Fatalf("search = %v, err %v", hits, err)
	}
	if hits, err := c.SearchFoods(ctx, as(tok, &pb.SearchFoodsRequest{Query: "paneer", Source: "usda_sr"})); err != nil || len(hits.Msg.GetFoods()) != 0 {
		t.Fatalf("source filter = %v, err %v", hits, err)
	}
	if hits, err := c.SearchFoods(ctx, as(tok, &pb.SearchFoodsRequest{Query: "paneer", Source: "custom"})); err != nil || len(hits.Msg.GetFoods()) != 1 {
		t.Fatalf("custom source filter = %v, err %v", hits, err)
	}
	_, err = c.SearchFoods(ctx, as(tok, &pb.SearchFoodsRequest{Query: "paneer", Source: "usda"}))
	assertCode(t, err, connect.CodeInvalidArgument)

	bob := e.invitee(t, tok, "bob").GetToken()
	if hits, err := c.SearchFoods(ctx, as(bob, &pb.SearchFoodsRequest{Query: "paneer"})); err != nil || len(hits.Msg.GetFoods()) != 0 {
		t.Fatalf("bob sees alice's food: %v, err %v", hits, err)
	}
	_, err = c.GetFood(ctx, as(bob, &pb.GetFoodRequest{Id: f.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	f.Name, f.Barcode = "Paneer, fresh", "8901234567890"
	_, err = c.UpdateCustomFood(ctx, as(bob, &pb.UpdateCustomFoodRequest{Food: f}))
	assertCode(t, err, connect.CodeNotFound)
	up := call(c.UpdateCustomFood(ctx, as(tok, &pb.UpdateCustomFoodRequest{Food: f}))).ok(t).GetFood()
	if up.GetName() != "Paneer, fresh" || up.GetBarcode() != "8901234567890" || up.GetId() != f.GetId() || up.GetUnits()["cube"] != 20 {
		t.Fatalf("update = %v", up)
	}

	// A barcode the catalog knows never leaves the server.
	fakeOFF(t, func(http.ResponseWriter, *http.Request) { t.Error("open food facts asked about a known barcode") })
	known, err := c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "8901234567890"}))
	if err != nil || known.Msg.GetFood().GetId() != f.GetId() {
		t.Fatalf("lookup known = %v, err %v", known, err)
	}

	ro := e.key(t, tok, pb.ApiKeyScope_API_KEY_SCOPE_READ).GetKey()
	_, err = c.DeleteCustomFood(ctx, as(ro, &pb.DeleteCustomFoodRequest{Id: f.GetId()}))
	assertCode(t, err, connect.CodePermissionDenied)
	_, err = c.CreateCustomFood(ctx, as(tok, &pb.CreateCustomFoodRequest{Food: &pb.Food{Name: "No macros"}}))
	assertCode(t, err, connect.CodeInvalidArgument)
	_, err = c.DeleteCustomFood(ctx, as(bob, &pb.DeleteCustomFoodRequest{Id: f.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
	if _, err := c.DeleteCustomFood(ctx, as(tok, &pb.DeleteCustomFoodRequest{Id: f.GetId()})); err != nil {
		t.Fatal(err)
	}
	_, err = c.GetFood(ctx, as(tok, &pb.GetFoodRequest{Id: f.GetId()}))
	assertCode(t, err, connect.CodeNotFound)
}

func TestLookupBarcodeOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	c := plusultrav1connect.NewFoodServiceClient(http.DefaultClient, e.url)
	var calls, status atomic.Int32
	status.Store(http.StatusOK)
	fakeOFF(t, func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(offBody))
	})

	first, err := c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "3017620422003"}))
	if err != nil {
		t.Fatal(err)
	}
	f := first.Msg.GetFood()
	if f.GetSource() != "off" || f.GetSourceId() != "3017620422003" || f.GetBarcode() != "3017620422003" || f.GetName() != "Choco Spread" ||
		f.GetPer100G().GetKcal() != 539 || f.GetPer100G().GetProtein() != 6.3 || f.GetCustom() {
		t.Fatalf("lookup = %v", f)
	}
	second, err := c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "3017620422003"}))
	if err != nil || second.Msg.GetFood().GetId() != f.GetId() || calls.Load() != 1 {
		t.Fatalf("second lookup = %v, err %v, upstream calls %d", second, err, calls.Load())
	}

	status.Store(http.StatusNotFound)
	_, err = c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "4000000000001"}))
	assertCode(t, err, connect.CodeNotFound)
	status.Store(http.StatusInternalServerError)
	_, err = c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "4000000000002"}))
	assertCode(t, err, connect.CodeUnavailable)
	_, err = c.LookupBarcode(ctx, as(tok, &pb.LookupBarcodeRequest{Barcode: "12"}))
	assertCode(t, err, connect.CodeInvalidArgument)
}

// Concept members collapse in search, carry their concept, and list preferred first.
func TestFoodConceptsOverAPI(t *testing.T) {
	e := newEnv(t, httpsURL)
	ctx := t.Context()
	tok := e.register(t, "alice", "").GetToken()
	c := plusultrav1connect.NewFoodServiceClient(http.DefaultClient, e.url)
	q := sqlc.New(e.pool)
	var ids []string
	for _, src := range [][2]string{{"usda_sr", "u1"}, {"ifct", "r1"}} {
		f, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{Source: src[0], SourceID: src[1], Name: "Rice, raw (" + src[0] + ")", Per100g: []byte(`{"Kcal":360}`), Units: []byte(`[]`)})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.ID.String())
	}
	concept := "rice-white-raw"
	if err := q.UpsertFoodConcept(ctx, sqlc.UpsertFoodConceptParams{ID: concept, Name: "Rice, white, raw", PreferredSource: "ifct", PreferredSourceID: "r1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := q.AssignConceptMembers(ctx, sqlc.AssignConceptMembersParams{ConceptID: &concept, Keys: []string{"usda_sr:u1", "ifct:r1"}}); err != nil {
		t.Fatal(err)
	}

	hits := call(c.SearchFoods(ctx, as(tok, &pb.SearchFoodsRequest{Query: "rice"}))).ok(t).GetFoods()
	if len(hits) != 1 || hits[0].GetId() != ids[1] || hits[0].GetConceptId() != concept || hits[0].GetConceptName() != "Rice, white, raw" {
		t.Fatalf("search = %v, want the preferred ifct member under its concept", hits)
	}
	got := call(c.GetFood(ctx, as(tok, &pb.GetFoodRequest{Id: ids[0]}))).ok(t).GetFood()
	if got.GetConceptId() != concept || got.GetConceptName() != "Rice, white, raw" || got.GetSource() != "usda_sr" {
		t.Fatalf("get = %v", got)
	}
	members := call(c.ListConceptMembers(ctx, as(tok, &pb.ListConceptMembersRequest{ConceptId: concept}))).ok(t)
	if len(members.GetFoods()) != 2 || members.GetFoods()[0].GetId() != ids[1] || members.GetPreferredFoodId() != ids[1] ||
		members.GetFoods()[1].GetConceptName() != "Rice, white, raw" {
		t.Fatalf("members = %v", members)
	}
	_, err := c.ListConceptMembers(ctx, as(tok, &pb.ListConceptMembersRequest{ConceptId: "nope"}))
	assertCode(t, err, connect.CodeNotFound)
}
