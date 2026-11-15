# GZCLP, after Cody Lefever's GZCL method, in kg.
#
# Four workouts (A1, B1, A2, B2) rotate over three training days a week, so
# this is a one-week floating program that loops. Each workout has a T1 main
# lift, a T2 secondary lift and a T3 accessory.
#
# T1 runs in stages: 5x3+, then 6x2+, then 10x1+ when a stage is missed, then
# a 5RM test that restarts stage 1 at 85% of the new 5RM. T2 runs 3x10, 3x8,
# 3x6, then restarts at the stage 1 weight plus a jump. T3 is 3x15+ and adds
# weight once the last set reaches 25 reps.
program("GZCLP", weeks=1, units="kg", notes="Rotate A1, B1, A2, B2 over three days a week")

T1_DESC = [
    "T1. Start at about 75% of 1RM (85% of your 5RM). The last set is as many reps as you can with good form.",
    "T1.",
    "T1. Retest: work up in sets of 5 to a hard but clean 5RM and log it on the 5RM Test set.",
]
T2_DESC = ["T2. Start at about 62% of 1RM.", "T2."]

def t1_progress(ctx):
    if ctx.description_index == 1:
        ctx.description_index = 2
    if ctx.set_variation == 4:
        ctx.description_index = 2
        ctx.set_variation = 1
        ctx.weights = ctx.completed_weights[1] * 0.85
        ctx.rm1 = ctx.completed_weights[1] / rpe_multiplier(5, 10)
    elif ctx.completed_reps >= ctx.reps:
        ctx.weights = ctx.completed_weights[ctx.number_of_sets] + ctx.state.increase
    elif ctx.set_variation == 3:
        ctx.description_index = 3
        ctx.set_variation += 1
    else:
        ctx.set_variation += 1

def t2_progress(ctx):
    if ctx.description_index == 1:
        ctx.description_index = 2
    if ctx.completed_reps >= ctx.reps:
        ctx.weights = ctx.completed_weights[ctx.number_of_sets] + ctx.state.increase
    elif ctx.set_variation == 1:
        ctx.state.stage1weight = ctx.completed_weights[ctx.number_of_sets]
        ctx.set_variation += 1
    elif ctx.set_variation == 2:
        ctx.set_variation += 1
    else:
        ctx.set_variation = 1
        ctx.weights = ctx.state.stage1weight + ctx.state.stage3increase

def t3_progress(ctx):
    if ctx.completed_reps[ctx.number_of_sets] >= 25:
        ctx.weights = ctx.completed_weights[ctx.number_of_sets] + ctx.state.increase

def t1(name, increase):
    weight = pct(75)
    ex(name, label="t1", desc=T1_DESC, variations=[
        [s(4, 3, weight), s(1, 3, weight, amrap=True)],
        [s(5, 2, weight), s(1, 2, weight, amrap=True)],
        [s(9, 1, weight), s(1, 1, weight, amrap=True)],
        [s(1, 5, weight, label="5RM Test")],
    ], progress=custom(t1_progress, increase=increase))

def t2(name, increase, stage3increase):
    weight = pct(62)
    ex(name, label="t2", desc=T2_DESC,
       variations=[[s(3, 10, weight)], [s(3, 8, weight)], [s(3, 6, weight)]],
       progress=custom(t2_progress, stage1weight=kg(0), increase=increase, stage3increase=stage3increase))

def t3(name, weight):
    ex(name, label="t3", desc="T3.", sets=[s(2, 15, weight, rest=90), s(1, 15, weight, rest=90, amrap=True)],
       progress=custom(t3_progress, increase=kg(2.5)))

day(1, 1, "A1")
t1("Squat", kg(5))
t2("Bench Press", kg(2.5), kg(5))
t3("Lat Pulldown", kg(40))

day(1, 2, "B1")
t1("Overhead Press", kg(2.5))
t2("Deadlift", kg(5), kg(7.5))
t3("Bent Over Row", kg(40))

day(1, 3, "A2")
t1("Bench Press", kg(2.5))
t2("Squat", kg(5), kg(7.5))
t3("Lat Pulldown", kg(40))

day(1, 4, "B2")
t1("Deadlift", kg(5))
t2("Overhead Press", kg(2.5), kg(5))
t3("Bent Over Row", kg(40))
