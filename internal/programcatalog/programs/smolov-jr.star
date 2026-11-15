# Smolov Jr, after Sergey Smolov, in kg.
#
# Three weeks, four days a week, one lift: 6x6 at 70%, 7x5 at 75%, 8x4 at 80%
# and 10x3 at 85% of 1RM. After the fourth day of each week every set gains
# 2.5 kg for the bench press (5 kg for the squat), so week 2 is 2.5 kg and
# week 3 is 5 kg above week 1. The program ends after week 3: rest a few days,
# then test a new max.
program("Smolov Jr", weeks=3, units="kg", loop=False, notes="One lift, four days a week for three weeks, then retest")

LIFT = "Bench Press"  # Squat is the other common choice; it adds 5 kg a week instead of 2.5
INC = {"Bench Press": kg(2.5), "Squat": kg(5)}[LIFT]
DAYS = [(6, 6, 70), (7, 5, 75), (8, 4, 80), (10, 3, 85)]

def add(ctx):
    ctx.weights += ctx.state.inc

for d, (sets, reps, p) in enumerate(DAYS):
    day(1, d + 1, "%dx%d at %d%%" % (sets, reps, p), weeks="1-3")
    ex(LIFT, sets=[s(sets, reps, pct(p), rest=180)], progress=custom(add, inc=INC) if d == 3 else "none")
