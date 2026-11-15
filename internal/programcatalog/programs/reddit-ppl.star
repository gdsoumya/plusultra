# Reddit PPL, after Metallicadpa, in kg.
#
# Six days a week, Monday to Saturday: pull, push, legs, pull, push, legs,
# then the week loops. Each day opens with a heavy barbell lift in sets of 5,
# the last one as many reps as possible: deadlift on the first pull day and
# rows on the second, bench on the first push day and press on the second,
# squat on both leg days. Accessories follow for 8-12 reps, face pulls and
# lateral raises for 15-20.
#
# Main lifts add 2.5 kg every session, 5 kg for squat and deadlift, and drop
# 10% after a session that misses its reps. Accessories add a rep each
# successful session until the top of their range, then add weight and start
# the range over.
program("Reddit PPL", weeks=1, units="kg", notes="Six days a week, Monday to Saturday")

OWN_WEIGHT = "Starting weight is a placeholder: set yours by editing the program."

def main(name, weight, inc, n=4, **kw):
    """n sets of 5 and a 5+ set; +inc after a full session, -10% after a miss."""
    sets = [s(n, 5, weight, rest=180)] if n else []
    sets.append(s(1, 5, weight, rest=180, amrap=True))
    ex(name, sets=sets, progress=lp(inc, fail_reset=0.9), **kw)

def acc(name, weight, lo=8, hi=12, n=3, inc=kg(2.5), warmup=[], desc=OWN_WEIGHT, **kw):
    ex(name, warmup=warmup, desc=desc, sets=[s(n, lo, weight, rest=90)], progress=dp(inc, lo, hi), **kw)

def pull(d):
    day(1, d, "Pull")
    if d == 1:
        main("Deadlift", pct(75), kg(5), n=0)
    else:
        main("Bent Over Row", kg(60), kg(2.5), desc=OWN_WEIGHT)
    acc("Pull Up", bw(), alts=["Lat Pulldown"], desc=None)
    acc("Seated Row", kg(50))
    acc("Face Pull", kg(20), 15, 20, n=5)
    acc("Hammer Curl", kg(12.5), n=4, eq="Dumbbell")
    acc("Bicep Curl", kg(12.5), n=4, eq="Dumbbell")

def push(d, heavy, light):
    day(1, d, "Push")
    main(heavy, pct(70), kg(2.5))
    acc(light, pct(60), label="volume", warmup=None, desc=None)
    acc("Incline Bench Press, Dumbbell", kg(22.5))
    acc("Triceps Pushdown", kg(25), superset="A")
    acc("Lateral Raise", kg(10), 15, 20, superset="A", eq="Dumbbell")
    acc("Overhead Triceps Extension", kg(15), superset="B", eq="Dumbbell")
    acc("Lateral Raise", kg(10), 15, 20, label="B", superset="B", eq="Dumbbell", desc=None)

def legs(d):
    day(1, d, "Legs")
    main("Squat", pct(70), kg(5), n=2)
    acc("Romanian Deadlift", kg(80), inc=kg(5), warmup=None)
    acc("Leg Press", kg(120), inc=kg(5))
    acc("Lying Leg Curl", kg(40), alts=["Seated Leg Curl"])
    acc("Standing Calf Raise", kg(60), n=5, inc=kg(5), alts=["Seated Calf Raise"])

pull(1)
push(2, "Bench Press", "Overhead Press")
legs(3)
pull(4)
push(5, "Overhead Press", "Bench Press")
legs(6)
