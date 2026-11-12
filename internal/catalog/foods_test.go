package catalog_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

func foodSetup(t *testing.T) (context.Context, *sqlc.Queries) {
	t.Helper()
	ctx, q, _ := foodPool(t)
	return ctx, q
}

func foodPool(t *testing.T) (context.Context, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	pool := dbtest.New(t)
	return context.Background(), sqlc.New(pool), pool
}

func importUSDA(ctx context.Context, t *testing.T, q *sqlc.Queries) {
	t.Helper()
	n, err := catalog.ImportUSDA(ctx, q, "testdata/usda-sample.zip")
	if err != nil || n != 10 {
		t.Fatalf("ImportUSDA = %d, %v; want 10, nil", n, err)
	}
}

func macros(t *testing.T, f catalog.Food) model.Macros {
	t.Helper()
	var m model.Macros
	if err := json.Unmarshal(f.Per100g, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func firstFood(ctx context.Context, t *testing.T, q *sqlc.Queries, query string) catalog.Food {
	t.Helper()
	got, err := catalog.SearchFoods(ctx, q, nil, query, "", 5)
	if err != nil || len(got) == 0 {
		t.Fatalf("SearchFoods(%q) = %v, %v", query, got, err)
	}
	return got[0].Food
}

func TestImportUSDASample(t *testing.T) {
	ctx, q := foodSetup(t)
	importUSDA(ctx, t, q)
	importUSDA(ctx, t, q) // idempotent

	chicken := firstFood(ctx, t, q, "chicken breast raw")
	if chicken.Source != "usda_foundation" || chicken.SourceID != "100001" {
		t.Errorf("chicken = %s/%s", chicken.Source, chicken.SourceID)
	}
	want := model.Macros{Kcal: 120, Protein: 22.5, Fat: 2.6, WaterML: 74.8}
	if got := macros(t, chicken); got != want {
		t.Errorf("chicken per100g = %+v, want %+v", got, want)
	}

	var units []catalog.Unit
	egg := firstFood(ctx, t, q, "egg")
	if err := json.Unmarshal(egg.Units, &units); err != nil {
		t.Fatal(err)
	}
	if len(units) != 3 || units[0] != (catalog.Unit{Name: "egg, large", Grams: 50}) || units[2] != (catalog.Unit{Name: "extra large", Grams: 56}) {
		t.Errorf("egg units = %+v", units)
	}
	oats := firstFood(ctx, t, q, "oats")
	if err := json.Unmarshal(oats.Units, &units); err != nil || units[0] != (catalog.Unit{Name: "cup", Grams: 80}) {
		t.Errorf("oats units = %+v, %v; want 0.5 cup = 40 g stored as 80 g per cup", units, err)
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "protein bar", "", 5); len(got) != 0 {
		t.Errorf("branded food imported: %v", got)
	}
	if all, _ := catalog.SearchFoods(ctx, q, nil, "e", "usda", 50); len(all) > 10 {
		t.Errorf("re-import duplicated rows: %d", len(all))
	}
}

// The real release zips keep the CSVs inside a folder.
func TestImportUSDANestedZip(t *testing.T) {
	ctx, q := foodSetup(t)
	src, err := zip.OpenReader("testdata/usda-sample.zip")
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close() //nolint:errcheck // test
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range src.File {
		w, err := zw.Create("FoodData_Central_csv/" + f.Name)
		if err != nil {
			t.Fatal(err)
		}
		r, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.Copy(w, r); err != nil { //nolint:gosec // test fixture
			t.Fatal(err)
		}
		r.Close() //nolint:errcheck,gosec // test
	}
	path := filepath.Join(t.TempDir(), "nested.zip")
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if n, err := catalog.ImportUSDA(ctx, q, path); err != nil || n != 10 {
		t.Errorf("ImportUSDA = %d, %v; want 10", n, err)
	}
}

func TestImportINDBSample(t *testing.T) {
	ctx, q := foodSetup(t)
	for range 2 {
		if n, err := catalog.ImportINDB(ctx, q, "testdata/indb-sample.csv"); err != nil || n != 10 {
			t.Fatalf("ImportINDB = %d, %v; want 10, nil", n, err)
		}
	}
	f := firstFood(ctx, t, q, "oatmeal porridge")
	if f.Source != "indb" || f.SourceID != "ASC050" {
		t.Errorf("food = %s/%s", f.Source, f.SourceID)
	}
	want := model.Macros{Kcal: 72.85, Protein: 2.6, Carbs: 8.77, Fat: 3.22, Fiber: 0.39}
	if got := macros(t, f); got != want {
		t.Errorf("per100g = %+v, want %+v", got, want)
	}
	var units []catalog.Unit
	if err := json.Unmarshal(f.Units, &units); err != nil || len(units) != 1 || units[0].Name != "bowl" || units[0].Grams != 398 {
		t.Errorf("units = %+v, %v; want a 398 g bowl (289.96 kcal at 72.85 kcal/100 g)", units, err)
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "blank energy", "", 5); len(got) != 0 {
		t.Errorf("row without energy imported: %v", got)
	}
	if all, _ := catalog.SearchFoods(ctx, q, nil, "sandwich", "indb", 50); len(all) != 5 {
		t.Errorf("sandwiches = %d, want 5", len(all))
	}
}

func TestSearchRanksRawFirst(t *testing.T) {
	ctx, q := foodSetup(t)
	importUSDA(ctx, t, q)
	got, err := catalog.SearchFoods(ctx, q, nil, "chicken breast", "", 10)
	if err != nil || len(got) != 3 {
		t.Fatalf("got %d foods, %v; want 3", len(got), err)
	}
	if !strings.HasSuffix(got[0].Name, "raw") || !strings.Contains(got[1].Name, "cooked") || !strings.Contains(got[2].Name, "cooked") {
		t.Errorf("order = %q, %q, %q", got[0].Name, got[1].Name, got[2].Name)
	}
	if rice := firstFood(ctx, t, q, "rice white"); !strings.Contains(rice.Name, "raw") {
		t.Errorf("rice = %q, want the raw entry first", rice.Name)
	}
}

func TestSearchPrefix(t *testing.T) {
	ctx, q := foodSetup(t)
	importUSDA(ctx, t, q)
	if f := firstFood(ctx, t, q, "oat"); f.Name != "Oats" {
		t.Errorf("oat -> %q", f.Name)
	}
	if got, err := catalog.SearchFoods(ctx, q, nil, "!!", "", 5); err != nil || len(got) != 0 {
		t.Errorf("punctuation-only query = %v, %v", got, err)
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "oat", "indb", 5); len(got) != 0 {
		t.Errorf("source filter ignored: %v", got)
	}
}

type toServer struct{ base *url.URL }

func (r toServer) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = r.base.Scheme, r.base.Host
	return http.DefaultTransport.RoundTrip(req)
}

func offClient(t *testing.T, h http.HandlerFunc) *http.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	c := catalog.NewOFFClient(time.Second)
	c.Transport = toServer{u}
	return c
}

