# Recommended Routine, after r/bodyweightfitness, in kg.
#
# Three days a week. Three pairs of exercises, each pair done as a superset
# with 90 s rest: pull-ups with squats, dips with a hinge, rows with push-ups,
# all for 3 sets of 5-8. A core triplet follows for 3 sets of 8-12, the plank
# as timed holds. Every slot is a ladder of harder variations: a successful
# session adds a rep to every set, and reaching the top of the range moves to
# the next rung at the bottom; past the last rung it adds 2.5 kg on a belt or
# vest instead.
program("Recommended Routine", weeks=1, units="kg", notes="Three days a week with a day off between sessions")

def ladder(ctx):
    if not (ctx.completed_reps >= ctx.reps):
        return
    if ctx.reps[1] < ctx.state.hi:
        ctx.reps += 1
    elif ctx.exercise_variation < ctx.state.rungs:
        ctx.exercise_variation += 1
        ctx.reps = ctx.state.lo
    else:
        ctx.reps = ctx.state.lo
        ctx.weights += kg(2.5)

def rung(names, group, desc, lo=5, hi=8, rest=90):
    ex(names[0], alts=names[1:], superset=group, desc=desc, sets=[s(3, lo, bw(), rest=rest)],
       progress=custom(ladder, rungs=len(names), lo=lo, hi=hi))

def session(n):
    day(1, n, "Full body")
    rung(["Scapular Pull-Up", "Pull Up, Band", "Pull Up"], "1",
         "Ladder: scapular pulls, band or negative pull-ups, pull-ups, then weighted.")
    rung(["Squat, Bodyweight", "Split Squat", "Bulgarian Split Squat, Bodyweight", "Pistol Squat"], "1",
         "Ladder: squats, split squats, Bulgarian split squats, pistol or shrimp squats. Reps are per leg.")
    rung(["Bench Dip", "Dip", "Ring Dip"], "2",
         "Ladder: bench dips, parallel bar dips (negatives first if needed), ring dips, then weighted.")
    rung(["Single Leg Deadlift", "Nordic Curl, Band", "Nordic Curl"], "2",
         "Ladder: single leg deadlifts, band assisted Nordic curls, Nordic curls. Reps are per leg where it applies.")
    rung(["Inverted Row"], "3",
         "Ladder by angle: upright, incline, horizontal, then wide rows; reset to 3x5 when you lower the bar.")
    rung(["Incline Push Up", "Push Up", "Push Up Close Grip"], "3",
         "Ladder: incline push-ups, push-ups, diamond push-ups, then pseudo planche push-ups.")
    ex("Plank", superset="core", desc="Hold 30 s each. When all three are easy, swap in Ab Wheel 3x8-12.",
       sets=[s(3, 1, bw(), work=30, rest=60)])
    ex("Pallof Press", eq="Cable", superset="core", desc="Band or cable; set your own first-session weight by editing the program.",
       warmup=[], sets=[s(3, 8, kg(10), rest=60)], progress=dp(kg(2.5), 8, 12))
    rung(["Superman", "Back Extension"], "core",
         "Ladder: supermans, back extensions on a bench, then holding a plate.", lo=8, hi=12, rest=60)

session(1)
session(3)
session(5)
