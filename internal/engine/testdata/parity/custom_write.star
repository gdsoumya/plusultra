# Reference progress writes: weights, reps, minReps, RPE, timers, setTime,
# amraps, askweights, logrpes, rm1, numberOfSets, setVariationIndex,
# exerciseVariationIndex, descriptionIndex, state. The update hook writes
# setTime, amraps, askweights and logrpes for the workout under way.
program("Custom progress writes", weeks=2)

def sets_and_load(ctx):
    ctx.weights += kg(5)
    ctx.reps[1] = 6
    ctx.min_reps = 4
    ctx.rpe = 8.5
    ctx.rest += 30
    ctx.number_of_sets = 4

def one_rm(ctx):
    ctx.rm1 += kg(5)

def indexes(ctx):
    ctx.set_variation = 2
    ctx.exercise_variation = 2
    ctx.description_index = 2
    ctx.state.count += 1

def timers_and_flags(ctx):
    ctx.work += 15
    ctx.amrap[ctx.number_of_sets] = True
    ctx.ask = True
    ctx.log_rpe = True

def live(ctx):
    if ctx.set_index == 0:
        ctx.work = 40
        ctx.amrap[ctx.number_of_sets] = True
        ctx.ask = True
        ctx.log_rpe = True

day(1, 1, "Full body", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100), rpe=7, rest=120)], progress=custom(sets_and_load), update=custom(live))
ex("Bench Press", warmup=[], sets=[s(3, 5, pct(70))], progress=custom(one_rm))
ex("Overhead Press", warmup=[], sets=[s(3, 5, kg(40), work=30)], progress=custom(timers_and_flags))
ex("Deadlift", alts=["Deadlift, Trap Bar"], desc=["Hook grip", "Mixed grip"], warmup=[],
   variations=[[s(1, 5, kg(140))], [s(3, 3, kg(150))]], progress=custom(indexes, count=0))
