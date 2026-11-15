package programcatalog

// volume holds the volume-first and bodyweight programs.
var volume = []Entry{
	{Slug: "dumbbell-ppl", Name: "Dumbbell PPL", Author: "common dumbbell push/pull/legs split", Level: "beginner", Days: 3,
		Summary: "Push, pull and legs once a week with dumbbells only, three sets per exercise. Reps climb through the range, then the dumbbells go up 2.5 kg and the reps drop back."},
	{Slug: "bodyweight-rr", Name: "Recommended Routine", Author: "r/bodyweightfitness", Level: "beginner", Days: 3,
		Summary: "Three full-body days of paired bodyweight exercises for 3x5-8 and a core triplet for 3x8-12. Each slot is a ladder of variations: reaching the top of the range moves to the harder one at the bottom."},
	{Slug: "german-volume-training", Name: "German Volume Training", Author: "Charles Poliquin", Level: "intermediate", Days: 3,
		Summary: "Three days a week for six weeks. Each day pairs two lifts for ten sets of ten at one weight with 90 seconds rest, then two accessories for three sets. A lift adds weight only once all hundred reps are done."},
	{Slug: "lyles-generic-bulking", Name: "Lyle's Generic Bulking Routine", Author: "Lyle McDonald", Level: "intermediate", Days: 4,
		Summary: "Lower and upper days twice a week. Compound lifts for 6-8 reps, lighter lifts for 10-12 and arms for 12-15; every lift adds a rep per good session and adds weight when all sets reach the top of its range."},
}
