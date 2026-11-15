# Juggernaut Method 2.0, after Chad Wesley Smith, in kg.
#
# Four days a week, one main lift each, for sixteen weeks: four waves of
# 10s, 8s, 5s and 3s. Every wave runs an accumulation week (straight sets,
# the last a rep-out), an intensification week (two ramp sets, then fewer
# work sets heavier, the last a rep-out), a realization week (a ramp to one
# all-out set) and a deload. Loads are percentages of a training max, 90% of
# the 1RM, written here as percentages of the 1RM.
#
# After the realization set every rep past the wave's target adds 2.5 kg
# (squat, deadlift) or 1.25 kg (bench, press) to the training max of the
# next wave. Accessories run double progression and rest on deload weeks.
program("Juggernaut Method 2.0", weeks=16, units="kg", loop=False,
        notes="Four days a week; four 4-week waves of 10s, 8s, 5s and 3s")

TM = 0.9
PHASES = ["Accumulation", "Intensification", "Realization", "Deload"]
# wave: (reps, accumulation sets, intensification work sets, percent of TM in accumulation, intensification, realization)
WAVES = [(10, 5, 3, 60, 67.5, 75), (8, 5, 3, 65, 72.5, 80), (5, 6, 4, 70, 77.5, 85), (3, 7, 5, 75, 82.5, 90)]
# Ramp sets as (reps, percent of TM) before the work sets, by the wave's reps.
INTENS_RAMP = {10: [(5, 55), (5, 62.5)], 8: [(3, 60), (3, 67.5)], 5: [(2, 65), (2, 72.5)], 3: [(1, 70), (1, 77.5)]}
REAL_RAMP = {10: [(5, 50), (3, 60), (1, 70)], 8: [(5, 50), (3, 60), (2, 70), (1, 75)],
             5: [(5, 50), (3, 60), (2, 70), (1, 75), (1, 80)], 3: [(5, 50), (3, 60), (2, 70), (1, 75), (1, 80), (1, 85)]}
LIFTING_WEEKS = [w for w in range(1, 17) if w % 4 != 0]

def tm(p):
    return pct(p * TM)

def ramp(table):
    return [s(1, reps, tm(p), rest=120) for reps, p in table]

def realize(ctx):
    # pct() is of the 1RM and the training max is 90% of it, so a TM step is an rm1 step of inc / TM.
    if ctx.week % 4 == 3:
        extra = ctx.completed_reps[ctx.number_of_sets] - ctx.reps[ctx.number_of_sets]
        if extra > 0:
            ctx.rm1 += ctx.state.inc * extra / TM

def main(name, inc):
    for w in range(1, 17):
        reps, n_acc, n_int, p_acc, p_int, p_real = WAVES[(w - 1) // 4]
        phase = (w - 1) % 4
        if phase == 0:
            sets = [s(n_acc - 1, reps, tm(p_acc), rest=180), s(1, reps, tm(p_acc), rest=180, amrap=True)]
        elif phase == 1:
            sets = ramp(INTENS_RAMP[reps]) + [s(n_int - 1, reps, tm(p_int), rest=180), s(1, reps, tm(p_int), rest=180, amrap=True)]
        elif phase == 2:
            sets = ramp(REAL_RAMP[reps]) + [s(1, reps, tm(p_real), rest=300, amrap=True, label="Realization")]
        else:
            sets = [s(1, 5, tm(p), rest=120) for p in (40, 50, 60)]
        ex(name, weeks=[w], sets=sets, progress=custom(realize, inc=inc) if w == 1 else None,
           desc="Percentages of a training max, 90% of your 1RM. The rep-out sets raise it: every rep past the wave's target on the realization set adds to the next wave." if w == 1 else None)

def accessory(name, sets, reps, weight, inc, **kw):
    ex(name, weeks=LIFTING_WEEKS, sets=[s(sets, reps, weight, rest=90)], progress=dp(inc, reps, reps + 4) if inc else None,
       desc="Set your own first-session weight by editing the program." if inc else None, **kw)

for w in range(1, 17):
    week(w, desc="%ds wave: %s" % (WAVES[(w - 1) // 4][0], PHASES[(w - 1) % 4]))

day(1, 1, "Squat", weeks="1-16")
main("Squat", kg(2.5))
accessory("Romanian Deadlift", 3, 8, kg(80), kg(5))
accessory("Hanging Leg Raise", 3, 12, kg(0), None)

day(1, 2, "Bench", weeks="1-16")
main("Bench Press", kg(1.25))
accessory("Bent Over Row, Dumbbell", 4, 10, kg(25), kg(2.5))
accessory("Incline Bench Press, Dumbbell", 3, 10, kg(22.5), kg(2.5))
accessory("Triceps Pushdown", 3, 12, kg(25), kg(2.5))

day(1, 4, "Deadlift", weeks="1-16")
main("Deadlift", kg(2.5))
accessory("Bent Over Row", 4, 8, kg(60), kg(2.5))
accessory("Lying Leg Curl", 3, 10, kg(40), kg(2.5))
accessory("Ab Wheel", 3, 10, kg(0), None)

day(1, 5, "Press", weeks="1-16")
main("Overhead Press", kg(1.25))
accessory("Chin Up", 4, 8, bw(), None)
accessory("Lateral Raise", 3, 15, kg(10), kg(2.5), eq="Dumbbell", warmup=[])
accessory("Face Pull", 3, 15, kg(20), kg(2.5))
