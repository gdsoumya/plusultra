# Reference syntax: Squat / 4x3, 1x3+ / 5x2, 1x2+ / 9x1, 1x1+ / 100kg / progress: custom() {~
#                if (completedReps >= reps) { weights = completedWeights[ns] + 5kg }
#                else { setVariationIndex += 1 }
#              ~}
#              Bench Press / 3x8 70kg / ! 5x5 80kg / progress: custom() {~ setVariationIndex += 1 ~}
program("Set variations", weeks=2)

def stage(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.weights = ctx.completed_weights[ctx.number_of_sets] + kg(5)
    else:
        ctx.set_variation += 1

def rotate(ctx):
    ctx.set_variation += 1

day(1, 1, "Upper", weeks="1-2")
ex("Squat", warmup=[], variations=[
    [s(4, 3, kg(100)), s(1, 3, kg(100), amrap=True)],
    [s(5, 2, kg(100)), s(1, 2, kg(100), amrap=True)],
    [s(9, 1, kg(100)), s(1, 1, kg(100), amrap=True)],
], progress=custom(stage))
# set_variation= marks the starting variation, as ! does.
ex("Bench Press", warmup=[], variations=[[s(3, 8, kg(70))], [s(5, 5, kg(80))]], set_variation=2, progress=custom(rotate))
