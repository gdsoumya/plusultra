# Reference syntax: Squat, Bodyweight | Pistol Squat | Front Squat / 3x8 0kg / progress: custom() {~
#                if (completedReps >= reps) { exerciseVariationIndex += 1 }
#              ~}
program("Exercise variations", weeks=2)

def ladder(ctx):
    if ctx.completed_reps >= ctx.reps:
        ctx.exercise_variation += 1

day(1, 1, "Lower", weeks="1-2")
ex(alts=["Squat, Bodyweight", "Pistol Squat", "Front Squat"], sets=[s(3, 8, bw())], progress=custom(ladder))
