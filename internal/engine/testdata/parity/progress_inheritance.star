# Reference syntax:
#   # Week 1
#   ## Day 1
#   Squat / 3x5 100kg / progress: lp(2.5kg) / update: custom() {~
#     if (setIndex == 1) { weights = completedWeights[1] + 2.5kg }
#   ~}
#   Bench Press / 3x5 80kg / progress: lp(2.5kg)
#   # Week 2
#   ## Day 1
#   Squat / 3x3 110kg                         (lp and the update hook still apply)
#   Bench Press / 3x5 70kg / progress: none   (a deload week: no progression)
# This case progresses and updates week 2.
program("Progression inheritance", weeks=2)

def top_up(ctx):
    if ctx.set_index == 1:
        ctx.weights = ctx.completed_weights[1] + kg(2.5)

day(1, 1, "Full body")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=lp(kg(2.5)), update=custom(top_up))
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(80))], progress=lp(kg(2.5)))

day(2, 1, "Full body")
ex("Squat", warmup=[], sets=[s(3, 3, kg(110))])
ex("Bench Press", warmup=[], sets=[s(3, 5, kg(70))], progress="none")