const offBody = `{"status":1,"product":{"product_name":"Choco Spread","nutriments":{
"energy-kcal_100g":539,"proteins_100g":6.3,"carbohydrates_100g":57.5,"fat_100g":30.9,"fiber_100g":0}}}`

func TestBarcodeCached(t *testing.T) {
	ctx, q := foodSetup(t)
	var hits atomic.Int32
	c := offClient(t, func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/api/v2/product/3017620422003.json" || !strings.HasPrefix(r.Header.Get("User-Agent"), "plusultra/") {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(offBody))
	})
	first, err := catalog.LookupBarcode(ctx, q, nil, c, "3017620422003")
	if err != nil {
		t.Fatal(err)
	}
	second, err := catalog.LookupBarcode(ctx, q, nil, c, "3017620422003")
	if err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 || first.ID != second.ID {
		t.Errorf("hits = %d, ids %v / %v; want one fetch then the cached row", hits.Load(), first.ID, second.ID)
	}
	want := model.Macros{Kcal: 539, Protein: 6.3, Carbs: 57.5, Fat: 30.9}
	if first.Source != "off" || first.Name != "Choco Spread" || *first.Barcode != "3017620422003" || macros(t, first) != want {
		t.Errorf("food = %+v", first)
	}
}

func TestOFFTimeoutErrors(t *testing.T) {
	ctx, q := foodSetup(t)
	c := offClient(t, func(_ http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	c.Timeout = 50 * time.Millisecond
	start := time.Now()
	if _, err := catalog.LookupBarcode(ctx, q, nil, c, "3017620422003"); !errors.Is(err, catalog.ErrUpstream) || time.Since(start) > 2*time.Second {
		t.Errorf("slow server: err = %v after %v, want ErrUpstream by the client timeout", err, time.Since(start))
	}
	if _, err := catalog.LookupBarcode(ctx, q, nil, c, "12ab"); !errors.Is(err, catalog.ErrInvalid) {
		t.Errorf("bad code: err = %v, want ErrInvalid", err)
	}
	for name, body := range map[string]string{
		"unknown product": `{"status":0}`,
		"no nutriments":   `{"status":1,"product":{"product_name":"Mystery","nutriments":{}}}`,
	} {
		c := offClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte(body)) })
		if _, err := catalog.LookupBarcode(ctx, q, nil, c, "3017620422003"); !errors.Is(err, catalog.ErrNotFound) {
			t.Errorf("%s: err = %v, want ErrNotFound", name, err)
		}
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "mystery", "", 5); len(got) != 0 {
		t.Errorf("product without nutriments was cached: %v", got)
	}
}

