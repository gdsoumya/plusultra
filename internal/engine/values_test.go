package engine

import (
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

func evalAs[T starlark.Value](t *testing.T, expr string) T {
	t.Helper()
	g, err := exec("x = "+expr, "test.star", builtins())
	if err != nil {
		t.Fatalf("%s: %v", expr, err)
	}
	v, ok := g["x"].(T)
	if !ok {
		t.Fatalf("%s = %s, want %T", expr, g["x"].Type(), v)
	}
	return v
}

func TestKgPlusKg(t *testing.T) {
	got := evalAs[*weightValue](t, "kg(2.5) + kg(5)").Weight()
	if want := (model.Weight{Value: 7.5, Unit: model.UnitKg}); got != want {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestKgTimesNumber(t *testing.T) {
	for _, expr := range []string{"kg(10) * 2", "2 * kg(10)", "kg(40) * 0.5"} {
		got := evalAs[*weightValue](t, expr).Weight()
		if want := (model.Weight{Value: 20, Unit: model.UnitKg}); got != want {
			t.Errorf("%s = %+v, want %+v", expr, got, want)
		}
	}
}

func TestMixedUnitsError(t *testing.T) {
	assertContains(t, execErr(t, "x = kg(1) + lb(1)\n", builtins()), "cannot mix kg and lb")
}

func TestPctCompare(t *testing.T) {
	if !evalAs[starlark.Bool](t, "pct(75) < pct(80)") {
		t.Fatal("pct(75) < pct(80) is False")
	}
}

func TestBwDelta(t *testing.T) {
	cases := map[string]model.Load{
		"bw(5)":              {Kind: model.LoadBodyweight, Delta: model.Weight{Value: 5, Unit: model.UnitKg}},
		"bw()":               {Kind: model.LoadBodyweight, Delta: model.Weight{Unit: model.UnitKg}},
		`bw(-10, unit="lb")`: {Kind: model.LoadBodyweight, Delta: model.Weight{Value: -10, Unit: model.UnitLb}},
	}
	for expr, want := range cases {
		if got := evalAs[*bwValue](t, expr).Load(); got != want {
			t.Errorf("%s = %+v, want %+v", expr, got, want)
		}
	}
}

func TestLoads(t *testing.T) {
	if got, want := evalAs[*weightValue](t, "lb(135)").Load(), (model.Load{Kind: model.LoadAbsolute, Weight: model.Weight{Value: 135, Unit: model.UnitLb}}); got != want {
		t.Errorf("lb(135) = %+v, want %+v", got, want)
	}
	if got, want := evalAs[*pctValue](t, "pct(70)").Load(), (model.Load{Kind: model.LoadPercent, Pct: 70}); got != want {
		t.Errorf("pct(70) = %+v, want %+v", got, want)
	}
}

func TestQuantityArithmetic(t *testing.T) {
	cases := map[string]string{
		"kg(10) - kg(2.5)":     "kg(7.5)",
		"kg(10) / 4":           "kg(2.5)",
		"kg(10) / kg(4)":       "2.5",
		"5 + lb(10)":           "lb(15)",
		"20 - lb(5)":           "lb(15)",
		"pct(70) + 5":          "pct(75)",
		"bw(5) + bw(2.5)":      `bw(7.5, unit="kg")`,
		"kg(1) == lb(1)":       "False",
		"kg(2) >= kg(2)":       "True",
		"{kg(1): 1}[kg(1.0)]":  "1",
		"[pct(1), pct(2)][-1]": "pct(2)",
	}
	for expr, want := range cases {
		if got := evalAs[starlark.Value](t, expr).String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}

func TestQuantityErrors(t *testing.T) {
	cases := map[string]string{
		"kg(1) * kg(1)":            "unknown binary op",
		"2 / kg(1)":                "unknown binary op",
		"kg(1) / 0":                "division by zero",
		"kg(1) + pct(1)":           "unknown binary op",
		`bw(1) + bw(1, unit="lb")`: "cannot mix kg and lb",
		"kg(1) < lb(1)":            "cannot compare kg and lb",
		`bw(unit="st")`:            "unit must be kg or lb",
		`kg("x")`:                  "want number",
		"kg(1e9)":                  "weight too large",
		"lb(-4401)":                "weight too large",
	}
	for expr, want := range cases {
		assertContains(t, execErr(t, "x = "+expr, builtins()), want)
	}
}

func TestUnaryMinus(t *testing.T) {
	cases := map[string]string{"-kg(2.5)": "kg(-2.5)", "+lb(3)": "lb(3)", "-pct(5)": "pct(-5)", "-bw(5)": `bw(-5, unit="kg")`}
	for expr, want := range cases {
		if got := evalAs[starlark.Value](t, expr).String(); got != want {
			t.Errorf("%s = %s, want %s", expr, got, want)
		}
	}
}
