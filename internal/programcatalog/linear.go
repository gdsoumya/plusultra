package programcatalog

// linear holds the beginner barbell programs with a linear progression.
var linear = []Entry{
	{Slug: "starting-strength-1", Name: "Starting Strength, Phase 1", Author: "Mark Rippetoe", Level: "beginner", Days: 3,
		Summary: "Workouts A and B alternate three days a week: squat and deadlift every session, press or bench in turn. Each lift adds weight every session it is completed; three misses in a row take 10% off and build back."},
	{Slug: "starting-strength-2", Name: "Starting Strength, Phase 2", Author: "Mark Rippetoe", Level: "beginner", Days: 3,
		Summary: "Phase 1 with the power clean: workout A deadlifts and workout B power cleans, so each is trained every other session. Each lift adds weight every session it is completed; three misses in a row take 10% off and build back."},
	{Slug: "stronglifts-5x5", Name: "StrongLifts 5x5", Author: "Mehdi Hadim", Level: "beginner", Days: 3,
		Summary: "Workouts A and B alternate three days a week, five sets of five on every lift but a single deadlift set. Each lift adds 2.5 kg every session it is completed, the deadlift 5 kg; three misses in a row take 10% off and build back."},
	{Slug: "greyskull-lp", Name: "Phrak's Greyskull LP", Author: "John Sheaffer, Phrak's variant", Level: "beginner", Days: 3,
		Summary: "Two workouts alternate three days a week, every lift 2x5 then a last set for as many reps as possible. A completed session adds weight, double when the last set reaches 10 reps, and a miss takes the lift back to 90%."},
	{Slug: "ivysaur-4-4-8", Name: "Ivysaur 4-4-8", Author: "Ivysaur", Level: "beginner", Days: 3,
		Summary: "Two weeks of three full-body days with bench and press every day. Each lift rotates 4x4, 4x8 at 90% and 3x4 plus an AMRAP set, and jumps once a week, double when the AMRAP set reaches 8 reps."},
	{Slug: "fierce-5", Name: "Fierce 5 Novice", Author: "Fierce 5", Level: "beginner", Days: 3,
		Summary: "Workouts A and B alternate three days a week: 3x5 squat, front squat, bench and press, 3x8 rows, Romanian deadlifts and pulldowns, then isolation work. The barbell lifts add weight as they are completed, two misses take 10% off, and the isolation work adds reps before weight."},
}
