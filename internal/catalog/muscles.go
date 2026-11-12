package catalog

import (
	"maps"
	"slices"
	"strings"
)

// The muscle taxonomy (R83): groups, each with optional regions. Mirrored in
// app/src/domain/muscles.ts; change both together.

// Groups in display order.
var Groups = []string{
	"chest", "shoulders", "rotator cuff", "biceps", "triceps", "forearms",
	"abdominals", "obliques", "serratus", "lats", "middle back", "lower back", "traps", "neck",
	"glutes", "hip flexors", "quadriceps", "hamstrings", "adductors", "abductors", "calves",
}

// Regions maps each region to its group.
var Regions = map[string]string{
	"upper chest": "chest", "mid chest": "chest", "lower chest": "chest",
	"front delts": "shoulders", "side delts": "shoulders", "rear delts": "shoulders",
	"biceps long head": "biceps", "biceps short head": "biceps", "brachialis": "biceps",
	"triceps long head": "triceps", "triceps lateral head": "triceps", "triceps medial head": "triceps",
	"forearm flexors": "forearms", "forearm extensors": "forearms", "brachioradialis": "forearms",
	"upper abs": "abdominals", "lower abs": "abdominals",
	"rhomboids":   "middle back",
	"erectors":    "lower back",
	"upper traps": "traps", "mid traps": "traps", "lower traps": "traps",
	"glute max": "glutes", "glute med": "glutes",
	"rectus femoris": "quadriceps", "vastus": "quadriceps",
	"gastrocnemius": "calves", "soleus": "calves", "tibialis": "calves",
}

var synonyms = map[string]string{
	"pecs": "chest", "pectorals": "chest", "upper pecs": "upper chest", "lower pecs": "lower chest",
	"delts": "shoulders", "deltoids": "shoulders", "anterior deltoid": "front delts", "front delt": "front delts",
	"lateral deltoid": "side delts", "medial deltoid": "side delts", "side delt": "side delts",
	"posterior deltoid": "rear delts", "rear delt": "rear delts",
	"abs": "abdominals", "quads": "quadriceps", "hams": "hamstrings", "latissimus": "lats",
	"gluteus maximus": "glute max", "gluteus medius": "glute med", "erector spinae": "erectors", "spinal erectors": "erectors",
}

// Normalize maps a group, region or synonym, in any case and spacing, to its
// canonical name.
func Normalize(name string) (string, bool) {
	n := normalizeToken(name)
	if slices.Contains(Groups, n) {
		return n, true
	}
	if _, ok := Regions[n]; ok {
		return n, true
	}
	n, ok := synonyms[n]
	return n, ok
}

// MuscleNames lists every group and region, sorted, for error messages.
func MuscleNames() []string {
	return slices.Sorted(slices.Values(slices.Concat(Groups, slices.Collect(maps.Keys(Regions)))))
}

func normalizeMuscles(names []string) ([]string, error) {
	var out []string
	for _, name := range names {
		n, ok := Normalize(name)
		if !ok {
			return nil, invalid("unknown muscle %q; use one of: %s", name, strings.Join(MuscleNames(), ", "))
		}
		if !slices.Contains(out, n) {
			out = append(out, n)
		}
	}
	return out, nil
}
