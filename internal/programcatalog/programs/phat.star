# PHAT, after Layne Norton, in kg.
#
# Five days a week, and the week loops: upper power Monday, lower power
# Tuesday, then three hypertrophy days, back and shoulders Thursday, legs
# Friday, chest and arms Saturday. Power days work a row, a dumbbell press
# and the squat for 3-5 reps with assistance for 6-10. Each hypertrophy day
# opens with speed work, 6x3 at about two thirds of the power day's weight,
# then pump work for 8-20 reps.
#
# A power lift adds weight once every set reaches the top of its range, and
# its speed sets move with it. Everything else adds a rep each successful
# session until the top of its range, then adds weight and starts over.
program("PHAT", weeks=1, units="kg", notes="Power Monday and Tuesday, hypertrophy Thursday to Saturday")

OWN_WEIGHT = "Starting weight is a placeholder: set yours by editing the program."
SPEED = 0.675

def power_up(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.at(day=ctx.day_in_week).weights += ctx.state.inc
        ctx.at(day=ctx.state.speed_day).weights += ctx.state.inc * SPEED

def power(name, weight, inc, speed_day, **kw):
    """3x3-5 today; the same lift is 6x3 at SPEED of this weight on speed_day."""
    ex(name, sets=[s(3, (3, 5), weight, rest=180)], progress=custom(power_up, inc=inc, speed_day=speed_day), **kw)

def speed(name, weight, **kw):
    ex(name, desc="Speed work: explosive reps, well short of failure.", sets=[s(6, 3, weight, rest=90)],
       progress="none", **kw)

def hyper(name, n, lo, hi, weight, inc=kg(2.5), warmup=[], desc=OWN_WEIGHT, **kw):
    ex(name, warmup=warmup, desc=desc, sets=[s(n, lo, weight, rest=75)], progress=dp(inc, lo, hi), **kw)

ROW = kg(70)
PRESS = kg(25)
SQUAT = 82  # percent of 1RM

day(1, 1, "Upper power")
power("Bent Over Row", ROW, kg(2.5), 4, alts=["Pendlay Row"], desc=OWN_WEIGHT)
hyper("Weighted Pull Up", 2, 6, 10, bw(), desc=None)
hyper("Chin Up", 2, 6, 10, bw(), desc="Rack chins: feet on a bench in front, torso leaning back.")
power("Bench Press, Dumbbell", PRESS, kg(2.5), 6, desc=OWN_WEIGHT)
hyper("Dip", 2, 6, 10, bw(), desc=None)
hyper("Dumbbell Shoulder Press", 3, 6, 10, kg(20), eq="Dumbbell")
hyper("Bicep Curl, EZ Bar", 3, 6, 10, kg(20))
hyper("Skullcrusher", 3, 6, 10, kg(20))

day(1, 2, "Lower power")
power("Squat", pct(SQUAT), kg(5), 5)
hyper("Hack Squat", 2, 6, 10, kg(80), kg(5))
hyper("Leg Extension", 2, 6, 10, kg(50), kg(5))
hyper("Stiff Leg Deadlift", 3, 5, 8, kg(80), kg(5), warmup=None)
hyper("Lying Leg Curl", 2, 6, 10, kg(40), alts=["Glute Ham Raise"])
hyper("Standing Calf Raise", 3, 6, 10, kg(80), kg(5))
hyper("Seated Calf Raise", 2, 6, 10, kg(50), kg(5))

day(1, 4, "Back and shoulders")
speed("Bent Over Row", ROW * SPEED, alts=["Pendlay Row"])
hyper("Chin Up", 3, 8, 12, bw(), label="volume", desc=None)
hyper("Seated Row", 3, 8, 12, kg(55))
hyper("Incline Row", 2, 12, 15, kg(15), alts=["Shrug, Dumbbell"], eq="Dumbbell")
hyper("Close Grip Lat Pulldown", 2, 15, 20, kg(40))
hyper("Dumbbell Shoulder Press", 3, 8, 12, kg(17.5), label="volume", eq="Dumbbell")
hyper("Upright Row", 2, 12, 15, kg(25))
hyper("Lateral Raise", 3, 12, 20, kg(7.5), alts=["Lateral Raise, Cable"], eq="Dumbbell")

day(1, 5, "Lower hypertrophy")
speed("Squat", pct(SQUAT * SPEED))
hyper("Hack Squat", 3, 8, 12, kg(70), kg(5), label="volume")
hyper("Leg Press", 2, 12, 15, kg(120), kg(5))
hyper("Leg Extension", 3, 15, 20, kg(40), kg(5), label="volume")
hyper("Romanian Deadlift", 3, 8, 12, kg(70), kg(5), warmup=None)
hyper("Lying Leg Curl", 2, 12, 15, kg(35), label="volume")
hyper("Seated Leg Curl", 2, 15, 20, kg(30))
hyper("Standing Calf Raise", 4, 10, 15, kg(60), kg(5), label="volume", alts=["Donkey Calf Raise"])
hyper("Seated Calf Raise", 3, 15, 20, kg(35), kg(5), label="volume")

day(1, 6, "Chest and arms")
speed("Bench Press, Dumbbell", PRESS * SPEED)
hyper("Incline Bench Press, Dumbbell", 3, 8, 12, kg(22.5))
hyper("Chest Press", 3, 12, 15, kg(50), kg(5))
hyper("Incline Chest Fly, Cable", 2, 15, 20, kg(10))
hyper("Preacher Curl", 3, 8, 12, kg(20))
hyper("Concentration Curl", 2, 12, 15, kg(10), eq="Dumbbell")
hyper("Spider Curl", 2, 15, 20, kg(15))
hyper("Triceps Extension, EZ Bar", 3, 8, 12, kg(20), desc="Seated, pressed overhead. " + OWN_WEIGHT)
hyper("Triceps Pushdown Rope", 2, 12, 15, kg(20))
hyper("Triceps Kickback", 2, 15, 20, kg(7.5), eq="Dumbbell")
