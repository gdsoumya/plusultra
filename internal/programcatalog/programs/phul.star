# PHUL, after Brandon Campbell, in kg.
#
# Four days a week, and the week loops: upper power Monday, lower power
# Tuesday, upper hypertrophy Thursday, lower hypertrophy Friday. Power days
# work the big barbell lifts for 3-5 reps with assistance for 6-10; the
# hypertrophy days use 8-12 reps, 10-15 for machines.
#
# Power lifts add weight once every set reaches the top of its range. The
# rest add a rep each successful session until the top of their range, then
# add weight and start the range over.
program("PHUL", weeks=1, units="kg", notes="Power Monday and Tuesday, hypertrophy Thursday and Friday")

OWN_WEIGHT = "Starting weight is a placeholder: set yours by editing the program."

def power(name, n, lo, hi, weight, inc, **kw):
    ex(name, sets=[s(n, (lo, hi), weight, rest=180)], progress=lp(inc), **kw)

def hyper(name, n, lo, hi, weight, inc=kg(2.5), warmup=[], desc=OWN_WEIGHT, **kw):
    ex(name, warmup=warmup, desc=desc, sets=[s(n, lo, weight, rest=90)], progress=dp(inc, lo, hi), **kw)

day(1, 1, "Upper power")
power("Bench Press", 4, 3, 5, pct(80), kg(2.5))
hyper("Incline Bench Press, Dumbbell", 3, 6, 10, kg(22.5))
power("Bent Over Row", 4, 3, 5, kg(70), kg(2.5), desc=OWN_WEIGHT)
hyper("Lat Pulldown", 3, 6, 10, kg(55))
power("Overhead Press", 3, 5, 8, pct(65), kg(2.5))
hyper("Bicep Curl, Barbell", 3, 6, 10, kg(25))
hyper("Skullcrusher", 3, 6, 10, kg(20))

day(1, 2, "Lower power")
power("Squat", 4, 3, 5, pct(80), kg(5))
power("Deadlift", 3, 3, 5, pct(80), kg(5))
hyper("Leg Press", 4, 10, 15, kg(120), kg(5))
hyper("Lying Leg Curl", 3, 6, 10, kg(45), alts=["Seated Leg Curl"])
hyper("Standing Calf Raise", 4, 6, 10, kg(80), kg(5))

day(1, 4, "Upper hypertrophy")
hyper("Incline Bench Press", 4, 8, 12, kg(50), warmup=None)
hyper("Chest Fly", 3, 8, 12, kg(12.5), eq="Dumbbell")
hyper("Seated Row", 4, 8, 12, kg(55))
hyper("One Arm Row", 3, 8, 12, kg(22.5), eq="Dumbbell")
hyper("Lateral Raise", 3, 8, 12, kg(10), eq="Dumbbell")
hyper("Incline Bicep Curl", 3, 8, 12, kg(10), eq="Dumbbell")
hyper("Triceps Extension, Cable", 3, 8, 12, kg(25))

day(1, 5, "Lower hypertrophy")
hyper("Front Squat", 4, 8, 12, kg(60), kg(5), warmup=None)
hyper("Lunge, Barbell", 3, 8, 12, kg(40), warmup=None)
hyper("Leg Extension", 3, 10, 15, kg(50), kg(5))
hyper("Lying Leg Curl", 3, 10, 15, kg(40), label="volume")
hyper("Seated Calf Raise", 3, 8, 12, kg(50), kg(5))
hyper("Calf Press on Leg Press", 3, 8, 12, kg(100), kg(5))
