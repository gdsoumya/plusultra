package engine

import (
	"bytes"
	"cmp"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

var (
	updateGolden = flag.Bool("update", false, "rewrite the parity golden files")
	testdata     = os.DirFS("testdata")
)

// parityCases lists every reference construct (spec 18.1) with the fixture
// in testdata/parity that expresses it.
var parityCases = []struct{ Construct, File string }{
	{"set groups and labels", "set_groups"},
	{"rep ranges", "rep_ranges"},
	{"AMRAP", "amrap"},
	{"RPE", "rpe"},
	{"percent and absolute weights", "weights"},
	{"ask", "ask"},
	{"timers: work, rest, auto, count up", "timers"},
	{"warmups: explicit, percent, none, default", "warmups"},
	{"equipment variants", "equipment"},
	{"alternates with current marker", "alternates"},
	{"week ranges and later-week overrides", "week_ranges"},
	{"descriptions: week, day, exercise, inheritance, stop, description_index", "descriptions"},
	{"lp", "lp"},
	{"dp", "dp"},
	{"sum_reps", "sum_reps"},
	{"custom progress reads every variable", "custom_read"},
	{"custom progress writes every variable", "custom_write"},
	{"state and prompted state", "state"},
	{"temporary variables", "temp_vars"},
	{"loops", "loops"},
	{"built-in functions", "functions"},
	{"text", "text"},
	{"set variations", "set_variations"},
	{"exercise variations", "exercise_variations"},
	{"number-of-sets progression", "number_of_sets"},
	{"supersets", "supersets"},
	{"exercise labels", "labels"},
	{"tags and shared state", "tags"},
	{"reuse: templates and functions", "templates"},
	{"percentage-based programs", "percentage"},
	{"bodyweight and assisted exercises", "bodyweight"},
	{"update hooks", "update_hooks"},
	{"ctx.at(week=, day=) targeting", "at_targeting"},
	{"progression and update hook inheritance, progress none", "progress_inheritance"},
}

// runWeek is the week whose workouts a case runs, when not week 1: a later
// week shows what it inherits.
var runWeek = map[string]int{"progress_inheritance": 2}

// parityState is the user every parity case and example resolves for.
func parityState() *model.UserState {
	return &model.UserState{
		Units:      model.UnitKg,
		Bodyweight: kgW(80),
		RM1: map[string]model.Weight{
			"Squat": kgW(140), "Bench Press": kgW(100), "Deadlift": kgW(180), "Overhead Press": kgW(60),
		},
		Rounding: map[string]model.Rounding{
			"dumbbell":   {Kind: model.RoundFixed, Plates: []float64{5, 7.5, 10, 12.5, 15, 17.5, 20, 22.5, 25, 27.5, 30}},
			"cable":      {Kind: model.RoundStack, Increment: 2.5},
			"bodyweight": {Kind: model.RoundBodyweight},
		},
		DefaultRounding: plates20,
	}
}

// promptAnswers answers prompt() state after every exercise of a parity run.
var promptAnswers = map[string]starlark.Value{"rating": starlark.MakeInt(4)}

// snapshot is what a parity case produces: weeks 1-2 as planned, update hook
// runs, and weeks 1-2 again after every exercise of the run week progressed.
type snapshot struct {
	Program  programHeader
	Weeks    []model.ResolvedDay
	Updates  []updateRun  `json:",omitempty"`
	Progress *progressRun `json:",omitempty"`
}

type programHeader struct {
	Name, Notes string
	Weeks       int
	Units       model.Unit
	Loop        bool
	WeekDesc    map[int]string
}

type updateRun struct {
	Exercise string
	SetIndex int
	Sets     []model.ResolvedSet
}

type progressRun struct {
	Print []string
	State map[string]map[string]string
	Weeks []model.ResolvedDay
}

func TestParity(t *testing.T) {
	for _, c := range parityCases {
		t.Run(c.File, func(t *testing.T) {
			name := "parity/" + c.File
			got, err := json.MarshalIndent(runParity(t, name+".star", cmp.Or(runWeek[c.File], 1)), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			golden := name + ".golden.json"
			if *updateGolden {
				if err := os.WriteFile(filepath.Join("testdata", golden), got, 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := fs.ReadFile(testdata, golden)
			if err != nil {
				t.Fatalf("%s: %v (run with -update to create it)", c.Construct, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s: %s differs at %s; rerun with -update and review the diff", c.Construct, golden, firstDiff(got, want))
			}
		})
	}
}

// evalFile evaluates testdata file name.
func evalFile(t *testing.T, name string) *model.Program {
	t.Helper()
	src, err := fs.ReadFile(testdata, name)
	if err != nil {
		t.Fatal(err)
	}
	p, err := Evaluate(string(src), path.Base(name))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runParity(t *testing.T, name string, week int) snapshot {
	t.Helper()
	p := evalFile(t, name)
	st := parityState()
	out := snapshot{
		Program: programHeader{Name: p.Name, Notes: p.Notes, Weeks: p.Weeks, Units: p.Units, Loop: p.Loop, WeekDesc: p.WeekDesc},
		Weeks:   resolveWeeks(t, p, st, 2),
		Updates: runUpdates(t, p, st, week),
	}
	if hasProgress(p) {
		lines := progressWeek(t, p, st, week)
		out.Progress = &progressRun{Print: lines, State: stateStrings(st.Overlay.State), Weeks: resolveWeeks(t, p, st, 2)}
	}
	return out
}

func hasProgress(p *model.Program) bool {
	for _, d := range p.Days {
		for _, e := range d.Exercises {
			if e.Progress != nil {
				return true
			}
		}
	}
	return false
}

// resolveWeeks resolves p's declared days of weeks 1 to n.
func resolveWeeks(t *testing.T, p *model.Program, st *model.UserState, n int) []model.ResolvedDay {
	t.Helper()
	days, err := Resolve(p, st)
	if err != nil {
		t.Fatal(err)
	}
	var out []model.ResolvedDay
	for _, d := range days {
		if d.Week <= n {
			out = append(out, d)
		}
	}
	return out
}

func progKey(e model.ResolvedExercise) model.ProgKey {
	return model.ProgKey{Exercise: e.Alternates[0], Label: e.Label}
}

// asPlanned is e's working sets done exactly as planned, the first done of
// them completed and the rest pending.
func asPlanned(e model.ResolvedExercise, done int) []model.SetResult {
	var out []model.SetResult
	for _, s := range e.Sets {
		if !s.Warmup {
			out = append(out, model.SetResult{Reps: s.Reps, MinReps: s.MinReps, Weight: s.Weight, RPE: s.RPE, Rest: s.Rest,
				Completed: len(out) < done, WorkTime: s.Work, RepsLeft: s.Reps})
		}
	}
	return out
}

// progressWeek finishes every exercise of week as planned, in day and
// exercise order, merging each progression before the next. It returns the
// print output, each line prefixed with its exercise.
func progressWeek(t *testing.T, p *model.Program, st *model.UserState, week int) []string {
	t.Helper()
	var lines []string
	for _, planned := range resolveWeeks(t, p, st, week) {
		if planned.Week != week {
			continue
		}
		d, err := ResolveDay(p, st, week, planned.Day) // again: earlier progressions may have changed it
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range d.Exercises {
			res := model.ExerciseResult{Key: progKey(e), Week: week, Day: d.Day, Sets: asPlanned(e, len(e.Sets)),
				PromptValues: promptAnswers}
			delta, out, err := RunProgress(p, st, res)
			if err != nil {
				t.Fatalf("week %d day %d %s: %v", week, d.Day, programName(res.Key), err)
			}
			st.Overlay.Merge(delta)
			for _, l := range out {
				lines = append(lines, programName(res.Key)+": "+l)
			}
		}
	}
	return lines
}

// runUpdates runs every update hook of week before the first set and after it.
func runUpdates(t *testing.T, p *model.Program, st *model.UserState, week int) []updateRun {
	t.Helper()
	var runs []updateRun
	for _, d := range resolveWeeks(t, p, st, week) {
		if d.Week != week {
			continue
		}
		decl := p.DayAt(d.Week, d.Day)
		for i, e := range d.Exercises {
			if decl.Exercises[i].Update == nil {
				continue
			}
			for setIndex := range 2 {
				res := model.ExerciseResult{Key: progKey(e), Week: d.Week, Day: d.Day, Sets: asPlanned(e, setIndex)}
				live, err := RunUpdate(p, st, res, setIndex)
				if err != nil {
					t.Fatalf("update %s at set %d: %v", programName(res.Key), setIndex, err)
				}
				runs = append(runs, updateRun{Exercise: programName(res.Key), SetIndex: setIndex, Sets: live.Sets})
			}
		}
	}
	return runs
}

func stateStrings(state map[model.ProgKey]map[string]starlark.Value) map[string]map[string]string {
	out := map[string]map[string]string{}
	for k, vars := range state {
		m := map[string]string{}
		for name, v := range vars {
			m[name] = v.String()
		}
		out[programName(k)] = m
	}
	return out
}

// firstDiff names the first line where got and want differ.
func firstDiff(got, want []byte) string {
	g, w := strings.Split(string(got), "\n"), strings.Split(string(want), "\n")
	for i := range min(len(g), len(w)) {
		if g[i] != w[i] {
			return fmt.Sprintf("line %d: got %s, want %s", i+1, strings.TrimSpace(g[i]), strings.TrimSpace(w[i]))
		}
	}
	return "the end: lengths differ"
}
