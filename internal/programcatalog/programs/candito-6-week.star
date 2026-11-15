# Candito 6-Week Strength, after Jonnie Candito, in kg.
#
# Lower and upper days for squat, bench press and deadlift: five days in weeks
# 1 and 2, four in weeks 3 and 4, three in weeks 5 and 6. Week 1 is muscular
# conditioning, week 2 hypertrophy with max-rep squats and extra volume, week 3
# linear max OT, week 4 heavy weight acclimation, week 5 a heavy set of 1-4 on
# each lift, week 6 a deload or a 1RM test.
#
# Loads are percentages of 1RM from the published sheet; its 2.5 kg steps are
# written as 2%. A missed required rep lowers that lift's max 2.5%. The week 5
# set projects the max for week 6: its weight times 1.03, 1.06 or 1.09 for 2,
# 3 or 4 reps. Upper days add a row, a shoulder press and a vertical pull with
# the sheet's rep schemes, plus two optional isolation lifts.
program("Candito 6-Week Strength", weeks=6, units="kg", loop=False,
        notes="Five, four then three days a week. Skip week 6 to start a new cycle from the projected max, or test")

PROJECTED = {1: 1.0, 2: 1.03, 3: 1.06, 4: 1.09}
# Upper-day accessory reps per set by week: row, shoulder press, vertical pull.
ACC = {
    1: ([10, 10, 8, 6], [12, 12, 10, 8], [12, 12, 10, 8]),
    2: ([10, 8, 8], [10, 8, 6], [10, 8, 6]),
    3: ([6, 6, 6], [6, 6, 6], [6, 6, 6]),
    4: ([10, 10, 8, 6], [12, 12, 10, 8], [12, 12, 10, 8]),
    5: ([8, 6, 6], [6, 4, 4], [8, 6, 6]),
}
OPTIONAL_SETS = {1: 4, 2: 4, 4: 4, 5: 3}

def miss(ctx):
    if not (ctx.completed_reps >= ctx.min_reps):
        ctx.rm1 *= 0.975

def project(ctx):
    r = ctx.completed_reps[1]
    if r > 0:
        ctx.rm1 = ctx.completed_weights[1] * PROJECTED[min(r, 4)]

def backoff_count(ctx):
    # Week 2 day 4: 10 reps on the MR10 set earn 10 back-off sets, 8-9 earn 8, 7 earn 5, fewer none.
    if ctx.week == 2 and ctx.day_in_week == 4 and ctx.set_index == 1:
        r = ctx.completed_reps[1]
        ctx.number_of_sets = 1 + (10 if r >= 10 else 8 if r >= 8 else 5 if r == 7 else 0)

def work(*groups):
    """Single sets of (reps, percent of 1RM), 3 minutes apart."""
    return [s(1, reps, pct(p), rest=180) for reps, p in groups]

def lift(name, sets, **kw):
    ex(name, sets=sets, progress=custom(miss) if name not in SEEN else None, **kw)
    SEEN.append(name)

SEEN = []

def max_set(name):
    ex(name, label="max", desc="Work up to one heavy set of 1 to 4 reps at your max; it projects next week's max.",
       sets=[s(1, (1, 4), pct(100), rest=300)], progress=custom(project))

def variation():
    ex("Stiff Leg Deadlift", alts=["Deficit Deadlift", "Sumo Deadlift"], sets=[s(3, 8, kg(100), rest=150)],
       desc="Deadlift variation: stiff-legged, snatch grip, deficit or paused. Set your own weight by editing the program.")

def upper(w, n, name, bench, optional=True):
    day(w, n, name)
    lift("Bench Press", bench)
    rows, shoulders, pulls = ACC[w]
    ex("Bent Over Row", alts=["One Arm Row", "Seated Row"], sets=[s(1, r, kg(50), rest=90) for r in rows],
       desc="Horizontal pull. Set your own weight by editing the program." if w == 1 else None)
    ex("Dumbbell Shoulder Press", alts=["Overhead Press", "Lateral Raise"], eq="Dumbbell",
       sets=[s(1, r, kg(17.5), rest=90) for r in shoulders], desc="Shoulders. Set your own weight by editing the program." if w == 1 else None)
    ex("Pull Up", alts=["Chin Up", "Lat Pulldown"], sets=[s(1, r, bw(), rest=90) for r in pulls],
       desc="Vertical pull; add weight when bodyweight is easy." if w == 1 else None)
    if optional and w in OPTIONAL_SETS:
        n_opt = OPTIONAL_SETS[w]
        ex("Triceps Pushdown", sets=[s(n_opt, (8, 12), kg(25), rest=60)],
           desc="Optional isolation work, light, for your weak points." if w == 1 else None)
        ex("Bicep Curl, EZ Bar", sets=[s(n_opt, (8, 12), kg(20), rest=60)],
           desc="Optional isolation work, light, for your weak points." if w == 1 else None)

week(1, desc="Muscular conditioning")
week(2, desc="Hypertrophy and conditioning, higher difficulty")
week(3, desc="Linear max OT")
week(4, desc="Heavy weight acclimation")
week(5, desc="Intense strength")
week(6, desc="Deload or test")

