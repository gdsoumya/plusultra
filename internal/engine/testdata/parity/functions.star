# Reference built-ins: calculate1RM, calculateTrainingMax, rpeMultiplier,
# roundWeight, increment, decrement, isExercise, isProgramExercise, print,
# floor, ceil, round, sum, min, max, zeroOrGte.
program("Built-in functions", weeks=2)

def show(ctx):
    print("calc_1rm", calc_1rm(kg(100), 5), "training_max", training_max(kg(100), 5))
    print("rpe_multiplier", rpe_multiplier(5, 8))
    print("round_weight", round_weight(kg(101.4)), "increment", increment(kg(100)), "decrement", decrement(kg(100)))
    print("is_exercise", is_exercise("squat"), is_exercise("Bench Press"))
    print("is_program_exercise", is_program_exercise("heavy: Squat"), is_program_exercise("Squat"))
    print("floor", floor(7 / 2), "ceil", ceil(7 / 2), "round", round(7 / 2), round(-7 / 2), "abs", abs(-3))
    print("floor", floor(kg(101.6)), "ceil", ceil(kg(101.2)), "round", round(kg(101.5)))
    print("min", min(ctx.completed_reps), "max", max(ctx.completed_weights))
    print("sum", sum(ctx.completed_reps), sum(ctx.completed_weights))
    print("zero_or_gte", zero_or_gte(ctx.completed_reps, ctx.reps), zero_or_gte([0, 4, 5], ctx.reps))

day(1, 1, "Lower", weeks="1-2")
ex("Squat", label="heavy", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(show))
