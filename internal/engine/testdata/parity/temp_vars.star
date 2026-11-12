# Reference syntax: Squat / 2x8 100kg / progress: custom() {~
#                var.x = floor(completedReps[1] / 2)
#                reps[2] = var.x
#              ~}
program("Temporary variables", weeks=2)

def halve(ctx):
    x = ctx.completed_reps[1] // 2
    target = ctx.completed_weights[1] + kg(2.5)
    ctx.reps[2] = x
    ctx.weights[1] = target

day(1, 1, "Lower", weeks="1-2")
ex("Squat", warmup=[], sets=[s(2, 8, kg(100))], progress=custom(halve))
