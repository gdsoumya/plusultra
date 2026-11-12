package engine

import (
	"math"
	"reflect"
	"testing"

	"go.starlark.net/starlark"

	"github.com/gdsoumya/plusultra/internal/model"
)

func TestStateJSONRoundTrip(t *testing.T) {
	big := starlark.MakeInt(1).Lsh(100)
	dict := starlark.NewDict(1)
	_ = dict.SetKey(starlark.String("k"), starlark.NewList([]starlark.Value{starlark.MakeInt(1)}))
	vars := map[string]starlark.Value{
		"none": starlark.None, "yes": starlark.True, "n": starlark.MakeInt(-3), "big": big,
		"f": starlark.Float(1.5), "nan": starlark.Float(math.NaN()), "inf": starlark.Float(math.Inf(-1)),
		"s": starlark.String(`a "b"`), "w": weightOf(model.Weight{Value: 102.5, Unit: model.UnitLb}),
		"p": &pctValue{quantity{v: 72.5}}, "bw": &bwValue{quantity{-10, model.UnitKg}},
		"list": starlark.NewList([]starlark.Value{starlark.MakeInt(1), starlark.Tuple{starlark.String("x"), starlark.Float(2)}}),
		"dict": dict,
	}
	key := model.ProgKey{Exercise: `Bench "Press", Dumbbell`, Label: "heavy"}
	abs := model.Weight{Value: 60, Unit: model.UnitKg}
	five := 5
	in := model.Overlay{
		Weights:      map[model.SetAddr]model.WeightOverride{{Key: key, Week: 2, Set: 3}: {Abs: &abs}, {Key: key}: {Delta: kgW(2.5), Scale: 1}},
		Reps:         map[model.SetAddr]model.IntOverride{{Key: key, Day: 1}: {Abs: &five}},
		NumSets:      map[model.DayAddr]model.IntOverride{{Key: key, Variation: 2}: {Delta: 1}},
		SetVariation: map[model.ProgKey]int{key: 2},
		State:        map[model.ProgKey]map[string]starlark.Value{key: vars},
		RM1:          map[string]model.Weight{"Squat": kgW(140)},
	}
	b, err := MarshalOverlay(in)
	if err != nil {
		t.Fatal(err)
	}
	out, err := UnmarshalOverlay(b)
	if err != nil {
		t.Fatalf("%v in %s", err, b)
	}
	got := out.State[key]
	if len(got) != len(vars) {
		t.Fatalf("state %v", got)
	}
	for name, want := range vars {
		if got[name].Type() != want.Type() || got[name].String() != want.String() {
			t.Errorf("%s = %s %v, want %s %v", name, got[name].Type(), got[name], want.Type(), want)
		}
	}
	if err := got["list"].(*starlark.List).Append(starlark.None); err == nil {
		t.Error("decoded state is not frozen")
	}
	in.State, out.State = nil, nil
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("overlay\n got %+v\nwant %+v", out, in)
	}
	if _, err := UnmarshalOverlay([]byte(`{}`)); err != nil {
		t.Fatalf("empty overlay: %v", err)
	}
}

func TestStateJSONRejectsUnknownValues(t *testing.T) {
	key := model.ProgKey{Exercise: "Squat"}
	o := model.Overlay{State: map[model.ProgKey]map[string]starlark.Value{key: {"f": starlark.NewBuiltin("f", nil)}}}
	if _, err := MarshalOverlay(o); err == nil {
		t.Fatal("marshalled a builtin")
	}
	if _, err := UnmarshalOverlay([]byte(`{"State":{"\"Squat\" \"\"":{"x":{"set":[]}}}}`)); err == nil {
		t.Fatal("decoded an unknown kind")
	}
}
