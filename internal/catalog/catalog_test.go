package catalog_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/gdsoumya/plusultra/internal/catalog"
	"github.com/gdsoumya/plusultra/internal/db/dbtest"
	"github.com/gdsoumya/plusultra/internal/db/sqlc"
	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

// sampleDir lays the sample dataset out like a free-exercise-db checkout.
func sampleDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "dist"), 0o750); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/free-exercise-db-sample.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "dist", "exercises.json"), data, 0o600); err != nil { //nolint:gosec // test temp dir
		t.Fatal(err)
	}
	if err := os.CopyFS(filepath.Join(dir, "exercises"), os.DirFS("testdata/images")); err != nil {
		t.Fatal(err)
	}
	return dir
}

type env struct {
	ctx    context.Context
	q      *sqlc.Queries
	images string
	user   uuid.UUID
}

func setup(t *testing.T) env {
	t.Helper()
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	e := env{ctx: ctx, q: q, images: t.TempDir(), user: newUser(ctx, t, q, "ann")}
	if _, err := catalog.ImportFreeExerciseDB(ctx, q, sampleDir(t), e.images); err != nil {
		t.Fatal(err)
	}
	return e
}

func newUser(ctx context.Context, t *testing.T, q *sqlc.Queries, name string) uuid.UUID {
	t.Helper()
	u, err := q.CreateUser(ctx, sqlc.CreateUserParams{Username: name, PasswordHash: "x", Units: "kg", Timezone: "UTC"})
	if err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func TestImportSampleIdempotent(t *testing.T) {
	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	dir, images := sampleDir(t), t.TempDir()
	n1, err := catalog.ImportFreeExerciseDB(ctx, q, dir, images)
	if err != nil {
		t.Fatal(err)
	}
	n2, err := catalog.ImportFreeExerciseDB(ctx, q, dir, images)
	if err != nil {
		t.Fatal(err)
	}
	all, err := q.ListExercises(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if n1 != n2 || n1 != len(all) || n1 < 300 {
		t.Fatalf("n1=%d n2=%d rows=%d", n1, n2, len(all))
	}
	bench, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "bench-press-barbell"})
	if err != nil {
		t.Fatal(err)
	}
	if bench.Source != "curated" || !slices.Contains(bench.PrimaryMuscles, "chest") ||
		!slices.Contains(bench.Aliases, "Barbell Bench Press - Medium Grip") {
		t.Fatalf("bench not enriched: %+v", bench)
	}
	if want := []string{"bench-press-barbell/0.jpg", "bench-press-barbell/1.jpg"}; !slices.Equal(bench.Images, want) {
		t.Fatalf("images = %v", bench.Images)
	}
	if want := []string{"Set up.", "Perform the movement with control."}; !slices.Equal(bench.Instructions, want) {
		t.Fatalf("instructions = %v", bench.Instructions)
	}
	if _, err := os.Stat(filepath.Join(images, "bench-press-barbell", "0.jpg")); err != nil {
		t.Fatal(err)
	}
	// An entry the curated list does not know is still imported, under its own name.
	if x, err := q.GetExerciseBySlug(ctx, sqlc.GetExerciseBySlugParams{Slug: "landmine-180s-barbell"}); err != nil || x.Name != "Landmine 180's" {
		t.Fatalf("unmapped entry = %+v, %v", x, err)
	}
}

