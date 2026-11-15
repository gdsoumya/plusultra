# Starting Strength, Phase 2, after Mark Rippetoe, in kg.
#
# Phase 1 with the power clean added: workout A pulls a deadlift, workout B
# power cleans, so each is trained every other session. A B A in week 1,
# B A B in week 2, then the program loops. Every lift adds weight each
# session it is completed: 5 kg on squat and deadlift, 2.5 kg on press,
# bench and power clean. Three misses in a row take 10% off and build back.
program("Starting Strength, Phase 2", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

START_DESC = "First session: work up from the empty bar in sets of 5 until the bar slows, and set that weight here."

def main(name, start, inc, n=3, reps=5):
    ex(name, desc=START_DESC, sets=[s(n, reps, kg(start), rest=180)], progress=lp(inc, fail_after=3, fail_reset=0.9))

def workout_a(week, n):
    day(week, n, "Workout A")
    main("Squat", 40, kg(5))
    main("Overhead Press", 20, kg(2.5))
    main("Deadlift", 60, kg(5), n=1)

def workout_b(week, n):
    day(week, n, "Workout B")
    main("Squat", 40, kg(5))
    main("Bench Press", 30, kg(2.5))
    main("Power Clean", 30, kg(2.5), n=5, reps=3)

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
