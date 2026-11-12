# 5/3/1 for Beginners, after Jim Wendler, in kg.
#
# Three full-body days a week over a three-week wave. Each day trains two
# main lifts with the 5/3/1 sets, the last one as many reps as possible, then
# 5x5 First Set Last at the first set's weight, then bodyweight assistance.
#
# Percentages are of 1RM: 58/67/76% is 65/75/85% of a 90% training max.
# After week 3 the 1RM goes up 5 kg for squat and deadlift and 2.5 kg for
# bench and press, which raises every percentage of the next wave.
program("5/3/1 for Beginners", weeks=3, units="kg", notes="Three days a week; raise the max after week 3")

WAVES = {
    1: [(5, 58), (5, 67), (5, 76)],
    2: [(3, 63), (3, 72), (3, 81)],
    3: [(5, 67), (3, 76), (1, 85)],
}
WEEK_DESC = {1: "5s week", 2: "3s week", 3: "5/3/1 week. The max goes up after this week."}

def cycle(ctx):
    # Squat and bench come twice a week; only the later day raises the max.
    if ctx.day_in_week > 1 and ctx.week == 3:
        ctx.rm1 += ctx.state.increment

def main(n, name, increment):
    wave = WAVES[n]
    sets = [s(1, reps, pct(p), rest=180, amrap=i == len(wave) - 1) for i, (reps, p) in enumerate(wave)]
    sets.append(s(5, 5, pct(wave[0][1]), rest=120, label="FSL"))
    ex(name, sets=sets, progress=custom(cycle, increment=increment))

def assistance(*moves):
    for name, reps in moves:
        ex(name, sets=[s(5, reps, bw(), rest=60)])

for n in range(1, 4):
    week(n, desc=WEEK_DESC[n])
    day(n, 1, "Squat and bench")
    main(n, "Squat", kg(5))
    main(n, "Bench Press", kg(2.5))
    assistance(("Hanging Leg Raise", 10), ("Chin Up", 10), ("Push Up", 15))

    day(n, 2, "Deadlift and press")
    main(n, "Deadlift", kg(5))
    main(n, "Overhead Press", kg(2.5))
    assistance(("Triceps Dip", 10), ("Inverted Row", 10), ("Bulgarian Split Squat", 10))

    day(n, 3, "Bench and squat")
    main(n, "Bench Press", kg(2.5))
    main(n, "Squat", kg(5))
    assistance(("Hanging Leg Raise", 10), ("Pull Up", 10), ("Push Up", 15))
