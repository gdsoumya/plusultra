# Reference syntax: Bench Press / 1x8 80kg / update: custom() {~
#                if (setIndex == 1 && completedReps[1] >= reps[1]) {
#                  numberOfSets = 4
#                  sets(2, 4, floor(reps[1] / 2), floor(reps[1] / 2), 0, weights[1], 90, 0, 0)
#                }
#              ~}
#              Squat / 3x5 100kg / update: custom() {~ if (setIndex == 1) { weights = completedWeights[1] - 10kg } ~}
program("Update hooks", weeks=2)

def back_off(ctx):
    if ctx.set_index == 1 and ctx.completed_reps[1] >= ctx.reps[1]:
        ctx.number_of_sets = 4
        ctx.sets(2, 4, ctx.reps[1] // 2, ctx.reps[1] // 2, False, ctx.weights[1], 90, 0, False)

def drop(ctx):
    if ctx.set_index == 1:
        ctx.weights = ctx.completed_weights[1] - kg(10)
        ctx.rest = 150

day(1, 1, "Full body", weeks="1-2")
ex("Bench Press", warmup=[], sets=[s(1, 8, kg(80))], update=custom(back_off))
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], update=custom(drop), progress=lp(kg(2.5)))
