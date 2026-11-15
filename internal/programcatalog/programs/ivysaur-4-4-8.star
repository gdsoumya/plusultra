# Ivysaur 4-4-8, after Ivysaur, in kg.
#
# Two weeks, A and B, of three full-body days each, then the program loops.
# Bench and press are trained every day, squat and deadlift twice a week,
# with chin-ups or rows. Each lift rotates 4x4, 4x8 and 3x4 plus a set for
# as many reps as possible; the 8-rep weight is 90% of the 4-rep weight.
#
# Each lift jumps once a week, after its last session of the week: 7.5 kg
# on squat and deadlift, 5 kg on bench and rows, 2.5 kg on the press. An
# AMRAP set of 8 or more reps doubles that jump. The big four load from the
# 1RM, so the jump raises the max and both rep schemes move together.
program("Ivysaur 4-4-8", weeks=2, units="kg", notes="Weeks A and B alternate, three days a week")

P4 = 70   # the 4-rep sets, in percent of 1RM; the 8-rep sets are 90% of that

# The days of weeks A and B as (lift, scheme): "4" is 4x4, "8" is 4x8 and "4+" is 3x4 plus an AMRAP set.
WEEKS = {
    1: [
        [("Bench Press", "4"), ("Squat", "8"), ("Overhead Press", "8"), ("Chin Up", "8")],
        [("Bench Press", "8"), ("Deadlift", "4"), ("Overhead Press", "8"), ("Bent Over Row", "8")],
        [("Bench Press", "4+"), ("Squat", "4+"), ("Overhead Press", "8"), ("Chin Up", "8")],
    ],
    2: [
        [("Bench Press", "8"), ("Deadlift", "8"), ("Overhead Press", "4"), ("Bent Over Row", "4")],
        [("Bench Press", "4"), ("Squat", "8"), ("Overhead Press", "8"), ("Chin Up", "8")],
        [("Bench Press", "8"), ("Deadlift", "4+"), ("Overhead Press", "4+"), ("Bent Over Row", "8")],
    ],
}

# The weekly jump and the lift's last training day of weeks A and B, when the jump is applied.
JUMPS = {
    "Squat": (kg(7.5), [3, 2]),
    "Deadlift": (kg(7.5), [2, 3]),
    "Bench Press": (kg(5), [3, 3]),
    "Overhead Press": (kg(2.5), [3, 3]),
    "Bent Over Row": (kg(5), [2, 3]),
}

def jump(ctx):
    """The week's jump after the lift's last session of the week, else None."""
    if ctx.day_in_week != ctx.state.last[ctx.week - 1]:
        return None
    n = ctx.number_of_sets
    return ctx.state.rate * (2 if ctx.amrap[n] and ctx.completed_reps[n] >= 8 else 1)

def by_max(ctx):
    inc = jump(ctx)
    if inc != None:
        ctx.rm1 += inc / (P4 / 100)

def by_weight(ctx):
    inc = jump(ctx)
    if inc != None:
        # The week ends on a 4x8 day, so set 1 holds 90% of the 4-rep weight.
        ctx.weights *= 1 + inc / (ctx.weights[1] / 0.9)

def sets(scheme, w4, w8):
    if scheme == "8":
        return [s(4, 8, w8, rest=120)]
    if scheme == "4":
        return [s(4, 4, w4, rest=180)]
    return [s(3, 4, w4, rest=180), s(1, 4, w4, rest=180, amrap=True)]

def lift(name, scheme):
    if name == "Chin Up":
        ex(name, desc="Add weight on a belt once 4x8 is easy.", sets=sets(scheme, bw(), bw()))
        return
    rate, last = JUMPS[name]
    if name == "Bent Over Row":
        ex(name, desc="Set your own first-session weight by editing the program.",
           sets=sets(scheme, kg(40), kg(36)), progress=custom(by_weight, rate=rate, last=last))
    else:
        ex(name, sets=sets(scheme, pct(P4), pct(P4 * 0.9)), progress=custom(by_max, rate=rate, last=last))

for wk, days in WEEKS.items():
    letter = "AB"[wk - 1]
    week(wk, desc="Week %s" % letter)
    for n, lifts in enumerate(days, 1):
        day(wk, n, "Week %s day %d" % (letter, n))
        for name, scheme in lifts:
            lift(name, scheme)
