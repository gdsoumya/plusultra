package programcatalog

// splits holds the push/pull/legs and power/hypertrophy splits.
var splits = []Entry{
	{Slug: "reddit-ppl", Name: "Reddit PPL", Author: "Metallicadpa", Level: "intermediate", Days: 6,
		Summary: "Pull, push and legs twice a week, Monday to Saturday. Each day opens with a barbell lift in sets of 5, the last one for as many reps as possible, that adds weight every session and drops 10% after a miss; accessories work 8-12 reps and add weight at the top of the range."},
	{Slug: "phul", Name: "PHUL", Author: "Brandon Campbell", Level: "intermediate", Days: 4,
		Summary: "Upper and lower power days Monday and Tuesday work the compound lifts for 3-5 reps and add weight once every set reaches 5; upper and lower hypertrophy days Thursday and Friday use 8-12 and 10-15 reps with double progression."},
	{Slug: "phat", Name: "PHAT", Author: "Layne Norton", Level: "intermediate", Days: 5,
		Summary: "Two power days, Monday upper and Tuesday lower, for 3-5 reps, then three hypertrophy days Thursday to Saturday for back and shoulders, legs, and chest and arms. Each hypertrophy day opens with 6x3 speed sets at two thirds of the power day's weight, which move with it."},
	{Slug: "golden-six", Name: "Arnold's Golden Six", Author: "Arnold Schwarzenegger", Level: "beginner", Days: 3,
		Summary: "One full-body workout of six exercises Monday, Wednesday and Friday: squat, wide-grip bench, chin-ups, behind-the-neck press, barbell curls and sit-ups. Barbell lifts add 2.5 kg after every completed session; chin-ups and sit-ups go to failure and add a rep."},
}
