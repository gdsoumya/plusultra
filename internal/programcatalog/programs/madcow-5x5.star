# Madcow 5x5, after Bill Starr and Madcow, in kg.
#
# Three days a week, looping weekly. Monday ramps five sets of five to a top
# set in squat, bench and row. Wednesday is light: squat ramps to Monday's
# third set, press and deadlift ramp over four sets. Friday ramps four sets,
# then a triple 2.5% above Monday's top set and a back-off eight at Monday's
# third set. Making the Friday triple raises the lift 2.5% for the next week,
# so Monday's top set becomes the weight of Friday's triple; two misses in a
# row take 10% off.
#
# Squat, bench, press and deadlift are percentages of 1RM. The first top set
# is 80%, about 93% of a 5RM, so the program reaches a 5RM in week 4. The row
# starts from a guess.
program("Madcow 5x5", weeks=1, units="kg", notes="Monday, Wednesday, Friday; Friday's triple sets next week's weights")

RAMP5 = [0.5, 0.625, 0.75, 0.875, 1]
RAMP4 = [0.625, 0.75, 0.875, 1]
TOP = pct(80)
START = "The starting weight is a guess: edit the program to set your own."

def weekly(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.weights *= 1.025
        ctx.state.misses = 0
    else:
        ctx.state.misses += 1
        if ctx.state.misses == 2:
            ctx.weights *= 0.9
            ctx.state.misses = 0

def ramp(top, fracs, reps=5):
    return [s(1, reps, top * f, rest=180) for f in fracs]

def lift(name, sets, **kw):
    # The ramp is the warmup.
    ex(name, warmup=[], sets=sets, **kw)

LIFTS = [("Squat", TOP, None), ("Bench Press", TOP, None), ("Bent Over Row", kg(60), START)]

day(1, 1, "Monday volume")
for name, top, desc in LIFTS:
    lift(name, ramp(top, RAMP5), desc=desc, progress="none")
ex("Back Extension", sets=[s(2, 10, kg(0), rest=90)])
ex("Sit Up", sets=[s(4, 25, bw(), rest=60)])

day(1, 3, "Wednesday light")
lift("Squat", ramp(TOP, [0.5, 0.625, 0.75, 0.75]), progress="none")
lift("Overhead Press", ramp(TOP, RAMP4), progress=custom(weekly, misses=0))
lift("Deadlift", ramp(TOP, RAMP4), progress=custom(weekly, misses=0))
ex("Sit Up", sets=[s(3, 25, bw(), rest=60)])

day(1, 5, "Friday intensity")
for name, top, desc in LIFTS:
    sets = ramp(top, RAMP5[:4]) + [s(1, 3, top * 1.025, rest=300, label="Triple"), s(1, 8, top * 0.75, rest=180, label="Back-off")]
    lift(name, sets, progress=custom(weekly, misses=0))
ex("Dip", sets=[s(3, 8, bw(), rest=90)])
ex("Bicep Curl, Barbell", desc=START, sets=[s(3, 8, kg(20), rest=90)], progress=dp(kg(2.5), 8, 10))
ex("Triceps Extension, Barbell", desc=START, sets=[s(3, 8, kg(20), rest=90)], progress=dp(kg(2.5), 8, 10))
