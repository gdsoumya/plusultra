# nSuns 5/3/1 LP, after nSuns, in kg.
#
# Four days a week, looping weekly. Each day has a T1 main lift over nine sets
# that climb to a 1+ single and pyramid back down, a T2 lift over eight
# lighter sets, and three or four accessories on double progression.
#
# Percentages are of a 90% training max, written as percentages of 1RM: the
# T1 single at 85.5% is 95% of it. The reps of the 1+ set raise the 1RM once
# a week: 0-1 reps nothing, 2-3 reps 2.5 kg, 4-5 reps 2.5 kg for bench and
# 5 kg for squat and deadlift, 6 or more 5 and 7.5 kg. The second bench day
# follows the first. Overhead press as T2 adds 2.5 kg to its 1RM when every
# set is done; the other T2 lifts start from a guess and add 2.5 kg the same
# way.
program("nSuns 5/3/1 LP", weeks=1, units="kg", notes="Four days a week; the 1+ set drives the training max")

T1 = [(5, 75), (3, 85), (1, 95), (3, 90), (3, 85), (3, 80), (5, 75), (5, 70), (5, 65)]
T2 = [(5, 50), (5, 60), (3, 70), (5, 70), (7, 70), (4, 70), (6, 70), (8, 70)]
TM = pct(90)
START = "The starting weight is a guess: edit the program to set your own."

def t1_progress(ctx):
    reps = ctx.completed_reps[3]
    if reps >= 6:
        ctx.rm1 += ctx.state.step + kg(2.5)
    elif reps >= 4:
        ctx.rm1 += ctx.state.step
    elif reps >= 2:
        ctx.rm1 += kg(2.5)

def t2_progress(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.rm1 += kg(2.5)

UPPER = custom(t1_progress, step=kg(2.5))
LOWER = custom(t1_progress, step=kg(5))

def t1(name, progress):
    sets = [s(1, reps, TM * p / 100, rest=180, amrap=i in (2, 8)) for i, (reps, p) in enumerate(T1)]
    ex(name, label="T1", sets=sets, progress=progress)

def t2(name, tm, progress, desc=None):
    ex(name, label="T2", desc=desc, sets=[s(1, reps, tm * p / 100, rest=120) for reps, p in T2], progress=progress)

def acc(name, weight, reps, hi, eq=None):
    ex(name, eq=eq, desc=START, warmup=[], sets=[s(3, reps, weight, rest=90)], progress=dp(kg(2.5), reps, hi))

day(1, 1, "Bench and press")
t1("Bench Press", UPPER)
t2("Overhead Press", TM, custom(t2_progress))
acc("Lat Pulldown", kg(40), 10, 12)
acc("Incline Bench Press, Dumbbell", kg(20), 10, 12)
acc("Bicep Curl", kg(10), 10, 15, eq="Dumbbell")
acc("Triceps Pushdown", kg(25), 12, 15)

day(1, 2, "Squat and sumo deadlift")
t1("Squat", LOWER)
t2("Sumo Deadlift", kg(120), lp(kg(2.5)), desc=START)
acc("Romanian Deadlift", kg(60), 8, 10)
acc("Lying Leg Curl", kg(40), 10, 12)
ex("Hanging Leg Raise", sets=[s(3, 10, bw(), rest=90)])

day(1, 3, "Bench and close grip")
t1("Bench Press", "none")
t2("Bench Press Close Grip", kg(70), lp(kg(2.5)), desc=START)
acc("Bent Over Row", kg(40), 8, 10)
acc("Lateral Raise", kg(10), 12, 15, eq="Dumbbell")
acc("Face Pull", kg(20), 15, 20)
acc("Hammer Curl", kg(10), 10, 15, eq="Dumbbell")

day(1, 4, "Deadlift and front squat")
t1("Deadlift", LOWER)
t2("Front Squat", kg(80), lp(kg(2.5)), desc=START)
ex("Pull Up", sets=[s(3, 8, bw(), rest=120)], progress=dp(kg(2.5), 8, 12))
acc("Seated Row", kg(40), 10, 12)
ex("Ab Wheel", sets=[s(3, 10, kg(0), rest=90)])