func TestResolveDefaultEquipment(t *testing.T) {
	e := setup(t)
	for name, want := range map[string]string{
		"Bench Press":          "bench-press-barbell",
		"bench  press":         "bench-press-barbell",
		"Bench Press, Barbell": "bench-press-barbell",
		"Squat":                "squat-barbell",
		"Pull Up":              "pull-up-bodyweight",
		"Jump Rope":            "jump-rope",
		// Two curated variants, one a stub without a dataset row (R80).
		"Back Extension":                   "back-extension-bodyweight",
		"Back Extension, Leverage Machine": "back-extension-leverage-machine",
	} {
		got, err := catalog.ResolveName(e.ctx, e.q, e.user, name)
		if err != nil || got != want {
			t.Errorf("ResolveName(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestResolveVariant(t *testing.T) {
	e := setup(t)
	for name, want := range map[string]string{
		"Bench Press, Dumbbell": "bench-press-dumbbell",
		"squat,bodyweight":      "squat-bodyweight",
		"Squat, Bodyweight":     "squat-bodyweight",
		"Barbell Squat":         "squat-barbell",
	} {
		got, err := catalog.ResolveName(e.ctx, e.q, e.user, name)
		if err != nil || got != want {
			t.Errorf("ResolveName(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
}

func TestUnknownNameSuggests(t *testing.T) {
	e := setup(t)
	_, err := catalog.ResolveName(e.ctx, e.q, e.user, "Bench Pres")
	var un *catalog.UnknownNameError
	if !errors.As(err, &un) {
		t.Fatalf("err = %v", err)
	}
	if len(un.Suggestions) == 0 || len(un.Suggestions) > 3 || !slices.Contains(un.Suggestions, "Bench Press") {
		t.Fatalf("suggestions = %v", un.Suggestions)
	}
	if !strings.Contains(err.Error(), "Bench Pres") {
		t.Fatalf("message = %q", err)
	}
}

func TestCustomExerciseVisibleOnlyToOwner(t *testing.T) {
	e := setup(t)
	bob := newUser(e.ctx, t, e.q, "bob")
	created, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Sandbag Toss", Equipment: "dumbbell", PrimaryMuscles: []string{"biceps"}})
	if err != nil {
		t.Fatal(err)
	}
	slug := created.Slug
	if !strings.HasPrefix(slug, "custom-"+e.user.String()[:8]+"-sandbag-toss") {
		t.Fatalf("slug = %q", slug)
	}
	if got, err := catalog.ResolveName(e.ctx, e.q, e.user, "sandbag toss"); err != nil || got != slug {
		t.Fatalf("owner resolve = %q, %v", got, err)
	}
	if _, err := catalog.ResolveName(e.ctx, e.q, bob, "Sandbag Toss"); err == nil {
		t.Fatal("other user resolved a custom exercise")
	}
	if got, _ := catalog.SearchExercises(e.ctx, e.q, bob, "sandbag", "", "", 10); len(got) != 0 {
		t.Fatalf("other user found %v", got)
	}
	if got, _ := catalog.SearchExercises(e.ctx, e.q, e.user, "sandbag", "", "", 10); len(got) != 1 {
		t.Fatalf("owner found %v", got)
	}
	if err := catalog.DeleteCustomExercise(e.ctx, e.q, bob, slug); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("other user delete = %v", err)
	}
	if _, err := catalog.UpdateCustomExercise(e.ctx, e.q, bob, slug, catalog.Custom{Name: "Mine"}); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("other user update = %v", err)
	}
	if _, err := catalog.UpdateCustomExercise(e.ctx, e.q, e.user, "bench-press-barbell", catalog.Custom{Name: "Mine"}); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("catalog update = %v", err)
	}
	if _, err := catalog.UpdateCustomExercise(e.ctx, e.q, e.user, slug, catalog.Custom{Name: "Sandbag Toss 2"}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteCustomExercise(e.ctx, e.q, e.user, slug); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.ResolveName(e.ctx, e.q, e.user, "Sandbag Toss 2"); err == nil {
		t.Fatal("deleted exercise still resolves")
	}
}

func TestSearchExercises(t *testing.T) {
	e := setup(t)
	got, err := catalog.SearchExercises(e.ctx, e.q, e.user, "bench press", "dumbbell", "", 5)
	if err != nil || len(got) == 0 || got[0].Slug != "bench-press-dumbbell" {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, x := range got {
		if x.Equipment != "dumbbell" {
			t.Fatalf("equipment filter leaked %v", x)
		}
	}
	// Canonical names rank above alias-only hits, and aliases are searched.
	got, _ = catalog.SearchExercises(e.ctx, e.q, e.user, "medium grip", "", "", 5)
	if len(got) == 0 || got[0].Slug != "bench-press-barbell" {
		t.Fatalf("alias search = %v", got)
	}
	got, _ = catalog.SearchExercises(e.ctx, e.q, e.user, "", "", "lats", 50)
	if len(got) == 0 {
		t.Fatal("muscle filter found nothing")
	}
}

func TestProgramValidationUsesCatalog(t *testing.T) {
	e := setup(t)
	const src = `program("t", weeks=1, units="kg")
day(1, 1, "A")
ex("Bench Press", alts=["Sandbag Toss"], sets=[s(3, 5, kg(60))])
ex("Squat, Bodyweight", sets=[s(3, 5, bw())])
ex("Bench Pres", sets=[s(3, 5, kg(60))])
`
	p, err := engine.Evaluate(src, "t.star")
	if err != nil {
		t.Fatal(err)
	}
	errs, err := catalog.ValidateProgramNames(e.ctx, e.q, e.user, p)
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) != 2 || errs[0].Name != "Sandbag Toss" || errs[1].Name != "Bench Pres" || errs[1].Line != 5 ||
		!slices.Contains(errs[1].Suggestions, "Bench Press") {
		t.Fatalf("errs = %+v", errs)
	}
	if _, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Sandbag Toss"}); err != nil {
		t.Fatal(err)
	}
	if errs, _ := catalog.ValidateProgramNames(e.ctx, e.q, e.user, p); len(errs) != 1 || errs[0].Name != "Bench Pres" {
		t.Fatalf("after custom: %+v", errs)
	}
}

func TestExampleNamesResolve(t *testing.T) {
	e := setup(t)
	files, err := filepath.Glob("../engine/testdata/examples/*.star")
	if err != nil || len(files) == 0 {
		t.Fatalf("no examples: %v", err)
	}
	for _, f := range files {
		src, err := os.ReadFile(f) //nolint:gosec // repo testdata
		if err != nil {
			t.Fatal(err)
		}
		p, err := engine.Evaluate(string(src), filepath.Base(f))
		if err != nil {
			t.Fatal(err)
		}
		if errs, err := catalog.ValidateProgramNames(e.ctx, e.q, e.user, p); err != nil || len(errs) != 0 {
			t.Errorf("%s unresolved: %+v, %v", f, errs, err)
		}
	}
}

func TestAliasesWellFormed(t *testing.T) {
	rows, err := catalog.CuratedRows()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) < 300 {
		t.Errorf("only %d rows", len(rows))
	}
	defaults, sources, names := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, r := range rows {
		names[r.Name] = true
		if r.Default {
			if defaults[r.Name] {
				t.Errorf("two defaults for %q", r.Name)
			}
			defaults[r.Name] = true
		}
		if r.SourceID != "" {
			if sources[r.SourceID] {
				t.Errorf("source id %q mapped twice", r.SourceID)
			}
			sources[r.SourceID] = true
		}
	}
	for n := range names {
		if !defaults[n] {
			t.Errorf("no default variant for %q", n)
		}
	}
}

// Every stub (no dataset row) names its muscles in the dataset's vocabulary (R80).
func TestStubsHaveMuscles(t *testing.T) {
	vocab := map[string]bool{}
	for _, m := range strings.Split("abdominals abductors adductors biceps calves chest forearms glutes hamstrings lats lower_back middle_back neck quadriceps shoulders traps triceps", " ") {
		vocab[strings.ReplaceAll(m, "_", " ")] = true
	}
	rows, err := catalog.CuratedRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.SourceID == "" && len(r.Muscles) == 0 {
			t.Errorf("%s, %s: stub without muscles", r.Name, r.Equipment)
		}
		for _, m := range slices.Concat(r.Muscles, r.Secondary) {
			if !vocab[m] {
				t.Errorf("%s, %s: unknown muscle %q", r.Name, r.Equipment, m)
			}
		}
	}
}

// Run with FREE_EXERCISE_DB_JSON=<path to dist/exercises.json> to check the curated ids against a real dataset.
func TestAliasesMatchDataset(t *testing.T) {
	path := os.Getenv("FREE_EXERCISE_DB_JSON")
	if path == "" {
		t.Skip("FREE_EXERCISE_DB_JSON not set")
	}
	raw, err := os.ReadFile(path) //nolint:gosec // developer-supplied path
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct{ ID string }
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, e := range entries {
		ids[e.ID] = true
	}
	rows, err := catalog.CuratedRows()
	if err != nil {
		t.Fatal(err)
	}
	matched := 0
	for _, r := range rows {
		switch {
		case r.SourceID == "":
		case !ids[r.SourceID]:
			t.Errorf("%s, %s: source id %q is not in the dataset", r.Name, r.Equipment, r.SourceID)
		default:
			matched++
		}
	}
	t.Logf("%d rows, %d matched to %d dataset entries", len(rows), matched, len(ids))

	ctx := context.Background()
	q := sqlc.New(dbtest.New(t))
	if _, err := catalog.ImportFreeExerciseDB(ctx, q, filepath.Dir(filepath.Dir(path)), t.TempDir()); err != nil {
		t.Fatal(err)
	}
	all, err := q.ListExercises(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range all {
		if len(e.PrimaryMuscles) == 0 {
			t.Errorf("%s: no primary muscles", e.Slug)
		}
	}
}

func TestExerciseData(t *testing.T) {
	e := setup(t)
	rm := 140.0
	good := &model.Rounding{Kind: "plates", Bar: 20, Plates: []float64{20, 10, 5, 2.5, 1.25}}
	if err := catalog.SetExerciseData(e.ctx, e.q, e.user, catalog.ExerciseData{Slug: "bench-press-barbell", RM1: &rm, Rounding: good, Notes: "n"}); err != nil {
		t.Fatal(err)
	}
	got, err := catalog.GetExerciseData(e.ctx, e.q, e.user, "bench-press-barbell")
	if err != nil || got.RM1 == nil || *got.RM1 != 140 || got.Rounding.Bar != 20 || got.Notes != "n" {
		t.Fatalf("got %+v, %v", got, err)
	}
	if _, err := catalog.GetExerciseData(e.ctx, e.q, e.user, "squat-barbell"); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("missing data err = %v", err)
	}
	bad := map[string]*model.Rounding{
		"kind":      {Kind: "magic"},
		"increment": {Kind: "stack", Increment: 0},
		"negative":  {Kind: "stack", Increment: -5},
		"plates":    {Kind: "plates", Bar: 20, Plates: make([]float64, 21)},
		"plate0":    {Kind: "plates", Plates: []float64{0}},
		"infbar":    {Kind: "plates", Bar: math.Inf(1)},
		"infinc":    {Kind: "stack", Increment: math.Inf(1)},
	}
	for name, r := range bad {
		err := catalog.SetExerciseData(e.ctx, e.q, e.user, catalog.ExerciseData{Slug: "bench-press-barbell", Rounding: r})
		if !errors.Is(err, catalog.ErrInvalid) {
			t.Errorf("%s: err = %v", name, err)
		}
	}
	if err := catalog.SetExerciseData(e.ctx, e.q, e.user, catalog.ExerciseData{Slug: "nope"}); !errors.Is(err, catalog.ErrNotFound) {
		t.Errorf("unknown slug err = %v", err)
	}
}

func TestSeedDefaultEquipment(t *testing.T) {
	e := setup(t)
	for range 2 {
		if err := catalog.SeedDefaultEquipment(e.ctx, e.q, e.user); err != nil {
			t.Fatal(err)
		}
	}
	got, err := catalog.GetEquipment(e.ctx, e.q, e.user)
	if err != nil || len(got) != 4 {
		t.Fatalf("equipment = %+v, %v", got, err)
	}
	by := map[string]model.Rounding{}
	for _, x := range got {
		by[x.Name] = x.Config
	}
	if b := by["Barbell"]; b.Kind != "plates" || b.Bar != 20 || len(b.Plates) != 7 {
		t.Errorf("barbell = %+v", b)
	}
	if by["Stack"].Increment != 5 || by["Bodyweight"].Kind != "bodyweight" || by["Dumbbell"].Kind != "fixed" {
		t.Errorf("by = %+v", by)
	}
	// Seeding must not overwrite what the user changed.
	if err := catalog.SetEquipment(e.ctx, e.q, e.user, "Stack", model.Rounding{Kind: "stack", Increment: 2.5}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.SeedDefaultEquipment(e.ctx, e.q, e.user); err != nil {
		t.Fatal(err)
	}
	got, _ = catalog.GetEquipment(e.ctx, e.q, e.user)
	for _, x := range got {
		if x.Name == "Stack" && x.Config.Increment != 2.5 {
			t.Errorf("seed overwrote stack: %+v", x)
		}
	}
	if err := catalog.SetEquipment(e.ctx, e.q, e.user, "Bad", model.Rounding{Kind: "x"}); !errors.Is(err, catalog.ErrInvalid) {
		t.Errorf("bad equipment err = %v", err)
	}
	if err := catalog.DeleteEquipment(e.ctx, e.q, e.user, "Stack"); err != nil {
		t.Fatal(err)
	}
}

func TestCustomShadowsCatalogName(t *testing.T) {
	e := setup(t)
	bob := newUser(e.ctx, t, e.q, "bob")
	c, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Bench Press"})
	if err != nil {
		t.Fatal(err)
	}
	if got, err := catalog.ResolveName(e.ctx, e.q, e.user, "bench press"); err != nil || got != c.Slug {
		t.Fatalf("owner = %q, %v", got, err)
	}
	if got, err := catalog.ResolveName(e.ctx, e.q, bob, "bench press"); err != nil || got != "bench-press-barbell" {
		t.Fatalf("other user = %q, %v", got, err)
	}
}

func TestResolveTieBreak(t *testing.T) {
	e := setup(t)
	// "Foo" is the name of one row and an alias of another; the name match must win whatever the insert order.
	for _, order := range [][]string{{"a-foo", "b-bar"}, {"b-bar", "a-foo"}} {
		for _, slug := range order {
			p := sqlc.UpsertExerciseParams{Slug: slug, Name: "Foo", Source: "free-exercise-db"}
			if slug == "b-bar" {
				p.Name, p.Aliases = "Bar", []string{"Foo"}
			}
			if _, err := e.q.UpsertExercise(e.ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := catalog.ResolveName(e.ctx, e.q, e.user, "foo"); err != nil || got != "a-foo" {
			t.Fatalf("order %v: got %q, %v", order, got, err)
		}
	}
	// Equal name matches: curated beats an imported entry, then the slug decides.
	for _, p := range []sqlc.UpsertExerciseParams{
		{Slug: "a-baz", Name: "Baz", Source: "free-exercise-db"},
		{Slug: "z-baz", Name: "Baz", Source: "curated"},
	} {
		if _, err := e.q.UpsertExercise(e.ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	if got, _ := catalog.ResolveName(e.ctx, e.q, e.user, "baz"); got != "z-baz" {
		t.Fatalf("curated tie = %q", got)
	}
}

func TestDeleteCustomExerciseRemovesItsData(t *testing.T) {
	e := setup(t)
	c, err := catalog.CreateCustomExercise(e.ctx, e.q, e.user, catalog.Custom{Name: "Sandbag Toss"})
	if err != nil {
		t.Fatal(err)
	}
	if err := catalog.SetExerciseData(e.ctx, e.q, e.user, catalog.ExerciseData{Slug: c.Slug, Notes: "n"}); err != nil {
		t.Fatal(err)
	}
	if err := catalog.DeleteCustomExercise(e.ctx, e.q, e.user, c.Slug); err != nil {
		t.Fatal(err)
	}
	if _, err := catalog.GetExerciseData(e.ctx, e.q, e.user, c.Slug); !errors.Is(err, catalog.ErrNotFound) {
		t.Fatalf("data survived: %v", err)
	}
}