func TestOFFRedirectsStayOnHost(t *testing.T) {
	ctx, q := foodSetup(t)
	var hit atomic.Bool
	evil := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit.Store(true) }))
	t.Cleanup(evil.Close)
	c := offClient(t, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, evil.URL, http.StatusFound) })
	if _, err := catalog.LookupBarcode(ctx, q, nil, c, "3017620422003"); err == nil || hit.Load() {
		t.Errorf("followed a redirect off the OFF host: err = %v, hit = %v", err, hit.Load())
	}
}

func TestBarcodeFindsCustomFoodFirst(t *testing.T) {
	ctx, q := foodSetup(t)
	user := newUser(ctx, t, q, "ann")
	mine, err := catalog.CreateCustomFood(ctx, q, user, catalog.CustomFood{Name: "My spread", Barcode: "3017620422003"})
	if err != nil {
		t.Fatal(err)
	}
	failing := offClient(t, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected fetch", http.StatusInternalServerError)
	})
	got, err := catalog.LookupBarcode(ctx, q, &user, failing, "3017620422003")
	if err != nil || got.ID != mine.ID {
		t.Errorf("lookup = %v, %v; want the custom food", got.ID, err)
	}
}

func TestImportINDBFctTables(t *testing.T) {
	ctx, q := foodSetup(t)
	if n, err := catalog.ImportINDB(ctx, q, "testdata/indb-fct-sample.csv"); err != nil || n != 4 {
		t.Fatalf("ImportINDB = %d, %v; want 4", n, err)
	}
	f := firstFood(ctx, t, q, "amla")
	if f.SourceID != "UK-14-001" || macros(t, f).Kcal != 58 {
		t.Errorf("amla = %s %+v", f.SourceID, macros(t, f))
	}
	if agar := firstFood(ctx, t, q, "agar"); macros(t, agar).Carbs != 0 { // "Tr" reads as zero
		t.Errorf("agar carbs = %v", macros(t, agar).Carbs)
	}
}

