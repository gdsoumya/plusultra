# Jacked & Tan 2.0, after Cody Lefever, in kg.
#
# Four days a week for twelve weeks in two six-week blocks, each ending in a
# 1RM. The T1 lift of each day works up to the week's rep max (10, 8, 6, 4, 2
# and a 1RM in block 1; 6, 4, 2, 5, 3 and a 1RM in block 2), then does drop
# sets whose last set is a rep-out: in block 1 at a percentage of a training
# max (90% of 1RM), in block 2 at 85% then 90% of the day's rep max, which the
# update hook applies once the top set is logged. The 1RM weeks reset the max
# for the next block.
#
# T2a is a variation of a main lift on a linear percentage wave of its own
# training max, set in T2A_TM. T2b (back work) and the T3 accessories find the
# week's rep max and follow it with three max rep sets (MRS) at that weight;
# the next week's weight comes from the rep max set.
program("Jacked & Tan 2.0", weeks=12, units="kg", loop=False,
        notes="Four days a week in two six-week blocks, each ending in a 1RM. Leave 1-2 reps in the tank on every rep max and MRS")

# A rep max as a percent of 1RM (Epley): the target to work up to.
RM_PCT = {10: 75, 8: 79, 6: 83, 5: 86, 4: 88, 3: 91, 2: 94, 1: 97}

# T1 by week: (rep max, drop set percent, drop set reps, drop sets). Block 1
# percents are of the training max, written as a percent of 1RM; block 2
# percents are of the day's rep max.
T1 = {
    1: (10, 63, 6, 3), 2: (8, 67.5, 5, 3), 3: (6, 72, 4, 3), 4: (4, 74, 3, 3), 5: (2, 76.5, 2, 4), 6: (1, 0, 0, 0),
    7: (6, 85, 3, 5), 8: (4, 85, 2, 5), 9: (2, 85, 1, 5), 10: (5, 90, 2, 3), 11: (3, 90, 1, 3), 12: (1, 0, 0, 0),
}
T1_DESC = {
    1: "T1. Work up to the heaviest clean set of the week's rep max, 1-2 reps left, and log it on the RM set. Then the drop sets; the last one is a rep-out.",
    6: "T1. Work up to a single you could confidently double, within 5% of your true max. No drop sets. It sets the max for block 2.",
    7: "T1. Drop sets are 85% of today's rep max for half the reps; they follow the weight you log on the RM set.",
    10: "T1. Drop sets are 90% of today's rep max.",
    12: "T1. Work up to a new 1RM, then rest.",
}

# T2a by week: (percent of its training max, reps, sets). Weeks 6 and 12 rest.
T2A = {1: (50, 10, 4), 2: (60, 8, 4), 3: (70, 6, 4), 4: (75, 4, 5), 5: (80, 2, 7),
       7: (70, 6, 5), 8: (75, 5, 5), 9: (80, 4, 5), 10: (82.5, 3, 6), 11: (85, 2, 7)}
# Training max of each T2a lift in kg: set these before the first session.
T2A_TM = {"Romanian Deadlift": 120, "Incline Bench Press": 70, "Front Squat": 90, "Bench Press Close Grip": 80}

# Rep max to find each week before the three MRS.
T2B_RM = {1: 15, 2: 12, 3: 10, 4: 8, 5: 6, 7: 15, 8: 12, 9: 10, 10: 6}
T3_RM = {1: 20, 2: 18, 3: 16, 4: 14, 5: 12, 6: 10, 8: 18, 9: 16, 10: 14, 11: 12}

def t1_progress(ctx):
    if ctx.week in (6, 12):
        ctx.rm1 = calc_1rm(ctx.completed_weights[1], ctx.completed_reps[1])

def rebase(ctx):
    # Block 2 drop sets follow the rep max logged on the top set.
    if ctx.week > 6 and ctx.set_index == 1 and ctx.number_of_sets > 1:
        ctx.weights = ctx.completed_weights[1] * (0.9 if ctx.week > 9 else 0.85)

