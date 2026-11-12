package engine

import (
	"testing"
	"time"

	"github.com/gdsoumya/plusultra/internal/model"
)

var (
	plates20 = model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{25, 20, 15, 10, 5, 2.5, 1.25}}
	micro    = model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{25, 20, 15, 10, 5, 2.5, 1.25, 0.5}}
)

func kgW(v float64) model.Weight { return model.Weight{Value: v, Unit: model.UnitKg} }

func assertRound(t *testing.T, in float64, r model.Rounding, want float64) {
	t.Helper()
	if got := RoundWeight(kgW(in), r); got != kgW(want) {
		t.Fatalf("RoundWeight(%v, %s) = %v, want %v", in, r.Kind, got, want)
	}
}

func TestRoundPlates(t *testing.T) {
	assertRound(t, 102.3, plates20, 102.5)
}

func TestRoundPlatesNearest(t *testing.T) {
	r := model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{20, 15}}
	assertRound(t, 80, r, 80)
	assertRound(t, 82, r, 80)
	assertRound(t, 100, model.Rounding{Kind: model.RoundPlates, Bar: 20, Plates: []float64{25, 20}}, 100)
	assertRound(t, 22.8, micro, 23)
}

func TestRoundPlatesBounded(t *testing.T) {
	start := time.Now()
	assertRound(t, 2000, micro, 2000)
	assertRound(t, 1e9, micro, 2000) // clamped to the weight limit
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Fatalf("rounding took %v, want under 100ms", d)
	}
}

func TestRoundPlatesWithoutBarKeepsOnePair(t *testing.T) {
	assertRound(t, 1, model.Rounding{Kind: model.RoundPlates, Plates: []float64{2.5}}, 5)
}

func TestRoundNeverBelowBar(t *testing.T) {
	assertRound(t, 10, plates20, 20)
}

func TestRoundStack(t *testing.T) {
	assertRound(t, 47, model.Rounding{Kind: model.RoundStack, Increment: 5}, 45)
	assertRound(t, 2, model.Rounding{Kind: model.RoundStack, Increment: 5}, 5) // the lightest pin
}

func TestRoundBodyweightZero(t *testing.T) {
	assertRound(t, 0, plates20, 0)
	assertRound(t, 0, model.Rounding{Kind: model.RoundBodyweight}, 0)
	assertRound(t, 81.3, model.Rounding{Kind: model.RoundBodyweight}, 81.3)
}

func TestRoundFixed(t *testing.T) {
	assertRound(t, 23, model.Rounding{Kind: model.RoundFixed, Plates: []float64{10, 12.5, 15, 17.5, 20, 22.5, 25}}, 22.5)
}
