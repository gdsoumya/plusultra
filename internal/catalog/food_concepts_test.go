package catalog_test

import (
	"context"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/model"
)

const conceptHeader = "concept_id,name,aliases,category,preferred_source,preferred_source_id,member_keys\n"

// testConcepts maps the fake ids of usda-sample.zip and the IFCT seed to concepts.
const testConcepts = conceptHeader +
	`rice-white-raw,"Rice, white, raw",chawal; basmati,grains,ifct,A015,ifct:A015;usda_sr:100004;usda_sr:999` + "\n" +
	`chicken-breast-raw,"Chicken breast, raw",murgh,meat,usda_foundation,100001,usda_foundation:100001` + "\n"

func writeConcepts(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "concepts.csv")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

// conceptSetup imports the USDA sample, the IFCT seed and the test concepts.
func conceptSetup(t *testing.T) (context.Context, *sqlc.Queries, *pgxpool.Pool) {
	t.Helper()
	ctx, q, pool := foodPool(t)
	importUSDA(ctx, t, q)
	if _, err := catalog.ImportIFCT(ctx, q, ""); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.ImportFoodConcepts(ctx, pool, writeConcepts(t, testConcepts)); err != nil {
		t.Fatal(err)
	}
	return ctx, q, pool
}

func TestConceptsCSVWellFormed(t *testing.T) {
	cs, err := catalog.FoodConcepts()
	if err != nil {
		t.Fatal(err)
	}
	if len(cs) < 150 {
		t.Errorf("concepts = %d, want at least 150", len(cs))
	}
	ids, owner := map[string]bool{}, map[string]string{}
	for _, c := range cs {
		if ids[c.ID] {
			t.Errorf("duplicate concept %s", c.ID)
		}
		ids[c.ID] = true
		if !slices.Contains(c.Members, c.Preferred) {
			t.Errorf("%s: preferred %s is not a member", c.ID, c.Preferred)
		}
		for _, m := range c.Members {
			if o, ok := owner[m]; ok {
				t.Errorf("%s is in %s and %s", m, o, c.ID)
			}
			owner[m] = c.ID
		}
	}
}