def t1(name):
    for wk, (rm, p, reps, n) in T1.items():
        sets = [s(1, rm, pct(RM_PCT[rm]), rest=300, amrap=True, label="%dRM" % rm)]
        if n:
            drop = pct(p) if wk <= 6 else pct(RM_PCT[rm] * p / 100)
            sets += [s(n - 1, reps, drop, rest=180, label="Drop"), s(1, reps, drop, rest=180, amrap=True, label="Drop")]
        ex(name, label="t1", weeks=[wk], desc=T1_DESC.get(wk), sets=sets,
           progress=custom(t1_progress) if wk == 1 else None, update=custom(rebase) if wk == 1 else None)

def t2a_progress(ctx):
    # The last T2a set must always be completed; a miss takes 5% off this lift.
    if not (ctx.completed_reps >= ctx.reps):
        ctx.weights *= 0.95

def t2a(name):
    for wk, (p, reps, n) in T2A.items():
        ex(name, label="t2a", weeks=[wk], sets=[s(n, reps, kg(T2A_TM[name] * p / 100), rest=150)],
           desc="T2a. Percentages of this lift's training max; set it in T2A_TM at the top of the script." if wk == 1 else None,
           progress=custom(t2a_progress) if wk == 1 else None)

def mrs_progress(ctx):
    # Next week's weight is this week's estimated max scaled to the next rep max, when the equipment can load the difference.
    for wk, rm in ctx.state.table:
        if wk > ctx.week:
            w = round_weight(calc_1rm(ctx.completed_weights[1], ctx.completed_reps[1]) / (1 + rm / 30.0))
            if w != ctx.weights[1]:
                ctx.weights = w
            break

def same_weight(ctx):
    # The MRS use the weight the rep max set was done with.
    if ctx.set_index == 1:
        ctx.weights = ctx.completed_weights[1]

def mrs(name, label, table, weight, **kw):
    desc = "%s. Work up to the week's rep max, log it, then three max rep sets at that weight with 1-2 reps left. Set your first weight by editing the program." % label.upper()
    for i, (wk, rm) in enumerate(table.items()):
        ex(name, label=label, weeks=[wk], desc=desc if i == 0 else None,
           sets=[s(1, rm, kg(weight), rest=60, amrap=True, label="%dRM" % rm), s(3, (rm // 2, rm), kg(weight), rest=60, label="MRS")],
           progress=custom(mrs_progress, table=list(table.items())) if i == 0 else None,
           update=custom(same_weight) if i == 0 else None, **kw)

for wk, (rm, p, reps, n) in T1.items():
    week(wk, desc="Block %d: %s" % (1 if wk <= 6 else 2, "1RM week" if rm == 1 else "%dRM week" % rm))

day(1, 1, "Squat", weeks="1-12")
t1("Squat")
t2a("Romanian Deadlift")
mrs("Lat Pulldown", "t2b", T2B_RM, 50)
mrs("Lying Leg Curl", "t3", T3_RM, 35)
mrs("Leg Extension", "t3", T3_RM, 35)
mrs("Hammer Curl", "t3", T3_RM, 12.5, eq="Dumbbell", warmup=[])

day(1, 2, "Bench", weeks="1-12")
t1("Bench Press")
t2a("Incline Bench Press")
mrs("Seated Row", "t2b", T2B_RM, 50)
mrs("Triceps Pushdown", "t3", T3_RM, 25)
mrs("Lateral Raise", "t3", T3_RM, 10, eq="Dumbbell", warmup=[])
mrs("Chest Fly", "t3", T3_RM, 15, eq="Dumbbell", warmup=[])

day(1, 4, "Deadlift", weeks="1-12")
t1("Deadlift")
t2a("Front Squat")
mrs("Incline Row", "t2b", T2B_RM, 20, eq="Dumbbell")
mrs("Back Extension, Leverage Machine", "t3", T3_RM, 20)
mrs("Crunch, Cable", "t3", T3_RM, 30)
mrs("Bicep Curl, EZ Bar", "t3", T3_RM, 20)

day(1, 5, "Press", weeks="1-12")
t1("Overhead Press")
t2a("Bench Press Close Grip")
mrs("T Bar Row", "t2b", T2B_RM, 40)
mrs("Skullcrusher", "t3", T3_RM, 20)
mrs("Face Pull", "t3", T3_RM, 20)
mrs("Reverse Fly", "t3", T3_RM, 7.5, eq="Dumbbell", warmup=[])
