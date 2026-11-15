# Fierce 5 Novice, after Fierce 5, in kg.
#
# Two workouts alternate over three days a week: A B A in week 1, B A B in
# week 2, then the program loops. The 3x5 lifts add weight every session
# they are completed, 5 kg on the squat and 2.5 kg otherwise; the 3x8 lifts
# add 2.5 kg every other completed session. Two misses in a row take 10%
# off and build back. The isolation work runs a double progression: reps go
# up first, then the weight.
program("Fierce 5 Novice", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

SET_DESC = "Set your own first-session weight by editing the program."

def main(name, reps, weight, inc=kg(2.5), after=1, desc=SET_DESC):
    ex(name, desc=desc, sets=[s(3, reps, weight, rest=180)], progress=lp(inc, after=after, fail_after=2, fail_reset=0.9))

def accessory(name, n, lo, hi, weight, inc=kg(2.5), **kw):
    ex(name, desc=SET_DESC, sets=[s(n, lo, weight, rest=90)], progress=dp(inc, lo, hi), **kw)

def workout_a(week, n):
    day(week, n, "Workout A")
    main("Squat", 5, pct(70), inc=kg(5), desc="Starts at 70% of 1RM.")
    main("Bench Press", 5, pct(70), desc="Starts at 70% of 1RM.")
    main("Pendlay Row", 8, kg(40), after=2)
    accessory("Face Pull", 3, 10, 12, kg(20))
    accessory("Standing Calf Raise", 2, 15, 20, kg(60), inc=kg(5), superset="A")
    accessory("Triceps Pushdown", 2, 10, 12, kg(20), superset="A")

def workout_b(week, n):
    day(week, n, "Workout B")
    main("Front Squat", 5, kg(40))
    main("Overhead Press", 5, pct(70), desc="Starts at 70% of 1RM.")
    main("Romanian Deadlift", 8, kg(60), after=2)
    main("Lat Pulldown", 8, kg(40), after=2)
    ex("Crunch", alts=["Hanging Knee Raise"], superset="B", sets=[s(2, 15, bw(), rest=60)])
    accessory("Bicep Curl, EZ Bar", 2, 10, 12, kg(20), superset="B")

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