// datasetKeys lists "source:source_id" of every row the importers would load from FOOD_DATASETS_DIR.
func datasetKeys(t *testing.T, dir string) map[string]bool {
	t.Helper()
	keys := map[string]bool{}
	read := func(path string, comment rune, add func(map[string]string)) {
		f, err := os.Open(path) //nolint:gosec // test dataset
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close() //nolint:errcheck // read-only
		r := csv.NewReader(f)
		r.Comment, r.FieldsPerRecord = comment, -1
		head, err := r.Read()
		if err != nil {
			t.Fatal(err)
		}
		for {
			rec, err := r.Read()
			if err == io.EOF {
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			row := map[string]string{}
			for i, h := range head {
				if i < len(rec) {
					row[strings.TrimPrefix(h, "\xef\xbb\xbf")] = rec[i]
				}
			}
			add(row)
		}
	}
	usda, err := filepath.Glob(filepath.Join(dir, "usda", "x", "*", "food.csv"))
	if err != nil || len(usda) != 1 {
		t.Fatalf("usda food.csv: %v, %v", usda, err)
	}
	read(usda[0], 0, func(r map[string]string) {
		if r["data_type"] == "sr_legacy_food" {
			keys["usda_sr:"+r["fdc_id"]] = true
		}
	})
	read(filepath.Join(dir, "ifct2017", "x", "package", "compositions", "index.csv"), 0, func(r map[string]string) { keys["ifct:"+r["code"]] = true })
	read(filepath.Join(dir, "indb", "indb.csv"), 0, func(r map[string]string) {
		id := r["food_code_org"]
		if id == "" {
			id = r["food_code"]
		}
		keys["indb:"+id] = true
	})
	read("data/indian-raw.csv", '#', func(r map[string]string) { keys[r["source"]+":"+r["source_id"]] = true })
	return keys
}

func TestConceptMembersExistInDatasets(t *testing.T) {
	dir := os.Getenv("FOOD_DATASETS_DIR")
	if dir == "" {
		t.Skip("set FOOD_DATASETS_DIR to the folder holding the usda, ifct2017 and indb datasets")
	}
	have := datasetKeys(t, dir)
	cs, err := catalog.FoodConcepts()
	if err != nil {
		t.Fatal(err)
	}
	members, missing := 0, 0
	for _, c := range cs {
		for _, m := range c.Members {
			members++
			if !have[m] {
				missing++
				t.Errorf("%s: member %s is in no dataset", c.ID, m)
			}
		}
	}
	t.Logf("%d concepts, %d members, %d missing", len(cs), members, missing)
}

func TestSearchCollapsesConcept(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	got, err := catalog.SearchFoods(ctx, q, nil, "rice", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	var concept []catalog.FoodHit
	for _, h := range got {
		if h.ConceptID == "rice-white-raw" {
			concept = append(concept, h)
		}
	}
	if len(concept) != 1 {
		t.Fatalf("rice-white-raw hits = %d, want 1 in %+v", len(concept), got)
	}
	if h := concept[0]; h.Name != "Rice, white, raw" || h.Source != "ifct" || h.SourceID != "A015" || h.OtherSources != 1 {
		t.Errorf("hit = %q from %s:%s, other sources %d", h.Name, h.Source, h.SourceID, h.OtherSources)
	}
	for _, h := range got {
		if h.SourceID == "100004" {
			t.Errorf("collapsed member listed separately: %+v", h)
		}
	}
}

func TestConceptFallsBackToPresentMember(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	got, err := catalog.SearchFoods(ctx, q, nil, "rice", "usda", 20)
	if err != nil || len(got) == 0 || got[0].ConceptID != "rice-white-raw" || got[0].Source != "usda_sr" || got[0].OtherSources != 1 {
		t.Errorf("usda-only search = %+v, %v", got, err)
	}
}

func TestConceptAliasSearch(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	for _, query := range []string{"chawal", "basmat", "chiken"} {
		got, err := catalog.SearchFoods(ctx, q, nil, query, "usda", 5)
		if err != nil || len(got) == 0 {
			t.Fatalf("SearchFoods(%q) = %v, %v", query, got, err)
		}
		if want := map[string]string{"chawal": "rice-white-raw", "basmat": "rice-white-raw", "chiken": "chicken-breast-raw"}[query]; got[0].ConceptID != want {
			t.Errorf("%q -> %q, want concept %s", query, got[0].ConceptID, want)
		}
	}
}

func TestConceptlessRowsStillSearchable(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	got, err := catalog.SearchFoods(ctx, q, nil, "oat", "usda", 5)
	if err != nil || len(got) != 1 || got[0].Name != "Oats" || got[0].ConceptID != "" || got[0].OtherSources != 0 {
		t.Errorf("oat = %+v, %v", got, err)
	}
	ann := newUser(ctx, t, q, "ann")
	if _, err := catalog.CreateCustomFood(ctx, q, ann, catalog.CustomFood{Name: "Ann's rice mix", Per100g: model.Macros{Kcal: 1}}); err != nil {
		t.Fatal(err)
	}
	got, err = catalog.SearchFoods(ctx, q, &ann, "rice", "", 5)
	if err != nil || len(got) < 2 || got[0].Source != "custom" {
		t.Errorf("custom food not first: %+v, %v", got, err)
	}
}

func TestImportAssignsConcepts(t *testing.T) {
	ctx, q, pool := conceptSetup(t)
	path := writeConcepts(t, testConcepts)
	n, err := catalog.ImportFoodConcepts(ctx, pool, path)
	if err != nil || n != 3 {
		t.Fatalf("re-import = %d, %v; want 3 foods (the unknown member is skipped)", n, err)
	}
	members, err := catalog.ListConceptMembers(ctx, q, "rice-white-raw")
	if err != nil || len(members) != 2 || members[0].SourceID != "A015" || members[1].Source != "usda_sr" {
		t.Fatalf("members = %+v, %v", members, err)
	}

	smaller := conceptHeader + `rice-white-raw,"Rice, white, raw",chawal,grains,ifct,A015,ifct:A015` + "\n"
	if n, err := catalog.ImportFoodConcepts(ctx, pool, writeConcepts(t, smaller)); err != nil || n != 1 {
		t.Fatalf("shrunk import = %d, %v", n, err)
	}
	if members, _ := catalog.ListConceptMembers(ctx, q, "rice-white-raw"); len(members) != 1 {
		t.Errorf("stale member kept: %+v", members)
	}
	if members, _ := catalog.ListConceptMembers(ctx, q, "chicken-breast-raw"); len(members) != 0 {
		t.Errorf("dropped concept kept members: %+v", members)
	}
	if got, _ := catalog.SearchFoods(ctx, q, nil, "chicken breast", "", 5); len(got) == 0 || got[0].ConceptID != "" {
		t.Errorf("chicken still grouped: %+v", got)
	}
}

func TestImportFoodConceptsRejectsBadRows(t *testing.T) {
	ctx, _, pool := foodPool(t)
	for name, body := range map[string]string{
		"preferred outside members": conceptHeader + `a,A,,x,ifct,A015,ifct:A001` + "\n",
		"duplicate id":              conceptHeader + `a,A,,x,ifct,A015,ifct:A015` + "\n" + `a,B,,x,ifct,A001,ifct:A001` + "\n",
		"blank name":                conceptHeader + `a,,,x,ifct,A015,ifct:A015` + "\n",
	} {
		if _, err := catalog.ImportFoodConcepts(ctx, pool, writeConcepts(t, body)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestFuzzyMatchesRankLast(t *testing.T) {
	ctx, q, _ := conceptSetup(t)
	if _, err := q.UpsertFood(ctx, sqlc.UpsertFoodParams{Source: "t", SourceID: "1", Name: "Chiken tikka", Per100g: []byte(`{}`), Units: []byte(`[]`)}); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.SearchFoods(ctx, q, nil, "chiken", "", 10)
	if err != nil || len(got) < 2 || got[0].Name != "Chiken tikka" {
		t.Fatalf("hits = %+v, %v; want the word match before the fuzzy concept", got, err)
	}
	if got[1].ConceptID != "chicken-breast-raw" {
		t.Errorf("second hit = %+v, want the fuzzy chicken concept", got[1])
	}
}