// seedByID imports the embedded seed and returns every seed row by source id.
func seedByID(ctx context.Context, t *testing.T, q *sqlc.Queries) map[string]catalog.Food {
	t.Helper()
	n, err := catalog.ImportIFCT(ctx, q, "")
	if err != nil || n < 120 {
		t.Fatalf("ImportIFCT = %d, %v; want at least 120", n, err)
	}
	if again, err := catalog.ImportIFCT(ctx, q, ""); err != nil || again != n {
		t.Errorf("re-import = %d, %v", again, err)
	}
	byID := map[string]catalog.Food{}
	for _, l := range "abcdefghijklmnopqrstuvwxyz" { // every name starts with a letter
		for _, src := range []string{"ifct", "reference"} {
			got, err := catalog.SearchFoods(ctx, q, nil, string(l), src, 200)
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range got {
				byID[f.SourceID] = f.Food
			}
		}
	}
	if len(byID) != n {
		t.Fatalf("search reached %d of %d rows", len(byID), n)
	}
	return byID
}

func TestImportIFCTSeed(t *testing.T) {
	ctx, q := foodSetup(t)
	byID := seedByID(ctx, t, q)
	for id, f := range byID {
		m := macros(t, f)
		for _, v := range []float64{m.Kcal, m.Protein, m.Carbs, m.Fat, m.Fiber} {
			if math.IsNaN(v) || v < 0 {
				t.Errorf("%s: invalid %+v", f.Name, m)
			}
		}
		if atwater := 4*m.Protein + 4*m.Carbs + 9*m.Fat; math.Abs(m.Kcal-atwater) > 0.25*atwater {
			t.Errorf("%s (%s): kcal %v is more than 25%% off the %v its macros give", f.Name, id, m.Kcal, atwater)
		}
		if m.Kcal <= 0 || m.Kcal > 950 || m.Fiber > 100 || m.Protein+m.Carbs+m.Fat > 100.1 || (f.Source != "ifct" && f.Source != "reference") || id == "" {
			t.Errorf("%s: implausible %+v", f.Name, m)
		}
	}

	// Published values: IFCT 2017 (energy kJ / 4.184) and USDA SR Legacy for reference rows.
	for id, want := range map[string]model.Macros{
		"A015":  {Kcal: 356.4, Protein: 7.9, Carbs: 78.2, Fat: 0.5, Fiber: 2.8},    // rice, raw, milled
		"A019":  {Kcal: 320.3, Protein: 10.6, Carbs: 64.2, Fat: 1.5, Fiber: 11.4},  // wheat flour, atta
		"B021":  {Kcal: 330.8, Protein: 21.7, Carbs: 55.2, Fat: 1.6, Fiber: 9.1},   // red gram dal (toor)
		"L003":  {Kcal: 257.9, Protein: 18.9, Carbs: 12.4, Fat: 14.8},              // paneer
		"M002":  {Kcal: 44.7, Protein: 10.8, Fat: 0.1},                             // egg white, raw
		"T013":  {Kcal: 884.3, Fat: 100},                                           // ghee, energy from IFCT kJ factors
		"D056":  {Kcal: 27.5, Protein: 2.1, Carbs: 3.6, Fat: 0.2, Fiber: 4.1},      // ladies finger
		"E053":  {Kcal: 43, Protein: 0.5, Carbs: 9.4, Fat: 0.2, Fiber: 3.5},        // pineapple
		"H012":  {Kcal: 520.1, Protein: 23.6, Carbs: 17.3, Fat: 39.6, Fiber: 10.4}, // ground nut
		"honey": {Kcal: 304, Protein: 0.3, Carbs: 82.2, Fiber: 0.2},                // USDA FDC 169640
	} {
		if got := macros(t, byID[id]); got != want {
			t.Errorf("%s = %+v, want %+v", id, got, want)
		}
	}
	for query, want := range map[string]string{"paneer": "Paneer", "toor dal": "Red gram, dal", "egg white": "Egg, poultry, white, raw", "idli rice": "Rice, parboiled, milled"} {
		if f := firstFood(ctx, t, q, query); f.Name != want {
			t.Errorf("%q -> %q, want %q", query, f.Name, want)
		}
	}
}

