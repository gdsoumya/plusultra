# Reference syntax (5/3/1 style):
#   main / used: none / 1x5 65%, 1x5 75%, 1x5+ 85% / progress: custom(increment: 5kg) {~
#     if (completedReps >= reps && dayInWeek == 1) { rm1 += state.increment }
#   ~}
#   # Week 2: main / 1x3 70%, 1x3 80%, 1x3+ 90%
program("Percentage-based program", weeks=2)

def cycle(ctx):
    if ctx.completed_reps >= ctx.reps and ctx.day_in_week == 1:
        ctx.rm1 += ctx.state.increment

waves = {1: [(5, 65), (5, 75), (5, 85)], 2: [(3, 70), (3, 80), (3, 90)]}
for week, wave in waves.items():
    day(week, 1, "Squat day")
    sets = [s(1, reps, pct(p), amrap=i == 2) for i, (reps, p) in enumerate(wave)]
    ex("Squat", warmup=[], sets=sets, progress=custom(cycle, increment=kg(5)))