day(1, 1, "Lower heavy")
lift("Squat", [s(4, 6, pct(80), rest=180)], desc="A missed required rep on any set lowers the max 2.5%.")
lift("Deadlift", [s(2, 6, pct(80), rest=180)], desc="A missed required rep on any set lowers the max 2.5%.")
upper(1, 2, "Upper", work((10, 50), (10, 67.5), (8, 75), (6, 77.5)))
upper(1, 4, "Upper", work((10, 50), (10, 67.5), (8, 75), (6, 77.5)))
day(1, 5, "Lower volume")
lift("Squat", [s(4, 8, pct(70), rest=180)])
lift("Deadlift", [s(2, 8, pct(70), rest=180)])
upper(1, 6, "Upper, max reps", [s(1, 1, pct(80), rest=180, amrap=True, label="Max reps")])

day(2, 1, "Lower, max reps")
lift("Squat", [s(1, (8, 10), pct(80), rest=180, label="MR10"), s(5, 3, pct(82), rest=60, label="Volume")],
     desc="MR10: as many reps as you can, stopping at 10; under 8 lowers the max 2.5%. Then add 2.5 kg for 5x3 with a minute's rest.")
variation()
upper(2, 2, "Upper", work((10, 72.5), (8, 77.5), ((6, 8), 82)))
day(2, 4, "Lower, max reps and back-offs")
lift("Squat", [s(1, (7, 10), pct(82), rest=180, label="MR10"), s(10, 3, pct(78), rest=60, label="Back-off")],
     desc="MR10 again with 2.5 kg more. Back-offs 5 kg lighter: 10 reps earn 10 sets of 3, 8-9 earn 8, 7 earn 5, fewer none and the max drops 2.5%.",
     update=custom(backoff_count))
variation()
upper(2, 5, "Upper", work((10, 72.5), (8, 77.5), ((6, 8), 82)))
upper(2, 6, "Upper, max reps", [s(1, 1, pct(78), rest=180, amrap=True, label="Max reps")])

day(3, 1, "Lower")
lift("Squat", [s(3, (4, 6), pct(87), rest=240)], desc="Linear max OT: the same heavy weight for every set, 4-6 reps.")
lift("Deadlift", [s(2, (3, 6), pct(87.5), rest=240)], desc="Linear max OT: 3-6 reps.")
upper(3, 3, "Upper", [s(3, (4, 6), pct(85), rest=240)])
day(3, 5, "Lower, one set")
lift("Squat", [s(1, (4, 6), pct(89), rest=240)])
ex("Deficit Deadlift", alts=["Stiff Leg Deadlift", "Sumo Deadlift"], sets=[s(1, 8, kg(100), rest=150)],
   desc="One set of a deadlift variation. Set your own weight by editing the program.")
upper(3, 6, "Upper", [s(3, (4, 6), pct(87), rest=240)])

day(4, 1, "Lower heavy")
lift("Squat", work((3, 88), (3, 90), (3, 92)), desc="Heavy triples climbing 2.5 kg a set.")
variation()
ex("Jump Squat", sets=[s(3, 4, bw(), rest=90)], desc="Optional explosive work, 4 reps a set: jump squats, box jumps or medicine ball throws.")
upper(4, 2, "Upper heavy", work((3, 82.5), (3, 85), (3, 90)))
day(4, 4, "Lower heavy")
lift("Squat", work((3, 92), ((1, 2), 95)), desc="Heavy triple, then 1-2 reps at 95%.")
lift("Deadlift", work((3, 92), ((1, 2), 95)), desc="Heavy triple, then 1-2 reps at 95%.")
upper(4, 5, "Upper heavy", work((3, 87.5), ((2, 4), 90), ((1, 2), 95)))

day(5, 1, "Squat max")
max_set("Squat")
lift("Deadlift", work((4, 67.5), (4, 70), (2, 72.5)), desc="Light deadlifts before Friday's heavy set.")
day(5, 3, "Bench max")
max_set("Bench Press")
rows, shoulders, pulls = ACC[5]
ex("Bent Over Row", sets=[s(1, r, kg(50), rest=90) for r in rows])
ex("Dumbbell Shoulder Press", sets=[s(1, r, kg(17.5), rest=90) for r in shoulders])
ex("Pull Up", sets=[s(1, r, bw(), rest=90) for r in pulls])
ex("Triceps Pushdown", sets=[s(3, (8, 12), kg(25), rest=60)])
ex("Bicep Curl, EZ Bar", sets=[s(3, (8, 12), kg(20), rest=60)])
day(5, 5, "Deadlift max", desc="Optional light lower-body work after the heavy set.")
max_set("Deadlift")

TEST_DESC = "Option 3 of the method: find your 1RM, from the max week 5 projected. Or skip this week and start a new cycle from that max, or repeat week 1 as a deload."
day(6, 1, "Squat test")
lift("Squat", [s(1, 1, pct(100), rest=300, label="1RM")], desc=TEST_DESC)
day(6, 3, "Bench test")
lift("Bench Press", [s(1, 1, pct(100), rest=300, label="1RM")], desc=TEST_DESC)
day(6, 5, "Deadlift test")
lift("Deadlift", [s(1, 1, pct(100), rest=300, label="1RM")], desc=TEST_DESC)
