# Reference syntax: Pull Up / 3x8 0kg / update: custom() {~ if (setIndex == 0) { weights = bodyweight } ~}
#              Dip, Bodyweight / 3x8 20kg / update: ... weights = bodyweight + originalWeights[ns]
#              Pull Up, Leverage Machine / 3x8 25kg / update: ... weights = bodyweight - originalWeights[ns]
#                / progress: lp(-2.5kg)
# bw() loads say it directly; the update form works too.
program("Bodyweight and assisted", weeks=2)

def add_bodyweight(ctx):
    if ctx.set_index == 0:
        ctx.weights = ctx.bodyweight + ctx.original_weights[ctx.number_of_sets]

day(1, 1, "Upper", weeks="1-2")
ex("Pull Up", sets=[s(3, 8, bw())])
ex("Dip", sets=[s(3, 8, bw(20))], progress=lp(kg(2.5)))
# Less assistance is more load, so the assisted lift progresses upward.
ex("Pull Up, Leverage Machine", sets=[s(3, 8, bw(-25))], progress=lp(kg(2.5)))
# Bodyweight equipment does not round, like a bodyweight-for-bar setting.
ex("Chin Up", eq="bodyweight", warmup=[], sets=[s(3, 8, kg(10))], update=custom(add_bodyweight))
