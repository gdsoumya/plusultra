# Reference syntax: Squat / 3x5 100kg / progress: custom() {~
#                for (var.i in completedReps) {
#                  if (completedReps[var.i] >= reps[var.i]) { weights[var.i] = weights[var.i] + 2.5kg }
#                }
#              ~}
# Indexed writes are absolute and cover every week, as in the reference syntax, so
# week 2 takes week 1's new weights. Starlark loops also build the program.
program("Loops", weeks=2)

def per_set(ctx):
    for i in range(1, ctx.number_of_sets + 1):
        if ctx.completed_reps[i] >= ctx.reps[i]:
            ctx.weights[i] = ctx.weights[i] + kg(2.5)
    total = 0
    for r in ctx.completed_reps:
        total += r
    print("total reps", total)

for week, top in [(1, 100), (2, 105)]:
    day(week, 1, "Lower")
    ex("Squat", warmup=[], sets=[s(1, 5, kg(top)), s(2, 5, kg(top - 10))], progress=custom(per_set))