func TestImportIFCTCompositions(t *testing.T) {
	ctx, q := foodSetup(t)
	if n, err := catalog.ImportIFCT(ctx, q, "testdata/ifct-compositions-sample.csv"); err != nil || n != 5 {
		t.Fatalf("ImportIFCT = %d, %v; want 5", n, err)
	}
	rice := firstFood(ctx, t, q, "chawal") // a local name from the lang column
	if rice.Source != "ifct" || rice.SourceID != "A015" || macros(t, rice).Kcal != 356.4 {
		t.Errorf("rice = %s %s %+v", rice.Source, rice.SourceID, macros(t, rice))
	}
	if ghee := firstFood(ctx, t, q, "ghee"); macros(t, ghee).Kcal != 884.3 {
		t.Errorf("ghee kcal = %v, want 884.3 from the IFCT kJ factors", macros(t, ghee).Kcal)
	}
	if n, err := catalog.ImportIFCT(ctx, q, ""); err != nil || n < 120 { // the seed updates the same rows
		t.Fatalf("seed after full import = %d, %v", n, err)
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "pineapple", "ifct", 5); len(got) != 1 {
		t.Errorf("pineapple rows = %d, want 1 after both imports", len(got))
	}
}

func TestCustomFoodCannotOverwriteCatalog(t *testing.T) {
	ctx, q := foodSetup(t)
	importUSDA(ctx, t, q)
	user := newUser(ctx, t, q, "ann")

	_, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{
		Source: "usda_foundation", SourceID: "100001", Name: "hijacked", Per100g: []byte(`{}`), Units: []byte(`[]`), UserID: &user,
	})
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("upsert over a catalog row: err = %v, want no rows", err)
	}
	if got := firstFood(ctx, t, q, "chicken breast raw"); got.Name == "hijacked" || got.UserID != nil {
		t.Errorf("catalog row overwritten: %+v", got)
	}

	c := catalog.CustomFood{Name: "Chicken, breast, raw", Per100g: model.Macros{Kcal: 1}}
	a, err := catalog.CreateCustomFood(ctx, q, user, c)
	if err != nil || a.Source != "custom" {
		t.Fatalf("custom = %+v, %v", a, err)
	}
	b, err := catalog.CreateCustomFood(ctx, q, user, c)
	if err != nil || b.ID == a.ID || b.SourceID == a.SourceID {
		t.Errorf("same-name customs must be separate rows: %v, %v", b.ID, err)
	}
}

func TestCustomFoodScoped(t *testing.T) {
	ctx, q := foodSetup(t)
	ann, bob := newUser(ctx, t, q, "ann"), newUser(ctx, t, q, "bob")
	f, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{
		Name: "Ann's granola", Per100g: model.Macros{Kcal: 450}, Units: []catalog.Unit{{Name: "bowl", Grams: 60}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := catalog.SearchFoods(ctx, q, &ann, "granola", "", 5); len(got) != 1 {
		t.Errorf("owner search = %d foods", len(got))
	}
	if got, _ := catalog.SearchFoods(ctx, q, &bob, "granola", "", 5); len(got) != 0 {
		t.Errorf("other user sees %d foods", len(got))
	}
	if _, err := catalog.GetFood(ctx, q, bob, f.ID); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("other user get: %v", err)
	}
	upd := catalog.CustomFood{Name: "Granola", Per100g: model.Macros{Kcal: 400}}
	if _, err := catalog.UpdateCustomFood(ctx, q, bob, f.ID, upd); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("other user update: %v", err)
	}
	if got, err := catalog.UpdateCustomFood(ctx, q, ann, f.ID, upd); err != nil || got.Name != "Granola" || macros(t, got).Kcal != 400 {
		t.Errorf("update = %+v, %v", got, err)
	}
	if err := catalog.DeleteCustomFood(ctx, q, bob, f.ID); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("other user delete: %v", err)
	}
	if err := catalog.DeleteCustomFood(ctx, q, ann, f.ID); err != nil {
		t.Errorf("delete: %v", err)
	}
	if _, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{Name: " "}); !errors.Is(err, catalog.ErrInvalid) {
		t.Errorf("blank name: %v", err)
	}
	if _, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{Name: "x", Per100g: model.Macros{Kcal: -1}}); !errors.Is(err, catalog.ErrInvalid) {
		t.Errorf("negative macros: %v", err)
	}
}
