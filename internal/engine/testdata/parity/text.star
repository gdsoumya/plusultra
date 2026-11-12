# Reference syntax: print("e1RM: " + calculate1RM(completedWeights[1], completedReps[1]))
#              var.tier = isProgramExercise("t1: Squat") ? "heavy" : "light"
program("Text", weeks=2)

def note(ctx):
    e1rm = round_weight(calc_1rm(ctx.completed_weights[1], ctx.completed_reps[1]))
    print("e1RM: " + str(e1rm / kg(1)) + " kg")
    tier = "heavy" if ctx.label == "t1" else "light"
    print("%s %s x%d" % (tier, ctx.exercise, ctx.completed_reps[1]))
    print(ctx.exercise.upper(), "squat" in ctx.exercise.lower(), tier < "medium")
    ctx.state.last = tier + ": " + ctx.exercise

day(1, 1, "Lower", weeks="1-2")
ex("Squat", label="t1", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(note, last=""))
