# Reference syntax: Squat / 3x5 100kg / progress: custom() {~
#                if (dayInWeek == 1) {
#                  weights[2:*:*:*] += 5kg
#                  reps[2:3:*:1] = 3
#                  timers[*:3:*:*] = 240
#                  numberOfSets[2:1] = 4
#                }
#              ~}
program("ctx.at targeting", weeks=2)

def target(ctx):
    if ctx.day_in_week == 1:
        ctx.at(week=2).weights += kg(5)
        ctx.at(week=2, day=3).reps[1] = 3
        ctx.at(day=3).rest = 240
        ctx.at(week=2, day=1).number_of_sets = 4

day(1, 1, "Heavy", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(100))], progress=custom(target))
day(1, 3, "Light", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 5, kg(80))], progress=custom(target))
