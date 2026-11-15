# Beginner full body, in kg.
#
# Two workouts alternate over three days a week: A B A in week 1, B A B in
# week 2, then the program loops. Main lifts add weight after every session
# where all reps are done; three misses in a row take 10% off and build back.
# Chin-ups add reps up to 8 before adding weight on a belt.
program("Beginner full body", weeks=2, units="kg", notes="Alternate workouts A and B, three days a week")

def linear(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.weights += ctx.state.inc
        ctx.state.misses = 0
    else:
        ctx.state.misses += 1
        if ctx.state.misses == 3:
            ctx.weights *= 0.9
            ctx.state.misses = 0

def main(name, start, inc, desc, n=3):
    """n sets of 5 at start kg, the last one as many reps as possible."""
    sets = [s(1, 5, kg(start), rest=180, amrap=True)]
    if n > 1:
        sets.insert(0, s(n - 1, 5, kg(start), rest=180))
    ex(name, desc=desc, sets=sets, progress=custom(linear, inc=inc, misses=0))

def workout_a(week, n):
    day(week, n, "Workout A", desc="Squat, bench, row.")
    main("Squat", 60, kg(5), "Sit between the heels, chest up.")
    main("Bench Press", 40, kg(2.5), "Shoulder blades pinned, feet planted.")
    main("Bent Over Row", 40, kg(2.5), "Torso near parallel, pull to the belly.")
    ex("Plank", warmup=[], sets=[s(3, 1, bw(), work=30, count_up=True, rest=60)])

def workout_b(week, n):
    day(week, n, "Workout B", desc="Deadlift, press, chin-ups.")
    main("Deadlift", 80, kg(5), "One hard set after the warmups.", n=1)
    main("Overhead Press", 30, kg(2.5), "Squeeze the glutes, press straight up.")
    ex("Chin Up", alts=["Lat Pulldown"], sets=[s(3, 5, bw(), rest=120)], progress=dp(kg(2.5), 5, 8))

workout_a(1, 1)
workout_b(1, 2)
workout_a(1, 3)
workout_b(2, 1)
workout_a(2, 2)
workout_b(2, 3)
