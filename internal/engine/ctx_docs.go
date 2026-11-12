package engine

const (
	inBoth     = "progress, update"
	inProgress = "progress"
	inNone     = "no"
)

// plannedDoc is the doc of a planned per-set vector.
func plannedDoc(name, typ, doc string) AttrDoc {
	return AttrDoc{Name: name, Type: typ, Read: inBoth, Write: inBoth, Doc: doc}
}

// resultDoc is the doc of a per-set vector of the workout's results.
func resultDoc(name, typ, doc string) AttrDoc {
	return AttrDoc{Name: name, Type: typ, Read: inBoth, Write: inNone, Doc: doc + " A set not done reads 0."}
}

var ctxDocs = []AttrDoc{
	{"week", "int", inBoth, inNone, "Week of the workout."},
	{"day", "int", inBoth, inNone, "Day number of the workout in its week, the same as day_in_week. There is no count of days across the program."},
	{"day_in_week", "int", inBoth, inNone, "Day number of the workout in its week."},
	{"exercise", "string", inBoth, inNone, "Name of the active alternate as written, such as \"Bench Press, Dumbbell\"."},
	{"program_exercise", "string", inBoth, inNone, "The program line: \"label: Name\" with the first name, or \"Name\" without a label. the reference syntax's exercise."},
	{"label", "string", inBoth, inNone, "The exercise's label, \"\" when it has none."},
	{"rm1", "weight", inBoth, inProgress, "1RM of the active alternate, a zero weight when unknown. Setting it moves every pct() set: ctx.rm1 += kg(5)."},
	{"bodyweight", "weight", inBoth, inNone, "The user's bodyweight."},
	{"set_index", "int", "update", inNone, "Working sets completed so far in this workout, 0 before the first."},
	{"number_of_sets", "int", inBoth, inBoth, "Working sets of this workout. Assigning it, += included, sets the count in every week and day (absolute); add_sets(n) keeps each week's own count. In update it resizes this workout only."},
	{"program_number_of_sets", "int", inBoth, inNone, "Working sets the program plans for this week, day and set variation, before this run's writes."},
	{"completed_number_of_sets", "int", inBoth, inNone, "Working sets completed."},
	{"add_sets", "method", inBoth, inBoth, "add_sets(n) adds n sets (negative removes) to every week and day, each keeping its own count plus n (relative)."},
	plannedDoc("weights", "vector of weights", "Planned weight of each working set, after rounding, in program units."),
	plannedDoc("reps", "vector of ints", "Planned reps of each set: the top of a range."),
	plannedDoc("min_reps", "vector of ints", "Minimum reps of each set: the bottom of a range, else reps."),
	plannedDoc("rpe", "vector of floats", "Planned RPE of each set, 0 when none."),
	plannedDoc("rest", "vector of ints", "Rest timer of each set in seconds."),
	plannedDoc("work", "vector of ints", "Active set timer of each set in seconds, 0 when none."),
	plannedDoc("amrap", "vector of bools", "Whether each set is as many reps as possible."),
	plannedDoc("ask", "vector of bools", "Whether each set asks the user for the weight."),
	plannedDoc("log_rpe", "vector of bools", "Whether each set asks for the RPE done."),
	{"original_weights", "vector of weights", inBoth, inNone, "Planned weight of each set before rounding."},
	resultDoc("completed_weights", "vector of weights", "Weight done in each set."),
	resultDoc("completed_reps", "vector of ints", "Reps done in each set."),
	resultDoc("completed_reps_left", "vector of ints", "Reps done with the left side in each set, for one-sided exercises."),
	resultDoc("completed_rpe", "vector of floats", "RPE logged for each set."),
	resultDoc("completed_work", "vector of ints", "Set timer recorded for each set in seconds."),
	{"set_variation", "int", inBoth, inProgress, "Active set variation, 1-based; 1 without variations. Assigned values wrap around the count."},
	{"exercise_variation", "int", inBoth, inProgress, "Active alternate, 1-based over name then alts. Assigned values wrap around the count."},
	{"description_index", "int", inBoth, inProgress, "Active description of a desc list, 1-based. Assigned values wrap around the count."},
	{"state", "state", inBoth, inProgress, "The progression's state variables as attributes: ctx.state.misses += 1. Only declared names can be set."},
	{"shared", "method", inBoth, inProgress, "shared(tag) is one state store per int tag that any exercise can read and set, open to new names: ctx.shared(1).rating = 5."},
	{"at", "method", inProgress, inProgress, "at(week=0, day=0, variation=0) is a view of the exercise's sets there, 0 being this workout's own for reading and all for writing."},
	{"sets", "method", "update", "update", "sets(frm, to, min_reps, max_reps, amrap, weight, rest, rpe, log_rpe) sets sets frm to to of this workout, adding sets up to to."},
}
