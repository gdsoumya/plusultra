# Starting Strength, Phase 1, after Mark Rippetoe, in kg.
#
# Two workouts alternate over three days a week: A B A in week 1, B A B in
# week 2, then the program loops. Every lift adds weight each session it is
# completed: 5 kg on squat and deadlift, 2.5 kg on press and bench. Three
# misses in a row take 10% off and build back. Starts are modest; the first
# session sets the real work weights.
program("Starting Strength, Phase 1", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

START_DESC = "First session: work up from the empty bar in sets of 5 until the bar slows, and set that weight here."

def main(name, start, inc, n=3):
    ex(name, desc=START_DESC, sets=[s(n, 5, kg(start), rest=180)], progress=lp(inc, fail_after=3, fail_reset=0.9))

def workout_a(week, n):
    day(week, n, "Workout A")
    main("Squat", 40, kg(5))
    main("Overhead Press", 20, kg(2.5))
    main("Deadlift", 60, kg(5), n=1)

def workout_b(week, n):
    day(week, n, "Workout B")
    main("Squat", 40, kg(5))
    main("Bench Press", 30, kg(2.5))
    main("Deadlift", 60, kg(5), n=1)

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
