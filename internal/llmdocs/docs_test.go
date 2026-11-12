package llmdocs

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

const testURL = "https://lift.example.com"

// served are the routes the server serves the documents on.
var served = []string{"/llms.txt", "/llms/program-language.md", "/llms/program-examples.md",
	"/llms/meal-language.md", "/llms/log-format.md"}

func all(d Docs) map[string]string {
	return map[string]string{"llms.txt": d.LLMSTxt, "program-language.md": d.ProgramLanguage,
		"program-examples.md": d.ProgramExamples, "meal-language.md": d.MealLanguage, "log-format.md": d.LogFormat}
}

func TestEveryBuiltinDocumented(t *testing.T) {
	d := Build(testURL)
	refs := d.ProgramLanguage + d.MealLanguage
	for _, b := range engine.Docs() {
		if !strings.Contains(refs, "`"+b.Signature+"`") {
			t.Errorf("%s: signature %q is not in the references", b.Name, b.Signature)
		}
	}
	for _, a := range engine.CtxDocs() {
		if !strings.Contains(d.ProgramLanguage, "`ctx."+a.Name+"`") {
			t.Errorf("ctx.%s is not in the program reference", a.Name)
		}
	}
	for name, doc := range all(d) {
		if strings.Contains(doc, "<no value>") || strings.TrimSpace(doc) == "" {
			t.Errorf("%s: empty or has a missing template value", name)
		}
	}
	if !strings.Contains(d.ProgramLanguage, "## Common mistakes") {
		t.Error("program reference has no Common mistakes section")
	}
}

var pythonBlock = regexp.MustCompile("(?s)```python\n(.*?)```")

const (
	workoutPrelude = "program(\"Example\", weeks=8)\nday(1, 1, \"Day 1\")\n"
	mealPrelude    = "meal_program(\"Example\", weeks=4)\nday(1, 1, \"Day 1\")\n"
)

// TestEveryExampleEvaluates runs every python block of the documents. A first
// line "# meal" marks a meal program, "# fragment" and "# meal fragment" a
// part of one, which runs after a minimal program and day. Workout programs
// also resolve and finish their first week as planned.
func TestEveryExampleEvaluates(t *testing.T) {
	n := 0
	for name, doc := range all(Build(testURL)) {
		for _, m := range pythonBlock.FindAllStringSubmatch(doc, -1) {
			n++
			src := m[1]
			tag, _, _ := strings.Cut(src, "\n")
			var err error
			switch tag {
			case "# meal":
				_, err = engine.EvaluateMeal(src, name)
			case "# meal fragment":
				_, err = engine.EvaluateMeal(mealPrelude+src, name)
			case "# fragment":
				err = runWorkout(workoutPrelude+src, name)
			default:
				err = runWorkout(src, name)
			}
			if err != nil {
				t.Errorf("%s: %v in\n%s", name, err, src)
			}
		}
	}
	if n < 40 {
		t.Errorf("only %d python blocks", n)
	}
}

// runWorkout evaluates and resolves src, then finishes week 1 twice: as
// planned, then one rep short on every set, so examples take their success
// and failure branches. Update hooks run before the first set and after it.
func runWorkout(src, name string) error {
	p, err := engine.Evaluate(src, name)
	if err != nil {
		return err
	}
	st := &model.UserState{Units: model.UnitKg, Bodyweight: model.Weight{Value: 80, Unit: model.UnitKg},
		RM1: map[string]model.Weight{}}
	for _, d := range p.Days {
		for _, e := range d.Exercises {
			for _, n := range e.Names {
				st.RM1[n] = model.Weight{Value: 100, Unit: model.UnitKg}
			}
		}
	}
	for short := range 2 {
		if err := finishWeek1(p, st, short); err != nil {
			return err
		}
	}
	_, err = engine.Resolve(p, st)
	return err
}

// finishWeek1 completes every working set of week 1 with short reps fewer
// than planned, running each update hook and then each progression.
func finishWeek1(p *model.Program, st *model.UserState, short int) error {
	days, err := engine.Resolve(p, st)
	if err != nil {
		return err
	}
	for _, d := range days {
		if d.Week != 1 {
			continue
		}
		decl := p.DayAt(1, d.Day)
		for i, e := range d.Exercises {
			res := model.ExerciseResult{Key: model.ProgKey{Exercise: decl.Exercises[i].Names[0], Label: e.Label}, Week: 1, Day: d.Day}
			for _, s := range e.Sets {
				if !s.Warmup {
					res.Sets = append(res.Sets, model.SetResult{Reps: max(s.Reps-short, 0), Weight: s.Weight, RPE: s.RPE, Completed: true})
				}
			}
			if decl.Exercises[i].Update != nil {
				if err := runHook(p, st, res); err != nil {
					return err
				}
			}
			delta, _, err := engine.RunProgress(p, st, res)
			if err != nil {
				return err
			}
			st.Overlay.Merge(delta)
		}
	}
	return nil
}

// runHook runs the update hook of res before its first set and after it.
func runHook(p *model.Program, st *model.UserState, res model.ExerciseResult) error {
	for setIndex := range min(2, len(res.Sets)+1) {
		live := res
		live.Sets = slices.Clone(res.Sets)
		for j := setIndex; j < len(live.Sets); j++ {
			live.Sets[j].Completed = false
		}
		if _, err := engine.RunUpdate(p, st, live, setIndex); err != nil {
			return err
		}
	}
	return nil
}

// catalogRef is a ref or catalog_snapshot key in a document, as source:id.
var catalogRef = regexp.MustCompile(`(?:ref=|catalog_snapshot\(\{)"([a-z_]+):`)

func TestRefsUseCatalogSources(t *testing.T) {
	sources := []string{"usda_sr", "usda_foundation", "ifct", "indb", "off", "reference", "custom"}
	n := 0
	for name, doc := range all(Build(testURL)) {
		for _, m := range catalogRef.FindAllStringSubmatch(doc, -1) {
			n++
			if !slices.Contains(sources, m[1]) {
				t.Errorf("%s: ref source %q is not a catalog source", name, m[1])
			}
		}
	}
	if n == 0 {
		t.Error("no refs found")
	}
}

var mdLink = regexp.MustCompile(`\]\(([^)\s]+)\)`)

func TestLLMSTxtLinksResolve(t *testing.T) {
	for _, url := range []string{testURL, testURL + "/"} {
		d := Build(url)
		linked := map[string]bool{}
		for name, doc := range all(d) {
			for _, m := range mdLink.FindAllStringSubmatch(doc, -1) {
				path, ok := strings.CutPrefix(m[1], testURL)
				if !ok || !slices.Contains(served, path) {
					t.Errorf("%s: link %s is not a served document", name, m[1])
				}
				if name == "llms.txt" {
					linked[path] = true
				}
			}
		}
		for _, p := range served[1:] {
			if !linked[p] {
				t.Errorf("llms.txt does not link %s", p)
			}
		}
	}
}

func TestDocsDeterministic(t *testing.T) {
	if a, b := Build(testURL), Build(testURL); a != b {
		t.Error("two builds differ")
	}
}
