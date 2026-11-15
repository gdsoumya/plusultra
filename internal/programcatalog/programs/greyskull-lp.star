# Phrak's Greyskull LP, after John Sheaffer's Greyskull LP, in kg.
#
# Two workouts alternate over three days a week: A B A in week 1, B A B in
# week 2, then the program loops. Press and bench, chin-ups and rows, squat
# and deadlift alternate between the workouts. Every lift is 2x5 then 1x5+,
# the last set for as many reps as possible; the deadlift is one 5+ set.
#
# A completed session adds 2.5 kg to upper-body lifts and 5 kg to squat
# and deadlift, double that when the last set reaches 10 reps. A miss takes
# the lift back to 90%. Chin-ups add weight on a belt and never reset.
program("Phrak's Greyskull LP", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

def gslp(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.weights += ctx.state.inc * (2 if ctx.completed_reps[ctx.number_of_sets] >= 10 else 1)
    elif ctx.state.reset:
        ctx.weights *= 0.9

def main(name, weight, inc, n=3, reset=True, desc=None):
    sets = [s(1, 5, weight, rest=180, amrap=True)]
    if n > 1:
        sets.insert(0, s(n - 1, 5, weight, rest=180))
    ex(name, desc=desc, sets=sets, progress=custom(gslp, inc=inc, reset=reset))

def workout_a(week, n):
    day(week, n, "Workout A")
    main("Overhead Press", pct(70), kg(2.5), desc="Start light: about 10 reps on the last set.")
    main("Chin Up", bw(), kg(2.5), reset=False, desc="Fewer than 5 chin-ups: do negatives or lat pulldowns for now.")
    main("Squat", pct(70), kg(5), desc="Start light: about 10 reps on the last set.")

def workout_b(week, n):
    day(week, n, "Workout B")
    main("Bench Press", pct(70), kg(2.5), desc="Start light: about 10 reps on the last set.")
    main("Bent Over Row", kg(40), kg(2.5), desc="Set your own first-session weight by editing the program.")
    main("Deadlift", pct(70), kg(5), n=1, desc="Start light: about 10 reps on the set.")

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
