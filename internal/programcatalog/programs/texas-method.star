# Texas Method, after Mark Rippetoe and Glenn Pendlay, in kg.
#
# Three days a week over a two-week loop in which bench and press swap places.
# Monday is volume: squat and one press for 5x5 at 90% of Friday's weight, and
# a heavy set of five deadlift. Wednesday is light: squat 2x5 at 80% of
# Monday, the other press 3x5, chin-ups and back extensions. Friday is
# intensity: one set of five for a new best in squat and the press, then
# power cleans.
#
# Squat, presses and deadlift are percentages of 1RM: Friday's 85% is about a
# 5RM, Monday's 76% is 90% of that and Wednesday's 61% is 80% of Monday.
# Making Friday's set raises the 1RM so that set gains 5 kg for squat and
# 2.5 kg for a press, and Monday and Wednesday follow. The deadlift gains 5 kg
# the same way.
program("Texas Method", weeks=2, units="kg", notes="Monday volume, Wednesday light, Friday intensity; bench and press alternate weekly")

HEAVY = 85
START = "The starting weight is a guess: edit the program to set your own."

def pr(ctx):
    if ctx.completed_reps >= ctx.reps:
        # The 1RM step that moves the heavy set by inc.
        ctx.rm1 += ctx.state.inc * 100 / HEAVY

def volume(name):
    ex(name, sets=[s(5, 5, pct(76), rest=180)], progress="none")

def light(name, n, p):
    ex(name, sets=[s(n, 5, pct(p), rest=180)], progress="none")

def heavy(name, inc):
    ex(name, sets=[s(1, 5, pct(HEAVY), rest=300)], progress=custom(pr, inc=inc))

PRESSES = {1: ("Bench Press", "Overhead Press"), 2: ("Overhead Press", "Bench Press")}

for n, (press, other) in PRESSES.items():
    day(n, 1, "Monday volume")
    volume("Squat")
    volume(press)
    heavy("Deadlift", kg(5))

    day(n, 3, "Wednesday light")
    light("Squat", 2, 61)
    light(other, 3, 68)
    ex("Chin Up", sets=[s(3, 8, bw(), rest=120)], progress=dp(kg(2.5), 8, 12))
    ex("Back Extension", sets=[s(5, 10, kg(0), rest=90)])

    day(n, 5, "Friday intensity")
    heavy("Squat", kg(5))
    heavy(press, kg(2.5))
    ex("Power Clean", desc=START, sets=[s(5, 3, kg(50), rest=180)], progress=lp(kg(2.5)))
