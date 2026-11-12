program("P", weeks=1)

def bump(ctx):
    pass

day(1, 1, "Lower")
ex("Squat", progress=lp(kg(2.5)), sets=[s(3, 5, kg(100))])
ex("Deadlift", progress=lp(kg(5), after=2, fail_reset=0.9), sets=[s(1, 5, kg(140))])
ex("Bench Press", progress=dp(kg(2.5), 6, 8), sets=[s(3, (6, 8), kg(60))])
ex("Pull Up", progress=sum_reps(kg(5), 25), sets=[s(3, 8, bw())])
ex("Row", progress=custom(bump, attempt=0, top=prompt(0)), update=custom(bump), sets=[s(3, 8, kg(60))])
