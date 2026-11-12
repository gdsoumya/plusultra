# Reference syntax: Squat / 3x8 100kg / progress: custom() {~
#                if (completedReps >= reps) {
#                  if (numberOfSets < 5) { numberOfSets += 1 } else { weights += 5kg; numberOfSets = 3 }
#                }
#              ~}
#              Bench Press / 3x8 70kg / progress: custom() {~ numberOfSets[2:1] += 1 ~}
#              Overhead Press / 3x8 40kg / 5x5 45kg / progress: custom() {~
#                numberOfSets[2:1:2] += 1
#                setVariationIndex = 2
#              ~}
program("Number-of-sets progression", weeks=2)

def more_sets(ctx):
    if ctx.completed_reps >= ctx.reps:
        if ctx.number_of_sets < 5:
            ctx.add_sets(1)
        else:
            ctx.weights += kg(5)
            ctx.number_of_sets = 3

def week2_set(ctx):
    ctx.at(week=2, day=1).add_sets(1)

def variation_set(ctx):
    ctx.at(week=2, day=1, variation=2).add_sets(1)
    ctx.set_variation = 2

day(1, 1, "Full body", weeks="1-2")
ex("Squat", warmup=[], sets=[s(3, 8, kg(100))], progress=custom(more_sets))
ex("Deadlift", warmup=[], sets=[s(5, 5, kg(140))], progress=custom(more_sets))
ex("Bench Press", warmup=[], sets=[s(3, 8, kg(70))], progress=custom(week2_set))
ex("Overhead Press", warmup=[], variations=[[s(3, 8, kg(40))], [s(5, 5, kg(45))]], progress=custom(variation_set))
