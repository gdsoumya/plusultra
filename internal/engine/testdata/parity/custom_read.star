# Reference progress reads: weights, originalWeights, completedWeights, reps,
# minReps, completedReps, completedRepsLeft, RPE, completedRPE, timers,
# setTime, completedSetTime, amraps, askweights, logrpes, rm1, bodyweight,
# week, day, dayInWeek, numberOfSets, completedNumberOfSets,
# setVariationIndex, exerciseVariationIndex, descriptionIndex, exercise,
# programNumberOfSets.
program("Custom progress reads", weeks=2)

def show(ctx):
    print("weights", ctx.weights, "original", ctx.original_weights, "completed", ctx.completed_weights)
    print("reps", ctx.reps, "min", ctx.min_reps, "completed", ctx.completed_reps, "left", ctx.completed_reps_left)
    print("rpe", ctx.rpe, "completed", ctx.completed_rpe, "log", ctx.log_rpe)
    print("rest", ctx.rest, "work", ctx.work, "completed work", ctx.completed_work)
    print("amrap", ctx.amrap, "ask", ctx.ask)
    print("rm1", ctx.rm1, "bodyweight", ctx.bodyweight)
    print("week", ctx.week, "day", ctx.day, "day_in_week", ctx.day_in_week)
    print("sets", ctx.number_of_sets, "completed", ctx.completed_number_of_sets, "program", ctx.program_number_of_sets)
    print("set_variation", ctx.set_variation, "exercise_variation", ctx.exercise_variation,
          "description_index", ctx.description_index)
    print("exercise", ctx.exercise, "program_exercise", ctx.program_exercise, "label", ctx.label)

day(1, 1, "Lower", weeks="1-2")
ex("Squat", label="main", alts=["Front Squat"], desc=["Brace", "Breathe"], warmup=[],
   variations=[
       [s(2, (3, 5), kg(100), rpe=8, rest=180, log_rpe=True), s(1, 5, kg(90), amrap=True, ask=True)],
       [s(5, 3, kg(110))],
   ],
   progress=custom(show))
ex("Plank", sets=[s(2, 1, bw(), work=45, rest=60)], progress=custom(show))
# 73% of 140 kg is 102.2 kg before rounding.
ex("Squat", label="light", warmup=[], sets=[s(2, 5, pct(73))], progress=custom(show))
