# 5/3/1 Boring But Big, after Jim Wendler, in kg.
#
# Four days a week over a four-week cycle, one main lift a day: press,
# deadlift, bench, squat. Each day works the 5/3/1 sets, the last one as many
# reps as possible, then 5x10 of the same lift at 50% of the training max,
# then one pulling or core accessory for 5x10. Week 4 is a deload.
#
# Percentages are of 1RM: 58/67/76% is 65/75/85% of a 90% training max and
# the 5x10 at 45% is 50% of it. After week 4 the 1RM goes up 5 kg for squat
# and deadlift and 2.5 kg for bench and press, which raises every percentage
# of the next cycle.
program("5/3/1 Boring But Big", weeks=4, units="kg", notes="Four days a week; raise the max after week 4")

WAVES = {
    1: [(5, 58), (5, 67), (5, 76)],
    2: [(3, 63), (3, 72), (3, 81)],
    3: [(5, 67), (3, 76), (1, 85)],
    4: [(5, 36), (5, 45), (5, 54)],
}
WEEK_DESC = {1: "5s week", 2: "3s week", 3: "5/3/1 week", 4: "Deload. The max goes up after this week."}
START = "The starting weight is a guess: edit the program to set your own."

def cycle(ctx):
    if ctx.week == 4:
        ctx.rm1 += ctx.state.increment

def main(n, name, increment):
    sets = [s(1, reps, pct(p), rest=180, amrap=n < 4 and i == 2) for i, (reps, p) in enumerate(WAVES[n])]
    ex(name, sets=sets, progress=custom(cycle, increment=increment))
    ex(name, label="BBB", sets=[s(5, 10, pct(45), rest=90)])

def accessory(name, weight, **kw):
    ex(name, sets=[s(5, 10, weight, rest=90)], **kw)

for n in range(1, 5):
    week(n, desc=WEEK_DESC[n])
    day(n, 1, "Press")
    main(n, "Overhead Press", kg(2.5))
    accessory("Chin Up", bw())

    day(n, 2, "Deadlift")
    main(n, "Deadlift", kg(5))
    accessory("Hanging Leg Raise", bw())

    day(n, 3, "Bench")
    main(n, "Bench Press", kg(2.5))
    accessory("Bent Over Row", kg(40), desc=START, progress=lp(kg(2.5)))

    day(n, 4, "Squat")
    main(n, "Squat", kg(5))
    accessory("Lat Pulldown", kg(40), desc=START, progress=lp(kg(2.5)))
