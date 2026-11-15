// Package programcatalog holds the built-in workout programs shipped with the
// server: popular, well-documented methods written in the program language,
// which users copy into drafts of their own.
package programcatalog

import (
	"cmp"
	"embed"
	"fmt"
	"slices"

	"github.com/gdsoumya/plusultra/internal/engine"
	"github.com/gdsoumya/plusultra/internal/model"
)

//go:embed programs/*.star
var scripts embed.FS

// Levels orders the catalogue: beginners first.
var Levels = []string{"beginner", "intermediate", "advanced"}

// Entry is one built-in program. Weeks, Loop and Script are read from
// programs/<Slug>.star, whose program() name must equal Name.
type Entry struct {
	Slug, Name, Author, Summary string
	// Level is one of Levels.
	Level string
	// Days is how many days a week the program trains.
	Days   int
	Weeks  int
	Loop   bool
	Script string
	// Maxes are the exercises loaded with pct(), which need a 1RM.
	Maxes []string
}

var core = []Entry{
	{Slug: "beginner-full-body", Name: "Beginner full body", Author: "r/Fitness basic beginner routine", Level: "beginner", Days: 3,
		Summary: "Two workouts alternate three days a week. Main lifts add weight after every completed session; three misses in a row take 10% off and build back. Chin-ups add reps up to 8 before adding weight."},
	{Slug: "gzclp", Name: "GZCLP", Author: "Cody Lefever", Level: "beginner", Days: 3,
		Summary: "Four workouts rotate over three days a week. T1 main lifts run 5x3+, then 6x2+, then 10x1+ before a 5RM retest; T2 lifts step through 3x10, 3x8 and 3x6; T3 accessories add weight once the last set reaches 25 reps."},
	{Slug: "531-for-beginners", Name: "5/3/1 for Beginners", Author: "Jim Wendler", Level: "beginner", Days: 3,
		Summary: "Three full-body days over a three-week wave. Each day trains two main lifts with the 5/3/1 sets, the last one for as many reps as possible, then 5x5 at the first set's weight and bodyweight assistance. Maxes go up after week 3."},
}

// entries is every batch; files beside this one each hold one.
var entries = slices.Concat(core, linear, wave, splits, volume, blocks)

func init() {
	for i := range entries {
		e := &entries[i]
		src, err := scripts.ReadFile("programs/" + e.Slug + ".star")
		if err != nil {
			panic(err)
		}
		p, err := engine.Evaluate(string(src), e.Slug+".star")
		if err != nil {
			panic(fmt.Sprintf("catalog program %s: %v", e.Slug, err))
		}
		if p.Name != e.Name {
			panic(fmt.Sprintf("catalog program %s is named %q, not %q", e.Slug, p.Name, e.Name))
		}
		e.Script, e.Weeks, e.Loop, e.Maxes = string(src), p.Weeks, p.Loop, maxes(p)
	}
	slices.SortStableFunc(entries, func(a, b Entry) int {
		return cmp.Compare(slices.Index(Levels, a.Level), slices.Index(Levels, b.Level))
	})
}

// All lists every program, beginners first.
func All() []Entry { return slices.Clone(entries) }

// Get returns the program with slug.
func Get(slug string) (Entry, bool) {
	i := slices.IndexFunc(entries, func(e Entry) bool { return e.Slug == slug })
	if i < 0 {
		return Entry{}, false
	}
	return entries[i], true
}

// maxes lists the exercises whose working sets are a percentage of a 1RM.
func maxes(p *model.Program) []string {
	var out []string
	for _, d := range p.Days {
		for _, ex := range d.Exercises {
			groups := append(slices.Concat(ex.Variations...), ex.Sets...)
			if slices.ContainsFunc(groups, func(g model.SetGroup) bool { return g.Load.Kind == model.LoadPercent }) {
				out = append(out, ex.Names[0])
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}
