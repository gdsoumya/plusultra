# Bullmastiff, base phase, after Alex Bromley, in kg.
#
# The first nine weeks of the eighteen-week program: three 3-week waves that
# build volume, four days a week with one main lift each. Main lifts start a
# wave at 65%, 70% then 75% of 1RM for 4x6, 5x5 then 6x4 with the last set a
# rep-out; every rep past the target adds 1% of the 1RM to the rest of the
# wave, and the next wave starts afresh from its own percentage.
#
# The developmental lift of each day is a variation of the other lift of
# its half (deadlift on squat day and so on) at a fixed weight for RPE 6-8,
# adding a set each week: 3, 4, 5 sets of 12, 10 then 8. Accessories add a
# set each week too. The peak phase that follows is not part of this program.
program("Bullmastiff, base phase", weeks=9, units="kg", loop=False,
        notes="Base phase only, the first nine weeks of the program; the nine-week peak phase follows")

# Per wave: main lifts (sets, reps, percent of 1RM); developmental lifts (reps, RPE per week);
# accessories (reps, sets per week).
MAIN = [(4, 6, 65), (5, 5, 70), (6, 4, 75)]
DEV = [(12, [6, 7, 8]), (10, [6, 7, 8]), (8, [6, 6, 6])]
ACC = [((12, 15), [2, 3, 4]), ((8, 12), [3, 4, 5]), ((8, 12), [3, 4, 5])]

def wave(w):
    return (w - 1) // 3

def carry(ctx):
    # Every rep past the target on the rep-out adds 1% of the 1RM to the later weeks of this wave.
    extra = ctx.completed_reps[ctx.number_of_sets] - ctx.reps[ctx.number_of_sets]
    if extra > 0:
        for w in range(ctx.week + 1, 3 * wave(ctx.week) + 4):
            ctx.at(week=w).weights += ctx.rm1 * extra / 100

def main(name):
    for w in range(1, 10):
        n, reps, p = MAIN[wave(w)]
        ex(name, weeks=[w], sets=[s(n - 1, reps, pct(p), rest=180), s(1, reps, pct(p), rest=180, amrap=True)],
           progress=custom(carry) if w == 1 else None,
           desc="The last set is a rep-out: every rep past the target adds 1% of your max to next week." if w == 1 else None)

def developmental(name, start, **kw):
    for w in range(1, 10):
        reps, rpes = DEV[wave(w)]
        k = (w - 1) % 3
        ex(name, weeks=[w], sets=[s(3 + k, reps, kg(start * (1 + 0.075 * wave(w))), rest=120, rpe=rpes[k])],
           desc="Developmental lift. Never grind: the weight suits RPE 6-8, sets go up each week. Set your own first weight by editing the program." if w == 1 else None, **kw)

def accessory(name, weight, **kw):
    for w in range(1, 10):
        reps, sets = ACC[wave(w)]
        ex(name, weeks=[w], sets=[s(sets[(w - 1) % 3], reps, kg(weight), rest=90)],
           desc="Bodybuilding style, volume over load; a set more each week. Set your own first weight by editing the program." if w == 1 else None, **kw)

for w in range(1, 10):
    week(w, desc="Wave %d, week %d" % (wave(w) + 1, (w - 1) % 3 + 1))

day(1, 1, "Squat", weeks="1-9")
main("Squat")
developmental("Romanian Deadlift", 90)
accessory("Leg Extension", 40)
accessory("Hanging Leg Raise", 0)

day(1, 2, "Bench", weeks="1-9")
main("Bench Press")
developmental("Dumbbell Shoulder Press", 17.5, eq="Dumbbell")
accessory("Lat Pulldown", 50)
accessory("Triceps Pushdown", 25)

day(1, 4, "Deadlift", weeks="1-9")
main("Deadlift")
developmental("Front Squat", 60)
accessory("Seated Row", 50)
accessory("Lying Leg Curl", 40)

day(1, 5, "Press", weeks="1-9")
main("Overhead Press")
developmental("Bench Press Close Grip", 60)
accessory("Lateral Raise", 10, eq="Dumbbell", warmup=[])
accessory("Face Pull", 20)
