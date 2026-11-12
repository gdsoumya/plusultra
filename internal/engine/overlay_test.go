package engine

import (
	"math/rand/v2"
	"testing"

	"github.com/gdsoumya/plusultra/internal/model"
)

// TestAssignMatchesConcreteWrites checks random write sequences against a
// model that applies each write to every concrete address it matches.
func TestAssignMatchesConcreteWrites(t *testing.T) {
	key := model.ProgKey{Exercise: "Squat"}
	var grid []model.SetAddr
	for w := 1; w <= 2; w++ {
		for d := 1; d <= 2; d++ {
			for v := 1; v <= 2; v++ {
				for s := 1; s <= 2; s++ {
					grid = append(grid, model.SetAddr{Key: key, Week: w, Day: d, Variation: v, Set: s})
				}
			}
		}
	}
	script := func(x model.SetAddr) int { return 10*x.Week + 5*x.Day + 2*x.Variation + x.Set }
	r := rand.New(rand.NewPCG(1, 2))                 //nolint:gosec // seeded so failures reproduce
	pick := func(n int) int { return r.IntN(n + 1) } // 0 (all) to n
	for seq := range 3000 {
		base, delta := map[model.SetAddr]model.IntOverride{}, map[model.SetAddr]model.IntOverride{}
		want := map[model.SetAddr]int{}
		for _, x := range grid {
			want[x] = script(x)
		}
		for step := range 6 {
			if r.IntN(3) == 0 { // a new run: earlier deltas are now the user's overlay
				var o model.Overlay
				o.Reps, base = base, nil
				o.Merge(model.Overlay{Reps: delta})
				base, delta = o.Reps, map[model.SetAddr]model.IntOverride{}
			}
			a := model.SetAddr{Key: key, Week: pick(2), Day: pick(2), Variation: pick(2), Set: pick(2)}
			e := edit{v: float64(r.IntN(7) - 3)}
			if r.IntN(2) == 0 {
				e = edit{abs: true, v: float64(r.IntN(100))}
			}
			if err := assign(delta, base, a, setAddr, joinSet, func(o model.IntOverride) model.IntOverride {
				return composeNum(o, e, maxListLen)
			}); err != nil {
				t.Fatal(err)
			}
			for _, x := range grid {
				if coversSet(a, x) {
					want[x] = applyNum(composeNum(model.IntOverride{}, e, maxListLen), want[x], maxListLen)
				}
			}
			var merged model.Overlay
			merged.Merge(model.Overlay{Reps: base})
			merged.Merge(model.Overlay{Reps: delta})
			for _, x := range grid {
				got := script(x)
				if o, ok := setValue(merged.Reps, x); ok {
					got = applyNum(o, got, maxListLen)
				}
				if got != want[x] {
					t.Fatalf("sequence %d step %d: write %+v %+v: at %+v got %d, want %d", seq, step, a, e, x, got, want[x])
				}
			}
		}
	}
}
