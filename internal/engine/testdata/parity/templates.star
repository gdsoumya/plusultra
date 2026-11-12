# Reference syntax: t1 / used: none / 3x5 75% / progress: custom(inc: 5kg) {~ if (completedReps >= reps) { rm1 += state.inc } ~}
#              Squat / ...t1
#              Bench Press / ...t1 / progress: custom(inc: 2.5kg) { ...t1 }
#              Lateral Raise / 3x12 10kg / progress: dp(2.5kg, 12, 15)
#              Rear Delt Fly / ...Lateral Raise / 7.5kg
program("Reuse: templates and functions", weeks=2)

def bump_rm1(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.rm1 += ctx.state.inc

t1 = template(warmup=[], sets=[s(3, 5, pct(75))], progress=custom(bump_rm1, inc=kg(5)))

def accessory(name, weight, reps=12):
    ex(name, eq="dumbbell", warmup=[], sets=[s(3, reps, weight)], progress=dp(kg(2.5), reps, reps + 3))

day(1, 1, "Full body", weeks="1-2")
ex("Squat", **t1)
ex("Bench Press", **dict(t1, progress=custom(bump_rm1, inc=kg(2.5))))
accessory("Lateral Raise", kg(10))
accessory("Rear Delt Fly", kg(7.5))
