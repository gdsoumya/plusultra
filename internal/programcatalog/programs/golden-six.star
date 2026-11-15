# Arnold's Golden Six, after Arnold Schwarzenegger, in kg.
#
# One full-body workout three days a week, Monday, Wednesday and Friday, and
# the week loops: squats, wide-grip bench press, chin-ups, behind-the-neck
# press, barbell curls and bent-knee sit-ups. The barbell lifts add 2.5 kg
# after every session that completes all its sets of 10; chin-ups and
# sit-ups are taken to failure and add a rep to their target instead.
program("Arnold's Golden Six", weeks=1, units="kg", notes="The same workout Monday, Wednesday and Friday")

OWN_WEIGHT = "Starting weight is a placeholder: set yours by editing the program."

def add_rep(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.reps += 1

def lift(name, n, weight, **kw):
    ex(name, sets=[s(n, 10, weight, rest=120)], progress=lp(kg(2.5)), **kw)

def to_failure(name, reps, **kw):
    ex(name, sets=[s(3, reps, bw(), rest=90, amrap=True)], progress=custom(add_rep), **kw)

for d in (1, 3, 5):
    day(1, d, "Golden Six")
    lift("Squat", 4, pct(60))
    lift("Bench Press", 3, pct(60), desc="Wide grip: hands a palm's width outside the rings.")
    to_failure("Chin Up", 5)
    lift("Behind The Neck Press", 4, kg(30), desc=OWN_WEIGHT)
    lift("Bicep Curl, Barbell", 3, kg(20), desc=OWN_WEIGHT)
    to_failure("Sit Up", 15, desc="Knees bent, feet flat.")
