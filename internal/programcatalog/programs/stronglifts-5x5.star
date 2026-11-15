# StrongLifts 5x5, after Mehdi Hadim, in kg.
#
# Two workouts alternate over three days a week: A B A in week 1, B A B in
# week 2, then the program loops. Every lift adds 2.5 kg each session it is
# completed, the deadlift 5 kg. Three misses in a row at a weight take 10%
# off and build back. Starts are the method's: the empty bar for squat,
# bench and press, 30 kg for rows and 40 kg for the deadlift.
program("StrongLifts 5x5", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

START_DESC = "New to lifting: start here. Otherwise start at about half your 5RM by editing this weight."

def main(name, start, inc=kg(2.5), n=5):
    ex(name, desc=START_DESC, sets=[s(n, 5, kg(start), rest=180)], progress=lp(inc, fail_after=3, fail_reset=0.9))

def workout_a(week, n):
    day(week, n, "Workout A")
    main("Squat", 20)
    main("Bench Press", 20)
    main("Bent Over Row", 30)

def workout_b(week, n):
    day(week, n, "Workout B")
    main("Squat", 20)
    main("Overhead Press", 20)
    main("Deadlift", 40, inc=kg(5), n=1)

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
