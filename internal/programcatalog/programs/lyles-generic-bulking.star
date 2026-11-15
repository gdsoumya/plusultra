# Lyle's Generic Bulking Routine, after Lyle McDonald, in kg.
#
# Upper/lower split: lower on Monday and Thursday, upper on Tuesday and
# Friday, the same workouts both times. Each day leads with compound lifts for
# 6-8 reps and three minutes rest, then lighter lifts for 10-12 with two
# minutes; the upper days finish with arms for 12-15. Every lift adds a rep
# per successful session until all sets reach the top of its range, then adds
# weight and drops back to the bottom.
program("Lyle's Generic Bulking Routine", weeks=1, units="kg",
        notes="Four days a week; run six to eight weeks, then take a light week")

START = "Starting weight is a guess; set your own before the first session by editing the program."

def lift(name, n, lo, hi, weight, inc, rest, alts=None, desc=START):
    ex(name, alts=alts, desc=desc, sets=[s(n, lo, weight, rest=rest)], progress=dp(inc, lo, hi))

def lower(n):
    day(1, n, "Lower")
    lift("Squat", 3, 6, 8, pct(75), kg(5), 180, desc=None)
    lift("Stiff Leg Deadlift", 3, 6, 8, kg(80), kg(5), 180)
    lift("Leg Press", 3, 10, 12, kg(140), kg(5), 120)
    lift("Lying Leg Curl", 3, 10, 12, kg(40), kg(2.5), 120)
    lift("Standing Calf Raise", 3, 6, 8, kg(100), kg(5), 180)
    lift("Seated Calf Raise", 3, 10, 12, kg(50), kg(5), 120)

def upper(n):
    day(1, n, "Upper")
    lift("Bench Press", 3, 6, 8, pct(75), kg(2.5), 180, desc=None)
    lift("Bent Over Row", 3, 6, 8, kg(60), kg(2.5), 180)
    lift("Incline Bench Press", 3, 10, 12, kg(50), kg(2.5), 120, alts=["Overhead Press"])
    lift("Lat Pulldown", 3, 10, 12, kg(50), kg(2.5), 120)
    lift("Triceps Pushdown", 2, 12, 15, kg(25), kg(2.5), 90)
    lift("Bicep Curl, Barbell", 2, 12, 15, kg(20), kg(2.5), 90)

lower(1)
upper(2)
lower(4)
upper(5)
